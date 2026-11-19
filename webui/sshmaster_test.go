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

package webui

import (
	"errors"
	"strings"
	"testing"
)

func TestClassifiesSSHHostKeyMismatch(t *testing.T) {
	if !isHostKeyMismatch("@@@@@@@@@ WARNING: REMOTE HOST IDENTIFICATION HAS CHANGED! @@@@@@@@@") {
		t.Fatal("expected changed host identification to be classified")
	}
	if !isHostKeyMismatch("Offending ED25519 key in /home/user/.ssh/known_hosts:4") {
		t.Fatal("expected offending known_hosts key to be classified")
	}
	if isHostKeyMismatch("Permission denied (publickey)") {
		t.Fatal("ordinary authentication failure was misclassified")
	}

	err := sshMasterError("diag", "db1", "REMOTE HOST IDENTIFICATION HAS CHANGED!", errors.New("exit status 255"))
	for _, want := range []string{"verify the server fingerprint", "update the stale known_hosts entry"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("guidance %q missing %q", err, want)
		}
	}
	for _, unsafe := range []string{"StrictHostKeyChecking=no", "disable"} {
		if strings.Contains(err.Error(), unsafe) {
			t.Fatalf("guidance contains unsafe advice %q: %v", unsafe, err)
		}
	}
}
