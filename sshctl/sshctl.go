// Copyright 2023 MongoDB Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package sshctl shares one OpenSSH connection per node (ControlMaster) so
// password SSH prompts once for FTDC, logs, and host commands.
package sshctl

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"unicode"
)

const maxControlPathLen = 90

// PreparePath creates a private directory and a ControlPath socket name.
// The caller must run the returned cleanup (after Exit).
func PreparePath() (controlPath string, cleanup func(), err error) {
	base := os.TempDir()
	if len(base) > 40 {
		if _, stErr := os.Stat("/tmp"); stErr == nil {
			base = "/tmp"
		}
	}
	dir, err := os.MkdirTemp(base, "dcrssh-*")
	if err != nil {
		return "", nil, err
	}
	cleanup = func() { _ = os.RemoveAll(dir) }
	controlPath = filepath.Join(dir, "c")
	if len(controlPath) > maxControlPathLen || !safeControlPath(controlPath) {
		cleanup()
		return "", nil, fmt.Errorf("SSH ControlPath %q is too long or unsafe for connection sharing", controlPath)
	}
	return controlPath, cleanup, nil
}

// MuxArgs are OpenSSH client options that make the first connection the
// master and later ssh/rsync calls reuse it. Empty path yields nil.
func MuxArgs(controlPath string) []string {
	if controlPath == "" {
		return nil
	}
	return []string{
		"-o", "ControlMaster=auto",
		"-o", "ControlPath=" + controlPath,
		"-o", "ControlPersist=yes",
	}
}

// RsyncSSH builds the rsync -e / RSYNC_RSH value. Empty if mux is off or if
// any token would be unsafe inside that string.
func RsyncSSH(controlPath string) string {
	args := MuxArgs(controlPath)
	if len(args) == 0 {
		return ""
	}
	parts := append([]string{"ssh"}, args...)
	for _, p := range parts {
		if !safeRsyncSSHToken(p) {
			return ""
		}
	}
	return strings.Join(parts, " ")
}

// Exit closes the multiplexed master for user@host. Best-effort.
func Exit(user, host, controlPath string) {
	if controlPath == "" || user == "" || host == "" {
		return
	}
	if !safeControlPath(controlPath) {
		return
	}
	cmd := exec.Command(
		"ssh",
		"-o", "ControlPath="+controlPath,
		"-O", "exit",
		user+"@"+host,
	)
	_ = cmd.Run()
}

func safeControlPath(p string) bool {
	if p == "" || strings.Contains(p, "..") {
		return false
	}
	for _, r := range p {
		if r > unicode.MaxASCII || r < 32 {
			return false
		}
		if !(unicode.IsLetter(r) || unicode.IsDigit(r) || r == '/' || r == '.' || r == '_' || r == '-') {
			return false
		}
	}
	return true
}

func safeRsyncSSHToken(s string) bool {
	if s == "" {
		return false
	}
	return !strings.ContainsAny(s, " \t\n\r\"'`$&|;<>(){}[]*?!~\\")
}
