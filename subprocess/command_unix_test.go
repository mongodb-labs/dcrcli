//go:build !windows

package subprocess

import (
	"bufio"
	"context"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func TestCommandContextKillsChildrenAndDetachesTerminal(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// The shell starts a long-running child, reports its pid, and waits on it.
	cmd := CommandContext(ctx, "sh", "-c", "sleep 60 & echo $!; wait")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	line, err := bufio.NewReader(stdout).ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	childPID, err := strconv.Atoi(strings.TrimSpace(line))
	if err != nil {
		t.Fatalf("child pid %q: %v", line, err)
	}

	// A new session means no controlling terminal is inherited, so ssh uses
	// SSH_ASKPASS rather than stopping on /dev/tty.
	// unix.Getsid, not syscall.Getsid: the standard library does not export
	// Getsid on Linux.
	if sid, err := unix.Getsid(cmd.Process.Pid); err != nil || sid != cmd.Process.Pid {
		t.Fatalf("command session = %d (err %v), want its own session %d", sid, err, cmd.Process.Pid)
	}

	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("command did not stop after cancellation")
	}

	deadline := time.Now().Add(3 * time.Second)
	for {
		if err := syscall.Kill(childPID, 0); err == syscall.ESRCH {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("grandchild %d survived cancellation", childPID)
		}
		time.Sleep(25 * time.Millisecond)
	}
}
