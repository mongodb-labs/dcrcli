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

// Package collectnodes defines how many cluster members dcrcli collects from (secondaries-only vs all nodes).
package collectnodes

import (
	"errors"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

	"dcrcli/termui"
	"dcrcli/topologyfinder"
)

// Mode controls which discovered nodes are included in data collection.
type Mode int

const (
	// ModeOneSecondary collects from a single SECONDARY only (no mongos/config unless that node is a secondary).
	ModeOneSecondary Mode = iota
	// ModeAllSecondaries collects every SECONDARY; when the topology is sharded, also one mongos and one config server (deterministic by host/port sort).
	ModeAllSecondaries
	// ModeAllNodes collects every discovered node (shard primaries, secondaries, mongos, config, etc.).
	ModeAllNodes
)

func (m Mode) String() string {
	switch m {
	case ModeOneSecondary:
		return flagOneSecondary
	case ModeAllSecondaries:
		return flagAllSecondaries
	case ModeAllNodes:
		return flagAllNodes
	default:
		return "unknown"
	}
}

// Description is a short human-readable summary for stdout (prompt confirmation and progress lines).
func (m Mode) Description() string {
	switch m {
	case ModeOneSecondary:
		return "one secondary only"
	case ModeAllSecondaries:
		return "all secondaries plus one mongos and one config server when sharded"
	case ModeAllNodes:
		return "all discovered nodes (every mongod primary/secondary, all mongos, all config members)"
	default:
		return m.String()
	}
}

const (
	flagOneSecondary    = "one-secondary"
	flagAllSecondaries  = "all-secondaries"
	flagAllNodes        = "all-nodes"
)

// ErrNoSecondaries is returned by Select for secondary-only modes when no node is classified as SECONDARY.
var ErrNoSecondaries = errors.New("no SECONDARY members among discovered nodes")

// ParseMode parses --collect-nodes flag values.
func ParseMode(s string) (Mode, error) {
	switch strings.TrimSpace(strings.ToLower(s)) {
	case flagOneSecondary:
		return ModeOneSecondary, nil
	case flagAllSecondaries:
		return ModeAllSecondaries, nil
	case flagAllNodes:
		return ModeAllNodes, nil
	case "":
		return 0, errors.New("collect-nodes value is empty")
	default:
		return 0, fmt.Errorf("invalid --collect-nodes value %q (want %s, %s, or %s)", s, flagOneSecondary, flagAllSecondaries, flagAllNodes)
	}
}

// Prompt asks the user which collection scope to use.
func Prompt(stdin io.Reader, stdout io.Writer) (Mode, error) {
	return PromptUI(termui.New(stdin, stdout))
}

// PromptUI asks the user which collection scope to use with the shared terminal UI.
// Call ui.Header("Collection scope") before topology discovery when starting this section.
func PromptUI(ui *termui.UI) (Mode, error) {
	ui.Note("Which nodes should dcrcli collect diagnostic data from?")
	ui.Menu([]string{
		"One secondary only (default)",
		"All secondaries (+ one mongos and one config server when sharded)",
		"All discovered nodes (may add load and storage usage)",
	})
	ui.Blank()
	line, err := ui.AskChoice("Choice [1]")
	if err != nil {
		return 0, err
	}
	var mode Mode
	var choice string
	switch {
	case line == "" || line == "1":
		mode = ModeOneSecondary
		choice = "1"
	case line == "2":
		mode = ModeAllSecondaries
		choice = "2"
	case line == "3":
		mode = ModeAllNodes
		choice = "3"
	default:
		return 0, fmt.Errorf("invalid choice %q: enter 1, 2, or 3", line)
	}
	ui.Blank()
	ui.Ok(fmt.Sprintf("Selected option %s — %s", choice, mode.Description()))
	ui.Blank()
	return mode, nil
}

// LooksLikeStandaloneMongod reports a single discovered data mongod (typical standalone: one host, not mongos/arbiter).
func LooksLikeStandaloneMongod(nodes []topologyfinder.ClusterNode) bool {
	if len(nodes) != 1 {
		return false
	}
	r := strings.ToUpper(strings.TrimSpace(nodes[0].ReplicaState))
	switch r {
	case "MONGOS", "ARBITER":
		return false
	default:
		return true
	}
}

// StandalonePrimaryTargets returns the lone node for collection after the user opts into primary on standalone.
func StandalonePrimaryTargets(nodes []topologyfinder.ClusterNode) []topologyfinder.ClusterNode {
	if len(nodes) != 1 {
		return nil
	}
	return []topologyfinder.ClusterNode{nodes[0]}
}

