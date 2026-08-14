package verify

import (
	"bytes"
	"context"
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
	failed        bool
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
	taskVariant := strings.ToUpper(variant[:1]) + variant[1:]
	commands := []struct {
		name string
		args []string
	}{
		{name: "gradle-help", args: []string{"help", "--no-daemon"}},
		{name: "gradle-dry-run", args: []string{"build", "--dry-run", "--no-daemon"}},
		{name: "aar-assemble", args: []string{library.GradlePath + ":assemble" + taskVariant, "--no-daemon"}},
	}
	result := buildResult{projectRoot: discovered.Root, libraryModule: library.GradlePath}
	for _, command := range commands {
		args := append([]string{}, command.args...)
		args = append(args, options.GradleArgs...)
		evidence := execute(parentContext, options.Timeout, discovered.Root, wrapper, command.name, args)
		result.commands = append(result.commands, evidence)
		if evidence.Status == StatusFail {
			result.failed = true
			return result, nil
		}
	}
	aarPath, err := locateAAR(library.Directory, variant)
	if err != nil {
		return result, err
	}
	result.aarPath = aarPath
	return result, nil
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

func locateAAR(moduleDirectory, variant string) (string, error) {
	pattern := filepath.Join(moduleDirectory, "build", "outputs", "aar", "*.aar")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return "", err
	}
	suffix := "-" + strings.ToLower(variant) + ".aar"
	var candidates []string
	for _, match := range matches {
		if strings.HasSuffix(strings.ToLower(match), suffix) {
			candidates = append(candidates, match)
		}
	}
	sort.Strings(candidates)
	if len(candidates) == 0 {
		return "", fmt.Errorf("assemble succeeded but no %s AAR was found under %s", variant, filepath.Dir(pattern))
	}
	if len(candidates) > 1 {
		return "", fmt.Errorf("assemble produced multiple %s AARs; pass one explicitly with --candidate-aar: %s", variant, strings.Join(candidates, ", "))
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
