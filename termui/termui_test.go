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
	if !cp.onHintLine {
		t.Fatal("expected cursor on the SSH hint row")
	}
}

func TestSSHHintReusesSameLine(t *testing.T) {
	var buf strings.Builder
	cp := makeProgress([]CollectionHost{{Hostname: "mongo1", Port: 27017}}, 3)
	cp.display = &buf
	ssh := &SSHTarget{
		User: "ubuntu", Host: "mongo1", Purpose: "FTDC data",
		MongoHost: "mongo1", MongoPort: 27017,
	}

	cp.writeProgressLine("  bar  10%")
	cp.handoffForSubprocess(ssh)
	ssh.Purpose = "mongod logs"
	cp.handoffForSubprocess(ssh)

	out := buf.String()
	if strings.Count(out, "\n") != 2 {
		t.Fatalf("SSH hints must overwrite one slot under the bar, got %d newlines in %q", strings.Count(out, "\n"), out)
	}
	if !strings.Contains(out, "mongod logs") {
		t.Fatalf("expected second hint to replace the first, got %q", out)
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
	cp.reconcileAfterSubprocess()
	cp.writeProgressLine("  bar  20%")

	out := buf.String()
	if !strings.Contains(out, "\033[1A") {
		t.Fatalf("expected return to bar after SSH, got %q", out)
	}
	if cp.onHintLine {
		t.Fatal("cursor should be back on the bar after reconcile")
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
}
