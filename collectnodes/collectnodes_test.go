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

package collectnodes

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"dcrcli/topologyfinder"
)

func TestModeString(t *testing.T) {
	if ModeOneSecondary.String() != "one-secondary" || ModeAllNodes.String() != "all-nodes" {
		t.Fatalf("Mode.String: %s %s", ModeOneSecondary.String(), ModeAllNodes.String())
	}
}

func TestParseMode(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want Mode
		err  bool
	}{
		{"one-secondary", ModeOneSecondary, false},
		{"ONE-SECONDARY", ModeOneSecondary, false},
		{" all-secondaries ", ModeAllSecondaries, false},
		{"all-nodes", ModeAllNodes, false},
		{"", Mode(0), true},
		{"nope", 0, true},
	} {
		m, err := ParseMode(tc.in)
		if tc.err {
			if err == nil {
				t.Fatalf("ParseMode(%q) wanted error", tc.in)
			}
			continue
		}
		if err != nil || m != tc.want {
			t.Fatalf("ParseMode(%q) = %v, %v want %v, nil", tc.in, m, err, tc.want)
		}
	}
}

func TestSelectModes(t *testing.T) {
	nodes := []topologyfinder.ClusterNode{
		{Hostname: "a", Port: 1, ReplicaState: "PRIMARY"},
		{Hostname: "b", Port: 2, ReplicaState: "SECONDARY"},
		{Hostname: "c", Port: 3, ReplicaState: "SECONDARY"},
		{Hostname: "d", Port: 4, ReplicaState: "ARBITER"},
	}

	all, err := Select(nodes, ModeAllNodes)
	if err != nil || len(all) != 4 {
		t.Fatalf("ModeAllNodes: got %v, %v", all, err)
	}

	sec, err := Select(nodes, ModeAllSecondaries)
	if err != nil || len(sec) != 2 {
		t.Fatalf("ModeAllSecondaries: got %v, %v", sec, err)
	}
	if sec[0].Hostname != "b" || sec[1].Hostname != "c" {
		t.Fatalf("ModeAllSecondaries order: %+v", sec)
	}

	one, err := Select(nodes, ModeOneSecondary)
	if err != nil || len(one) != 1 || one[0].Hostname != "b" {
		t.Fatalf("ModeOneSecondary: got %v, %v", one, err)
	}

	_, err = Select([]topologyfinder.ClusterNode{
		{Hostname: "x", Port: 1, ReplicaState: "PRIMARY"},
	}, ModeOneSecondary)
	if err == nil || !errors.Is(err, ErrNoSecondaries) {
		t.Fatalf("expected ErrNoSecondaries wrap: %v", err)
	}
}

func TestLooksLikeStandaloneMongod(t *testing.T) {
	if !LooksLikeStandaloneMongod([]topologyfinder.ClusterNode{{Hostname: "h", Port: 1, ReplicaState: "PRIMARY"}}) {
		t.Fatal("single primary should look like standalone")
	}
	if LooksLikeStandaloneMongod([]topologyfinder.ClusterNode{
		{ReplicaState: "PRIMARY"}, {ReplicaState: "SECONDARY"},
	}) {
		t.Fatal("two nodes is not standalone shape")
	}
	if LooksLikeStandaloneMongod([]topologyfinder.ClusterNode{{ReplicaState: "MONGOS"}}) {
		t.Fatal("mongos alone is not standalone mongod")
	}
}

func TestStandalonePrimaryTargets(t *testing.T) {
	n := []topologyfinder.ClusterNode{{Hostname: "solo", Port: 27017, ReplicaState: "PRIMARY"}}
	got := StandalonePrimaryTargets(n)
	if len(got) != 1 || got[0].Hostname != "solo" {
		t.Fatalf("%+v", got)
	}
}

func TestPromptStandaloneCollectPrimary(t *testing.T) {
	ok, err := PromptStandaloneCollectPrimary(strings.NewReader("y\n"), &bytes.Buffer{})
	if err != nil || !ok {
		t.Fatalf("%v %v", ok, err)
	}
	ok, err = PromptStandaloneCollectPrimary(strings.NewReader("no\n"), &bytes.Buffer{})
	if err != nil || ok {
		t.Fatalf("%v %v", ok, err)
	}
}

func TestSelectShardedOneSecondaryOnlyNoInfra(t *testing.T) {
	nodes := []topologyfinder.ClusterNode{
		{Hostname: "shardsec", Port: 27018, ReplicaState: "SECONDARY", ShardMapHostRole: "shard0"},
		{Hostname: "cfg1", Port: 27021, ReplicaState: "PRIMARY", ShardMapHostRole: "config"},
		{Hostname: "mongos1", Port: 27017, ReplicaState: "MONGOS"},
	}
	one, err := Select(nodes, ModeOneSecondary)
	if err != nil {
		t.Fatal(err)
	}
	if len(one) != 1 || one[0].Hostname != "shardsec" {
		t.Fatalf("ModeOneSecondary sharded: want exactly one secondary, got %+v", one)
	}
}

