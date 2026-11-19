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

package mongologarchiver

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"mongo-diagnostic-collector/archiver"
	"mongo-diagnostic-collector/collectorlogger"
	"mongo-diagnostic-collector/collectoroutdir"
	"mongo-diagnostic-collector/fscopy"
	"mongo-diagnostic-collector/mongosh"
)

type RemoteMongoDLogarchive struct {
	Mongo              mongosh.CaptureGetMongoData
	LogPath            string // full path to latest mongod log file
	LogArchiveFile     *os.File
	LogDir             string // derived base dir of latest mongod log file
	CurrentLogFileName string // name of latest mongod log file
	LogDestination     string
	Outputdir          *collectoroutdir.CollectorOutputDir
	TempOutputdir      *collectoroutdir.CollectorOutputDir
	RemoteCopyJob      *fscopy.FSCopyJobWithPattern
	CollectorLog       *collectorlogger.CollectorLogger
}

func (rla *RemoteMongoDLogarchive) getDiagnosticDataDirPath() string {
	err := rla.Mongo.RunGetCommandDiagnosticDataCollectionDirectoryPath()
	if err != nil {
		fmt.Printf("Error in getDiagnosticDataDirPath: %v", err)
		return ""
	}

	ddpath := trimQuote(rla.Mongo.Getparsedjsonoutput.String())
	rla.CollectorLog.Debug(fmt.Sprintf("diagnostic dir path: %s", ddpath))
	return ddpath
}

func (rla *RemoteMongoDLogarchive) getLogPathAndSetCurrentLogFileName() error {
	err := rla.Mongo.RunGetMongoDLogDetails()
	if err != nil {
		return err
	}

	// need to handle case where logAppend: boolean is there
	var systemLogOutput map[string]interface{}
	err = json.Unmarshal(rla.Mongo.Getparsedjsonoutput.Bytes(), &systemLogOutput)
	if err != nil {
		return err
	}

	rla.CollectorLog.Debug(fmt.Sprintf("mongod log destination: %s", systemLogOutput["destination"]))

	if systemLogOutput["destination"].(string) == "file" {
		rla.LogDestination = "file"

		lp := LogPathEstimator{}
		lp.CollectorLog = rla.CollectorLog

		lp.CurrentLogPath = trimQuote(systemLogOutput["path"].(string))
		lp.DiagDirPath = rla.getDiagnosticDataDirPath()

		rla.CollectorLog.Debug("processing mongod log path")
		lp.ProcessLogPath()
		rla.LogPath = lp.PreparedLogPath

		rla.LogDir = filepath.Dir(rla.LogPath)
		rla.CollectorLog.Debug(fmt.Sprintf("derived base dir of latest mongod log file: %s", rla.LogDir))

		rla.CurrentLogFileName = filepath.Base(rla.LogPath)
		rla.CollectorLog.Debug(fmt.Sprintf("name of latest mongod log file: %s", rla.CurrentLogFileName))

	}

	return nil
}

func (rla *RemoteMongoDLogarchive) createMongodTarArchiveFile() error {
	var err error
	rla.LogArchiveFile, err = os.Create(rla.Outputdir.Path() + "/logarchive.tar.gz")
	// fmt.Println("Estimating log path will then archive to:", rla.LogArchiveFile.Name())
	if err != nil {
		return fmt.Errorf("error creating archive file in outputs folder %s", err)
	}
	return nil
}

func (rla *RemoteMongoDLogarchive) archiveLogFiles() error {
	var err error
	// Define search pattern based on latest mongod log file name
	fileSearchPatterString := `^` + rla.CurrentLogFileName + `.*`

	// Debug messages
	// fmt.Println("la.Logdir", la.LogDir)
	// fmt.Println("fileSearchPatterString", fileSearchPatterString)

	err = archiver.TarWithPatternMatch(
		rla.TempOutputdir.Path(),
		fileSearchPatterString,
		rla.LogArchiveFile,
	)
	if err != nil {
		rla.CollectorLog.Debug(fmt.Sprintf("error in archiveRemoteLogFiles: %s", err))
		return fmt.Errorf("error in archiveRemoteLogFiles: %w", err)
	}
	return nil
}

func (rla *RemoteMongoDLogarchive) remoteCopyLogFilesToTemp() error {
	// Start from an empty staging directory so the archive holds only the files
	// copied now, not leftovers from an earlier run for the same host.
	if err := rla.TempOutputdir.ResetStaging(); err != nil {
		return err
	}
	rla.RemoteCopyJob.CopyJobDetails.Src.Path = []byte(rla.LogDir)
	rla.RemoteCopyJob.CurrentFileName = rla.CurrentLogFileName

	err := rla.RemoteCopyJob.StartCopyWithPattern()
	if err != nil {
		return err
	}
	return nil
}

func (rla *RemoteMongoDLogarchive) Start() error {
	var err error

	// The staged copy is only needed to build the archive.
	defer func() { _ = rla.TempOutputdir.RemoveStaging() }()

	err = rla.getLogPathAndSetCurrentLogFileName()
	if err != nil {
		return err
	}
	// return early if the mongod log destination is not file
	if rla.LogDestination != "file" {
		return fmt.Errorf("error: remote MongoDLogArchive only works for systemLog:file")
	}

	err = rla.createMongodTarArchiveFile()
	if err != nil {
		return err
	}

	err = rla.remoteCopyLogFilesToTemp()
	if err != nil {
		return err
	}

	err = rla.archiveLogFiles()
	if err != nil {
		return err
	}
	return err
}
