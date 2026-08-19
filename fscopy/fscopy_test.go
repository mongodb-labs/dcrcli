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

package fscopy

import (
	"bytes"
	"os/exec"
	"testing"
)

// Testing for
// - local copy job
// - remote copy job with Default sync port
// - remote copy job with non-Default sync port

// - local copy job
func TestStartCopyLocal(t *testing.T) {
	fcj := FSCopyJob{
		Src: SourceDir{
			IsLocal:  true,
			Path:     []byte(`/Users/nishant/myprojects/testclusters/standalone/data/db/diagnostic.data/`),
			Hostname: []byte(``),
			Username: []byte(`ubuntu`),
		},
		Dst: DestDir{
			Path: []byte(`/Users/nishant/myprojects/dcrcliProject/branches/remotecopier/dcrcli/outputs`),
		},
		State:  "N",
		Output: &bytes.Buffer{},
	}
	err := fcj.StartCopy()
	if err != nil {
		t.Error(err.Error())
	}
}

// - remote copy job with Default sync port
/**
func TestStartCopyRemote(t *testing.T) {
	fcj := FSCopyJob{
		SourceDir{
			false,
			[]byte(`/var/lib/mongodb/diagnostic.data/`),
			[]byte(`ec2-13-234-136-113.ap-south-1.compute.amazonaws.com`),
			0,
			[]byte(`ubuntu`),
		},
		DestDir{
			[]byte(`/Users/nishant/myprojects/dcrcliProject/branches/remotecopier/dcrcli/outputs/`),
		},
		"N",
		&bytes.Buffer{},
	}
	err := fcj.StartCopy()
	if err != nil {
		t.Error(err.Error())
	}
}
*/

func TestTolerateRsyncVanishedSources(t *testing.T) {
	if err := tolerateRsyncError(nil, nil); err != nil {
		t.Fatalf("nil error should pass through, got %v", err)
	}

	err24 := exec.Command("sh", "-c", "exit 24").Run()
	if err24 == nil {
		t.Fatal("expected sh to exit 24")
	}
	if err := tolerateRsyncError(err24, nil); err != nil {
		t.Fatalf("rsync exit 24 should be treated as success, got %v", err)
	}

	err23 := exec.Command("sh", "-c", "exit 23").Run()
	if err23 == nil {
		t.Fatal("expected sh to exit 23")
	}
	if err := tolerateRsyncError(err23, nil); err == nil {
		t.Fatal("rsync exit 23 should remain a failure")
	}
}
