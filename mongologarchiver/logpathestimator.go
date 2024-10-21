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
	"fmt"
	"strings"

	"dcrcli/dcrlogger"
)

type LogPathEstimator struct {
	DiagDirPath     string
	CurrentLogPath  string
	PreparedLogPath string
	Dcrlog          *dcrlogger.DCRLogger
}

func (lp *LogPathEstimator) ProcessLogPath() {
	lp.Dcrlog.Debug(fmt.Sprintf("currentlogpath: %s", lp.CurrentLogPath))
	lp.Dcrlog.Debug(fmt.Sprintf("diagdirpath: %s", lp.DiagDirPath))

	lp.PreparedLogPath = lp.CurrentLogPath
	if lp.logPathStartsWithDotSlash() && lp.DiagDirPath != "" {
		lp.logPathWithBestEstimatedParent()
	}
}

func (lp *LogPathEstimator) logPathWithBestEstimatedParent() {
	lp.Dcrlog.Debug("will attempt to estimate mongod log path Begin")

	// remove dot slash prefix from logPath
	// extract from logpath the dirname upto first slash
	// it could also not be a dir example if logPath was ./mongod.log
	logPathFirstPath := strings.Split(lp.CurrentLogPath[2:], "/")[0]
	lp.Dcrlog.Debug(fmt.Sprintf("logPathFirstPath: %s", logPathFirstPath))

	parentPath := []string{}

	for _, ddpath := range strings.Split(lp.DiagDirPath[1:len(lp.DiagDirPath)-1], "/") {

		if ddpath == logPathFirstPath {
			break
		}

		// diagnostic data path can be like /foo/bar/./data/mongo
		if ddpath != "." {
			parentPath = append(parentPath, ddpath)
		}

	}
	lp.Dcrlog.Debug(fmt.Sprintf("parentpath array: %s", parentPath))

	currentMongodLogFilePathWithoutDotSlash := lp.CurrentLogPath[2:]

	lp.PreparedLogPath = fmt.Sprintf(
		"/%s",
		strings.Join(parentPath, "/"),
	) + "/" + currentMongodLogFilePathWithoutDotSlash
	lp.Dcrlog.Debug(
		fmt.Sprintf("Estimated full file path to latest mongod log file: %s", lp.PreparedLogPath),
	)
}

func (lp *LogPathEstimator) logPathStartsWithDotSlash() bool {
	return strings.HasPrefix(lp.CurrentLogPath, "./")
}
