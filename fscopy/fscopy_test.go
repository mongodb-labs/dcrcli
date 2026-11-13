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
	"context"
	"io"
	"os"
	"os/exec"
	"reflect"
	"runtime"
	"testing"
	"time"
)

func TestContextCommandCancelsAndWaits(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test requires a POSIX shell and inherited file descriptors")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cmd := command(ctx, "sh", "-c", "sleep 30 & echo started >&3; wait")
	startedR, startedW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer startedR.Close()
	// The marker is written only after the grandchild exists, so cancellation
	// never races its fork and the test checks the whole group is killed.
	cmd.ExtraFiles = []*os.File{startedW}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	// The child has its own copy now; drop ours so EOF means the child is gone.
	_ = startedW.Close()
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	var marker [8]byte
	if _, err := io.ReadFull(startedR, marker[:]); err != nil {
		t.Fatalf("subprocess did not start: %v", err)
	}
	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("canceled subprocess returned nil")
		}
	case <-time.After(time.Second):
		t.Fatal("canceled subprocess was not waited for")
	}
	if err := startedR.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := startedR.Read(marker[:]); err != io.EOF {
		t.Fatalf("child process still holds inherited descriptor: %v", err)
	}
}

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

func TestRsyncSSHFromOptions(t *testing.T) {
	if got := rsyncSSHFromOptions(nil); got != "" {
		t.Fatalf("empty options: %q", got)
	}
	got := rsyncSSHFromOptions([]string{"-o", "ControlMaster=auto", "-o", "ControlPath=/tmp/c"})
	if got != "ssh -o ControlMaster=auto -o ControlPath=/tmp/c" {
		t.Fatalf("got %q", got)
	}
	if got := rsyncSSHFromOptions([]string{"-o", "ControlPath=/tmp/foo bar"}); got != "" {
		t.Fatalf("unsafe token should be rejected: %q", got)
	}
}

func TestRsyncPatternArgsDoNotRequireLocalShell(t *testing.T) {
	src := SourceDir{
		Username: []byte("diagnostic"),
		Hostname: []byte("mongo-0.example.net"),
		Path:     []byte("/var/log/mongodb"),
	}
	dst := DestDir{Path: []byte(`C:\dcr\output`)}
	got := rsyncPatternArgs(src, dst, "mongod.log")
	want := []string{
		"-az",
		"--include=mongod.log*",
		"--exclude=*",
		"diagnostic@mongo-0.example.net:/var/log/mongodb/",
		`C:\dcr\output`,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("rsync argv:\n got %#v\nwant %#v", got, want)
	}
}
