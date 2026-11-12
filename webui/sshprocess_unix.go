//go:build !windows

package webui

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
)

func configureAskpassProcess(cmd *exec.Cmd) {
	// With no controlling terminal, OpenSSH uses SSH_ASKPASS and forwards its
	// prompt to the browser.
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
