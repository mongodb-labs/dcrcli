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
	"bytes"
	"fmt"
	"os"
	"strconv"

	"dcrcli/dcroutdir" //"os"
	"dcrcli/fscopy"
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

	remoteCred := fscopy.RemoteCred{}
	remoteCred.Get()

	outputdir := dcroutdir.DCROutputDir{}
	outputdir.OutputPrefix = "./outputs/" + cred.Clustername + "/"
	outputdir.Hostname = cred.Seedmongodhost
	outputdir.Port = cred.Seedmongodport
	err = outputdir.CreateDCROutputDir()
	if err != nil {
		fmt.Println("Error creating output Directory for storing DCR outputs")
		return
	}

	c := mongosh.CaptureGetMongoData{
		S:                   &cred,
		Getparsedjsonoutput: nil,
		CurrentBin:          "",
		ScriptPath:          "",
		FilePathOnDisk:      "",
		Outputdir:           &outputdir,
	}

	fmt.Println("Running getMongoData/mongoWellnessChecker")
	err = c.RunMongoShellWithEval()
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(os.Hostname())
	// We choose to pull local or remote based on whether remote cred is setup or not

	if remoteCred.Available == true && cred.Seedmongodhost != "localhost" {
		// since we have remote cred so lets setup remote FTDC Archiver
		remotecopyJob := fscopy.FSCopyJob{}

		// now setup the ftdc archiver to archive remote files
		remoteFTDCArchiver := ftdcarchiver.RemoteFTDCarchive{}
		remoteFTDCArchiver.RemoteCopyJob = &remotecopyJob
		remoteFTDCArchiver.Mongo.S = &cred
		remoteFTDCArchiver.Outputdir = &outputdir

		tempdir := dcroutdir.DCROutputDir{}
		tempdir.OutputPrefix = "./outputs/temp/" + cred.Clustername + "/"
		tempdir.Hostname = cred.Seedmongodhost
		tempdir.Port = cred.Seedmongodport
		err = tempdir.CreateDCROutputDir()
		if err != nil {
			fmt.Println("Error creating temp output Directory for storing remote DCR outputs")
			return
		}

		remoteFTDCArchiver.TempOutputdir = &tempdir
		remoteFTDCArchiver.RemoteCopyJob.Src.IsLocal = false
		remoteFTDCArchiver.RemoteCopyJob.Src.Username = []byte(remoteCred.Username)
		remoteFTDCArchiver.RemoteCopyJob.Src.Hostname = []byte(cred.Seedmongodhost)
		remoteFTDCArchiver.RemoteCopyJob.Output = &bytes.Buffer{}

		remoteFTDCArchiver.RemoteCopyJob.Dst.Path = []byte(remoteFTDCArchiver.TempOutputdir.Path())

		err = remoteFTDCArchiver.Start()
		if err != nil {
			fmt.Println("Error in Remote FTDC Archive: ", err)
			return
		}

	}

	ftdcarchive := ftdcarchiver.FTDCarchive{}
	ftdcarchive.Mongo.S = &cred
	ftdcarchive.Outputdir = &outputdir
	err = ftdcarchive.Start()
	if err != nil {
		fmt.Println("Error in FTDCArchive:", err)
		return
	}

	logarchive := logarchiver.MongoDLogarchive{}
	logarchive.Mongo.S = &cred
	logarchive.Outputdir = &outputdir
	err = logarchive.Start()
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

				outputdir.Hostname = cred.Currentmongodhost
				outputdir.Port = cred.Currentmongodport
				err = outputdir.CreateDCROutputDir()
				if err != nil {
					fmt.Println("Error creating output Directory for storing DCR outputs")
					return
				}

				c := mongosh.CaptureGetMongoData{}
				c.S = &cred
				c.Outputdir = &outputdir

				fmt.Println("Running getMongoData/mongoWellnessChecker")
				err := c.RunMongoShellWithEval()
				if err != nil {
					fmt.Println(err)
					return
				}

				ftdcarchive := ftdcarchiver.FTDCarchive{}
				ftdcarchive.Mongo.S = &cred
				ftdcarchive.Outputdir = &outputdir
				ftdcarchive.Start()
				if err != nil {
					fmt.Println("Error in FTDCArchive:", err)
					return
				}

				logarchive := logarchiver.MongoDLogarchive{}
				logarchive.Mongo.S = &cred
				logarchive.Outputdir = &outputdir
				logarchive.Start()
				if err != nil {
					fmt.Println("Error in LogArchive:", err)
					return
				}
			}
		}
	}
}
