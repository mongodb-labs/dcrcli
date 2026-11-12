//go:build windows

package subprocess

import (
	"os"
	"os/exec"
	"strconv"
)

func configureCancellation(cmd *exec.Cmd) {
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return os.ErrProcessDone
		}
		// Windows does not provide Unix process groups. taskkill /T terminates
		// the command and descendants such as ssh started by rsync.
		err := exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(cmd.Process.Pid)).Run()
		if err == nil {
			return nil
		}
		return cmd.Process.Kill()
	}
}
