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

	"dcrcli/mongocredentials"
)

var Getparsedjsonoutput bytes.Buffer

/**func detect(currentBin *string, scriptPath *string) error {
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
**/

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

/**func execCommand(args ...string) error {
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
}*/

func printErrorIfNotNil(err error, msg string) error {
	if err != nil {
		fmt.Printf("Failed %s : %s\n", msg, err)
		return err
	}
	return nil
}

/**func runCommandAndCaptureOutputInVariable(
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
}*/

type CaptureGetMongoData struct {
	S                   *mongocredentials.Mongocredentials
	Getparsedjsonoutput *bytes.Buffer
	CurrentBin          string
	ScriptPath          string
	Unixts              string
	FilePathOnDisk      string
	CurrentCommand      *string
}

func (cgm *CaptureGetMongoData) setOutputDirPath() {
	cgm.FilePathOnDisk = "./outputs/" + cgm.Unixts + "/getMongoData.out"
}

// OBSOLETE: We capture it outside mongosh package now
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

func (cgm *CaptureGetMongoData) writeToFile() error {
	output := cgm.Getparsedjsonoutput.String()
	return printErrorIfNotNil(
		os.WriteFile(cgm.FilePathOnDisk, []byte(output), 0666),
		"writing collection script output",
	)
}

func (cgm *CaptureGetMongoData) execGetMongoDataWithEval() error {
	var cmd *exec.Cmd
	if cgm.S.Username == "" {
		cmd = exec.Command(
			"mongo",
			"--quiet",
			"--norc",
			cgm.S.Mongouri,
			"--eval",
			GetMongDataScriptCode,
		)
	} else {
		cmd = exec.Command(
			"mongo",
			"--quiet",
			"--norc",
			"-u",
			cgm.S.Username,
			"-p",
			cgm.S.Password,
			cgm.S.Mongouri,
			"--eval",
			GetMongDataScriptCode,
		)
	}

	cmd.Stdout = cgm.Getparsedjsonoutput
	cmd.Stderr = cgm.Getparsedjsonoutput

	fmt.Println("Running the cmdDotRun")
	return printErrorIfNotNil(
		cmd.Run(),
		"in execGetMongoDataWithEval() data collection script execution",
	)
}

func (cgm *CaptureGetMongoData) execMongoWellnessCheckerWithEval() error {
	var cmd *exec.Cmd
	if cgm.S.Username == "" {
		cmd = exec.Command(
			"mongosh",
			"--quiet",
			"--norc",
			cgm.S.Mongouri,
			"--eval",
			MongoWellnessCheckerScriptCode,
		)
	} else {
		cmd = exec.Command(
			"mongosh",
			"--quiet",
			"--norc",
			"-u",
			cgm.S.Username,
			"-p",
			cgm.S.Password,
			cgm.S.Mongouri,
			"--eval",
			MongoWellnessCheckerScriptCode,
		)
	}

	cmd.Stdout = cgm.Getparsedjsonoutput
	cmd.Stderr = cgm.Getparsedjsonoutput

	fmt.Println("Running the cmdDotRun")
	return printErrorIfNotNil(
		cmd.Run(),
		"in execMongoWellnessCheckerWithEval() data collection script execution",
	)
}

func (cgm *CaptureGetMongoData) RunMongoShellWithEval() error {
	cgm.setOutputDirPath()

	cgm.Getparsedjsonoutput = &bytes.Buffer{}

	fmt.Println("running detectMongoShellType")
	err := cgm.detectMongoShellType()
	if err != nil {
		return err
	}

	if cgm.CurrentBin == "mongo" {
		err := cgm.execGetMongoDataWithEval()
		if err != nil {
			return err
		}
	}

	if cgm.CurrentBin == "mongosh" {
		err := cgm.execMongoWellnessCheckerWithEval()
		if err != nil {
			return err
		}
	}

	fmt.Println("running writeToFile")
	err = cgm.writeToFile()
	if err != nil {
		return err
	}

	return nil
}

func (cgm *CaptureGetMongoData) RunHelloDBCommandWithEval() error {
	cgm.Getparsedjsonoutput = &bytes.Buffer{}
	cgm.CurrentCommand = &HelloDBCommand

	err := cgm.RunCurrentDBCommand()
	if err != nil {
		return err
	}

	return nil
}

func (cgm *CaptureGetMongoData) RunGetShardMapWithEval() error {
	cgm.Getparsedjsonoutput = &bytes.Buffer{}
	cgm.CurrentCommand = &GetShardMapScriptCode

	err := cgm.RunCurrentDBCommand()
	if err != nil {
		return err
	}
	return nil
}

// This method facilitates running mongosh --json=canonical so output is proper JSON
// But this is not used for mongoWellnessChecker output as that output format is not desired due to legacy reasons
func (cgm *CaptureGetMongoData) RunCurrentDBCommand() error {
	cgm.Getparsedjsonoutput = &bytes.Buffer{}
	cgm.Getparsedjsonoutput.Reset()

	err := cgm.detectMongoShellType()
	if err != nil {
		return err
	}
	if cgm.CurrentBin == "mongo" {
		err := cgm.execLegacyMongoShell()
		if err != nil {
			return err
		}
	}

	if cgm.CurrentBin == "mongosh" {
		err := cgm.execMongoSHShell()
		if err != nil {
			return err
		}
	}

	return nil
}

func (cgm *CaptureGetMongoData) execLegacyMongoShell() error {
	var cmd *exec.Cmd
	if cgm.S.Username == "" {
		cmd = exec.Command(
			cgm.CurrentBin,
			"--quiet",
			"--norc",
			cgm.S.Mongouri,
			"--eval",
			*cgm.CurrentCommand,
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
			"--eval",
			*cgm.CurrentCommand,
		)
	}

	cmd.Stdout = cgm.Getparsedjsonoutput
	cmd.Stderr = cgm.Getparsedjsonoutput

	fmt.Println("Running the cmdDotRun")
	return printErrorIfNotNil(
		cmd.Run(),
		*cgm.CurrentCommand,
	)
}

func (cgm *CaptureGetMongoData) execMongoSHShell() error {
	var cmd *exec.Cmd
	if cgm.S.Username == "" {
		cmd = exec.Command(
			cgm.CurrentBin,
			"--quiet",
			"--norc",
			cgm.S.Mongouri,
			"--eval",
			*cgm.CurrentCommand,
			"--json=canonical",
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
			"--eval",
			*cgm.CurrentCommand,
			"--json=canonical",
		)
	}

	cmd.Stdout = cgm.Getparsedjsonoutput
	cmd.Stderr = cgm.Getparsedjsonoutput

	fmt.Println("Running the cmdDotRun")
	return printErrorIfNotNil(
		cmd.Run(),
		*cgm.CurrentCommand,
	)
}

// Get the systemLog variable from getCmdLineOpts output
func (cgm *CaptureGetMongoData) RunGetMongoDLogDetails() error {
	cgm.Getparsedjsonoutput = &bytes.Buffer{}
	cgm.CurrentCommand = &GetSystemLogDBCommand

	err := cgm.RunCurrentDBCommand()
	if err != nil {
		return err
	}

	return nil
}

// Get the diagnostic parameter from getParameter command
func (cgm *CaptureGetMongoData) RunGetCommandDiagnosticDataCollectionDirectoryPath() error {
	cgm.Getparsedjsonoutput = &bytes.Buffer{}
	cgm.CurrentCommand = &GetCommandDiagnosticDataCollectionDirectoryPath

	err := cgm.RunCurrentDBCommand()
	if err != nil {
		return err
	}

	return nil
}
