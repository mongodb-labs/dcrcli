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
)

type LogPath struct {
	DiagDirPath     string
	CurrentLogPath  string
	PreparedLogPath string
}

func (lp *LogPath) ProcessLogPath() {

	lp.PreparedLogPath = lp.CurrentLogPath
	if lp.logPathStartsWithDotSlash() && lp.DiagDirPath != "" {
		lp.PreparedLogPath = lp.logPathWithBestEstimatedParent()
	}

}

func (lp *LogPath) logPathWithBestEstimatedParent() string {

	//remove dot slash prefix from logPath
	//extract from logpath the dirname upto first slash
	//it could also not be a dir example if logPath was ./mongod.log
	logPathFirstPath := strings.Split(lp.CurrentLogPath[2:], "/")[0]

	parentPath := []string{}

	for _, ddpath := range strings.Split(lp.DiagDirPath[1:len(lp.DiagDirPath)-1], "/") {
		if ddpath == logPathFirstPath {
			break
		}
		parentPath = append(parentPath, ddpath)
	}

	return fmt.Sprintf("/%s", strings.Join(parentPath, "/")) + "/" + logPathFirstPath

}

func (lp *LogPath) logPathStartsWithDotSlash() bool {
	return strings.HasPrefix(lp.CurrentLogPath, "./")
}
