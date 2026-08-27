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

// Package collectdata defines which diagnostic artifacts dcrcli collects (getMongoData, FTDC, mongod logs).
package collectdata

import (
	"fmt"
	"io"
	"strings"

	"dcrcli/termui"
)

// Selection controls which diagnostic artifacts are collected per target node.
type Selection struct {
	GetMongoData bool
	FTDC         bool
	Logs         bool
}

const (
	flagAll          = "all"
	flagGetMongoData = "getmongodata"
	flagFTDC         = "ftdc"
	flagLogs         = "logs"
)

// All returns a selection that collects every supported artifact type.
func All() Selection {
	return Selection{GetMongoData: true, FTDC: true, Logs: true}
}

// Empty reports whether no artifact type is enabled.
func (s Selection) Empty() bool {
	return !s.GetMongoData && !s.FTDC && !s.Logs
}

// NeedsSSH is true when FTDC or mongod log copy may require remote SSH/rsync.
func (s Selection) NeedsSSH() bool {
	return s.FTDC || s.Logs
}

// String returns the canonical comma-separated flag form (or "all").
func (s Selection) String() string {
	if s.GetMongoData && s.FTDC && s.Logs {
		return flagAll
	}
	parts := make([]string, 0, 3)
	if s.GetMongoData {
		parts = append(parts, flagGetMongoData)
	}
	if s.FTDC {
		parts = append(parts, flagFTDC)
	}
	if s.Logs {
		parts = append(parts, flagLogs)
	}
	return strings.Join(parts, ",")
}

// Description is a short human-readable summary for stdout and logs.
func (s Selection) Description() string {
	if s.GetMongoData && s.FTDC && s.Logs {
		return "getMongoData, FTDC, and mongod logs"
	}
	parts := make([]string, 0, 3)
	if s.GetMongoData {
		parts = append(parts, "getMongoData")
	}
	if s.FTDC {
		parts = append(parts, "FTDC")
	}
	if s.Logs {
		parts = append(parts, "mongod logs")
	}
	return strings.Join(parts, ", ")
}

// Parse parses --collect-data flag values.
// Accepts "all", a single type, or a comma-separated list of types
// (getmongodata, ftdc, logs). Order and duplicates do not matter.
func Parse(s string) (Selection, error) {
	raw := strings.TrimSpace(strings.ToLower(s))
	if raw == "" {
		return Selection{}, fmt.Errorf("collect-data value is empty")
	}
	if raw == flagAll {
		return All(), nil
	}

	parts := strings.Split(raw, ",")
	seen := make(map[string]bool, len(parts))
	var sel Selection
	for _, p := range parts {
		token := strings.TrimSpace(p)
		if token == "" {
			continue
		}
		if token == flagAll {
			return All(), nil
		}
		if seen[token] {
			continue
		}
		seen[token] = true
		switch token {
		case flagGetMongoData, "get-mongo-data", "gmd":
			sel.GetMongoData = true
		case flagFTDC:
			sel.FTDC = true
		case flagLogs, "mongod-logs", "mongodlogs", "log":
			sel.Logs = true
		default:
			return Selection{}, fmt.Errorf(
				"invalid --collect-data value %q (want %s, or a comma-separated list of %s, %s, %s)",
				s, flagAll, flagGetMongoData, flagFTDC, flagLogs,
			)
		}
	}
	if sel.Empty() {
		return Selection{}, fmt.Errorf(
			"invalid --collect-data value %q (want %s, or a comma-separated list of %s, %s, %s)",
			s, flagAll, flagGetMongoData, flagFTDC, flagLogs,
		)
	}
	return sel, nil
}

// Prompt asks the user which diagnostic artifacts to collect.
func Prompt(stdin io.Reader, stdout io.Writer) (Selection, error) {
	return PromptUI(termui.New(stdin, stdout))
}

// PromptUI asks the user which diagnostic artifacts to collect with the shared terminal UI.
// Call ui.Header("Collection data") before Resolve/PromptUI when starting this section.
func PromptUI(ui *termui.UI) (Selection, error) {
	ui.Note("Which diagnostic data should dcrcli collect from each target node?")
	ui.Menu([]string{
		"All — getMongoData, FTDC, and mongod logs (default)",
		"getMongoData only",
		"FTDC only",
		"mongod logs only",
	})
	ui.Blank()
	line, err := ui.AskChoice("Choice [1]")
	if err != nil {
		return Selection{}, err
	}
	var sel Selection
	var choice string
	switch {
	case line == "" || line == "1":
		sel = All()
		choice = "1"
	case line == "2":
		sel = Selection{GetMongoData: true}
		choice = "2"
	case line == "3":
		sel = Selection{FTDC: true}
		choice = "3"
	case line == "4":
		sel = Selection{Logs: true}
		choice = "4"
	default:
		return Selection{}, fmt.Errorf("invalid choice %q: enter 1–4", line)
	}
	ui.Blank()
	ui.Ok(fmt.Sprintf("Selected option %s — %s", choice, sel.Description()))
	ui.Blank()
	return sel, nil
}

// Resolve returns the data selection. Non-empty flagValue wins; otherwise on a TTY Prompt is used;
// non-interactive stdin defaults to All without prompting.
func Resolve(flagValue string, isTerminal bool, ui *termui.UI) (Selection, error) {
	if strings.TrimSpace(flagValue) != "" {
		return Parse(flagValue)
	}
	if isTerminal {
		return PromptUI(ui)
	}
	return All(), nil
}
