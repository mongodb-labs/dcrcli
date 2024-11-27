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

package mongosh

import (
	"bytes"
	"dcrcli/dcroutdir"
	"dcrcli/mongocredentials"
	"fmt"
	"os"
	"os/exec"
)

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

func printErrorIfNotNil(err error, msg string) error {
	if err != nil {
		return fmt.Errorf("Failed %s : %s\n", msg, err)
	}
	return nil
}

type CaptureGetMongoData struct {
	S                   *mongocredentials.Mongocredentials
	Getparsedjsonoutput *bytes.Buffer
	CurrentBin          string
	ScriptPath          string
	FilePathOnDisk      string
	CurrentCommand      *string
	Outputdir           *dcroutdir.DCROutputDir
}

func (cgm *CaptureGetMongoData) setOutputDirPath() {
	cgm.FilePathOnDisk = cgm.Outputdir.Path() + "/getMongoData.json"
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

	// fmt.Println("Running the cmdDotRun")
	return printErrorIfNotNil(
		cmd.Run(),
		"in execGetMongoDataWithEval() data collection script execution",
	)
}

func (cgm *CaptureGetMongoData) execMongoWellnessCheckerWithEval() error {
	var cmd *exec.Cmd
	// fmt.Println(cgm.S.Mongouri)
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

	return printErrorIfNotNil(
		cmd.Run(),
		"in execMongoWellnessCheckerWithEval() data collection script execution",
	)
}

func (cgm *CaptureGetMongoData) RunMongoShellWithEval() error {
	cgm.setOutputDirPath()

	cgm.Getparsedjsonoutput = &bytes.Buffer{}

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

	return printErrorIfNotNil(
		cmd.Run(),
		*cgm.CurrentCommand,
	)
}

func (cgm *CaptureGetMongoData) RunGetMongoDLogDetails() error {
	cgm.Getparsedjsonoutput = &bytes.Buffer{}
	cgm.CurrentCommand = &GetSystemLogDBCommand

	err := cgm.RunCurrentDBCommand()
	if err != nil {
		return err
	}

	return nil
}

func (cgm *CaptureGetMongoData) RunGetCommandDiagnosticDataCollectionDirectoryPath() error {
	cgm.Getparsedjsonoutput = &bytes.Buffer{}
	cgm.Getparsedjsonoutput.Reset()
	cgm.CurrentCommand = &GetCommandDiagnosticDataCollectionDirectoryPath

	err := cgm.RunCurrentDBCommand()
	if err != nil {
		return err
	}

	return nil
}
