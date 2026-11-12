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
	"strings"
	"testing"
)

func TestNewSSHForwardSpecDefault(t *testing.T) {
	spec, err := newSSHForwardSpec("user@jump", defaultWebAddr, 0)
	if err != nil {
		t.Fatal(err)
	}
	if spec.Via != "user@jump" || spec.LocalPort != 4041 || spec.RemotePort != 4041 || spec.RemoteHost != "127.0.0.1" {
		t.Fatalf("spec: %+v", spec)
	}
	args := spec.sshArgs()
	want := []string{"-N", "-o", "ExitOnForwardFailure=yes", "-L", "127.0.0.1:4041:127.0.0.1:4041", "--", "user@jump"}
	if strings.Join(args, " ") != strings.Join(want, " ") {
		t.Fatalf("args %v want %v", args, want)
	}
	if spec.localURL() != "http://127.0.0.1:4041" {
		t.Fatalf("url %q", spec.localURL())
	}
}

func TestNewSSHForwardSpecLocalPort(t *testing.T) {
	spec, err := newSSHForwardSpec("dcr-jump", "127.0.0.1:4041", 18080)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(spec.sshArgs(), " ") != "-N -o ExitOnForwardFailure=yes -L 127.0.0.1:18080:127.0.0.1:4041 -- dcr-jump" {
		t.Fatalf("args %v", spec.sshArgs())
	}
	if spec.localURL() != "http://127.0.0.1:18080" {
		t.Fatalf("url %q", spec.localURL())
	}
}

func TestSSHForwardPreservesNumericLoopbackAndBracketsIPv6(t *testing.T) {
	spec, err := newSSHForwardSpec("jump", "127.0.0.2:4041", 0)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(spec.sshArgs(), " "); got != "-N -o ExitOnForwardFailure=yes -L 127.0.0.1:4041:127.0.0.2:4041 -- jump" {
		t.Fatalf("IPv4 args %q", got)
	}

	spec, err = newSSHForwardSpec("jump", "[::1]:4041", 0)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(spec.sshArgs(), " "); got != "-N -o ExitOnForwardFailure=yes -L 127.0.0.1:4041:[::1]:4041 -- jump" {
		t.Fatalf("IPv6 args %q", got)
	}
}

func TestNewSSHForwardSpecRejectsNonLoopback(t *testing.T) {
	_, err := newSSHForwardSpec("user@jump", "0.0.0.0:4041", 0)
	if err == nil || !strings.Contains(err.Error(), "loopback") {
		t.Fatalf("err = %v", err)
	}
}

func TestNewSSHForwardSpecRejectsBadVia(t *testing.T) {
	_, err := newSSHForwardSpec("user@jump extra", defaultWebAddr, 0)
	if err == nil {
		t.Fatal("expected error for spaced via")
	}
	_, err = newSSHForwardSpec("  ", defaultWebAddr, 0)
	if err == nil {
		t.Fatal("expected error for empty via")
	}
	// An option-looking destination must never reach ssh as a flag.
	for _, via := range []string{"-oProxyCommand=touch/tmp/x", "-F/etc/passwd"} {
		if _, err := newSSHForwardSpec(via, defaultWebAddr, 0); err == nil {
			t.Fatalf("expected error for option-like via %q", via)
		}
	}
}

func TestSSHForwardFailsWhenLocalPortIsBusy(t *testing.T) {
	spec, err := newSSHForwardSpec("jump", defaultWebAddr, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(spec.sshArgs(), " "); !strings.Contains(got, "ExitOnForwardFailure=yes") {
		t.Fatalf("tunnel must exit when its local port cannot bind: %q", got)
	}
}

func TestWebViaUsesTunnelNotListen(t *testing.T) {
	// Invalid via should fail before attempting to listen or invoke ssh.
	err := Run([]string{"-via", "bad via"})
	if err == nil || !strings.Contains(err.Error(), "single SSH destination") {
		t.Fatalf("err = %v", err)
	}
}
