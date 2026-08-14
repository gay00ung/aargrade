//go:build !windows

package process

import (
	"context"
	"os/exec"
)

// This implementation only keeps the common source type-checkable. The
// runtime guard in CommandContext means it is never selected off Windows.
func windowsBatchCommandContext(ctx context.Context, executable string, args ...string) *exec.Cmd {
	return exec.CommandContext(ctx, executable, args...)
}
