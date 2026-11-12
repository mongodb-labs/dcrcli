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
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"dcrcli/subprocess"
)

// sshForwardSpec describes a local SSH port forward to a loopback listener
// on a remote host. It does not start the remote web UI.
type sshForwardSpec struct {
	Via        string // SSH destination: user@host or ssh config Host alias
	LocalPort  int
	RemoteHost string
	RemotePort int
}

func parseHostPort(addr string) (host string, port int, err error) {
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return "", 0, fmt.Errorf("invalid address %q: %w", addr, err)
	}
	port, err = strconv.Atoi(portStr)
	if err != nil || port < 1 || port > 65535 {
		return "", 0, fmt.Errorf("invalid port in address %q", addr)
	}
	return host, port, nil
}

func newSSHForwardSpec(via, addr string, localPort int) (sshForwardSpec, error) {
	via = strings.TrimSpace(via)
	if via == "" {
		return sshForwardSpec{}, fmt.Errorf("--via requires an SSH destination (example: user@jump-host)")
	}
	if strings.ContainsAny(via, " \t\n\r") {
		return sshForwardSpec{}, fmt.Errorf("--via must be a single SSH destination (user@host or Host alias), got %q", via)
	}
	if !isLoopbackAddr(addr) {
		return sshForwardSpec{}, fmt.Errorf("--via requires -addr on loopback (default %s); got %s", defaultWebAddr, addr)
	}
	host, port, err := parseHostPort(addr)
	if err != nil {
		return sshForwardSpec{}, err
	}
	if localPort == 0 {
		localPort = port
	}
	if localPort < 1 || localPort > 65535 {
		return sshForwardSpec{}, fmt.Errorf("invalid -local-port %d", localPort)
	}
	// Forward to the exact numeric loopback on the remote side. Hostnames use
	// IPv4 loopback so SSH does not choose a different address family.
	remoteHost := "127.0.0.1"
	if ip := net.ParseIP(strings.Trim(host, "[]")); ip != nil && ip.IsLoopback() {
		remoteHost = ip.String()
	}
	return sshForwardSpec{
		Via:        via,
		LocalPort:  localPort,
		RemoteHost: remoteHost,
		RemotePort: port,
	}, nil
}

func (s sshForwardSpec) sshArgs() []string {
	local := net.JoinHostPort("127.0.0.1", strconv.Itoa(s.LocalPort))
	remote := net.JoinHostPort(s.RemoteHost, strconv.Itoa(s.RemotePort))
	forward := local + ":" + remote
	return []string{"-N", "-L", forward, s.Via}
}

func (s sshForwardSpec) localURL() string {
	return fmt.Sprintf("http://127.0.0.1:%d", s.LocalPort)
}

// runSSHTunnel opens a local SSH port forward and blocks until SSH exits or
// the process is interrupted. The remote web UI must already be running.
func runSSHTunnel(spec sshForwardSpec) error {
	sshPath, err := exec.LookPath("ssh")
	if err != nil {
		return fmt.Errorf("ssh not found on PATH: %w", err)
	}
	args := spec.sshArgs()
	fmt.Printf("dcr web: SSH local port forward (tunnel only)\n")
	fmt.Printf("  assumes %s web is already listening on the jump host at %s:%d\n", productName(), spec.RemoteHost, spec.RemotePort)
	fmt.Printf("  %s %s\n", sshPath, strings.Join(args, " "))
	fmt.Printf("  open %s in this workstation's browser\n", spec.localURL())
	fmt.Printf("  Ctrl+C stops the tunnel (does not stop the remote web UI)\n")

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cmd := subprocess.CommandContext(ctx, sshPath, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if ctx.Err() != nil {
		fmt.Println("\nstopped SSH tunnel")
		return nil
	}
	if err != nil {
		return fmt.Errorf("ssh tunnel: %w", err)
	}
	return nil
}
