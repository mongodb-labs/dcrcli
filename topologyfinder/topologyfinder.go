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
	"log"
	"regexp"
	"strconv"
	"strings"

	"dcrcli/dcrlogger"
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
// - If multiple hostnames point to same IP address only the unique IP is returned

type TopologyFinder struct {
	Allnodes          ClusterNodes
	GetShardMapOutput bytes.Buffer
	MongoshCapture    mongosh.CaptureGetMongoData
	GetHelloOutput    bytes.Buffer
	Dcrlog            *dcrlogger.DCRLogger
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
		log.Fatalf("tftf - error parsing hello output during topology discovery: %s", err)
	}

	for _, mongonodestring := range hostsArray {

		mongonodeslice := strings.Split(mongonodestring, ":")
		if len(mongonodeslice) != 2 {
			log.Fatalf("tftf - invalid mongo node string: %s", mongonodeslice)
		}

		hostname := mongonodeslice[0]
		portStr := mongonodeslice[1]

		port, err := strconv.Atoi(portStr)
		if err != nil {
			log.Fatalf(
				"tftf - in parseHelloOutput: Invalid port string format for node %s: %s",
				mongonodestring,
				portStr,
			)
		}

		mongonode := ClusterNode{
			Hostname: hostname,
			Port:     port,
		}

		tf.Dcrlog.Debug(fmt.Sprintf("tftf - appending node %s to allnodes list", mongonodestring))
		tf.Allnodes.Nodes = append(tf.Allnodes.Nodes, mongonode)

	}
	return nil
}

func (tf *TopologyFinder) parseShardMapOutput() error {
	var shardMap map[string]interface{}

	tf.Dcrlog.Debug("tftf - parsing shard cluster output")
	if err := json.Unmarshal(tf.GetShardMapOutput.Bytes(), &shardMap); err != nil {
		log.Fatalf("tftf - error parsing shardmap output: %s", err)
	}

	tf.Dcrlog.Debug("tftf - extract hosts from shard cluster output")

	// hosts document is of format {'hostname1:port1' : 'shardn/config'... }
	allhosts, ok := shardMap["hosts"].(map[string]interface{})
	if !ok {
		log.Fatalf("tftf - error reading sharmap hosts document")
	}

	tf.Dcrlog.Debug(fmt.Sprintf("tftf - parsing shard output allhosts is: %s", allhosts))
	// hosts document is of format {'hostname1:port1' : 'shardn/config'... }
	// ignore the values part only need the keys which are 'hostname1:port1' ...
	for mongonodestring := range allhosts {

		mongonodeslice := strings.Split(mongonodestring, ":")
		tf.Dcrlog.Debug(
			fmt.Sprintf("tftf - parsing shard output mongonodestring is: %s", mongonodestring),
		)
		tf.Dcrlog.Debug(
			fmt.Sprintf("tftf - parsing shard output mongonodeslice is: %s", mongonodeslice),
		)
		tf.Dcrlog.Debug(
			fmt.Sprintf(
				"tftf - parsing shard output length of mongonodeslice is: %d",
				len(mongonodeslice),
			),
		)
		if len(mongonodeslice) != 2 {
			log.Fatalf("tftf - invalid mongo node string: %s", mongonodeslice)
		}

		// hosts document is of format {'hostname1:port1' : 'shardn/config'... }
		hostname := mongonodeslice[0]
		tf.Dcrlog.Debug(fmt.Sprintf("tftf - parsing shard output hostname is: %s", hostname))

		portStr := mongonodeslice[1]
		tf.Dcrlog.Debug(fmt.Sprintf("tftf - parsing shard output portStr is: %s", portStr))

		port, err := strconv.Atoi(portStr)
		if err != nil {
			log.Fatalf(
				"tftf - invalid port string format for node %s: %s",
				mongonodestring,
				portStr,
			)
		}

		mongonode := ClusterNode{
			Hostname: hostname,
			Port:     port,
		}

		tf.Dcrlog.Debug(fmt.Sprintf("tftf - appending node %s to allnodes list", mongonodestring))
		tf.Allnodes.Nodes = append(tf.Allnodes.Nodes, mongonode)

	}

	return nil
}

