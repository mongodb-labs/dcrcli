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
	"testing"

	"dcrcli/dcrlogger"
)

func TestParseRsStatusMembersOutput(t *testing.T) {
	rows, scriptErr := parseRsStatusMembersOutput([]byte(`[
	  {"name": "host1:27017", "stateStr": "PRIMARY"},
	  {"name": "host2:27017", "stateStr": "SECONDARY"}
	]`))
	if scriptErr || len(rows) != 2 {
		t.Fatalf("array: rows=%v scriptErr=%v", rows, scriptErr)
	}
	rows, scriptErr = parseRsStatusMembersOutput([]byte(`{"error":true,"message":"no replset"}`))
	if !scriptErr || rows != nil {
		t.Fatalf("error object: rows=%v scriptErr=%v", rows, scriptErr)
	}
}

func TestClassifyHelloJSON(t *testing.T) {
	if g := classifyHelloJSON([]byte(`{"secondary": true, "ok": 1}`)); g != "SECONDARY" {
		t.Fatalf("secondary: %s", g)
	}
	if g := classifyHelloJSON([]byte(`{"isWritablePrimary": true}`)); g != "PRIMARY" {
		t.Fatalf("primary: %s", g)
	}
	if g := classifyHelloJSON([]byte(`{"msg": "isdbgrid"}`)); g != "MONGOS" {
		t.Fatalf("mongos: %s", g)
	}
	if g := classifyHelloJSON([]byte(`{"arbiterOnly": true}`)); g != "ARBITER" {
		t.Fatalf("arbiter: %s", g)
	}
}

func TestMemberRowMatchesNode(t *testing.T) {
	log := dcrlogger.DCRLogger{OutputPrefix: t.TempDir() + "/", FileName: "noderoles_test"}
	if err := log.Create(); err != nil {
		t.Fatal(err)
	}
	tf := TopologyFinder{Dcrlog: &log}
	n := ClusterNode{Hostname: "HOST.example", Port: 27017}
	if !memberRowMatchesNode("host.example:27017", n, &tf) {
		t.Fatal("expected case-insensitive host match")
	}
	if memberRowMatchesNode("host.example:27018", n, &tf) {
		t.Fatal("port must match")
	}
}
