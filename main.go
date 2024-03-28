// Copyright 2020 MongoDB Inc
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

package main

import (
	"fmt"
	"os"
	"strconv"

	"dcrcli/ftdcarchiver"
	"dcrcli/logarchiver"
	"dcrcli/mongocredentials"
	"dcrcli/mongosh"
	"dcrcli/topologyfinder"
)

func main() {
	var err error
	// get initial mongo credentials
	cred := mongocredentials.Mongocredentials{}
	cred.Get()

	outputPrefix := "./outputs/" + cred.Clustername + "/"
	// get timestamp because its unique
	// unixts := strconv.FormatInt(time.Now().UnixNano(), 10)
	unixts := outputPrefix + cred.Seedmongodhost + "_" + cred.Seedmongodport
	os.MkdirAll(unixts, 0744)
	c := mongosh.CaptureGetMongoData{
		S:                   &cred,
		Getparsedjsonoutput: nil,
		CurrentBin:          "",
		ScriptPath:          "",
		Unixts:              unixts,
		FilePathOnDisk:      "",
	}

	fmt.Println("Running getMongoData/mongoWellnessChecker")
	err = c.RunMongoShellWithEval()
	if err != nil {
		fmt.Println(err)
		return
	}

	ftdcarchive := ftdcarchiver.FTDCarchive{}
	ftdcarchive.Unixts = unixts
	ftdcarchive.Mongo.S = &cred
	ftdcarchive.Start()
	if err != nil {
		fmt.Println("Error in FTDCArchive:", err)
		return
	}

	logarchive := logarchiver.MongoDLogarchive{}
	logarchive.Unixts = unixts
	logarchive.Mongo.S = &cred
	logarchive.Start()
	if err != nil {
		fmt.Println("Error in LogArchive:", err)
		return
	}

	// Always collect from the seed host above then remaining nodes
	clustertopology := topologyfinder.TopologyFinder{}
	clustertopology.MongoshCapture.S = &cred
	err = clustertopology.GetAllNodes()
	if err != nil {
		fmt.Println("Error in Topology finding:", err)
		return
	}
	// if the nodes array is empty means its a standalone
	if len(clustertopology.Allnodes.Nodes) != 0 {
		for _, host := range clustertopology.Allnodes.Nodes {

			fmt.Printf("host: %s, port: %d\n", host.Hostname, host.Port)
			fmt.Printf("Seedhost: %s, Seedport: %s\n", cred.Seedmongodhost, cred.Seedmongodport)

			if cred.Seedmongodhost != string(host.Hostname) ||
				cred.Seedmongodport != strconv.Itoa(host.Port) {

				cred.Currentmongodhost = host.Hostname
				cred.Currentmongodport = strconv.Itoa(host.Port)
				cred.SetMongoURI()

				//			unixts := strconv.FormatInt(time.Now().UnixNano(), 10)
				unixts := outputPrefix + cred.Currentmongodhost + "_" + cred.Currentmongodport
				// unixts = outputPrefix + unixts
				os.MkdirAll(unixts, 0744)

				c := mongosh.CaptureGetMongoData{}
				c.Unixts = unixts
				c.S = &cred

				fmt.Println("Running getMongoData/mongoWellnessChecker")
				err := c.RunMongoShellWithEval()
				if err != nil {
					fmt.Println(err)
					return
				}

				ftdcarchive := ftdcarchiver.FTDCarchive{}
				ftdcarchive.Unixts = unixts
				ftdcarchive.Mongo.S = &cred
				ftdcarchive.Start()
				if err != nil {
					fmt.Println("Error in FTDCArchive:", err)
					return
				}

				logarchive := logarchiver.MongoDLogarchive{}
				logarchive.Unixts = unixts
				logarchive.Mongo.S = &cred
				logarchive.Start()
				if err != nil {
					fmt.Println("Error in LogArchive:", err)
					return
				}
			}
		}
	}
}
