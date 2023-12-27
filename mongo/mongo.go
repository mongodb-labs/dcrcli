// Copyright 2020 MongoDB Inc
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

package mongo

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"dcrcli/mongocredetials"
)

func Detect() bool {
	return binPath() != ""
}

func binPath() string {
	if p, err := exec.LookPath(mongoBin); err == nil {
		return p
	}

	return ""
}

func execCommand(args ...string) error {
	a := append([]string{mongoBin}, args...)
	env := os.Environ()
	return syscall.Exec(
		binPath(),
		a,
		env,
	) //nolint:gosec // false positive, this path won't be tampered
}

func SetTelemetry(enable bool) error {
	cmd := "disableTelemetry()"
	if enable {
		cmd = "enableTelemetry()"
	}
	return execCommand("--nodb", "--eval", cmd)
}

func Run() error {
	var s mongocredentials.Mongocredentials
	err := mongocredentials.Get(&s)
	if err != nil {
		fmt.Println("Error:", err)
	}

	// let the mongo shell ask password from the operator
	return execCommand(
		"--nodb --quiet",
		"-u",
		s.Username,
		s.Mongouri,
		"./assets/getMongoData/getMongoData.js",
	)
}