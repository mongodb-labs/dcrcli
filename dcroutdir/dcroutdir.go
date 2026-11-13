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

package dcroutdir

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// TempRoot is where remote files are staged before they are archived.
const TempRoot = "./outputs/temp"

type DCROutputDir struct {
	OutputPrefix string
	Hostname     string
	Port         string
}

func (od *DCROutputDir) CreateDCROutputDir() error {
	err := os.MkdirAll(od.Path(), 0744)
	if err != nil {
		return err
	}
	return nil
}

func (od *DCROutputDir) Path() string {
	return od.OutputPrefix + od.Hostname + "_" + od.Port
}

// ResetStaging empties a staging directory and recreates it, so an archive is
// built only from files copied in this run. Without it, files left by an
// earlier run for the same cluster and host (rsync adds, it does not delete)
// would end up in the new archive.
//
// It refuses any path outside TempRoot, so a cluster name such as "../.." can
// never make it delete something else.
func (od *DCROutputDir) ResetStaging() error {
	if err := od.checkInsideTempRoot(); err != nil {
		return err
	}
	if err := os.RemoveAll(od.Path()); err != nil {
		return fmt.Errorf("clear staging directory %s: %w", od.Path(), err)
	}
	return od.CreateDCROutputDir()
}

// RemoveStaging deletes a staging directory once its archive is written.
func (od *DCROutputDir) RemoveStaging() error {
	if err := od.checkInsideTempRoot(); err != nil {
		return err
	}
	return os.RemoveAll(od.Path())
}

func (od *DCROutputDir) checkInsideTempRoot() error {
	root, err := filepath.Abs(TempRoot)
	if err != nil {
		return err
	}
	path, err := filepath.Abs(od.Path())
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("refusing to clear %s: not inside %s", od.Path(), TempRoot)
	}
	return nil
}
