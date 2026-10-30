// Copyright 2023 MongoDB Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package webui

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// openSSHMaster opens the shared ControlMaster connection before rsync and the
// host commands run, so any password or host-key prompt happens once, here.
// The process gets its own session (no controlling terminal), which makes every
// OpenSSH version use SSH_ASKPASS and therefore the browser prompt.
func openSSHMaster(user, host string, muxArgs []string) error {
	if user == "" || host == "" || strings.HasPrefix(user, "-") || strings.HasPrefix(host, "-") {
		return fmt.Errorf("unsafe SSH username or hostname")
	}
	errFile, err := os.CreateTemp("", "dcrssh-err-*")
	if err != nil {
		return err
	}
	defer os.Remove(errFile.Name())
	defer errFile.Close()

	args := append([]string{}, muxArgs...)
	args = append(args,
		"-o", "ConnectTimeout=15",
		"-o", "NumberOfPasswordPrompts=3",
		user+"@"+host,
		"true",
	)
	cmd := exec.Command("ssh", args...)
	cmd.Stdin = nil
	cmd.Stdout = nil
	// A pipe would keep Wait blocked on the backgrounded master; a file does not.
	cmd.Stderr = errFile
	configureAskpassProcess(cmd)
	runErr := cmd.Run()
	if runErr == nil {
		return nil
	}
	msg, _ := os.ReadFile(errFile.Name())
	detail := strings.TrimSpace(string(msg))
	if detail == "" {
		return fmt.Errorf("ssh %s@%s: %w", user, host, runErr)
	}
	return fmt.Errorf("ssh %s@%s: %s", user, host, detail)
}
