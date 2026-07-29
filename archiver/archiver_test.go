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

package archiver

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// Test TarWithPatternMatch - for handling non-existent directory source
func TestTarWithPatternMatchWithNonExistentSource(t *testing.T) {
	got := "/dummy/src/folder"

	err := TarWithPatternMatch(got, "metrics.*")
	if err == nil {
		t.Fatalf("Should error out on non-existent source folder")
	}
}

// Test TarWithPatternMatch - for handling a file that grows during archiving
func TestTarWithPatternMatchGrowingFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "archiver-growing-file")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	logPath := filepath.Join(tmpDir, "mongod.log")
	initial := make([]byte, 1024*1024)
	if err := os.WriteFile(logPath, initial, 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	var buf bytes.Buffer
	done := make(chan error, 1)
	go func() {
		done <- TarWithPatternMatch(tmpDir, `^mongod\.log.*`, &buf)
	}()

	for i := 0; i < 1000; i++ {
		f, err := os.OpenFile(logPath, os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			continue
		}
		f.Write([]byte("more log data during archive\n"))
		f.Close()
	}

	if err := <-done; err != nil {
		t.Fatalf("TarWithPatternMatch: %v", err)
	}
}
