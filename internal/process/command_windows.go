//go:build windows

package process

import (
	"context"
	"os"
	"os/exec"
	"syscall"
)

func windowsBatchCommandContext(ctx context.Context, executable string, args ...string) *exec.Cmd {
	interpreter := os.Getenv("ComSpec")
	if interpreter == "" {
		interpreter = "cmd.exe"
	}

	cmd := exec.CommandContext(ctx, interpreter)
	// os/exec's normal Windows quoting follows CommandLineToArgvW, but cmd.exe
	// uses different rules. Supplying the complete command line prevents paths
	// containing spaces from being split before the batch file starts.
	cmd.Args = nil
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CmdLine: windowsBatchCommandLine(executable, args...),
	}
	return cmd
}
