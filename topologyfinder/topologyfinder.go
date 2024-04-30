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
	"log"
	"regexp"
	"strconv"
	"strings"

	"dcrcli/mongosh"
)

type ClusterNode struct {
	Hostname string
	Port     int
}

type ClusterNodes struct {
	Nodes []ClusterNode
}

// TOPOLOGY FInder:
// - Can find nodes that are part of ReplicaSet
// - Can find nodes that are "not hidden", passives and arbiters
// - Returns the hostname information that mongod has - could be PRIVATE hostnames as well!!!!

type TopologyFinder struct {
	Allnodes          ClusterNodes
	GetShardMapOutput bytes.Buffer
	MongoshCapture    mongosh.CaptureGetMongoData
	GetHelloOutput    bytes.Buffer
}

func (tf *TopologyFinder) removeBsonFields() error {
	re := regexp.MustCompile(
		`(Timestamp|NumberLong|BinData|Binary\.createFromBase64|Long)\([^\)]*\)`,
	)

	replaced := re.ReplaceAll(tf.GetShardMapOutput.Bytes(), []byte(`"dummy"`))

	tf.GetShardMapOutput.Reset()
	tf.GetShardMapOutput.Write(replaced)

	return nil
}

func (tf *TopologyFinder) isShardMap() bool {
	var tempshardmapoutput map[string]interface{}

	if err := json.Unmarshal(tf.GetShardMapOutput.Bytes(), &tempshardmapoutput); err != nil {
		return false
	}

	hostsMap, ok := tempshardmapoutput["hosts"].(map[string]interface{})
	if !ok {
		return false
	}

	for hoststring, shardtype := range hostsMap {
		if hoststring == "" || shardtype == "" || len(strings.Split(hoststring, ":")) != 2 {
			return false
		}
	}

	return true
}

func (tf *TopologyFinder) parseHelloOutput() error {
	var hostsArray []string

	if err := json.Unmarshal(tf.GetHelloOutput.Bytes(), &hostsArray); err != nil {
		log.Fatal(err)
	}

	for _, mongonodestring := range hostsArray {

		mongonodeslice := strings.Split(mongonodestring, ":")
		if len(mongonodeslice) != 2 {
			log.Fatalf("Invalid mongo node string: %s", mongonodeslice)
		}

		hostname := mongonodeslice[0]
		portStr := mongonodeslice[1]

		port, err := strconv.Atoi(portStr)
		if err != nil {
			log.Fatalf(
				"In parseHelloOutput: Invalid port string format for node %s: %s ",
				mongonodestring,
				portStr,
			)
		}

		mongonode := ClusterNode{
			Hostname: hostname,
			Port:     port,
		}

		tf.Allnodes.Nodes = append(tf.Allnodes.Nodes, mongonode)

	}
	return nil
}

func (tf *TopologyFinder) parseShardMapOutput() error {
	var shardMap map[string]interface{}

	if err := json.Unmarshal(tf.GetShardMapOutput.Bytes(), &shardMap); err != nil {
		log.Fatal(err)
	}

	allhosts, ok := shardMap["hosts"].(map[string]interface{})
	if !ok {
		log.Fatalf("error reading sharmap hosts document")
	}

	for mongonodestring := range allhosts {

		mongonodeslice := strings.Split(mongonodestring, ":")
		if len(mongonodeslice) != 2 {
			log.Fatalf("Invalid mongo node string: %s", mongonodeslice)
		}

		hostname := mongonodeslice[0]
		portStr := mongonodeslice[1]

		port, err := strconv.Atoi(portStr)
		if err != nil {
			log.Fatalf("Invalid port string format for node %s: %s ", mongonodestring, portStr)
		}

		mongonode := ClusterNode{
			Hostname: hostname,
			Port:     port,
		}

		tf.Allnodes.Nodes = append(tf.Allnodes.Nodes, mongonode)

	}

	return nil
}

func (tf *TopologyFinder) addSeedMongosNode() error {
	seedport := tf.MongoshCapture.S.Seedmongodport
	seedhostname := tf.MongoshCapture.S.Seedmongodhost
	isSeedHostinList := false

	for _, host := range tf.Allnodes.Nodes {
		if seedhostname == string(host.Hostname) &&
			seedport == strconv.Itoa(host.Port) {
			isSeedHostinList = true
		}
	}

	if !isSeedHostinList {
		err := tf.addSeedNode()
		if err != nil {
			return err
		}
	}
	return nil
}

func (tf *TopologyFinder) GetAllNodes() error {
	err := tf.runShardMapDBCommand()
	if err != nil {
		return err
	}

	tf.GetShardMapOutput = *tf.MongoshCapture.Getparsedjsonoutput

	if tf.isShardMap() {
		err := tf.parseShardMapOutput()
		if err != nil {
			return err
		}
		err = tf.addSeedMongosNode()
		if err != nil {
			return err
		}
		return nil
	}

	err = tf.useHelloDBCommandHostsArray()
	if err != nil {
		return err
	}
	return nil
}

func (tf *TopologyFinder) addSeedNode() error {
	seedport, err := strconv.Atoi(tf.MongoshCapture.S.Seedmongodport)
	if err != nil {
		return err
	}
	mongonode := ClusterNode{}
	mongonode.Hostname = tf.MongoshCapture.S.Seedmongodhost
	mongonode.Port = seedport

	tf.Allnodes.Nodes = append(tf.Allnodes.Nodes, mongonode)

	return nil
}

func (tf *TopologyFinder) useHelloDBCommandHostsArray() error {
	err := tf.runHello()
	if err != nil {
		return err
	}

	tf.GetHelloOutput = *tf.MongoshCapture.Getparsedjsonoutput

	if tf.GetHelloOutput.Len() == 0 {

		err = tf.addSeedNode()
		if err != nil {
			return err
		}

		return nil
	}
	err = tf.parseHelloOutput()
	if err != nil {
		return err
	}

	return nil
}

func (tf *TopologyFinder) runHello() error {
	// empty the buffer for the next command
	tf.MongoshCapture.Getparsedjsonoutput.Reset()

	err := tf.MongoshCapture.RunHelloDBCommandWithEval()
	if err != nil {
		return err
	}
	return nil
}

func (tf *TopologyFinder) runShardMapDBCommand() error {
	err := tf.MongoshCapture.RunGetShardMapWithEval()
	if err != nil {
		if tf.MongoshCapture.CurrentBin == "mongo" {
			return err
		}
	}
	return nil
}
