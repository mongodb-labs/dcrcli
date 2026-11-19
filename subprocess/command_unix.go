//go:build !windows

package subprocess

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
)

// configureCancellation starts the command in its own session. That gives it
// its own process group, so cancellation can kill helpers it started (for
// example ssh under rsync). It also leaves the command without a controlling
// terminal: OpenSSH then asks SSH_ASKPASS for passwords and host-key answers
// instead of reading /dev/tty, where a background process group would be
// stopped with SIGTTIN. Interactive commands that need the terminal, such as
// the web -via tunnel, must not use this package.
func configureCancellation(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return os.ErrProcessDone
		}
		err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		if errors.Is(err, syscall.ESRCH) {
			return os.ErrProcessDone
		}
		return err
	}
}
