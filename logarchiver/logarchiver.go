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
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"dcrcli/archiver"
	"dcrcli/mongosh"
)

// get the index numbers of right and left curl braces for fileds with JSON objects example systemLogPath
// Does not support nested JSON objects
func estimateJsonIndexBoundsForFieldWithJSONValue(
	reader *bytes.Reader,
	fieldname string,
) (int, int) {
	pat := regexp.MustCompile(fieldname)

	// extract the path from string separated by colon :
	systemLogPatternIndex := pat.FindIndex(mongosh.Getparsedjsonoutput.Bytes())

	// check if systemLogPatternIndex is properly filed with 2 values else pattern not found return -1, -1
	if len(systemLogPatternIndex) == 2 {
		reader.Seek(int64(systemLogPatternIndex[1]), 0)
		buf := make([]byte, 1)
		var rightCurlyIndex, leftCurlyIndex int
		offset := 0
		for {

			numberOfBytesRead, err := reader.Read(buf)
			if err != nil {
				fmt.Println("Error while reading", err)
			}
			// fmt.Println(string(buf[:numberOfBytesRead]))
			offset++

			if string(buf[:numberOfBytesRead]) == "{" {
				rightCurlyIndex = systemLogPatternIndex[1] + offset
			}
			if string(buf[:numberOfBytesRead]) == "}" {
				leftCurlyIndex = systemLogPatternIndex[1] + offset
				break
			}
		}
		return rightCurlyIndex, leftCurlyIndex
	} else {
		return -1, -1
	}
}

func extractJSONfromBufferIntoMap(
	leftCurlyIndex int,
	rightCurlyIndex int,
	reader *bytes.Reader,
	objmap *map[string]interface{},
) error {
	// this buffer holds the JSON doc
	jsonbuf := make([]byte, leftCurlyIndex-rightCurlyIndex+1)
	// seek back to start of the right curly brace
	reader.Seek(int64(rightCurlyIndex-1), 0)
	numjsonbytes, err := reader.Read(jsonbuf)
	if err != nil {
		fmt.Println("Error while reading", err, "Num of bytes read:", numjsonbytes)
		return err
	}

	if err := json.Unmarshal(jsonbuf, objmap); err != nil {
		fmt.Println(err)
		return err
	}

	return nil
}

func estimateLogPath() (string, string, bool) {
	reader := bytes.NewReader(mongosh.Getparsedjsonoutput.Bytes())

	fieldName := `systemLog`
	rightCurlyIndex, leftCurlyIndex := estimateJsonIndexBoundsForFieldWithJSONValue(
		reader,
		fieldName,
	)

	var objmap map[string]interface{}
	extractJSONfromBufferIntoMap(leftCurlyIndex, rightCurlyIndex, reader, &objmap)

	if objmap["destination"] == "file" {
		logFilePath := objmap["path"].(string)
		return filepath.Dir(logFilePath), filepath.Base(logFilePath), true
	}
	return "", "", false
}

func Run(unixts string) error {
	logarchiveFile, err := os.Create("./outputs/" + unixts + "/logarchive.tar.gz")
	if err != nil {
		fmt.Println("Error: error creating log archive file in outputs folder", err)
		return err
	}
	fmt.Println("Estimating log path will then archive to:", logarchiveFile.Name())

	logDirPath, currentLogFileName, ok := estimateLogPath()
	if !ok {
		fmt.Println("The log destination is other than regular file.")
	} else {
		fmt.Println("The mongod log file path is: ", logDirPath)
	}

	fileSearchPatterString := `^` + currentLogFileName + `.*`
	err = archiver.TarWithPatternMatch(logDirPath, fileSearchPatterString, logarchiveFile)
	if err != nil {
		fmt.Println(err)
		return err
	}

	return nil
}

func trimQuote(s string) string {
	if len(s) > 0 && s[0] == '"' {
		s = s[1:]
	}
	if len(s) > 0 && s[len(s)-1] == '"' {
		s = s[:len(s)-1]
	}
	return s
}
