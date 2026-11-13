//go:build windows

package webui

import "os/exec"

func configureAskpassProcess(cmd *exec.Cmd) {
	// A web-server child has no interactive console input because Stdin is nil.
	// Windows OpenSSH can therefore honor SSH_ASKPASS_REQUIRE=force.
}
