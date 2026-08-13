package matrix

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gay00ung/aargrade/internal/artifact"
	"github.com/gay00ung/aargrade/internal/evidence"
)

const maxMatrixOutput = 256 << 10

var javaVersionPattern = regexp.MustCompile(`(?m)version "(?:1\.)?(\d+)`)

func Run(options Options) (Report, error) {
	parentContext := options.Context
	if parentContext == nil {
		parentContext = context.Background()
	}
	config, configPath, err := loadConfig(options)
	if err != nil {
		return Report{}, err
	}
	candidate, err := artifact.Inspect(config.CandidateAAR)
	if err != nil {
		return Report{}, fmt.Errorf("inspect candidate AAR: %w", err)
	}
	var baseline artifact.Snapshot
	if config.BaselineAAR != "" {
		baseline, err = artifact.Inspect(config.BaselineAAR)
		if err != nil {
			return Report{}, fmt.Errorf("inspect baseline AAR: %w", err)
		}
	}
	workRoot := options.WorkDirectory
	if workRoot == "" {
		workRoot = filepath.Join(filepath.Dir(configPath), ".aargrade", "matrix")
	}
	workRoot, err = filepath.Abs(workRoot)
	if err != nil {
		return Report{}, err
	}
	if err := os.MkdirAll(workRoot, 0o755); err != nil {
		return Report{}, err
	}
	runDirectory, err := os.MkdirTemp(workRoot, "run-")
	if err != nil {
		return Report{}, err
	}
	report := Report{
		SchemaVersion: SchemaVersion, ToolVersion: options.ToolVersion,
		Verdict: "pass", ConfigPath: configPath, WorkDirectory: runDirectory,
		CandidateAAR: candidate.Path, CandidateSHA256: candidate.SHA256,
	}
	if config.BaselineAAR != "" {
		report.BaselineAAR = baseline.Path
		report.BaselineSHA256 = baseline.SHA256
	}
	selected := stringSet(options.SelectedCells)
	probe := chooseProbe(baseline, candidate)
	for _, cell := range config.Cells {
		if len(selected) > 0 && !selected[cell.Name] {
			continue
		}
		report.SelectedCellCount++
		result := runCell(parentContext, runDirectory, cell, config, baseline, candidate, probe, options)
		report.Cells = append(report.Cells, result)
		if (result.Verdict == "regression" || result.Verdict == "fail") && !options.KeepGoing {
			break
		}
	}
	sort.Slice(report.Cells, func(i, j int) bool { return report.Cells[i].Name < report.Cells[j].Name })
	report.Verdict = overallVerdict(report.Cells)
	return report, nil
}

func runCell(ctx context.Context, runDirectory string, cell CellConfig, config Config, baseline, candidate artifact.Snapshot, probe string, options Options) CellResult {
	result := CellResult{Name: cell.Name, AGP: cell.AGP, Gradle: cell.Gradle, JDK: cell.JDK, CompileSDK: cell.CompileSDK, Language: strings.ToLower(cell.Language)}
	javaHome, err := resolveJavaHome(ctx, cell, options.JavaHomes)
	if err != nil {
		result.Verdict = "incomplete"
		result.Reason = err.Error()
		return result
	}
	gradle, err := resolveGradle(ctx, cell, options)
	if err != nil {
		result.Verdict = "incomplete"
		result.Reason = err.Error()
		return result
	}
	cellDirectory := filepath.Join(runDirectory, cell.Name)
	if config.BaselineAAR != "" {
		execution := runArtifact(ctx, cellDirectory, "baseline", cell, baseline, probe, javaHome, gradle, options.Timeout)
		result.Baseline = &execution
	}
	execution := runArtifact(ctx, cellDirectory, "candidate", cell, candidate, probe, javaHome, gradle, options.Timeout)
	result.Candidate = &execution
	result.Verdict, result.Reason = classifyCell(result.Baseline, result.Candidate)
	return result
}

func runArtifact(parent context.Context, cellDirectory, label string, cell CellConfig, snapshot artifact.Snapshot, probe, javaHome, gradle string, timeout time.Duration) Execution {
	projectDirectory := filepath.Join(cellDirectory, label)
	execution := Execution{Artifact: snapshot.Path, SHA256: snapshot.SHA256, ProjectDir: projectDirectory, ProbeClass: probe, Status: "incomplete", ExitCode: -1}
	if err := generateConsumer(projectDirectory, cell, snapshot.Path, probe); err != nil {
		execution.Output = err.Error()
		return execution
	}
	args := []string{"-p", projectDirectory, ":app:assembleRelease", "--no-daemon", "--stacktrace"}
	args = append(args, cell.GradleArgs...)
	execution.Command = append([]string{gradle}, evidence.Arguments(args)...)
	if timeout <= 0 {
		timeout = 15 * time.Minute
	}
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()
	command := exec.CommandContext(ctx, gradle, args...)
	command.Dir = projectDirectory
	command.Env = environmentWithJavaHome(javaHome)
	var output cappedBuffer
	command.Stdout = &output
	command.Stderr = &output
	started := time.Now()
	err := command.Run()
	execution.DurationMS = time.Since(started).Milliseconds()
	execution.Output = sanitizeMatrixOutput(output.String(), projectDirectory, snapshot.Path)
	if err == nil {
		execution.Status = "pass"
		execution.ExitCode = 0
		return execution
	}
	execution.Status = "fail"
	var exitError *exec.ExitError
	if errors.As(err, &exitError) {
		execution.ExitCode = exitError.ExitCode()
	}
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		execution.Output = strings.TrimSpace(execution.Output + "\ncommand timed out")
	} else if errors.Is(ctx.Err(), context.Canceled) {
		execution.Output = strings.TrimSpace(execution.Output + "\ncommand canceled")
	}
	return execution
}

