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
	"errors"
	"strings"
	"testing"
)

func makeProgress(hosts []CollectionHost, tasksPerNode int) *CollectionProgress {
	nodes := make([]collectionNodeStatus, len(hosts))
	for i, h := range hosts {
		nodes[i] = collectionNodeStatus{host: h, state: nodePending}
	}
	return &CollectionProgress{
		ui:         New(strings.NewReader(""), &strings.Builder{}),
		nodes:      nodes,
		totalTasks: len(hosts) * tasksPerNode,
		compact:    len(hosts) > progressCompactNodeThreshold,
	}
}

func TestProgressLinesCompactLargeCluster(t *testing.T) {
	// 9 shards × 3 members + 3 mongos + 9 config ≈ 39 nodes (typical large sharded scope).
	hosts := make([]CollectionHost, 39)
	for i := range hosts {
		hosts[i] = CollectionHost{Hostname: "shard-mongo", Port: 27017 + i}
	}
	cp := makeProgress(hosts, 3)
	if !cp.compact {
		t.Fatal("expected compact mode for 39-node cluster")
	}

	cp.BeginNode(0)
	lines := cp.progressLines()
	if len(lines) != 1 {
		t.Fatalf("compact live view should be 1 line (bar + active + counter), got %d", len(lines))
	}
	if !strings.Contains(lines[0], "›") {
		t.Fatalf("expected active node marker, got %q", lines[0])
	}
	if !strings.Contains(lines[0], "0/39 nodes") {
		t.Fatalf("expected node counter, got %q", lines[0])
	}
}

func TestProgressLinesSmallCluster(t *testing.T) {
	hosts := []CollectionHost{
		{Hostname: "mongo1", Port: 27017},
		{Hostname: "mongo2", Port: 27017},
		{Hostname: "mongo3", Port: 27017},
	}
	cp := makeProgress(hosts, 3)
	if cp.compact {
		t.Fatal("expected full node list for 3-node cluster")
	}
	cp.BeginNode(0)
	lines := cp.progressLines()
	if len(lines) != 1 {
		t.Fatalf("expected one-line live progress, got %d lines", len(lines))
	}
	if !strings.Contains(lines[0], "› mongo1:27017") {
		t.Fatalf("expected active node in progress line, got %q", lines[0])
	}
	if !strings.Contains(lines[0], "0/3 nodes") {
		t.Fatalf("expected node counter in progress line, got %q", lines[0])
	}
}

func TestSummaryLinesCompactShowsBarOnly(t *testing.T) {
	hosts := make([]CollectionHost, 10)
	for i := range hosts {
		hosts[i] = CollectionHost{Hostname: "node", Port: 27017 + i}
	}
	cp := makeProgress(hosts, 3)

	summary := cp.summaryLines()
	if len(summary) != 1 {
		t.Fatalf("compact summary should be bar only, got %d lines", len(summary))
	}
	if !strings.Contains(summary[0], "%") {
		t.Fatalf("expected progress bar, got %q", summary[0])
	}
}

func TestNodeResultLineShowsTaskOutcomes(t *testing.T) {
	cp := makeProgress([]CollectionHost{{Hostname: "mongo1", Port: 27017}}, 3)
	cp.nodes[0].state = nodeDone
	cp.nodes[0].tasks[0] = taskStatus{outcome: taskOK}
	cp.nodes[0].tasks[1] = taskStatus{outcome: taskNotSelected}
	cp.nodes[0].tasks[2] = taskStatus{outcome: taskNotSelected}

	line := cp.nodeResultLine(0)
	for _, want := range []string{"mongo1:27017", "getMongoData"} {
		if !strings.Contains(line, want) {
			t.Fatalf("expected %q in result line, got %q", want, line)
		}
	}
	for _, omit := range []string{"FTDC", "logs"} {
		if strings.Contains(line, omit) {
			t.Fatalf("did not expect %q in result line, got %q", omit, line)
		}
	}
}

func TestDeriveNodeState(t *testing.T) {
	cp := makeProgress([]CollectionHost{{Hostname: "a", Port: 1}}, 3)
	cp.nodes[0].tasks[0] = taskStatus{outcome: taskOK}
	cp.nodes[0].tasks[1] = taskStatus{outcome: taskNotSelected}
	if got := cp.deriveNodeState(0); got != nodeDone {
		t.Fatalf("expected done when only selected tasks succeed, got %v", got)
	}
	cp.nodes[0].tasks[1] = taskStatus{outcome: taskFailed}
	if got := cp.deriveNodeState(0); got != nodeFailed {
		t.Fatalf("expected failed, got %v", got)
	}
}

