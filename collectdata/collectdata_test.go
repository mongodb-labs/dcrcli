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

package collectdata

import (
	"bytes"
	"strings"
	"testing"

	"dcrcli/termui"
)

func TestAllAndString(t *testing.T) {
	a := All()
	if !a.GetMongoData || !a.FTDC || !a.Logs {
		t.Fatalf("All(): %+v", a)
	}
	if a.String() != "all" {
		t.Fatalf("All().String() = %q", a.String())
	}
	if a.Empty() {
		t.Fatal("All() should not be empty")
	}
	if !a.NeedsSSH() {
		t.Fatal("All() should need SSH")
	}
}

func TestParse(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want Selection
		err  bool
	}{
		{"all", All(), false},
		{"ALL", All(), false},
		{"getmongodata", Selection{GetMongoData: true}, false},
		{"get-mongo-data", Selection{GetMongoData: true}, false},
		{"gmd", Selection{GetMongoData: true}, false},
		{"ftdc", Selection{FTDC: true}, false},
		{"logs", Selection{Logs: true}, false},
		{"mongod-logs", Selection{Logs: true}, false},
		{"getmongodata,ftdc", Selection{GetMongoData: true, FTDC: true}, false},
		{" logs , getmongodata ", Selection{GetMongoData: true, Logs: true}, false},
		{"ftdc,ftdc,logs", Selection{FTDC: true, Logs: true}, false},
		{"getmongodata,all", All(), false},
		{"", Selection{}, true},
		{"nope", Selection{}, true},
		{",,,", Selection{}, true},
	} {
		got, err := Parse(tc.in)
		if tc.err {
			if err == nil {
				t.Fatalf("Parse(%q) wanted error", tc.in)
			}
			continue
		}
		if err != nil || got != tc.want {
			t.Fatalf("Parse(%q) = %+v, %v want %+v, nil", tc.in, got, err, tc.want)
		}
	}
}

func TestNeedsSSH(t *testing.T) {
	if (Selection{GetMongoData: true}).NeedsSSH() {
		t.Fatal("getMongoData-only should not need SSH")
	}
	if !(Selection{FTDC: true}).NeedsSSH() {
		t.Fatal("FTDC should need SSH")
	}
	if !(Selection{Logs: true}).NeedsSSH() {
		t.Fatal("logs should need SSH")
	}
}

func TestDescription(t *testing.T) {
	if All().Description() != "getMongoData, FTDC, and mongod logs" {
		t.Fatalf("%q", All().Description())
	}
	if (Selection{GetMongoData: true}).Description() != "getMongoData" {
		t.Fatalf("%q", (Selection{GetMongoData: true}).Description())
	}
}

func TestResolveFlagPrecedence(t *testing.T) {
	sel, err := Resolve("getmongodata", true, termui.New(strings.NewReader("1\n"), &bytes.Buffer{}))
	if err != nil || !sel.GetMongoData || sel.FTDC || sel.Logs {
		t.Fatalf("flag should ignore prompt stdin: got %+v, %v", sel, err)
	}
}

func TestResolveNonInteractiveDefault(t *testing.T) {
	sel, err := Resolve("", false, termui.New(strings.NewReader(""), &bytes.Buffer{}))
	if err != nil || sel != All() {
		t.Fatalf("non-TTY default: got %+v, %v", sel, err)
	}
}

func TestPromptChoices(t *testing.T) {
	for input, want := range map[string]Selection{
		"\n":  All(),
		"1\n": All(),
		"2\n": {GetMongoData: true},
		"3\n": {FTDC: true},
		"4\n": {Logs: true},
	} {
		var buf bytes.Buffer
		sel, err := Prompt(strings.NewReader(input), &buf)
		if err != nil || sel != want {
			t.Fatalf("Prompt(%q) = %+v, %v want %+v", input, sel, err, want)
		}
	}

	var buf bytes.Buffer
	sel, err := Prompt(strings.NewReader("5\ngetmongodata,logs\n"), &buf)
	if err != nil || !sel.GetMongoData || sel.FTDC || !sel.Logs {
		t.Fatalf("Prompt custom: got %+v, %v", sel, err)
	}

	_, err = Prompt(strings.NewReader("9\n"), &bytes.Buffer{})
	if err == nil {
		t.Fatal("expected error for invalid choice")
	}
}