func resolveJavaHome(parent context.Context, cell CellConfig, overrides map[int]string) (string, error) {
	value := overrides[cell.JDK]
	if value == "" && cell.JavaHome != "" {
		value = cell.JavaHome
	}
	if value == "" && cell.JavaHomeEnv != "" {
		value = os.Getenv(cell.JavaHomeEnv)
	}
	if value == "" {
		value = os.Getenv("AARGRADE_JAVA_HOME_" + strconv.Itoa(cell.JDK))
	}
	if value == "" {
		value = os.Getenv("JAVA_HOME")
	}
	if value == "" {
		return "", fmt.Errorf("JDK %d is not configured; set AARGRADE_JAVA_HOME_%d, JAVA_HOME, javaHomeEnv, or --java-home", cell.JDK, cell.JDK)
	}
	absolute, err := filepath.Abs(value)
	if err != nil {
		return "", err
	}
	javaName := "java"
	if runtime.GOOS == "windows" {
		javaName = "java.exe"
	}
	java := filepath.Join(absolute, "bin", javaName)
	ctx, cancel := context.WithTimeout(parent, 30*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, java, "-version").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("run JDK %d from %s: %w", cell.JDK, absolute, err)
	}
	match := javaVersionPattern.FindStringSubmatch(string(output))
	if len(match) == 0 || match[1] != strconv.Itoa(cell.JDK) {
		return "", fmt.Errorf("Java home %s does not report JDK %d", absolute, cell.JDK)
	}
	return absolute, nil
}

func environmentWithJavaHome(javaHome string) []string {
	pathValue := filepath.Join(javaHome, "bin") + string(os.PathListSeparator) + os.Getenv("PATH")
	var result []string
	for _, variable := range os.Environ() {
		name, _, _ := strings.Cut(variable, "=")
		if strings.EqualFold(name, "JAVA_HOME") || strings.EqualFold(name, "PATH") {
			continue
		}
		result = append(result, variable)
	}
	return append(result, "JAVA_HOME="+javaHome, "PATH="+pathValue)
}

func classifyCell(baseline, candidate *Execution) (string, string) {
	if candidate == nil || candidate.Status == "incomplete" || (baseline != nil && baseline.Status == "incomplete") {
		return "incomplete", "a consumer build could not be executed"
	}
	if baseline == nil {
		if candidate.Status == "pass" {
			return "pass", "candidate AAR built successfully"
		}
		return "fail", "candidate AAR failed in the consumer"
	}
	switch {
	case baseline.Status == "pass" && candidate.Status == "pass":
		return "pass", "baseline and candidate both build"
	case baseline.Status == "pass" && candidate.Status == "fail":
		return "regression", "baseline builds but candidate fails"
	case baseline.Status == "fail" && candidate.Status == "pass":
		return "improved", "candidate builds in a cell where baseline fails"
	default:
		return "unsupported", "baseline and candidate both fail in this cell"
	}
}

func overallVerdict(cells []CellResult) string {
	verdict := "pass"
	for _, cell := range cells {
		switch cell.Verdict {
		case "regression", "fail":
			return "fail"
		case "incomplete", "unsupported":
			verdict = "incomplete"
		}
	}
	return verdict
}

func sanitizeMatrixOutput(value, projectDirectory, artifactPath string) string {
	replacements := map[string]string{
		projectDirectory:                   "$CONSUMER",
		filepath.ToSlash(projectDirectory): "$CONSUMER",
		artifactPath:                       "$AAR",
		filepath.ToSlash(artifactPath):     "$AAR",
	}
	if home, err := os.UserHomeDir(); err == nil {
		replacements[home] = "$HOME"
		replacements[filepath.ToSlash(home)] = "$HOME"
	}
	return evidence.Text(value, replacements)
}

type cappedBuffer struct {
	buffer    bytes.Buffer
	truncated bool
}

func (buffer *cappedBuffer) Write(data []byte) (int, error) {
	original := len(data)
	remaining := maxMatrixOutput - buffer.buffer.Len()
	if remaining > 0 {
		if len(data) > remaining {
			data = data[:remaining]
			buffer.truncated = true
		}
		_, _ = buffer.buffer.Write(data)
	} else {
		buffer.truncated = true
	}
	return original, nil
}

func (buffer *cappedBuffer) String() string {
	value := buffer.buffer.String()
	if buffer.truncated {
		value += "\n[output truncated by AARGrade]"
	}
	return value
}

func stringSet(values []string) map[string]bool {
	result := map[string]bool{}
	for _, value := range values {
		result[value] = true
	}
	return result
}