func TestNodeResultLineShowsUnavailableSkip(t *testing.T) {
	cp := makeProgress([]CollectionHost{{Hostname: "mongo1", Port: 27017}}, 3)
	cp.nodes[0].state = nodePartial
	cp.nodes[0].tasks[0] = taskStatus{outcome: taskOK}
	cp.nodes[0].tasks[1] = taskStatus{outcome: taskSkipped, note: "node not on dcrcli host; no SSH user"}
	cp.nodes[0].tasks[2] = taskStatus{outcome: taskNotSelected}

	line := cp.nodeResultLine(0)
	for _, want := range []string{"mongo1:27017", "getMongoData", "FTDC"} {
		if !strings.Contains(line, want) {
			t.Fatalf("expected %q in result line, got %q", want, line)
		}
	}
	if strings.Contains(line, "logs") {
		t.Fatalf("did not expect unselected logs in result line, got %q", line)
	}
}

func TestDeriveNodeStateUnavailableOnly(t *testing.T) {
	cp := makeProgress([]CollectionHost{{Hostname: "a", Port: 1}}, 3)
	cp.nodes[0].tasks[0] = taskStatus{outcome: taskNotSelected}
	cp.nodes[0].tasks[1] = taskStatus{outcome: taskSkipped, note: "node not on dcrcli host; no SSH user"}
	cp.nodes[0].tasks[2] = taskStatus{outcome: taskNotSelected}
	if got := cp.deriveNodeState(0); got != nodeFailed {
		t.Fatalf("expected failed when selected artifact could not run, got %v", got)
	}
}

func TestDeriveNodeStatePartialOnUnavailable(t *testing.T) {
	cp := makeProgress([]CollectionHost{{Hostname: "a", Port: 1}}, 3)
	cp.nodes[0].tasks[0] = taskStatus{outcome: taskOK}
	cp.nodes[0].tasks[1] = taskStatus{outcome: taskSkipped, note: "node not on dcrcli host; no SSH user"}
	cp.nodes[0].tasks[2] = taskStatus{outcome: taskNotSelected}
	if got := cp.deriveNodeState(0); got != nodePartial {
		t.Fatalf("expected partial when some selected tasks succeed and others are unavailable, got %v", got)
	}
}

func TestWriteProgressLineStaysOnSameLine(t *testing.T) {
	var buf strings.Builder
	cp := makeProgress([]CollectionHost{{Hostname: "mongo1", Port: 27017}}, 3)
	cp.display = &buf

	cp.writeProgressLine("  bar   0%")
	cp.writeProgressLine("  bar  50%")
	cp.writeProgressLine("  bar 100%")

	out := buf.String()
	if strings.Count(out, "\n") != 1 {
		t.Fatalf("expected a single reserved SSH slot newline, got %q", out)
	}
	if strings.Count(out, "\r") < 3 {
		t.Fatalf("expected carriage-return bar updates, got %q", out)
	}
}

func TestFitLinePreventsWrap(t *testing.T) {
	long := "  " + strings.Repeat("█", 32) + "  100%  › shard01-mongo1.internal:27017  54/55 nodes"
	got := fitLine(long, 40)
	if visibleLen(got) > 40 {
		t.Fatalf("fitLine exceeded width: %d > 40 (%q)", visibleLen(got), got)
	}
	if !strings.HasSuffix(got, "…") {
		t.Fatalf("expected truncated line with ellipsis, got %q", got)
	}
}

func TestSSHHandoffAppearsBelowBar(t *testing.T) {
	var buf strings.Builder
	cp := makeProgress([]CollectionHost{{Hostname: "mongo1", Port: 27017}}, 3)
	cp.display = &buf

	cp.writeProgressLine("  bar  10%")
	cp.handoffForSubprocess(&SSHTarget{
		User:      "ubuntu",
		Host:      "mongo1",
		Purpose:   "FTDC data",
		MongoHost: "mongo1",
		MongoPort: 27017,
	})

	out := buf.String()
	barAt := strings.Index(out, "bar  10%")
	nlAt := strings.Index(out, "\n")
	sshAt := strings.Index(out, "ubuntu@mongo1")
	if barAt < 0 || nlAt < 0 || sshAt < 0 {
		t.Fatalf("expected bar, newline, and SSH hint, got %q", out)
	}
	if !(barAt < nlAt && nlAt < sshAt) {
		t.Fatalf("SSH hint must print below the bar, got %q", out)
	}
	if cp.onHintLine {
		t.Fatal("hint must end with a newline so an SSH password prompt gets a blank row")
	}
	if !strings.HasSuffix(out, "\n") {
		t.Fatalf("SSH hint must end with a newline so Password: does not overwrite it, got %q", out)
	}
}

