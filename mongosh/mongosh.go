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

package mongosh

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"dcrcli/mongocredentials"
)

var Getparsedjsonoutput bytes.Buffer

func detect(currentBin *string, scriptPath *string) error {
	if binPath() != "" {
		*currentBin = mongoshBin
		*scriptPath = "./assets/mongoWellnessChecker/mongoWellnessChecker.js"
	} else if legacybinPath() != "" {
		*currentBin = mongoBin
		*scriptPath = "./assets/getMongoData/getMongoData.js"
	} else {
		return fmt.Errorf("O Oh: Could not find the mongosh or legacy mongo shell. Install that first.")
	}
	return nil
}

func binPath() string {
	if p, err := exec.LookPath(mongoshBin); err == nil {
		return p
	}

	return ""
}

func legacybinPath() string {
	if p, err := exec.LookPath(mongoBin); err == nil {
		return p
	}

	return ""
}

func execCommand(args ...string) error {
	a := append([]string{mongoshBin}, args...)
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

func printErrorIfNotNil(err error, msg string) error {
	if err != nil {
		fmt.Printf("Failed %s : %s\n", msg, err)
		return err
	}
	return nil
}

func runCommandAndCaptureOutputInVariable(
	currentBin *string,
	scriptPath *string,
	s *mongocredentials.Mongocredentials,
	out *bytes.Buffer,
) error {
	var cmd *exec.Cmd
	if s.Username == "" {
		cmd = exec.Command(
			*currentBin,
			"--quiet",
			"--norc",
			s.Mongouri,
			*scriptPath,
		)
	} else {
		cmd = exec.Command(
			*currentBin,
			"--quiet",
			"--norc",
			"-u",
			s.Username,
			"-p",
			s.Password,
			s.Mongouri,
			*scriptPath,
		)
	}

	cmd.Stdout = out
	cmd.Stderr = out

	return printErrorIfNotNil(cmd.Run(), "data collection script execution")
}

func removeStaleOutputFiles() error {
	return printErrorIfNotNil(
		os.Remove("./outputs/getMongoData.out"),
		"unable to remove stale data from outputs directory.",
	)
}

func getMongoConnectionStringWithCredentials(s *mongocredentials.Mongocredentials) error {
	return printErrorIfNotNil(mongocredentials.Get(s), "getting credentials:")
}

func writeOutputFromVariableToFile(out *bytes.Buffer, outpath string) error {
	output := out.String()
	return printErrorIfNotNil(
		os.WriteFile(outpath, []byte(output), 0666),
		"writing collection script output",
	)
}

func Runshell() error {
	var s mongocredentials.Mongocredentials
	var out *bytes.Buffer
	out = &Getparsedjsonoutput
	var err error
	var currentBin string
	var scriptPath string
	outputPath := "./outputs/getMongoData.out"

	err = getMongoConnectionStringWithCredentials(&s)
	if err != nil {
		return err
	}

	err = detect(&currentBin, &scriptPath)
	if err != nil {
		return err
	}
	fmt.Println("currentBin:", currentBin, "scriptPath:", scriptPath, "mongouri", s.Mongouri)

	err = runCommandAndCaptureOutputInVariable(&currentBin, &scriptPath, &s, out)
	if err != nil {
		return err
	}

	err = writeOutputFromVariableToFile(out, outputPath)
	if err != nil {
		return err
	}

	fmt.Println("Data collection output written to outputs directory")
	return nil
}
