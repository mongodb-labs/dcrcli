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
	"testing"
)

// THESE are tests with mongo shell output
func TestIsParseShardMapWithValidShardMap(t *testing.T) {
	clustertopology := TopologyFinder{}

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
	clustertopology := TopologyFinder{}

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
	clustertopology := TopologyFinder{}

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
	clustertopology := TopologyFinder{}

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
	clustertopology := TopologyFinder{}

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
	clustertopology := TopologyFinder{}
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
	clustertopology := TopologyFinder{}

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
