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

//go:build unix

package termui

import (
	"os"
	"syscall"
	"time"

	"golang.org/x/term"
)

// queryCursorRow asks the terminal where the cursor is (ESC[6n) and returns the
// 1-based row. The terminal answers on the input stream, so the input fd is put
// in raw non-blocking mode for the round trip and restored before returning.
// ok is false when no reply arrives within cursorQueryTimeout, which is how
// terminals that do not implement cursor reports are detected.
func queryCursorRow(out *os.File, inFd int) (int, bool) {
	state, err := term.MakeRaw(inFd)
	if err != nil {
		return 0, false
	}
	defer func() { _ = term.Restore(inFd, state) }()

	if err := syscall.SetNonblock(inFd, true); err != nil {
		return 0, false
	}
	defer func() { _ = syscall.SetNonblock(inFd, false) }()

	if _, err := out.WriteString("\033[6n"); err != nil {
		return 0, false
	}

	deadline := time.Now().Add(cursorQueryTimeout)
	buf := make([]byte, 32)
	reply := make([]byte, 0, 32)
	for time.Now().Before(deadline) {
		n, err := syscall.Read(inFd, buf)
		if n > 0 {
			reply = append(reply, buf[:n]...)
			if row, done := parseCursorReport(reply); done {
				return row, row > 0
			}
			continue
		}
		switch err {
		case syscall.EAGAIN, syscall.EINTR, nil:
			time.Sleep(2 * time.Millisecond)
		default:
			return 0, false
		}
	}
	return 0, false
}