// PromptStandaloneCollectPrimary asks whether to collect from the standalone primary (y/N).
func PromptStandaloneCollectPrimary(stdin io.Reader, stdout io.Writer) (bool, error) {
	ui := termui.New(stdin, stdout)
	ui.Warn("A single MongoDB node was discovered and it is not a secondary (typical standalone).")
	ui.Note(
		"Options 1 and 2 normally avoid primaries; standalone has no secondary to collect from.",
	)
	return ui.AskYesNo("Collect from this primary (standalone) anyway?", true)
}

// ResolveMode returns the collection mode. Non-empty flagValue wins; otherwise on a TTY Prompt is used;
// non-interactive stdin defaults to ModeOneSecondary without prompting.
func ResolveMode(flagValue string, isTerminal bool, ui *termui.UI) (Mode, error) {
	if strings.TrimSpace(flagValue) != "" {
		return ParseMode(flagValue)
	}
	if isTerminal {
		return PromptUI(ui)
	}
	return ModeOneSecondary, nil
}

func nodeKey(n topologyfinder.ClusterNode) string {
	return strings.ToLower(strings.TrimSpace(n.Hostname)) + ":" + strconv.Itoa(n.Port)
}

// shardedTopologyDiscovery is true when getShardMap populated roles or any mongos appears in the node list.
func shardedTopologyDiscovery(nodes []topologyfinder.ClusterNode) bool {
	for _, n := range nodes {
		if strings.TrimSpace(n.ShardMapHostRole) != "" {
			return true
		}
		if strings.EqualFold(n.ReplicaState, "MONGOS") {
			return true
		}
	}
	return false
}

func isConfigServerFromShardMap(n topologyfinder.ClusterNode) bool {
	return strings.EqualFold(strings.TrimSpace(n.ShardMapHostRole), "config")
}

func sortNodesByHostPort(nodes []topologyfinder.ClusterNode) {
	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].Hostname != nodes[j].Hostname {
			return nodes[i].Hostname < nodes[j].Hostname
		}
		return nodes[i].Port < nodes[j].Port
	})
}

// appendShardedInfraOneEach adds at most one mongos and one config-server mongod (getShardMap role "config"),
// not already in targets, when topology looks sharded. Host/port sort picks which one if several exist.
func appendShardedInfraOneEach(all []topologyfinder.ClusterNode, targets []topologyfinder.ClusterNode) []topologyfinder.ClusterNode {
	if !shardedTopologyDiscovery(all) {
		return targets
	}
	seen := make(map[string]bool, len(targets)+len(all))
	for _, n := range targets {
		seen[nodeKey(n)] = true
	}
	var mongosCandidates, configCandidates []topologyfinder.ClusterNode
	for _, n := range all {
		if seen[nodeKey(n)] {
			continue
		}
		if strings.EqualFold(n.ReplicaState, "MONGOS") {
			mongosCandidates = append(mongosCandidates, n)
		}
		if isConfigServerFromShardMap(n) {
			configCandidates = append(configCandidates, n)
		}
	}
	sortNodesByHostPort(mongosCandidates)
	sortNodesByHostPort(configCandidates)

	out := append([]topologyfinder.ClusterNode(nil), targets...)
	if len(mongosCandidates) > 0 {
		out = append(out, mongosCandidates[0])
		seen[nodeKey(mongosCandidates[0])] = true
	}
	if len(configCandidates) > 0 {
		k := nodeKey(configCandidates[0])
		if !seen[k] {
			out = append(out, configCandidates[0])
		}
	}
	sortNodesByHostPort(out)
	return out
}

// Select returns the subset of nodes to collect, or an error if the mode cannot be satisfied.
func Select(nodes []topologyfinder.ClusterNode, mode Mode) ([]topologyfinder.ClusterNode, error) {
	switch mode {
	case ModeAllNodes:
		out := make([]topologyfinder.ClusterNode, len(nodes))
		copy(out, nodes)
		return out, nil
	case ModeOneSecondary, ModeAllSecondaries:
		var secondaries []topologyfinder.ClusterNode
		for _, n := range nodes {
			if strings.EqualFold(n.ReplicaState, "SECONDARY") {
				secondaries = append(secondaries, n)
			}
		}
		if len(secondaries) == 0 {
			return nil, fmt.Errorf("%w; use --collect-nodes=%s to include primaries, or confirm standalone interactively", ErrNoSecondaries, flagAllNodes)
		}
		sortNodesByHostPort(secondaries)
		if mode == ModeOneSecondary {
			return []topologyfinder.ClusterNode{secondaries[0]}, nil
		}
		targets := append([]topologyfinder.ClusterNode(nil), secondaries...)
		targets = appendShardedInfraOneEach(nodes, targets)
		return targets, nil
	default:
		return nil, fmt.Errorf("unknown collect mode")
	}
}
