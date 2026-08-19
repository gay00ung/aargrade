package verify

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/gay00ung/aargrade/internal/evidence"
	"github.com/gay00ung/aargrade/internal/process"
	"github.com/gay00ung/aargrade/internal/project"
)

const maxCommandOutput = 128 << 10

var variantPattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]*$`)

type buildResult struct {
	projectRoot   string
	libraryModule string
	aarPath       string
	commands      []Command
	rootDryRun    *RootDryRunComparison
	failed        bool
}

const (
	rootDryRunPass        = "pass"
	rootDryRunImproved    = "improved"
	rootDryRunPreExisting = "pre-existing-failure"
	rootDryRunRegression  = "regression"
)

func RunRootDryRun(options Options) (Command, error) {
	parentContext := options.Context
	if parentContext == nil {
		parentContext = context.Background()
	}
	discovered, err := project.Discover(options.ProjectPath)
	if err != nil {
		return Command{}, err
	}
	wrapper, err := gradleWrapper(discovered.Root)
	if err != nil {
		return Command{}, err
	}
	args := append([]string{"build", "--dry-run", "--no-daemon"}, options.GradleArgs...)
	return execute(parentContext, options.Timeout, discovered.Root, wrapper, "gradle-dry-run-before", args), nil
}

func buildCandidate(options Options) (buildResult, error) {
	parentContext := options.Context
	if parentContext == nil {
		parentContext = context.Background()
	}
	variant := options.Variant
	if variant == "" {
		variant = "release"
	}
	if !variantPattern.MatchString(variant) {
		return buildResult{}, fmt.Errorf("variant %q must match %s", variant, variantPattern)
	}
	discovered, err := project.Discover(options.ProjectPath)
	if err != nil {
		return buildResult{}, err
	}
	library, err := selectLibrary(discovered, options.LibraryPath)
	if err != nil {
		return buildResult{}, err
	}
	wrapper, err := gradleWrapper(discovered.Root)
	if err != nil {
		return buildResult{}, err
	}
	assembleTask, kmpAndroidLibrary, err := libraryAssembleTask(library, variant)
	if err != nil {
		return buildResult{}, err
	}
	result := buildResult{projectRoot: discovered.Root, libraryModule: library.GradlePath}
	run := func(name string, baseArgs ...string) Command {
		args := append([]string{}, baseArgs...)
		args = append(args, options.GradleArgs...)
		return execute(parentContext, options.Timeout, discovered.Root, wrapper, name, args)
	}

	help := run("gradle-help", "help", "--no-daemon")
	result.commands = append(result.commands, help)
	if help.Status == StatusFail {
		result.failed = true
		return result, nil
	}

	rootDryRun := run("gradle-dry-run", "build", "--dry-run", "--no-daemon")
	if options.BeforeUpgradeDryRun != nil {
		comparison := compareRootDryRuns(*options.BeforeUpgradeDryRun, rootDryRun)
		result.rootDryRun = &comparison
		if comparison.Verdict == rootDryRunPreExisting {
			rootDryRun.Status = StatusWarning
		}
	}
	result.commands = append(result.commands, rootDryRun)
	if rootDryRun.Status == StatusFail {
		result.failed = true
		return result, nil
	}

	moduleDryRun := run("aar-dry-run", assembleTask, "--dry-run", "--no-daemon")
	result.commands = append(result.commands, moduleDryRun)
	if moduleDryRun.Status == StatusFail {
		result.failed = true
		return result, nil
	}

	assemble := run("aar-assemble", assembleTask, "--no-daemon")
	result.commands = append(result.commands, assemble)
	if assemble.Status == StatusFail {
		result.failed = true
		return result, nil
	}
	aarPath, err := locateAAR(library.Directory, variant, kmpAndroidLibrary)
	if err != nil {
		return result, err
	}
	result.aarPath = aarPath
	return result, nil
}

func compareRootDryRuns(before, after Command) RootDryRunComparison {
	comparison := RootDryRunComparison{
		Verdict:      rootDryRunPass,
		Summary:      "The whole-project dry-run passed before and after migration",
		BeforeStatus: before.Status,
		AfterStatus:  after.Status,
	}
	beforeCanonical := canonicalGradleFailure(before.Output)
	afterCanonical := canonicalGradleFailure(after.Output)
	comparison.BeforeFailure = truncateFailureEvidence(beforeCanonical)
	comparison.AfterFailure = truncateFailureEvidence(afterCanonical)
	comparison.BeforeFingerprint = failureFingerprint(beforeCanonical)
	comparison.AfterFingerprint = failureFingerprint(afterCanonical)

	switch {
	case before.Status != StatusFail && after.Status == StatusFail:
		comparison.Verdict = rootDryRunRegression
		comparison.Summary = "The whole-project dry-run newly failed after migration"
	case before.Status == StatusFail && after.Status != StatusFail:
		comparison.Verdict = rootDryRunImproved
		comparison.Summary = "The pre-migration whole-project dry-run failure is resolved"
	case before.Status == StatusFail && after.Status == StatusFail:
		if comparableGradleFailure(before, after) && beforeCanonical == afterCanonical {
			comparison.Verdict = rootDryRunPreExisting
			comparison.Summary = "The whole-project dry-run has the same failure before and after migration"
		} else {
			comparison.Verdict = rootDryRunRegression
			comparison.Summary = "The post-migration whole-project dry-run failed differently from the pre-migration run"
		}
	}
	return comparison
}

func comparableGradleFailure(before, after Command) bool {
	if before.ExitCode <= 0 || before.ExitCode != after.ExitCode {
		return false
	}
	if !hasGradleFailureBlock(before.Output) || !hasGradleFailureBlock(after.Output) {
		return false
	}
	if canonicalGradleFailure(before.Output) == "" || canonicalGradleFailure(after.Output) == "" {
		return false
	}
	for _, output := range []string{before.Output, after.Output} {
		lower := strings.ToLower(output)
		if strings.Contains(lower, "command timed out") ||
			strings.Contains(lower, "command canceled") ||
			strings.Contains(lower, "[output truncated by aargrade]") {
			return false
		}
	}
	return true
}

func hasGradleFailureBlock(output string) bool {
	for _, line := range strings.Split(strings.ReplaceAll(output, "\r\n", "\n"), "\n") {
		if strings.EqualFold(strings.TrimSpace(line), "* What went wrong:") {
			return true
		}
	}
	return false
}

func canonicalGradleFailure(output string) string {
	lines := strings.Split(strings.ReplaceAll(output, "\r\n", "\n"), "\n")
	start := -1
	for index, line := range lines {
		if strings.EqualFold(strings.TrimSpace(line), "* What went wrong:") {
			start = index + 1
			break
		}
	}
	var selected []string
	if start >= 0 {
		for _, line := range lines[start:] {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "* Try:") || strings.HasPrefix(trimmed, "* Exception is:") || strings.HasPrefix(trimmed, "BUILD FAILED") {
				break
			}
			if normalized := normalizeFailureLine(trimmed); normalized != "" {
				selected = append(selected, normalized)
			}
		}
	} else {
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if isGradleBoilerplate(trimmed) {
				continue
			}
			if normalized := normalizeFailureLine(trimmed); normalized != "" {
				selected = append(selected, normalized)
			}
			if len(selected) == 8 {
				break
			}
		}
	}
	return strings.Join(selected, "\n")
}

func normalizeFailureLine(line string) string {
	line = strings.TrimSpace(line)
	for strings.HasPrefix(line, ">") {
		line = strings.TrimSpace(strings.TrimPrefix(line, ">"))
	}
	return strings.Join(strings.Fields(line), " ")
}

func isGradleBoilerplate(line string) bool {
	if line == "" {
		return true
	}
	lower := strings.ToLower(line)
	return lower == "failure: build failed with an exception." ||
		strings.HasPrefix(lower, "welcome to gradle ") ||
		strings.HasPrefix(lower, "starting a gradle daemon") ||
		strings.HasPrefix(lower, "daemon will be stopped") ||
		strings.HasPrefix(lower, "to honour the jvm settings") ||
		strings.HasPrefix(lower, "for more details on using") ||
		strings.HasPrefix(lower, "deprecated gradle features were used") ||
		strings.HasPrefix(lower, "build failed in ") ||
		strings.HasPrefix(lower, "[incubating] problems report") ||
		strings.HasPrefix(lower, "* try:") ||
		strings.HasPrefix(lower, "* get more help at")
}

func failureFingerprint(canonical string) string {
	if canonical == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(canonical))
	return fmt.Sprintf("%x", sum)
}

func truncateFailureEvidence(value string) string {
	const maximum = 512
	runes := []rune(value)
	if len(runes) <= maximum {
		return value
	}
	return string(runes[:maximum]) + "..."
}

func libraryAssembleTask(library project.Module, variant string) (string, bool, error) {
	if library.HasPlugin("com.android.kotlin.multiplatform.library") {
		if !strings.EqualFold(variant, "release") {
			return "", true, fmt.Errorf("Kotlin Multiplatform Android libraries expose one Android AAR; variant %q is not supported", variant)
		}
		return library.GradlePath + ":bundleAndroidMainAar", true, nil
	}
	taskVariant := strings.ToUpper(variant[:1]) + variant[1:]
	return library.GradlePath + ":assemble" + taskVariant, false, nil
}

func selectLibrary(discovered *project.Project, requested string) (project.Module, error) {
	var libraries []project.Module
	for _, module := range discovered.Modules {
		if module.Kind() == "library" {
			libraries = append(libraries, module)
		}
	}
	if requested != "" {
		if !strings.HasPrefix(requested, ":") {
			requested = ":" + requested
		}
		for _, library := range libraries {
			if library.GradlePath == requested {
				return library, nil
			}
		}
		return project.Module{}, fmt.Errorf("Android library module %s was not found", requested)
	}
	if len(libraries) == 0 {
		return project.Module{}, fmt.Errorf("no Android library module was resolved")
	}
	if len(libraries) > 1 {
		paths := make([]string, 0, len(libraries))
		for _, library := range libraries {
			paths = append(paths, library.GradlePath)
		}
		sort.Strings(paths)
		return project.Module{}, fmt.Errorf("multiple Android libraries found (%s); select one with --library", strings.Join(paths, ", "))
	}
	return libraries[0], nil
}

func gradleWrapper(root string) (string, error) {
	name := "gradlew"
	if runtime.GOOS == "windows" {
		name = "gradlew.bat"
	}
	path := filepath.Join(root, name)
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("Gradle Wrapper executable not found: %s", path)
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("Gradle Wrapper is not a regular file: %s", path)
	}
	return path, nil
}

func execute(parent context.Context, timeout time.Duration, directory, executable, name string, args []string) Command {
	if timeout <= 0 {
		timeout = 10 * time.Minute
	}
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()
	command := process.CommandContext(ctx, executable, args...)
	command.Dir = directory
	var output limitedBuffer
	command.Stdout = &output
	command.Stderr = &output
	started := time.Now()
	err := command.Run()
	duration := time.Since(started)
	evidence := Command{
		Name: name, Directory: directory, Executable: executable, Arguments: evidence.Arguments(args),
		ExitCode: 0, DurationMS: duration.Milliseconds(), Status: StatusPass,
		Output: sanitizeOutput(output.String(), directory),
	}
	if err != nil {
		evidence.Status = StatusFail
		evidence.ExitCode = -1
		var exitError *exec.ExitError
		if errors.As(err, &exitError) {
			evidence.ExitCode = exitError.ExitCode()
		}
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			evidence.Output = strings.TrimSpace(evidence.Output + "\ncommand timed out")
		} else if errors.Is(ctx.Err(), context.Canceled) {
			evidence.Output = strings.TrimSpace(evidence.Output + "\ncommand canceled")
		}
	}
	return evidence
}

func locateAAR(moduleDirectory, variant string, kmpAndroidLibrary bool) (string, error) {
	pattern := filepath.Join(moduleDirectory, "build", "outputs", "aar", "*.aar")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return "", err
	}
	suffix := "-" + strings.ToLower(variant) + ".aar"
	var candidates []string
	for _, match := range matches {
		if kmpAndroidLibrary || strings.HasSuffix(strings.ToLower(match), suffix) {
			candidates = append(candidates, match)
		}
	}
	sort.Strings(candidates)
	artifactKind := variant
	if kmpAndroidLibrary {
		artifactKind = "Kotlin Multiplatform Android"
	}
	if len(candidates) == 0 {
		return "", fmt.Errorf("assemble succeeded but no %s AAR was found under %s", artifactKind, filepath.Dir(pattern))
	}
	if len(candidates) > 1 {
		return "", fmt.Errorf("assemble produced multiple %s AARs; pass one explicitly with --candidate-aar: %s", artifactKind, strings.Join(candidates, ", "))
	}
	return candidates[0], nil
}

type limitedBuffer struct {
	buffer    bytes.Buffer
	truncated bool
}

func (b *limitedBuffer) Write(data []byte) (int, error) {
	originalLength := len(data)
	remaining := maxCommandOutput - b.buffer.Len()
	if remaining > 0 {
		if len(data) > remaining {
			data = data[:remaining]
			b.truncated = true
		}
		_, _ = b.buffer.Write(data)
	} else {
		b.truncated = true
	}
	return originalLength, nil
}

func (b *limitedBuffer) String() string {
	value := b.buffer.String()
	if b.truncated {
		value += "\n[output truncated by AARGrade]"
	}
	return value
}

func sanitizeOutput(value, root string) string {
	replacements := map[string]string{
		root:                   "$PROJECT",
		filepath.ToSlash(root): "$PROJECT",
	}
	if home, err := os.UserHomeDir(); err == nil {
		replacements[home] = "$HOME"
		replacements[filepath.ToSlash(home)] = "$HOME"
	}
	return evidence.Text(value, replacements)
}

var _ io.Writer = (*limitedBuffer)(nil)
