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

package termui

import (
	"strconv"
	"strings"
	"unicode/utf8"
)

// fakeTerm is a minimal terminal emulator: enough of the escape sequences the
// progress view uses (cursor up, erase line, erase below, line feed with
// scrolling) to assert what a user would actually see.
type fakeTerm struct {
	width, height int
	rows          [][]rune
	row, col      int      // 0-based cursor
	scrolledOff   []string // rows pushed above the top of the screen
}

func newFakeTerm() *fakeTerm {
	// Matches termSize's default so the emulator and the code agree on size.
	t := &fakeTerm{width: 80, height: 24}
	t.rows = make([][]rune, t.height)
	return t
}

func (t *fakeTerm) Write(p []byte) (int, error) {
	data := string(p)
	for i := 0; i < len(data); {
		switch c := data[i]; {
		case c == '\033' && i+1 < len(data) && data[i+1] == '[':
			j := i + 2
			for j < len(data) && (data[j] == ';' || (data[j] >= '0' && data[j] <= '9')) {
				j++
			}
			if j >= len(data) {
				return len(p), nil
			}
			t.csi(data[i+2:j], data[j])
			i = j + 1
		case c == '\r':
			t.col = 0
			i++
		case c == '\n':
			// The tty driver maps \n to CR LF in cooked mode.
			t.col = 0
			t.lineFeed()
			i++
		default:
			r, size := utf8.DecodeRuneInString(data[i:])
			t.putRune(r)
			i += size
		}
	}
	return len(p), nil
}

func (t *fakeTerm) csi(params string, final byte) {
	n, err := strconv.Atoi(params)
	if err != nil {
		n = 1
	}
	switch final {
	case 'A':
		t.row -= n
		if t.row < 0 {
			t.row = 0
		}
	case 'B':
		t.row += n
		if t.row > t.height-1 {
			t.row = t.height - 1
		}
	case 'K':
		if params == "2" {
			t.rows[t.row] = nil
		} else {
			t.rows[t.row] = t.truncated(t.row)
		}
	case 'J':
		if params == "" || params == "0" {
			t.rows[t.row] = t.truncated(t.row)
			for r := t.row + 1; r < t.height; r++ {
				t.rows[r] = nil
			}
		}
	case 'm': // colors do not affect layout
	}
}

func (t *fakeTerm) truncated(row int) []rune {
	if len(t.rows[row]) <= t.col {
		return t.rows[row]
	}
	return t.rows[row][:t.col]
}

func (t *fakeTerm) putRune(r rune) {
	if t.col >= t.width {
		return // the progress view fits its lines, so wrapping is out of scope
	}
	line := t.rows[t.row]
	for len(line) <= t.col {
		line = append(line, ' ')
	}
	line[t.col] = r
	t.rows[t.row] = line
	t.col++
}

func (t *fakeTerm) lineFeed() {
	t.row++
	if t.row < t.height {
		return
	}
	t.scrolledOff = append(t.scrolledOff, string(t.rows[0]))
	t.rows = append(t.rows[1:], nil)
	t.row = t.height - 1
}

func (t *fakeTerm) line(row int) string {
	if row < 0 || row >= t.height {
		return ""
	}
	return string(t.rows[row])
}

func (t *fakeTerm) screen() string {
	lines := make([]string, 0, t.height)
	for r := 0; r < t.height; r++ {
		lines = append(lines, t.line(r))
	}
	return strings.Join(lines, "\n")
}

func (t *fakeTerm) countLinesContaining(sub string) int {
	count := 0
	for r := 0; r < t.height; r++ {
		if strings.Contains(t.line(r), sub) {
			count++
		}
	}
	return count
}
