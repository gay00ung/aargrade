package process

import (
	"context"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// CommandContext executes native programs directly and routes Windows batch
// launchers through the platform command interpreter. Gradle Wrapper and
// distribution launchers use .bat on Windows and cannot be passed directly to
// CreateProcess.
func CommandContext(ctx context.Context, executable string, args ...string) *exec.Cmd {
	if runtime.GOOS == "windows" {
		extension := strings.ToLower(filepath.Ext(executable))
		if extension == ".bat" || extension == ".cmd" {
			return windowsBatchCommandContext(ctx, executable, args...)
		}
	}
	return exec.CommandContext(ctx, executable, args...)
}

// windowsBatchCommandLine creates the canonical cmd.exe /s /c form:
//
//	/d /s /c ""C:\path with spaces\gradlew.bat" "help""
//
// The extra outer quote pair is required because cmd.exe parses the text after
// /c as one command rather than with CommandLineToArgvW. The Windows-specific
// launcher installs this string as SysProcAttr.CmdLine.
func windowsBatchCommandLine(executable string, args ...string) string {
	parts := make([]string, 0, len(args)+1)
	parts = append(parts, quoteWindowsBatchToken(executable))
	for _, arg := range args {
		parts = append(parts, quoteWindowsBatchToken(arg))
	}
	return `/d /s /c "` + strings.Join(parts, " ") + `"`
}

func quoteWindowsBatchToken(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}