func (tf *TopologyFinder) addSeedMongosNode() error {
	seedport := tf.MongoshCapture.S.Seedmongodport
	seedhostname := tf.MongoshCapture.S.Seedmongodhost
	isSeedHostinList := false

	tf.Dcrlog.Debug("tftf - looking up seed node in the allnodes list")
	for _, host := range tf.Allnodes.Nodes {
		if seedhostname == string(host.Hostname) &&
			seedport == strconv.Itoa(host.Port) {
			isSeedHostinList = true
			tf.Dcrlog.Debug("tftf - found seed node in the allnodes list")
		}
	}

	if !isSeedHostinList {
		tf.Dcrlog.Debug("tftf - seed node not in the list, adding seed node in the allnodes list")
		err := tf.addSeedNode()
		if err != nil {
			return err
		}
	}
	return nil
}

func (tf *TopologyFinder) GetAllNodes() error {
	tf.Dcrlog.Debug("tftf - building allnodes list for data collection")
	err := tf.runShardMapDBCommand()
	if err != nil {
		return err
	}

	tf.GetShardMapOutput = *tf.MongoshCapture.Getparsedjsonoutput

	if tf.isShardMap() {

		tf.Dcrlog.Debug(
			"tftf - we are connected to sharded cluster proceeding with extracting mongo hostnames",
		)
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

// find and retain only unique nodes
func (tf *TopologyFinder) KeepUniqueNodes() error {
	tf.Dcrlog.Debug("tftf - will attempt to find if multiple hostnames mapped to single IP")
	uniqueipfinder := UniqueIPfinder{}
	uniqueipfinder.Dcrlog = tf.Dcrlog
	uniqueipfinder.AllNodes = tf.Allnodes

	// build list of strings with hostnames and port from the obtained Allnodes
	hostportList := make([]string, 0)
	for _, node := range tf.Allnodes.Nodes {
		hostportList = append(hostportList, node.Hostname+":"+fmt.Sprintf("%d", node.Port))
	}

	// generate a set of unique IP address to possible multiple hostnames
	ipportTohostportset, err := uniqueipfinder.IpportTohostportMap(hostportList)
	if err != nil {
		// TODO: add a help message here
		// Allnodes is intact at this point
		return err
	}

	allNodesNew := make([]ClusterNode, 0)
	tf.Allnodes.Nodes = allNodesNew

	// make split the ip port string and build Allnodes again
	// if there are more than one hostname mapped to single ip keep the hostname which is not internal
	// Allnodes is rebuilt with the hostnames removing duplicates
	for ipportKey, hostportList := range ipportTohostportset {
		if len(hostportList) == 0 {
			tf.Dcrlog.Warn(fmt.Sprintf(
				"tftf - warn - the ipport %s has no corresponding hostname match hence skipping",
				ipportKey,
			))
			// TODO: fill uniqueHostname as the IP
			continue
		}

		// of the possible multiple hostnames from the set keep only the first
		uniqueHostname, uniqueListenPort, err := splitHostPort(hostportList[0], tf.Dcrlog)
		if err != nil {
			tf.Dcrlog.Warn(
				fmt.Sprintf(
					"tftf - warn - unable to properly split hosport string: %s",
					hostportList[0],
				),
			)
			// TODO: fill uniqueHostname as the IP
			continue
		}

		// add to the Allnodes
		mongonode := ClusterNode{
			Hostname: uniqueHostname,
			Port:     uniqueListenPort,
		}

		tf.Dcrlog.Debug(
			fmt.Sprintf("tftf - appending node %s to allnodes list", hostportList[0]),
		)
		allNodesNew = append(allNodesNew, mongonode)
	}

	// replace existing Allnodes
	tf.Allnodes.Nodes = allNodesNew
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