func TestSSHHandoffLeavesBlankLineForPasswordPrompt(t *testing.T) {
	var buf strings.Builder
	cp := makeProgress([]CollectionHost{{Hostname: "mongo1", Port: 27017}}, 3)
	cp.display = &buf

	cp.writeProgressLine("  bar  25%")
	cp.handoffForSubprocess(&SSHTarget{
		User:      "test",
		Host:      "mongo2",
		Purpose:   "diagnostic commands",
		MongoHost: "mongo2",
		MongoPort: 27017,
	})

	out := buf.String()
	if !strings.Contains(out, "copying diagnostic commands for mongo2:27017") {
		t.Fatalf("expected SSH hint, got %q", out)
	}
	if !strings.HasSuffix(out, "\n") {
		t.Fatalf("hint must be followed by a newline before SSH can print Password:, got %q", out)
	}
	if strings.Contains(out, "(enter password") {
		t.Fatalf("hint should not share a line with the Password: prompt, got %q", out)
	}
}

func TestReconcileReturnsToBar(t *testing.T) {
	var buf strings.Builder
	cp := makeProgress([]CollectionHost{{Hostname: "mongo1", Port: 27017}}, 3)
	cp.display = &buf

	cp.writeProgressLine("  bar  10%")
	cp.handoffForSubprocess(&SSHTarget{
		User: "ubuntu", Host: "mongo1", Purpose: "FTDC data",
		MongoHost: "mongo1", MongoPort: 27017,
	})
	cp.reconcileAfterSubprocess(nil)
	cp.writeProgressLine("  bar  20%")

	out := buf.String()
	if cp.onHintLine {
		t.Fatal("cursor should not stay on the SSH hint after reconcile")
	}
	if !strings.Contains(out, "bar  20%") {
		t.Fatalf("expected progress bar to resume after SSH, got %q", out)
	}
}

func TestSSHRegionReusesTheSameThreeRows(t *testing.T) {
	screen := newFakeTerm()
	hosts := []CollectionHost{
		{Hostname: "mongo1", Port: 27017},
		{Hostname: "mongo2", Port: 27017},
		{Hostname: "mongo3", Port: 27017},
	}
	cp := makeProgress(hosts, 1)
	cp.display = screen
	cp.cursorReport = func() (int, bool) { return screen.row + 1, true }

	for i, host := range hosts {
		cp.BeginNode(i)
		ssh := &SSHTarget{
			User: "test", Host: host.Hostname, Purpose: "FTDC data",
			MongoHost: host.Hostname, MongoPort: host.Port,
		}
		err := cp.RunTask(1, ssh, func() error {
			barRow := screen.row - 2
			if !strings.Contains(screen.line(barRow), "%") {
				t.Fatalf("expected the bar two rows above the prompt, got %q", screen.line(barRow))
			}
			if !strings.Contains(screen.line(barRow+1), "SSH to test@"+host.Hostname) {
				t.Fatalf("expected the SSH hint under the bar, got %q", screen.line(barRow+1))
			}
			if strings.TrimSpace(screen.line(screen.row)) != "" {
				t.Fatalf("password prompt row is not clear: %q", screen.line(screen.row))
			}
			if screen.countLinesContaining("SSH to test@") != 1 {
				t.Fatalf("only the current node's hint should be on screen:\n%s", screen.screen())
			}
			// What OpenSSH prints on the reserved row, plus the newline it
			// emits once the password is entered.
			screen.Write([]byte("(test@" + host.Hostname + ") Password: \n"))
			return nil
		})
		if err != nil {
			t.Fatalf("RunTask returned %v", err)
		}
		cp.FinishNode()

		if got := screen.countLinesContaining("%"); got != 1 {
			t.Fatalf("expected exactly one progress bar on screen, got %d:\n%s", got, screen.screen())
		}
		if screen.countLinesContaining("SSH to test@") != 0 {
			t.Fatalf("SSH hint should be cleared after the task:\n%s", screen.screen())
		}
		if screen.countLinesContaining("Password:") != 0 {
			t.Fatalf("password prompt should be cleared after the task:\n%s", screen.screen())
		}
	}

	if len(screen.scrolledOff) != 0 {
		t.Fatalf("the three-row view should not scroll the screen, lost %d rows", len(screen.scrolledOff))
	}
}

