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

package topologyfinder

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

type rsMemberRow struct {
	Name     string `json:"name"`
	StateStr string `json:"stateStr"`
}

type rsStatusErr struct {
	Error   bool   `json:"error"`
	Message string `json:"message"`
}

// decodeFirstJSONValue reads one JSON value from b (handles extra trailing noise from the shell).
func decodeFirstJSONValue(b []byte) (json.RawMessage, error) {
	dec := json.NewDecoder(bytes.NewReader(bytes.TrimSpace(b)))
	dec.UseNumber()
	var raw json.RawMessage
	if err := dec.Decode(&raw); err != nil {
		return nil, err
	}
	return raw, nil
}

func parseRsStatusMembersOutput(b []byte) ([]rsMemberRow, bool) {
	raw, err := decodeFirstJSONValue(b)
	if err != nil {
		return nil, false
	}
	if len(raw) > 0 && raw[0] == '{' {
		var errObj rsStatusErr
		if json.Unmarshal(raw, &errObj) == nil && errObj.Error {
			return nil, true
		}
		return nil, false
	}
	var rows []rsMemberRow
	if json.Unmarshal(raw, &rows) != nil {
		return nil, false
	}
	return rows, false
}

func memberRowMatchesNode(memberName string, n ClusterNode, tf *TopologyFinder) bool {
	h, p, err := splitHostPort(memberName, tf.Dcrlog)
	if err != nil {
		return false
	}
	if p != n.Port {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(h), strings.TrimSpace(n.Hostname))
}

func replicaStateFromRsRow(stateStr string) string {
	return strings.ToUpper(strings.TrimSpace(stateStr))
}

// classifyHelloJSON inspects a hello / hello-like document and returns ReplicaState constants.
func classifyHelloJSON(b []byte) string {
	raw, err := decodeFirstJSONValue(b)
	if err != nil {
		return "UNKNOWN"
	}
	var m map[string]interface{}
	if json.Unmarshal(raw, &m) != nil {
		return "UNKNOWN"
	}
	if msg, ok := m["msg"].(string); ok && strings.EqualFold(msg, "isdbgrid") {
		return "MONGOS"
	}
	if truthy(m["arbiterOnly"]) {
		return "ARBITER"
	}
	if truthy(m["isWritablePrimary"]) {
		return "PRIMARY"
	}
	if truthy(m["secondary"]) {
		return "SECONDARY"
	}
	return "UNKNOWN"
}

func truthy(v interface{}) bool {
	switch t := v.(type) {
	case bool:
		return t
	case json.Number:
		i, err := t.Int64()
		return err == nil && i != 0
	case float64:
		return t != 0
	case string:
		return strings.EqualFold(t, "true") || t == "1"
	default:
		return false
	}
}

// ResolveReplicaStates fills ReplicaState on each node: rs.status from the seed URI first, then
// per-node hello for nodes still without a state. Seed Mongo URI is restored before return.
func (tf *TopologyFinder) ResolveReplicaStates() error {
	if tf.MongoshCapture.S == nil || tf.Dcrlog == nil {
		return fmt.Errorf("topologyfinder: MongoshCapture.S and Dcrlog must be set")
	}
	s := tf.MongoshCapture.S
	seedH, seedP := s.Seedmongodhost, s.Seedmongodport
	defer func() {
		s.Currentmongodhost = seedH
		s.Currentmongodport = seedP
		_ = s.SetMongoURI()
	}()

	s.Currentmongodhost = seedH
	s.Currentmongodport = seedP
	if err := s.SetMongoURI(); err != nil {
		return err
	}

	var rows []rsMemberRow
	if err := tf.MongoshCapture.RunRsStatusMembersWithEval(); err != nil {
		tf.Dcrlog.Debug(fmt.Sprintf("tftf - rs.status members eval failed, falling back to hello: %v", err))
	} else {
		out := tf.MongoshCapture.Getparsedjsonoutput.Bytes()
		var scriptErr bool
		rows, scriptErr = parseRsStatusMembersOutput(out)
		if scriptErr {
			tf.Dcrlog.Debug("tftf - rs.status script reported error (not a repl set from this connection)")
			rows = nil
		}
	}

	for i := range tf.Allnodes.Nodes {
		for _, row := range rows {
			if memberRowMatchesNode(row.Name, tf.Allnodes.Nodes[i], tf) {
				tf.Allnodes.Nodes[i].ReplicaState = replicaStateFromRsRow(row.StateStr)
				break
			}
		}
	}

	for i := range tf.Allnodes.Nodes {
		if tf.Allnodes.Nodes[i].ReplicaState != "" {
			continue
		}
		s.Currentmongodhost = tf.Allnodes.Nodes[i].Hostname
		s.Currentmongodport = fmt.Sprintf("%d", tf.Allnodes.Nodes[i].Port)
		if err := s.SetMongoURI(); err != nil {
			tf.Allnodes.Nodes[i].ReplicaState = "UNKNOWN"
			continue
		}
		if err := tf.MongoshCapture.RunHelloFullWithEval(); err != nil {
			tf.Dcrlog.Debug(
				fmt.Sprintf(
					"tftf - hello for %s:%d failed: %v",
					tf.Allnodes.Nodes[i].Hostname,
					tf.Allnodes.Nodes[i].Port,
					err,
				),
			)
			tf.Allnodes.Nodes[i].ReplicaState = "UNKNOWN"
			continue
		}
		tf.Allnodes.Nodes[i].ReplicaState = classifyHelloJSON(tf.MongoshCapture.Getparsedjsonoutput.Bytes())
	}

	return nil
}
