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
	"fmt"
	"net"
	"strings"
	"testing"

	"dcrcli/dcrlogger"
)

func testLogger(t *testing.T) *dcrlogger.DCRLogger {
	t.Helper()
	log := dcrlogger.DCRLogger{OutputPrefix: t.TempDir() + "/", FileName: "topologyfinder_test"}
	if err := log.Create(); err != nil {
		t.Fatal(err)
	}
	return &log
}

// THESE are tests with mongo shell output
func TestIsParseShardMapWithValidShardMap(t *testing.T) {
	clustertopology := TopologyFinder{Dcrlog: testLogger(t)}

	// sample getShardMap output from documentation
	clustertopology.GetShardMapOutput.WriteString(
		`{
  "map": {
    "shard01": "shard01/localhost:27018,localhost:27019,localhost:27020",
    "config": "configRepl/localhost:27021"
  },
  "hosts": {
    "localhost:27020": "shard01",
    "localhost:27019": "shard01",
    "localhost:27018": "shard01",
    "localhost:27021": "config"
  },
  "connStrings": {
    "shard01/localhost:27018": "shard01",
    "shard01/localhost:27018,localhost:27019,localhost:27020": "shard01",
    "shard01/localhost:27018,localhost:27020": "shard01",
    "configRepl/localhost:27021": "config"
  },
  "ok": 1,
  "$clusterTime": {
    "clusterTime": Timestamp(1711095775, 2),
    "signature": {
      "hash": BinData(0, "AAAAAAAAAAAAAAAAAAAAAAAAAAA="),
      "keyId": NumberLong("0")
    }
  },
  "operationTime": Timestamp(1711095775, 2)
}`)

	clustertopology.removeBsonFields()

	isShardMapOutput := clustertopology.isShardMap()
	if !isShardMapOutput {
		t.Error(
			"Error testing the Valid ShardMap output not recognised by isShardMap() wanted true got: ",
			isShardMapOutput,
		)
	}

	clustertopology.parseShardMapOutput()

	for _, host := range clustertopology.Allnodes.Nodes {
		fmt.Printf("host: %s, port: %d\n", host.Hostname, host.Port)
	}
}

func TestIsShardMapWithValidString(t *testing.T) {
	clustertopology := TopologyFinder{Dcrlog: testLogger(t)}

	// sample getShardMap output from documentation
	clustertopology.GetShardMapOutput.WriteString(
		`{
  "map": {
    "shard01": "shard01/localhost:27018,localhost:27019,localhost:27020",
    "config": "configRepl/localhost:27021"
  },
  "hosts": {
    "localhost:27020": "shard01",
    "localhost:27019": "shard01",
    "localhost:27018": "shard01",
    "localhost:27021": "config"
  },
  "connStrings": {
    "shard01/localhost:27018": "shard01",
    "shard01/localhost:27018,localhost:27019,localhost:27020": "shard01",
    "shard01/localhost:27018,localhost:27020": "shard01",
    "configRepl/localhost:27021": "config"
  },
  "ok": 1,
  "$clusterTime": {
    "clusterTime": Timestamp(1711095775, 2),
    "signature": {
      "hash": BinData(0, "AAAAAAAAAAAAAAAAAAAAAAAAAAA="),
      "keyId": NumberLong("0")
    }
  },
  "operationTime": Timestamp(1711095775, 2)
}`)

	clustertopology.removeBsonFields()
	isShardMapOutput := clustertopology.isShardMap()
	if !isShardMapOutput {
		t.Error(
			"Error testing the Valid ShardMap output not recognised by isShardMap() wanted true got: ",
			isShardMapOutput,
		)
	}
}

func TestIsShardMapWithInvalidJSON(t *testing.T) {
	clustertopology := TopologyFinder{Dcrlog: testLogger(t)}

	// sample getShardMap output from documentation
	clustertopology.GetShardMapOutput.WriteString(
		`not a json`)

	clustertopology.removeBsonFields()

	isShardMapOutput := clustertopology.isShardMap()
	if isShardMapOutput {
		t.Error(
			"Error testing the ShardMap with invalid JSON wanted false got: ",
			isShardMapOutput,
		)
	}
}

