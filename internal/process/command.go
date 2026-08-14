package process

import (
	"context"
	"os"
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
			interpreter := os.Getenv("ComSpec")
			if interpreter == "" {
				interpreter = "cmd.exe"
			}
			commandArgs := []string{"/d", "/s", "/c", executable}
			commandArgs = append(commandArgs, args...)
			return exec.CommandContext(ctx, interpreter, commandArgs...)
		}
	}
	return exec.CommandContext(ctx, executable, args...)
}
