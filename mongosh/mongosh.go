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

type CaptureGetMongoData struct {
	S                   *mongocredentials.Mongocredentials
	Getparsedjsonoutput *bytes.Buffer
	CurrentBin          string
	ScriptPath          string
	Unixts              string
	FilePathOnDisk      string
}

func (cgm *CaptureGetMongoData) RunMongoShell() error {
	cgm.setOutputDirPath()

	cgm.S = &mongocredentials.Mongocredentials{}
	cgm.Getparsedjsonoutput = &bytes.Buffer{}

	fmt.Println("running getMongoCreds")
	err := cgm.getMongoCreds()
	if err != nil {
		return err
	}

	fmt.Println("running detectMongoShellType")
	err = cgm.detectMongoShellType()
	if err != nil {
		return err
	}

	fmt.Println("running execGetMongoData")
	err = cgm.execGetMongoData()
	if err != nil {
		return err
	}

	fmt.Println("running writeToFile")
	err = cgm.writeToFile()
	if err != nil {
		return err
	}

	return nil
}

func (cgm *CaptureGetMongoData) setOutputDirPath() {
	cgm.FilePathOnDisk = "./outputs/" + cgm.Unixts + "/getMongoData.out"
}

func (cgm *CaptureGetMongoData) getMongoCreds() error {
	return printErrorIfNotNil(mongocredentials.Get(cgm.S), "getting credentials")
}

func (cgm *CaptureGetMongoData) detectMongoShellType() error {
	if binPath() != "" {
		cgm.CurrentBin = mongoshBin
		cgm.ScriptPath = "./assets/mongoWellnessChecker/mongoWellnessChecker.js"
	} else if legacybinPath() != "" {
		cgm.CurrentBin = mongoBin
		cgm.ScriptPath = "./assets/getMongoData/getMongoData.js"
	} else {
		return fmt.Errorf("O Oh: Could not find the mongosh or legacy mongo shell. Install that first.")
	}
	return nil
}

func (cgm *CaptureGetMongoData) execGetMongoData() error {
	var cmd *exec.Cmd
	if cgm.S.Username == "" {
		cmd = exec.Command(
			cgm.CurrentBin,
			"--quiet",
			"--norc",
			cgm.S.Mongouri,
			cgm.ScriptPath,
		)
	} else {
		cmd = exec.Command(
			cgm.CurrentBin,
			"--quiet",
			"--norc",
			"-u",
			cgm.S.Username,
			"-p",
			cgm.S.Password,
			cgm.S.Mongouri,
			cgm.ScriptPath,
		)
	}

	cmd.Stdout = cgm.Getparsedjsonoutput
	cmd.Stderr = cgm.Getparsedjsonoutput

	fmt.Println("Running the cmdDotRun")
	return printErrorIfNotNil(cmd.Run(), "data collection script execution")
}

func (cgm *CaptureGetMongoData) writeToFile() error {
	output := cgm.Getparsedjsonoutput.String()
	return printErrorIfNotNil(
		os.WriteFile(cgm.FilePathOnDisk, []byte(output), 0666),
		"writing collection script output",
	)
}

func Runshell(unixts string) error {
	var s mongocredentials.Mongocredentials
	var out *bytes.Buffer
	out = &Getparsedjsonoutput
	var err error
	var currentBin string
	var scriptPath string

	outputPath := "./outputs/" + unixts + "/getMongoData.out"

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
