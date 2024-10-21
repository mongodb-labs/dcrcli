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

package mongologarchiver

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"dcrcli/archiver"
	"dcrcli/dcrlogger"
	"dcrcli/dcroutdir"
	"dcrcli/mongosh"
)

type MongoDLogarchive struct {
	Mongo              mongosh.CaptureGetMongoData
	LogPath            string //full path to latest mongod log file
	LogArchiveFile     *os.File
	LogDir             string //derived base dir of latest mongod log file
	CurrentLogFileName string //name of latest mongod log file
	LogDestination     string
	Outputdir          *dcroutdir.DCROutputDir
	Dcrlog             *dcrlogger.DCRLogger
}

func (la *MongoDLogarchive) getDiagnosticDataDirPath() string {
	err := la.Mongo.RunGetCommandDiagnosticDataCollectionDirectoryPath()
	if err != nil {
		fmt.Printf("Error in getDiagnosticDataDirPath: %v", err)
		return ""
	}

	ddpath := trimQuote(la.Mongo.Getparsedjsonoutput.String())
	la.Dcrlog.Debug(fmt.Sprintf("diagnostic dir path: %s", ddpath))
	return ddpath
}

func (la *MongoDLogarchive) getLogPath() error {
	err := la.Mongo.RunGetMongoDLogDetails()
	if err != nil {
		return err
	}

	var systemLogOutput map[string]interface{}
	err = json.Unmarshal(la.Mongo.Getparsedjsonoutput.Bytes(), &systemLogOutput)
	if err != nil {
		return err
	}

	la.Dcrlog.Debug(fmt.Sprintf("mongod log destination: %s", systemLogOutput["destination"]))

	if systemLogOutput["destination"] == "file" {
		la.LogDestination = "file"

		lp := LogPathEstimator{}
		lp.Dcrlog = la.Dcrlog

		lp.CurrentLogPath = trimQuote(systemLogOutput["path"].(string))
		lp.DiagDirPath = la.getDiagnosticDataDirPath()

		la.Dcrlog.Debug("processing mongod log path")
		lp.ProcessLogPath()
		la.LogPath = lp.PreparedLogPath

		la.LogDir = filepath.Dir(la.LogPath)
		la.Dcrlog.Debug(fmt.Sprintf("derived base dir of latest mongod log file: %s", la.LogDir))

		la.CurrentLogFileName = filepath.Base(la.LogPath)
		la.Dcrlog.Debug(fmt.Sprintf("name of latest mongod log file: %s", la.CurrentLogFileName))
	}

	return nil
}

func (la *MongoDLogarchive) createMongodTarArchiveFile() error {
	var err error
	la.LogArchiveFile, err = os.Create(la.Outputdir.Path() + "/logarchive.tar.gz")
	// fmt.Println("Estimating log path will then archive to:", la.LogArchiveFile.Name())
	if err != nil {
		return fmt.Errorf("error: error creating archive file in outputs folder %w", err)
	}
	return nil
}

func (la *MongoDLogarchive) archiveLogFiles() error {
	var err error
	// Define search pattern based on latest mongod log file name
	fileSearchPatterString := `^` + la.CurrentLogFileName + `.*`

	// Debug messages
	// fmt.Println("la.Logdir", la.LogDir)
	// fmt.Println("fileSearchPatterString", fileSearchPatterString)

	err = archiver.TarWithPatternMatch(
		la.LogDir,
		fileSearchPatterString,
		la.LogArchiveFile,
	)
	if err != nil {
		la.Dcrlog.Debug(fmt.Sprintf("error in archiveLogFiles: %s", err))
		return fmt.Errorf("error in archiveLogFiles: %w", err)
	}
	return nil
}

func (la *MongoDLogarchive) Start() error {
	var err error

	err = la.getLogPath()
	if err != nil {
		return err
	}
	// return early if the mongod log destination is not file
	if la.LogDestination != "file" {
		return fmt.Errorf("error: MongoDLogArchive only works for systemLog:file")
	}

	err = la.createMongodTarArchiveFile()
	if err != nil {
		return err
	}

	err = la.archiveLogFiles()
	if err != nil {
		return err
	}
	return err
}

func trimQuote(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 0 && s[0] == '"' {
		s = s[1:]
	}
	if len(s) > 0 && s[len(s)-1] == '"' {
		s = s[:len(s)-1]
	}
	return s
}