func TestIsShardMapWithReplicaSetOutput(t *testing.T) {
	clustertopology := TopologyFinder{Dcrlog: testLogger(t)}

	// sample getShardMap output from documentation
	clustertopology.GetShardMapOutput.WriteString(
		`{
        "ok" : 0,
        "errmsg" : "Sharding is not enabled",
        "code" : 203,
        "codeName" : "ShardingStateNotInitialized",
        "$clusterTime" : {
                "clusterTime" : Timestamp(1711100115, 1),
                "signature" : {
                        "hash" : BinData(0,"AAAAAAAAAAAAAAAAAAAAAAAAAAA="),
                        "keyId" : NumberLong(0)
                }
        },
        "operationTime" : Timestamp(1711100115, 1)
}`)

	clustertopology.removeBsonFields()

	isShardMapOutput := clustertopology.isShardMap()
	if isShardMapOutput {
		t.Error(
			"Error testing the ShardMap without hosts wanted false got: ",
			isShardMapOutput,
		)
	}
}

// THESE are tests with mongosh shell output with --json=canonical
func TestIsParseShardMapWithValidShardMapMongoSHOutput(t *testing.T) {
	clustertopology := TopologyFinder{Dcrlog: testLogger(t)}

	// sample getShardMap output from documentation
	clustertopology.GetShardMapOutput.WriteString(
		`{
  "map": {
    "shard01": "shard01/localhost:27018,localhost:27019,localhost:27020",
    "config": "configRepl/localhost:27021"
  },
  "hosts": {
    "localhost:27018": "shard01",
    "localhost:27019": "shard01",
    "localhost:27020": "shard01",
    "localhost:27021": "config"
  },
  "connStrings": {
    "shard01/localhost:27018": "shard01",
    "shard01/localhost:27018,localhost:27019": "shard01",
    "shard01/localhost:27018,localhost:27019,localhost:27020": "shard01",
    "configRepl/localhost:27021": "config"
  },
  "ok": {
    "$numberInt": "1"
  },
  "$clusterTime": {
    "clusterTime": {
      "$timestamp": {
        "t": 1711105748,
        "i": 1
      }
    },
    "signature": {
      "hash": {
        "$binary": {
          "base64": "AAAAAAAAAAAAAAAAAAAAAAAAAAA=",
          "subType": "00"
        }
      },
      "keyId": {
        "$numberLong": "0"
      }
    }
  },
  "operationTime": {
    "$timestamp": {
      "t": 1711105748,
      "i": 1
    }
  }
    }`)

	clustertopology.removeBsonFields()

	isShardMapOutput := clustertopology.isShardMap()
	if !isShardMapOutput {
		t.Error(
			"Error testing the Valid ShardMap output not recognised by isShardMap() wanted true got: ",
			isShardMapOutput,
		)
	}

	clustertopology.parseShardMapOutput()

	for _, host := range clustertopology.Allnodes.Nodes {
		fmt.Printf("host: %s, port: %d\n", host.Hostname, host.Port)
	}
}

func TestIsShardMapWithValidStringMongoSHOutput(t *testing.T) {
	clustertopology := TopologyFinder{Dcrlog: testLogger(t)}
	// sample getShardMap output from documentation
	clustertopology.GetShardMapOutput.WriteString(
		`{
  "map": {
    "shard01": "shard01/localhost:27018,localhost:27019,localhost:27020",
    "config": "configRepl/localhost:27021"
  },
  "hosts": {
    "localhost:27018": "shard01",
    "localhost:27019": "shard01",
    "localhost:27020": "shard01",
    "localhost:27021": "config"
  },
  "connStrings": {
    "shard01/localhost:27018": "shard01",
    "shard01/localhost:27018,localhost:27019": "shard01",
    "shard01/localhost:27018,localhost:27019,localhost:27020": "shard01",
    "configRepl/localhost:27021": "config"
  },
  "ok": {
    "$numberInt": "1"
  },
  "$clusterTime": {
    "clusterTime": {
      "$timestamp": {
        "t": 1711105748,
        "i": 1
      }
    },
    "signature": {
      "hash": {
        "$binary": {
          "base64": "AAAAAAAAAAAAAAAAAAAAAAAAAAA=",
          "subType": "00"
        }
      },
      "keyId": {
        "$numberLong": "0"
      }
    }
  },
  "operationTime": {
    "$timestamp": {
      "t": 1711105748,
      "i": 1
    }
  }
    }`)

	clustertopology.removeBsonFields()
	isShardMapOutput := clustertopology.isShardMap()
	if !isShardMapOutput {
		t.Error(
			"Error testing the Valid ShardMap output not recognised by isShardMap() wanted true got: ",
			isShardMapOutput,
		)
	}
}

