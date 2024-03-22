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
	"time"

	"dcrcli/ftdcarchiver"
	"dcrcli/logarchiver"
	"dcrcli/mongocredentials"
	"dcrcli/mongosh"
	"dcrcli/topologyfinder"
)

func main() {
	// get initial mongo credentials
	cred := mongocredentials.Mongocredentials{}
	cred.Get()

	// get timestamp because its unique
	unixts := strconv.FormatInt(time.Now().UnixNano(), 10)
	os.MkdirAll("./outputs/"+unixts, 0744)
	c := mongosh.CaptureGetMongoData{
		S:                   &cred,
		Getparsedjsonoutput: nil,
		CurrentBin:          "",
		ScriptPath:          "",
		Unixts:              unixts,
		FilePathOnDisk:      "",
	}

	// c.RunMongoShell()
	err := c.RunMongoShellWithEval()
	if err != nil {
		fmt.Println(err)
		return
	}

	// this is used by ftdcarchiver and logarchiver
	mongosh.Getparsedjsonoutput = *c.Getparsedjsonoutput

	clustertopology := topologyfinder.TopologyFinder{}
	clustertopology.MongoshCapture.S = &cred
	clustertopology.GetAllNodes()

	for _, host := range clustertopology.Allnodes.Nodes {
		fmt.Printf("host: %s, port: %d\n", host.Hostname, host.Port)
	}

	// mongosh.Runshell(unixts)
	ftdcarchiver.Run(unixts)
	logarchiver.Run(unixts)
}