func TestSelectShardedAllSecondariesOneMongosOneConfig(t *testing.T) {
	// Two shard secondaries only (no config SECONDARY here, so "one config" adds cfg1 without duplicating a secondary).
	nodes := []topologyfinder.ClusterNode{
		{Hostname: "shardsec", Port: 27018, ReplicaState: "SECONDARY", ShardMapHostRole: "shard0"},
		{Hostname: "shardsec2", Port: 27118, ReplicaState: "SECONDARY", ShardMapHostRole: "shard1"},
		{Hostname: "shardpri", Port: 27019, ReplicaState: "PRIMARY", ShardMapHostRole: "shard0"},
		{Hostname: "cfg1", Port: 27021, ReplicaState: "PRIMARY", ShardMapHostRole: "config"},
		{Hostname: "cfg2", Port: 27022, ReplicaState: "SECONDARY", ShardMapHostRole: "config"},
		{Hostname: "mongos1", Port: 27017, ReplicaState: "MONGOS"},
		{Hostname: "mongos2", Port: 27027, ReplicaState: "MONGOS"},
	}
	allSec, err := Select(nodes, ModeAllSecondaries)
	if err != nil {
		t.Fatal(err)
	}
	// All secondaries = shardsec, shardsec2, cfg2 (3) + one mongos + one config primary not yet listed = cfg1 → 5, or we only add config not in set: cfg2 already in → add cfg1 → 5
	if len(allSec) != 5 {
		t.Fatalf("ModeAllSecondaries sharded: want 5 (3 secondaries + 1 mongos + 1 config primary), got %d %v", len(allSec), allSec)
	}
	got := map[string]bool{}
	for _, n := range allSec {
		got[n.Hostname] = true
	}
	if !got["shardsec"] || !got["shardsec2"] || !got["cfg2"] || got["shardpri"] {
		t.Fatalf("secondaries: %+v", allSec)
	}
	mongosCount := 0
	for _, n := range allSec {
		if n.ReplicaState == "MONGOS" {
			mongosCount++
		}
	}
	if mongosCount != 1 || !got["mongos1"] || got["mongos2"] {
		t.Fatalf("want exactly first sorted mongos: %+v", allSec)
	}
	if !got["cfg1"] {
		t.Fatalf("expected one added config host: %+v", allSec)
	}
}

func TestSelectShardedAllSecondariesExtraMongosConfigSkipped(t *testing.T) {
	nodes := []topologyfinder.ClusterNode{
		{Hostname: "shardsec", Port: 27018, ReplicaState: "SECONDARY", ShardMapHostRole: "shard0"},
		{Hostname: "shardsec2", Port: 27118, ReplicaState: "SECONDARY", ShardMapHostRole: "shard1"},
		{Hostname: "cfg1", Port: 27021, ReplicaState: "PRIMARY", ShardMapHostRole: "config"},
		{Hostname: "mongos1", Port: 27017, ReplicaState: "MONGOS"},
		{Hostname: "mongos2", Port: 27027, ReplicaState: "MONGOS"},
	}
	allSec, err := Select(nodes, ModeAllSecondaries)
	if err != nil || len(allSec) != 4 {
		t.Fatalf("want 4 (2 sec + 1 mongos + 1 cfg), got %d %v", len(allSec), allSec)
	}
	got := map[string]bool{}
	for _, n := range allSec {
		got[n.Hostname] = true
	}
	if !got["mongos1"] || got["mongos2"] || !got["cfg1"] {
		t.Fatalf("unexpected: %+v", allSec)
	}
}

func TestResolveModeFlagPrecedence(t *testing.T) {
	m, err := ResolveMode("all-nodes", true, strings.NewReader("2\n"), &bytes.Buffer{})
	if err != nil || m != ModeAllNodes {
		t.Fatalf("flag should ignore prompt stdin: got %v, %v", m, err)
	}
}

func TestResolveModeNonInteractiveDefault(t *testing.T) {
	m, err := ResolveMode("", false, strings.NewReader(""), &bytes.Buffer{})
	if err != nil || m != ModeOneSecondary {
		t.Fatalf("non-TTY default: got %v, %v", m, err)
	}
}

func TestPromptChoices(t *testing.T) {
	for input, want := range map[string]Mode{
		"\n":         ModeOneSecondary,
		"1\n":        ModeOneSecondary,
		"2\n":        ModeAllSecondaries,
		"3\n":        ModeAllNodes,
		"  2  \n":    ModeAllSecondaries,
	} {
		var buf bytes.Buffer
		m, err := Prompt(strings.NewReader(input), &buf)
		if err != nil || m != want {
			t.Fatalf("Prompt(%q) = %v, %v want %v", input, m, err, want)
		}
	}
	_, err := Prompt(strings.NewReader("9\n"), &bytes.Buffer{})
	if err == nil {
		t.Fatal("expected error for invalid choice")
	}
}
