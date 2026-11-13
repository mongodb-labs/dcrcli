// Package subprocess starts cancellable commands whose child processes share
// their lifecycle. This prevents helpers such as ssh, started by rsync, from
// surviving after the owning command is canceled.
package subprocess

import (
	"context"
	"os/exec"
)

// CommandContext is exec.CommandContext with platform-specific child cleanup.
func CommandContext(ctx context.Context, name string, arg ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, name, arg...)
	configureCancellation(cmd)
	return cmd
}
