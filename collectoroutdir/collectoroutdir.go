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

package collectoroutdir

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// TempRoot is where remote files are staged before they are archived.
const TempRoot = "./outputs/temp"

// NewStagingPrefix returns an OutputPrefix for staging remote files that is
// unique to one run. Two collections started from the same working directory
// for the same cluster and host therefore never share (or clean up) each
// other's staging files.
func NewStagingPrefix(clusterName string) string {
	var b [6]byte
	if _, err := rand.Read(b[:]); err != nil {
		// Extremely unlikely; the time and pid still keep runs apart.
		return fmt.Sprintf("%s/%s/run-%d-%d/", TempRoot, clusterName, time.Now().UnixNano(), os.Getpid())
	}
	return fmt.Sprintf("%s/%s/run-%s/", TempRoot, clusterName, hex.EncodeToString(b[:]))
}

type CollectorOutputDir struct {
	OutputPrefix string
	Hostname     string
	Port         string
}

func (od *CollectorOutputDir) CreateCollectorOutputDir() error {
	err := os.MkdirAll(od.Path(), 0744)
	if err != nil {
		return err
	}
	return nil
}

func (od *CollectorOutputDir) Path() string {
	return od.OutputPrefix + od.Hostname + "_" + od.Port
}

// ResetStaging empties a staging directory and recreates it, so an archive is
// built only from files copied in this run. Without it, files left by an
// earlier run for the same cluster and host (rsync adds, it does not delete)
// would end up in the new archive.
//
// It refuses any path outside TempRoot, so a cluster name such as "../.." can
// never make it delete something else.
func (od *CollectorOutputDir) ResetStaging() error {
	if err := od.checkInsideTempRoot(); err != nil {
		return err
	}
	if err := os.RemoveAll(od.Path()); err != nil {
		return fmt.Errorf("clear staging directory %s: %w", od.Path(), err)
	}
	return od.CreateCollectorOutputDir()
}

// RemoveStaging deletes a staging directory once its archive is written.
func (od *CollectorOutputDir) RemoveStaging() error {
	if err := od.checkInsideTempRoot(); err != nil {
		return err
	}
	if err := os.RemoveAll(od.Path()); err != nil {
		return err
	}
	// Drop the per-run folder too, but only if it is now empty (os.Remove
	// refuses a non-empty directory) and is not the staging root itself.
	if parent := filepath.Dir(od.Path()); parent != filepath.Clean(TempRoot) {
		_ = os.Remove(parent)
	}
	return nil
}

func (od *CollectorOutputDir) checkInsideTempRoot() error {
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