func TestIsShardMapWithReplicaSetOutputMongoSHOutput(t *testing.T) {
	clustertopology := TopologyFinder{Dcrlog: testLogger(t)}

	// sample getShardMap output from documentation
	clustertopology.GetShardMapOutput.WriteString(
		`{
  "ok": {
    "$numberInt": "0"
  },
  "code": {
    "$numberInt": "203"
  },
  "codeName": "ShardingStateNotInitialized",
  "$clusterTime": {
    "clusterTime": {
      "$timestamp": {
        "t": 1711105584,
        "i": 1
      }
    },
    "signature": {
      "hash": {
        "$binary": {
          "base64": "AAAAAAAAAAAAAAAAAAAAAAAAAAA=",
          "subType": "00"
        }
      },
      "keyId": {
        "$numberLong": "0"
      }
    }
  },
  "operationTime": {
    "$timestamp": {
      "t": 1711105584,
      "i": 1
    }
  },
  "message": "Sharding is not enabled",
  "stack": "MongoServerError: Sharding is not enabled\n    at Connection.onMessage (/opt/homebrew/Cellar/mongosh/2.1.1/libexec/lib/node_modules/@mongosh/cli-repl/node_modules/mongodb/lib/cmap/connection.js:205:26)\n    at MessageStream.<anonymous> (/opt/homebrew/Cellar/mongosh/2.1.1/libexec/lib/node_modules/@mongosh/cli-repl/node_modules/mongodb/lib/cmap/connection.js:64:60)\n    at MessageStream.emit (node:events:519:28)\n    at MessageStream.emit (node:domain:488:12)\n    at processIncomingData (/opt/homebrew/Cellar/mongosh/2.1.1/libexec/lib/node_modules/@mongosh/cli-repl/node_modules/mongodb/lib/cmap/message_stream.js:117:16)\n    at MessageStream._write (/opt/homebrew/Cellar/mongosh/2.1.1/libexec/lib/node_modules/@mongosh/cli-repl/node_modules/mongodb/lib/cmap/message_stream.js:33:9)\n    at writeOrBuffer (node:internal/streams/writable:564:12)\n    at _write (node:internal/streams/writable:493:10)\n    at Writable.write (node:internal/streams/writable:502:10)\n    at Socket.ondata (node:internal/streams/readable:1007:22)\n    at Socket.emit (node:events:519:28)\n    at Socket.emit (node:domain:488:12)\n    at addChunk (node:internal/streams/readable:559:12)\n    at readableAddChunkPushByteMode (node:internal/streams/readable:510:3)\n    at Readable.push (node:internal/streams/readable:390:5)\n    at TCP.onStreamRead (node:internal/stream_base_commons:190:23)\n    at TCP.callbackTrampoline (node:internal/async_hooks:130:17)",
  "name": "MongoServerError"
}`)

	clustertopology.removeBsonFields()

	isShardMapOutput := clustertopology.isShardMap()
	if isShardMapOutput {
		t.Error(
			"Error testing the ShardMap without hosts wanted false got: ",
			isShardMapOutput,
		)
	}
}

// These are tests with mongosh shell

func fakeLookup(t *testing.T, table map[string][]string) {
	t.Helper()
	old := lookupIP
	t.Cleanup(func() { lookupIP = old })
	lookupIP = func(host string) ([]net.IP, error) {
		addrs, ok := table[host]
		if !ok {
			return nil, fmt.Errorf("no such host")
		}
		ips := make([]net.IP, 0, len(addrs))
		for _, a := range addrs {
			ips = append(ips, net.ParseIP(a))
		}
		return ips, nil
	}
}

func hostPorts(nodes []ClusterNode) string {
	out := make([]string, 0, len(nodes))
	for _, n := range nodes {
		out = append(out, fmt.Sprintf("%s:%d", n.Hostname, n.Port))
	}
	return strings.Join(out, ",")
}

