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
	"errors"
	"fmt"
	"net"
	"sort"
	"time"

	"dcrcli/collectnodes"
	"dcrcli/topologyfinder"
)

type modePreview struct {
	ID          string     `json:"id"`
	Label       string     `json:"label"`
	Description string     `json:"description"`
	Targets     []nodeJSON `json:"targets"`
	Error       string     `json:"error,omitempty"`
	Standalone  bool       `json:"standalone,omitempty"`
}

func previewModes(nodes []topologyfinder.ClusterNode) []modePreview {
	specs := []struct {
		mode  collectnodes.Mode
		label string
	}{
		{collectnodes.ModeOneSecondary, "One secondary"},
		{collectnodes.ModeAllSecondaries, "All secondaries"},
		{collectnodes.ModeAllNodes, "All nodes"},
	}
	standalone := collectnodes.LooksLikeStandaloneMongod(nodes)
	out := make([]modePreview, 0, len(specs))
	for _, spec := range specs {
		item := modePreview{
			ID:          spec.mode.String(),
			Label:       spec.label,
			Description: spec.mode.Description(),
			Targets:     []nodeJSON{},
		}
		targets, err := collectnodes.Select(nodes, spec.mode)
		if err != nil {
			item.Error = err.Error()
			if errors.Is(err, collectnodes.ErrNoSecondaries) && standalone {
				item.Standalone = true
				item.Targets = toNodeJSON(collectnodes.StandalonePrimaryTargets(nodes))
			}
		} else {
			item.Targets = toNodeJSON(targets)
		}
		out = append(out, item)
	}
	return out
}

func resolveTargets(nodes []topologyfinder.ClusterNode, mode collectnodes.Mode, allowStandalone bool) ([]topologyfinder.ClusterNode, error) {
	targets, err := collectnodes.Select(nodes, mode)
	if err == nil {
		return targets, nil
	}
	if errors.Is(err, collectnodes.ErrNoSecondaries) &&
		allowStandalone &&
		mode != collectnodes.ModeAllNodes &&
		collectnodes.LooksLikeStandaloneMongod(nodes) {
		targets = collectnodes.StandalonePrimaryTargets(nodes)
		if len(targets) == 1 {
			return targets, nil
		}
	}
	return nil, err
}

func toNodeJSON(nodes []topologyfinder.ClusterNode) []nodeJSON {
	out := make([]nodeJSON, 0, len(nodes))
	for _, n := range nodes {
		out = append(out, nodeJSON{
			Hostname: n.Hostname,
			Port:     n.Port,
			Role:     n.ReplicaState,
			Shard:    n.ShardMapHostRole,
			State:    "pending",
		})
	}
	return out
}

func modeLabel(id string) string {
	switch id {
	case collectnodes.ModeOneSecondary.String():
		return "One secondary"
	case collectnodes.ModeAllSecondaries.String():
		return "All secondaries"
	case collectnodes.ModeAllNodes.String():
		return "All nodes"
	default:
		return id
	}
}

// unresolvedMembers lists the members discovery found and then dropped because
// none of their hostnames resolved on this machine. It is empty otherwise.
func unresolvedMembers(before, after []topologyfinder.ClusterNode) []string {
	if len(after) > 0 || len(before) == 0 {
		return nil
	}
	seen := map[string]bool{}
	names := make([]string, 0, len(before))
	for _, n := range before {
		key := fmt.Sprintf("%s:%d", n.Hostname, n.Port)
		if seen[key] {
			continue
		}
		seen[key] = true
		names = append(names, key)
	}
	sort.Strings(names)
	return names
}

// lookupHost is replaced in tests.
var lookupHost = func(ctx context.Context, host string) ([]string, error) {
	return net.DefaultResolver.LookupHost(ctx, host)
}

// unresolvableMembers lists every discovered member whose hostname does not
// resolve on this machine. Collection needs all members, so one name that only
// resolves on the cluster's own network is a problem to report, not a member to
// skip silently.
func unresolvableMembers(ctx context.Context, nodes []topologyfinder.ClusterNode) []string {
	bad := map[string]bool{}
	checked := map[string]bool{}
	for _, n := range nodes {
		if checked[n.Hostname] {
			continue
		}
		checked[n.Hostname] = true
		lctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		addrs, err := lookupHost(lctx, n.Hostname)
		cancel()
		if err != nil || len(addrs) == 0 {
			bad[n.Hostname] = true
		}
	}
	if len(bad) == 0 {
		return nil
	}
	seen := map[string]bool{}
	var names []string
	for _, n := range nodes {
		key := fmt.Sprintf("%s:%d", n.Hostname, n.Port)
		if bad[n.Hostname] && !seen[key] {
			seen[key] = true
			names = append(names, key)
		}
	}
	sort.Strings(names)
	return names
}

func validateClusterName(name string) error {
	if name == "" {
		return fmt.Errorf("cluster_name is required")
	}
	if name == "." || name == ".." {
		return fmt.Errorf("cluster_name cannot be %q", name)
	}
	if len(name) > 80 {
		return fmt.Errorf("cluster_name is too long")
	}
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case r == '-' || r == '_' || r == '.':
		default:
			return fmt.Errorf("cluster_name may contain only letters, numbers, dot, underscore, and hyphen")
		}
	}
	return nil
}
