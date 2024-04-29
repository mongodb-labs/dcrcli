package topologyfinder

import (
	"bytes"
	"encoding/json"
	"fmt"
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
// - Hidden nodes cannot be discovered by client application !!!!WARNING!!!!!!
// - Returns the hostname information that mongod has - could be PRIVATE hostnames as well!!!!

// For sharded clusters uses getShardMap command to list the hosts and config servers
//   - this command must be run from mongos or shard nodes (not the config server because it does not list all config ndoes in host: field)
// For sharded clusters does not collect data from all mongos - one mongos is enough
//

// First run getShardMap on the node if errmsg Sharding not enabled . Then the node is not part of sharded cluster
// if above fails run hello or isMaster(for older than 4.4) . Look for hosts field if not there then this is a standalone

type TopologyFinder struct {
	Allnodes          ClusterNodes
	GetShardMapOutput bytes.Buffer
	MongoshCapture    mongosh.CaptureGetMongoData
	GetHelloOutput    bytes.Buffer
}

// the mongosh output is json like but contains Fields like Timestamp, BinData etc which cause errors in JSON.Unmarshal
// So removing them manually before any operation
// Future note: Not needed when use mongo go driver in future
func (tf *TopologyFinder) removeBsonFields() error {
	// Compile a regular expression that matches "Timestamp", "NumberLong", or "BinData"
	// followed by parentheses with any content inside.
	re := regexp.MustCompile(
		`(Timestamp|NumberLong|BinData|Binary\.createFromBase64|Long)\([^\)]*\)`,
	)

	// Replace all matches with the string "dummy"
	replaced := re.ReplaceAll(tf.GetShardMapOutput.Bytes(), []byte(`"dummy"`))

	tf.GetShardMapOutput.Reset()
	tf.GetShardMapOutput.Write(replaced)

	return nil
}

func (tf *TopologyFinder) isShardMap() bool {
	var tempshardmapoutput map[string]interface{}

	if err := json.Unmarshal(tf.GetShardMapOutput.Bytes(), &tempshardmapoutput); err != nil {
		// not a valid shardmap value so not a sharded cluster nodes
		// Note: mongos, mongod of sharded cluster always return the shardmap even the config servers
		fmt.Println("isShardMap(): json unmarshaling error", err)
		return false
	}
	fmt.Println("tempshardmapoutput: ", tempshardmapoutput["hosts"])

	// check additionally the hosts key is there
	hostsMap, ok := tempshardmapoutput["hosts"].(map[string]interface{})
	if !ok {
		// the hosts key is not there
		fmt.Println("isShardMap(): hosts key not found in the shard map output")
		return false
	}

	// the value of hosts is like { 'host1:port': 'shard1', 'host2:port2': 'shard2' ...} confirm this is the case
	for hoststring, shardtype := range hostsMap {
		if hoststring == "" || shardtype == "" || len(strings.Split(hoststring, ":")) != 2 {
			// both shuld be non-empty
			fmt.Println(
				"isShardMap(): hoststring or shardtype cannot be empty in the shardmap or the hostsstring does not have host:port format",
			)
			return false
		}
	}

	return true
}

func (tf *TopologyFinder) parseHelloOutput() error {
	// this map will hold Unmarshalled data
	var hostsArray []string

	// the slice is now read fully into the shardMap variable
	if err := json.Unmarshal(tf.GetHelloOutput.Bytes(), &hostsArray); err != nil {
		fmt.Println("Error in parseHelloOutput(): ", err)
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
	// this map will hold Unmarshalled data
	var shardMap map[string]interface{}

	// the slice is now read fully into the shardMap variable
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

	// If seed node is not the list then the connection was made to mongos so addd that as well to the data collection
	if !isSeedHostinList {
		err := tf.addSeedNode()
		if err != nil {
			return err
		}
	}
	return nil
}

func (tf *TopologyFinder) GetAllNodes() error {
	// use the mongo shell to run getShardMap
	// note with new mongosh shell the exit status of command is 1 if run on replica ReplicaSet
	// so different behavour for replia set and sharded cluster for this command depending on the shell type
	err := tf.runShardMapDBCommand()
	if err != nil {
		fmt.Println("Error running ShardMapCommand")
		return err
	}

	// copy the mongosh command output to GetShardMapOutput
	tf.GetShardMapOutput = *tf.MongoshCapture.Getparsedjsonoutput

	// Deprecated because now using json=canonical with mongosh
	// tf.removeBsonFields()

	if tf.isShardMap() {
		err := tf.parseShardMapOutput()
		if err != nil {
			return err
		}
		// shard map output does not list mongos node so add that
		err = tf.addSeedMongosNode()
		if err != nil {
			return err
		}
		// must return because we know its sharded
		return nil
	}

	// if not sharded then check if its replica set or standalone in this function
	err = tf.useHelloDBCommandHostsArray()
	if err != nil {
		return err
	}
	return nil
}

func (tf *TopologyFinder) addSeedNode() error {
	seedport, err := strconv.Atoi(tf.MongoshCapture.S.Seedmongodport)
	if err != nil {
		fmt.Println("Error in addSeedNode()")
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
		fmt.Println("Error running runHello", err)
		return err
	}

	// copy the output from mongosh into the GetHelloOutput
	tf.GetHelloOutput = *tf.MongoshCapture.Getparsedjsonoutput

	// If the hosts field is absent in the hello output then its a standalone
	// it could be a shard but we check for shard first then replica set
	if tf.GetHelloOutput.Len() == 0 {

		err = tf.addSeedNode()
		if err != nil {
			return err
		}

		// if not replica set or sharded must be standalone
		return nil
	}
	err = tf.parseHelloOutput()
	if err != nil {
		fmt.Println("Error while parsing HelloOutput", err)
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

// The mongosh shell can return exit 1 with live replica set but legacy mongo works Fine
// So ignore error from mongosh - this is shortcut but we depend on later logic to parse its output
func (tf *TopologyFinder) runShardMapDBCommand() error {
	err := tf.MongoshCapture.RunGetShardMapWithEval()
	if err != nil {
		if tf.MongoshCapture.CurrentBin == "mongo" {
			return err
		}
	}
	return nil
}