func TestKeepUniqueNodesListsAMultiAddressHostOnce(t *testing.T) {
	// A name with two IPv4 addresses must not become two nodes: that would
	// collect the same member twice into the same output folder.
	fakeLookup(t, map[string][]string{
		"a.example": {"10.0.0.1", "10.0.0.2"},
		"b.example": {"10.0.0.3"},
	})
	tf := TopologyFinder{Dcrlog: testLogger(t)}
	tf.Allnodes.Nodes = []ClusterNode{
		{Hostname: "a.example", Port: 27017},
		{Hostname: "b.example", Port: 27017},
	}
	if err := tf.KeepUniqueNodes(); err != nil {
		t.Fatal(err)
	}
	if got := hostPorts(tf.Allnodes.Nodes); got != "a.example:27017,b.example:27017" {
		t.Fatalf("nodes = %s", got)
	}
}

func TestKeepUniqueNodesMergesNamesForOneAddress(t *testing.T) {
	fakeLookup(t, map[string][]string{
		"public.example":  {"10.0.0.1"},
		"private.example": {"10.0.0.1"},
	})
	tf := TopologyFinder{Dcrlog: testLogger(t)}
	tf.Allnodes.Nodes = []ClusterNode{
		{Hostname: "public.example", Port: 27017},
		{Hostname: "private.example", Port: 27017, ShardMapHostRole: "config"},
	}
	if err := tf.KeepUniqueNodes(); err != nil {
		t.Fatal(err)
	}
	if len(tf.Allnodes.Nodes) != 1 {
		t.Fatalf("nodes = %s", hostPorts(tf.Allnodes.Nodes))
	}
	if tf.Allnodes.Nodes[0].ShardMapHostRole != "config" {
		t.Fatalf("role lost: %+v", tf.Allnodes.Nodes[0])
	}
}

func TestKeepUniqueNodesKeepsMembersThatDoNotResolve(t *testing.T) {
	// A member this machine cannot resolve is still a member. Dropping it would
	// let the health check pass and collection run on a degraded cluster.
	fakeLookup(t, map[string][]string{
		"a.example":  {"10.0.0.1"},
		"v6only":     {"fd00::1"},
		"b.example":  {"10.0.0.2"},
		"alias.of.b": {"10.0.0.2"},
	})
	tf := TopologyFinder{Dcrlog: testLogger(t)}
	tf.Allnodes.Nodes = []ClusterNode{
		{Hostname: "a.example", Port: 27017},
		{Hostname: "gone.internal", Port: 27017, ShardMapHostRole: "shard01"},
		{Hostname: "v6only", Port: 27018},
		{Hostname: "b.example", Port: 27017},
		{Hostname: "alias.of.b", Port: 27017},
	}
	if err := tf.KeepUniqueNodes(); err != nil {
		t.Fatal(err)
	}
	got := hostPorts(tf.Allnodes.Nodes)
	for _, want := range []string{"a.example:27017", "b.example:27017", "gone.internal:27017", "v6only:27018"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %s in %s", want, got)
		}
	}
	if strings.Contains(got, "alias.of.b") {
		t.Fatalf("alias of an already listed node was kept: %s", got)
	}
	if len(tf.Allnodes.Nodes) != 4 {
		t.Fatalf("nodes = %s", got)
	}
	for _, n := range tf.Allnodes.Nodes {
		if n.Hostname == "gone.internal" && n.ShardMapHostRole != "shard01" {
			t.Fatalf("unresolved member lost its role: %+v", n)
		}
	}
}

func TestKeepUniqueNodesOrderIsStable(t *testing.T) {
	fakeLookup(t, map[string][]string{
		"c": {"10.0.0.3"}, "a": {"10.0.0.1"}, "b": {"10.0.0.2"},
	})
	var first string
	for i := 0; i < 20; i++ {
		tf := TopologyFinder{Dcrlog: testLogger(t)}
		tf.Allnodes.Nodes = []ClusterNode{{Hostname: "c", Port: 1}, {Hostname: "a", Port: 1}, {Hostname: "b", Port: 1}}
		if err := tf.KeepUniqueNodes(); err != nil {
			t.Fatal(err)
		}
		got := hostPorts(tf.Allnodes.Nodes)
		if i == 0 {
			first = got
		} else if got != first {
			t.Fatalf("order changed between runs: %s vs %s", first, got)
		}
	}
}
