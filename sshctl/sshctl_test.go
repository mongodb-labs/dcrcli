// Copyright 2023 MongoDB Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// See the License for the specific language governing permissions and
// limitations under the License.

package sshctl

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMuxArgsEmpty(t *testing.T) {
	if MuxArgs("") != nil {
		t.Fatal("expected nil args when ControlPath is empty")
	}
}

func TestMuxArgsAndRsyncSSH(t *testing.T) {
	path := "/tmp/dcrssh-test/c"
	args := MuxArgs(path)
	if len(args) != 6 || args[1] != "ControlMaster=auto" || args[3] != "ControlPath="+path || args[5] != "ControlPersist=yes" {
		t.Fatalf("MuxArgs: %#v", args)
	}
	rsh := RsyncSSH(path)
	if !strings.HasPrefix(rsh, "ssh ") || !strings.Contains(rsh, "ControlPath="+path) {
		t.Fatalf("RsyncSSH: %q", rsh)
	}
}

func TestPreparePathIsShortAndPrivate(t *testing.T) {
	path, cleanup, err := PreparePath()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cleanup)
	if !safeControlPath(path) {
		t.Fatalf("unsafe path: %q", path)
	}
	if len(path) > maxControlPathLen {
		t.Fatalf("path too long for unix socket: %q (%d)", path, len(path))
	}
	dir := filepath.Dir(path)
	st, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !st.IsDir() {
		t.Fatalf("expected dir: %q", dir)
	}
}

func TestRsyncSSHRejectsUnsafePath(t *testing.T) {
	if got := RsyncSSH("/tmp/foo bar/c"); got != "" {
		t.Fatalf("expected empty RSYNC_RSH, got %q", got)
	}
}
