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
	"bytes"
	"os"
	"syscall"
	"testing"
	"unsafe"
)

// openPTY returns a connected master/slave pair so the cursor query can be
// exercised against a real terminal device.
func openPTY(t *testing.T) (master, slave *os.File) {
	t.Helper()
	m, err := os.OpenFile("/dev/ptmx", os.O_RDWR, 0)
	if err != nil {
		t.Skipf("no /dev/ptmx: %v", err)
	}
	const (
		tioctlPtyGrant = 0x20007454
		tioctlPtyUnlk  = 0x20007452
		tioctlPtyGname = 0x40807453
	)
	for _, req := range []uintptr{tioctlPtyGrant, tioctlPtyUnlk} {
		if _, _, e := syscall.Syscall(syscall.SYS_IOCTL, m.Fd(), req, 0); e != 0 {
			m.Close()
			t.Skipf("pty ioctl %#x: %v", req, e)
		}
	}
	var name [128]byte
	if _, _, e := syscall.Syscall(syscall.SYS_IOCTL, m.Fd(), tioctlPtyGname, uintptr(unsafe.Pointer(&name[0]))); e != 0 {
		m.Close()
		t.Skipf("pty name ioctl: %v", e)
	}
	path := string(bytes.TrimRight(name[:bytes.IndexByte(name[:], 0)], "\x00"))
	s, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		m.Close()
		t.Skipf("open %s: %v", path, err)
	}
	t.Cleanup(func() { s.Close(); m.Close() })
	return m, s
}

func TestQueryCursorRowOnRealTTY(t *testing.T) {
	master, slave := openPTY(t)

	// Stand in for the terminal emulator: answer ESC[6n with row 12.
	go func() {
		buf := make([]byte, 64)
		for {
			n, err := master.Read(buf)
			if err != nil {
				return
			}
			if bytes.Contains(buf[:n], []byte("\033[6n")) {
				_, _ = master.Write([]byte("\033[12;7R"))
			}
		}
	}()

	row, ok := queryCursorRow(slave, int(slave.Fd()))
	if !ok || row != 12 {
		t.Fatalf("queryCursorRow = (%d, %v), want (12, true)", row, ok)
	}

	// The slave must be usable (blocking, cooked) again afterwards.
	if _, err := slave.WriteString("after\n"); err != nil {
		t.Fatalf("write after query: %v", err)
	}
}

func TestQueryCursorRowTimesOutWhenTerminalIsSilent(t *testing.T) {
	master, slave := openPTY(t)
	go func() {
		buf := make([]byte, 64)
		for {
			if _, err := master.Read(buf); err != nil {
				return
			}
		}
	}()

	if row, ok := queryCursorRow(slave, int(slave.Fd())); ok || row != 0 {
		t.Fatalf("expected no reply, got (%d, %v)", row, ok)
	}
}
