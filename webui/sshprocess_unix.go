//go:build !windows

package webui

import (
	"os/exec"
	"syscall"
)

func configureAskpassProcess(cmd *exec.Cmd) {
	// With no controlling terminal, OpenSSH uses SSH_ASKPASS and forwards its
	// prompt to the browser.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}
