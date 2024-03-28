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

package logarchiver

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"dcrcli/archiver"
	"dcrcli/mongosh"
)

type MongoDLogarchive struct {
	Mongo              mongosh.CaptureGetMongoData
	Unixts             string
	LogPath            string
	LogArchiveFile     *os.File
	LogDir             string
	CurrentLogFileName string
	LogDestination     string
}

func (la *MongoDLogarchive) getLogPath() error {
	err := la.Mongo.RunGetMongoDLogDetails()
	if err != nil {
		return err
	}

	var systemLogOutput map[string]string
	err = json.Unmarshal(la.Mongo.Getparsedjsonoutput.Bytes(), &systemLogOutput)
	if err != nil {
		return err
	}
	if systemLogOutput["destination"] == "file" {
		la.LogDestination = "file"
		la.LogPath = trimQuote(systemLogOutput["path"])
		la.LogDir = filepath.Dir(la.LogPath)
		la.CurrentLogFileName = filepath.Base(la.LogPath)
		fmt.Println("The mongod log file path is: ", la.LogDir)
	}

	return nil
}

func (la *MongoDLogarchive) createMongodTarArchiveFile() error {
	var err error
	la.LogArchiveFile, err = os.Create(la.Unixts + "/logarchive.tar.gz")
	fmt.Println("Estimating log path will then archive to:", la.LogArchiveFile.Name())
	if err != nil {
		fmt.Println("Error: error creating archive file in outputs folder", err)
	}
	return err
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
		fmt.Println(err)
	}
	return err
}

func (la *MongoDLogarchive) Start() error {
	var err error

	err = la.getLogPath()
	if err != nil {
		return err
	}
	// return early if the mongod log destination is not file
	if la.LogDestination != "file" {
		fmt.Println("WARNING: MongoDLogArchive only works for systemLog:file")
		return nil
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
