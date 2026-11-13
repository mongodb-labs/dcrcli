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
	"regexp"
	"sort"
	"strconv"
	"strings"

	"dcrcli/dcrlogger"
	"dcrcli/mongosh"
)

type ClusterNode struct {
	Hostname string
	Port     int
	// ReplicaState is PRIMARY, SECONDARY, ARBITER, MONGOS, or UNKNOWN after ResolveReplicaStates.
	ReplicaState string
	// ShardMapHostRole is the getShardMap "hosts" value (e.g. "config", "shard01") when discovered via the sharded path; empty for replica-set-only discovery.
	ShardMapHostRole string
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
		return fmt.Errorf("tftf - error parsing hello output during topology discovery: %s", err)
	}

	for _, mongonodestring := range hostsArray {

		mongonodeslice := strings.Split(mongonodestring, ":")
		if len(mongonodeslice) != 2 {
			return fmt.Errorf("tftf - invalid mongo node string: %s", mongonodeslice)
		}

		hostname := mongonodeslice[0]
		portStr := mongonodeslice[1]

		port, err := strconv.Atoi(portStr)
		if err != nil {
			return fmt.Errorf(
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
		return fmt.Errorf("tftf - error parsing shardmap output: %s", err)
	}

	tf.Dcrlog.Debug("tftf - extract hosts from shard cluster output")

	// hosts document is of format {'hostname1:port1' : 'shardn/config'... }
	allhosts, ok := shardMap["hosts"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("tftf - error reading sharmap hosts document")
	}

	tf.Dcrlog.Debug(fmt.Sprintf("tftf - parsing shard output allhosts is: %s", allhosts))
	// hosts document is of format {'hostname1:port1' : 'shardn/config'... }
	for mongonodestring, rawRole := range allhosts {
		hostRole := ""
		switch v := rawRole.(type) {
		case string:
			hostRole = v
		default:
			if rawRole != nil {
				hostRole = fmt.Sprintf("%v", rawRole)
			}
		}

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
			return fmt.Errorf("tftf - invalid mongo node string: %s", mongonodeslice)
		}

		// hosts document is of format {'hostname1:port1' : 'shardn/config'... }
		hostname := mongonodeslice[0]
		tf.Dcrlog.Debug(fmt.Sprintf("tftf - parsing shard output hostname is: %s", hostname))

		portStr := mongonodeslice[1]
		tf.Dcrlog.Debug(fmt.Sprintf("tftf - parsing shard output portStr is: %s", portStr))

		port, err := strconv.Atoi(portStr)
		if err != nil {
			return fmt.Errorf(
				"tftf - invalid port string format for node %s: %s",
				mongonodestring,
				portStr,
			)
		}

		mongonode := ClusterNode{
			Hostname:         hostname,
			Port:             port,
			ShardMapHostRole: hostRole,
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

	nodesBeforeDedup := append([]ClusterNode(nil), tf.Allnodes.Nodes...)

	// One entry per node. A name with several IPv4 addresses appears under every
	// one of them, and two names for the same address appear under one key, so
	// group by the hostname that is kept. Walking the keys in order makes the
	// result the same on every run.
	type keptNode struct {
		hostname string
		port     int
		aliases  []string
	}
	keys := make([]string, 0, len(ipportTohostportset))
	for k := range ipportTohostportset {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	kept := map[string]*keptNode{}
	order := make([]string, 0, len(keys))
	for _, ipportKey := range keys {
		hostportList := ipportTohostportset[ipportKey]
		if len(hostportList) == 0 {
			tf.Dcrlog.Warn(fmt.Sprintf(
				"tftf - warn - the ipport %s has no corresponding hostname match hence skipping",
				ipportKey,
			))
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
			continue
		}

		id := nodeIdentity(uniqueHostname, uniqueListenPort)
		k := kept[id]
		if k == nil {
			k = &keptNode{hostname: uniqueHostname, port: uniqueListenPort}
			kept[id] = k
			order = append(order, id)
		}
		k.aliases = append(k.aliases, hostportList...)
	}

	allNodesNew := make([]ClusterNode, 0, len(order)+len(uniqueipfinder.Unresolved))
	for _, id := range order {
		k := kept[id]
		tf.Dcrlog.Debug(fmt.Sprintf("tftf - appending node %s:%d to allnodes list", k.hostname, k.port))
		// carry over the shard map role from any alias for this node
		allNodesNew = append(allNodesNew, ClusterNode{
			Hostname:         k.hostname,
			Port:             k.port,
			ShardMapHostRole: shardMapRoleFromAliases(nodesBeforeDedup, k.aliases, tf.Dcrlog),
		})
	}

	// A member that does not resolve cannot be grouped, but it is still a member
	// of the cluster. Keep it, so the health check reports it and the run stops
	// instead of quietly collecting from the remaining nodes.
	if len(uniqueipfinder.Unresolved) > 0 {
		tf.Dcrlog.Warn(fmt.Sprintf(
			"tftf - warn - keeping members that do not resolve to an IPv4 address: %s",
			strings.Join(uniqueipfinder.Unresolved, ", "),
		))
		for _, n := range nodesBeforeDedup {
			id := nodeIdentity(n.Hostname, n.Port)
			if _, dup := kept[id]; dup {
				continue
			}
			if !containsFold(uniqueipfinder.Unresolved, n.Hostname+":"+strconv.Itoa(n.Port)) {
				continue
			}
			kept[id] = &keptNode{hostname: n.Hostname, port: n.Port}
			allNodesNew = append(allNodesNew, n)
		}
	}

	// replace existing Allnodes
	tf.Allnodes.Nodes = allNodesNew
	return nil
}

// nodeIdentity is the key two entries share when they are the same node.
func nodeIdentity(hostname string, port int) string {
	return strings.ToLower(strings.TrimSpace(hostname)) + ":" + strconv.Itoa(port)
}

func containsFold(list []string, want string) bool {
	for _, s := range list {
		if strings.EqualFold(strings.TrimSpace(s), strings.TrimSpace(want)) {
			return true
		}
	}
	return false
}

// shardMapRoleFromAliases picks a non-empty ShardMapHostRole from nodes matching any host:port alias; prefers "config".
func shardMapRoleFromAliases(snapshot []ClusterNode, aliases []string, log *dcrlogger.DCRLogger) string {
	var fallback string
	for _, alias := range aliases {
		ah, ap, err := splitHostPort(alias, log)
		if err != nil {
			continue
		}
		for _, n := range snapshot {
			if !strings.EqualFold(strings.TrimSpace(n.Hostname), strings.TrimSpace(ah)) || n.Port != ap {
				continue
			}
			if strings.EqualFold(strings.TrimSpace(n.ShardMapHostRole), "config") {
				return n.ShardMapHostRole
			}
			if n.ShardMapHostRole != "" {
				fallback = n.ShardMapHostRole
			}
		}
	}
	return fallback
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
