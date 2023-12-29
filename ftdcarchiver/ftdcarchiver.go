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

package ftdcarchiver

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"dcrcli/archiver"
	"dcrcli/mongosh"
)

func extractPathFromGetMongodataOutput() (string, bool) {
	pat := regexp.MustCompile(`diagnosticDataCollectionDirectoryPath.*`)

	// extract the path from string separated by colon :
	parameterName, pathtoDiagnosticDatawithComma, ok := strings.Cut(
		pat.FindString(mongosh.Getparsedjsonoutput.String()),
		":",
	)
	if !ok {
		fmt.Println("some issue parsing diagnostic path from getmongodata output", parameterName)
		return "", false
	}

	// remove the xtra comma in the end
	pathtoDiagnosticData, ok := strings.CutSuffix(pathtoDiagnosticDatawithComma, ",")
	if !ok {
		fmt.Println("some issue parsing diagnostic path from getmongodata output")
		return "", false
	}

	// trim the double quotes
	return trimQuote(strings.TrimSpace(pathtoDiagnosticData)), true
}

func Run() error {
	ftdcarchiveFile, err := os.Create("./outputs/ftdcarchive.tar.gz")
	if err != nil {
		fmt.Println("Error: error creating archive file in outputs folder", err)
		return err
	}

	diagnosticDirPath, ok := extractPathFromGetMongodataOutput()
	if !ok {
		fmt.Println("Error: unable to parse diagnostic data dir path from getMongoData output")
		return nil
	}
	err = archiver.Tar(diagnosticDirPath, ftdcarchiveFile)
	if err != nil {
		fmt.Println(err)
		return nil
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