func TestSSHRegionSurvivesScrollingAtScreenBottom(t *testing.T) {
	screen := newFakeTerm()
	// Start with the cursor on the last row: every hint/prompt row now scrolls.
	screen.Write([]byte(strings.Repeat("\n", screen.height-1)))

	hosts := make([]CollectionHost, 10)
	for i := range hosts {
		hosts[i] = CollectionHost{Hostname: "shardlab1", Port: 27017 + i}
	}
	cp := makeProgress(hosts, 3)
	cp.display = screen
	cp.cursorReport = func() (int, bool) { return screen.row + 1, true }

	for i, host := range hosts {
		cp.BeginNode(i)
		for _, purpose := range []string{"FTDC data", "mongod logs", "diagnostic commands"} {
			ssh := &SSHTarget{
				User: "test", Host: host.Hostname, Purpose: purpose,
				MongoHost: host.Hostname, MongoPort: host.Port,
			}
			_ = cp.RunTask(1, ssh, func() error {
				screen.Write([]byte("(test@shardlab1) Password: \n"))
				return nil
			})
			if got := screen.countLinesContaining("%"); got != 1 {
				t.Fatalf("expected one bar after %s on node %d, got %d:\n%s", purpose, i, got, screen.screen())
			}
			if screen.countLinesContaining("SSH to test@") != 0 {
				t.Fatalf("stale SSH hint after %s on node %d:\n%s", purpose, i, screen.screen())
			}
			if screen.countLinesContaining("Password:") != 0 {
				t.Fatalf("stale password prompt after %s on node %d:\n%s", purpose, i, screen.screen())
			}
		}
		cp.FinishNode()
	}

	for _, line := range screen.scrolledOff {
		if strings.Contains(line, "Password:") || strings.Contains(line, "SSH to test@") {
			t.Fatalf("SSH chatter scrolled into the scrollback: %q", line)
		}
	}
}

func TestSSHRegionKeepsFailedTaskOutput(t *testing.T) {
	screen := newFakeTerm()
	cp := makeProgress([]CollectionHost{{Hostname: "mongo1", Port: 27017}}, 1)
	cp.display = screen
	cp.cursorReport = func() (int, bool) { return screen.row + 1, true }

	cp.BeginNode(0)
	err := cp.RunTask(1, &SSHTarget{
		User: "test", Host: "mongo1", Purpose: "FTDC data",
		MongoHost: "mongo1", MongoPort: 27017,
	}, func() error {
		screen.Write([]byte("rsync: connection unexpectedly closed\n"))
		return errors.New("rsync failed")
	})
	if err == nil {
		t.Fatal("expected the task error to be returned")
	}
	if screen.countLinesContaining("rsync: connection unexpectedly closed") != 1 {
		t.Fatalf("a failed task must keep its output on screen:\n%s", screen.screen())
	}
}

func TestSSHRegionFallsBackWithoutCursorReports(t *testing.T) {
	screen := newFakeTerm()
	cp := makeProgress([]CollectionHost{{Hostname: "mongo1", Port: 27017}}, 1)
	cp.display = screen
	cp.cursorReport = func() (int, bool) { return 0, false }

	cp.BeginNode(0)
	err := cp.RunTask(1, &SSHTarget{
		User: "test", Host: "mongo1", Purpose: "FTDC data",
		MongoHost: "mongo1", MongoPort: 27017,
	}, func() error {
		screen.Write([]byte("(test@mongo1) Password: \n"))
		return nil
	})
	if err != nil {
		t.Fatalf("RunTask returned %v", err)
	}

	if !cp.noCursorReports {
		t.Fatal("a terminal without cursor reports should be remembered")
	}
	if screen.countLinesContaining("Password:") != 1 {
		t.Fatalf("fallback keeps SSH output in the scrollback:\n%s", screen.screen())
	}
	if !strings.Contains(screen.line(screen.row), "100%") {
		t.Fatalf("expected the bar to resume on a fresh row below SSH output:\n%s", screen.screen())
	}
}

func TestParseCursorReport(t *testing.T) {
	if row, done := parseCursorReport([]byte("\033[12;40R")); !done || row != 12 {
		t.Fatalf("expected row 12, got %d (done=%v)", row, done)
	}
	if row, done := parseCursorReport([]byte("\033[7;1")); done || row != 0 {
		t.Fatalf("incomplete reply must not be parsed, got %d (done=%v)", row, done)
	}
	if row, done := parseCursorReport([]byte("x\033[3;5R")); !done || row != 3 {
		t.Fatalf("expected row 3 despite leading input, got %d (done=%v)", row, done)
	}
}

func TestSSHHandoffMessageIncludesPurpose(t *testing.T) {
	msg := sshHandoffMessage(&SSHTarget{
		User:      "ubuntu",
		Host:      "mongo1",
		Purpose:   "FTDC data",
		MongoHost: "mongo1",
		MongoPort: 27017,
	})
	if !strings.Contains(msg, "FTDC data") || !strings.Contains(msg, "ubuntu@mongo1") {
		t.Fatalf("unexpected handoff message: %q", msg)
	}
	if strings.Contains(msg, "enter password") {
		t.Fatalf("hint should not include password instructions (SSH prints Password: itself): %q", msg)
	}
}
