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
	"path/filepath"
	"strings"
	"testing"

	"dcrcli/fscopy"
	"dcrcli/topologyfinder"
)

func TestRunProgressDerivesPartialAndFailed(t *testing.T) {
	tests := []struct {
		name   string
		states []string
		want   string
	}{
		{"mixed failure and success", []string{"failed", "ok", "omitted", "omitted"}, "partial"},
		{"all failures", []string{"failed", "failed", "omitted", "omitted"}, "failed"},
		{"skipped without success", []string{"skipped", "omitted", "omitted", "omitted"}, "failed"},
		{"skipped with success", []string{"skipped", "ok", "omitted", "omitted"}, "partial"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := newRunProgress([]topologyfinder.ClusterNode{{Hostname: "node", Port: 27017}}, "")
			for i, state := range tc.states {
				p.nodes[0].Tasks[i].State = state
			}
			p.derive(0)
			if got := p.nodes[0].State; got != tc.want {
				t.Fatalf("state = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestCommandTaskErrorPreservesBootstrapAndCommandErrors(t *testing.T) {
	err := commandTaskError(errors.New("bootstrap"), errors.New("mongodb check"))
	for _, want := range []string{"bootstrap", "mongodb check"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error %q missing %q", err, want)
		}
	}
}

func TestPrepareSSHSkipsBootstrapForEmptyControlPath(t *testing.T) {
	old := prepareSSHControlPath
	prepareSSHControlPath = func() (string, func(), error) {
		return "", func() {}, nil
	}
	t.Cleanup(func() { prepareSSHControlPath = old })

	app := testApp(t)
	opts, cleanup, err := app.prepareSSH(
		topologyfinder.ClusterNode{Hostname: "not-resolvable.invalid", Port: 27017},
		fscopy.RemoteCred{Username: "diag", Available: true},
	)
	if err != nil || len(opts) != 0 {
		t.Fatalf("opts=%v err=%v", opts, err)
	}
	cleanup()
}

func TestOutputDisplayPathIsAbsoluteWithoutChangingStoragePath(t *testing.T) {
	storage := "./outputs/cluster/"
	display, err := outputDisplayPath(storage)
	if err != nil {
		t.Fatal(err)
	}
	if !filepath.IsAbs(display) {
		t.Fatalf("display path is not absolute: %q", display)
	}
	if storage != "./outputs/cluster/" {
		t.Fatalf("storage path changed: %q", storage)
	}
}
