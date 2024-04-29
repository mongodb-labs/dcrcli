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
	"log"
	"net"
	"os"
	"strconv"

	"dcrcli/dcrlogger"
	"dcrcli/dcroutdir" //"os"
	"dcrcli/fscopy"
	"dcrcli/ftdcarchiver"
	"dcrcli/mongocredentials"
	"dcrcli/mongologarchiver"
	"dcrcli/mongosh"
	"dcrcli/topologyfinder"
)

func main() {
	var err error

	dcrlog := dcrlogger.DCRLogger{}
	dcrlog.OutputPrefix = "./"
	dcrlog.FileName = "dcrlogfile"

	err = dcrlog.Create()
	if err != nil {
		log.Fatal("Unable to create log file abormal Exit:", err)
	}
	// get initial mongo credentials
	cred := mongocredentials.Mongocredentials{}
	cred.Get()

	remoteCred := fscopy.RemoteCred{}
	remoteCred.Get()

	outputdir := dcroutdir.DCROutputDir{}
	outputdir.OutputPrefix = "./outputs/" + cred.Clustername + "/"
	// fmt.Println(os.Hostname())
	// We choose to pull local or remote based on whether remote cred is setup or not

	if remoteCred.Available == true {
		fmt.Println("Entering remote node handling logic")
		clustertopology := topologyfinder.TopologyFinder{}
		clustertopology.MongoshCapture.S = &cred
		err = clustertopology.GetAllNodes()
		if err != nil {
			fmt.Println("Error in Topology finding:", err)
			return
		}
		// if the nodes array is empty means its a standalone
		// if len(clustertopology.Allnodes.Nodes) != 0 {
		for _, host := range clustertopology.Allnodes.Nodes {

			fmt.Printf("host: %s, port: %d\n", host.Hostname, host.Port)
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

			isLocalHost := false
			var errtest error

			hostname := host.Hostname
			isLocalHost, errtest = isHostnameALocalHost(hostname)
			if errtest != nil {
				fmt.Println("Error determining if Hostname is a LocalHost or not :", err)
				os.Exit(1)
			}

			if isLocalHost {
				fmt.Printf("%s is a local hostname. Performing Local Copying.\n", hostname)
				/**
				clustertopology := topologyfinder.TopologyFinder{}
				clustertopology.MongoshCapture.S = &cred
				err = clustertopology.GetAllNodes()
				if err != nil {
					fmt.Println("Error in Topology finding:", err)
					return
				}

				//for _, host := range clustertopology.Allnodes.Nodes {
				/**
									fmt.Printf("host: %s, port: %d\n", host.Hostname, host.Port)

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
				        **/

				ftdcarchive := ftdcarchiver.FTDCarchive{}
				ftdcarchive.Mongo.S = &cred
				ftdcarchive.Outputdir = &outputdir
				err = ftdcarchive.Start()
				if err != nil {
					fmt.Println("Error in FTDCArchive:", err)
					return
				}

				logarchive := mongologarchiver.MongoDLogarchive{}
				logarchive.Mongo.S = &cred
				logarchive.Outputdir = &outputdir
				err = logarchive.Start()
				if err != nil {
					fmt.Println("Error in LogArchive:", err)
					return
				}
				//}
			} else {
				fmt.Printf("%s is not a local hostname. Proceeding with remote Copier.\n", hostname)
				// since we have remote cred so lets setup remote FTDC Archiver
				remotecopyJob := fscopy.FSCopyJob{}

				// now setup the ftdc archiver to archive remote files
				remoteFTDCArchiver := ftdcarchiver.RemoteFTDCarchive{}
				remoteFTDCArchiver.RemoteCopyJob = &remotecopyJob
				remoteFTDCArchiver.Mongo.S = &cred
				remoteFTDCArchiver.Outputdir = &outputdir

				tempdir := dcroutdir.DCROutputDir{}
				tempdir.OutputPrefix = "./outputs/temp/" + cred.Clustername + "/"
				tempdir.Hostname = cred.Currentmongodhost
				tempdir.Port = cred.Currentmongodport
				err = tempdir.CreateDCROutputDir()
				if err != nil {
					fmt.Println(
						"Error creating temp output Directory for storing remote DCR outputs",
					)
					return
				}

				remoteFTDCArchiver.TempOutputdir = &tempdir
				remoteFTDCArchiver.RemoteCopyJob.Src.IsLocal = false
				remoteFTDCArchiver.RemoteCopyJob.Src.Username = []byte(remoteCred.Username)
				remoteFTDCArchiver.RemoteCopyJob.Src.Hostname = []byte(cred.Currentmongodhost)
				remoteFTDCArchiver.RemoteCopyJob.Output = &bytes.Buffer{}

				remoteFTDCArchiver.RemoteCopyJob.Dst.Path = []byte(
					remoteFTDCArchiver.TempOutputdir.Path(),
				)

				err = remoteFTDCArchiver.Start()
				if err != nil {
					fmt.Println("Error in Remote FTDC Archive: ", err)
					return
				}

				// empty the old rsync command
				remotecopyJob.Output.Reset()

				remotecopyJobWithPattern := fscopy.FSCopyJobWithPattern{}
				remotecopyJobWithPattern.CopyJobDetails = &remotecopyJob

				remoteLogArchiver := mongologarchiver.RemoteMongoDLogarchive{}
				remoteLogArchiver.RemoteCopyJob = &remotecopyJobWithPattern
				remoteLogArchiver.Mongo.S = &cred
				remoteLogArchiver.Outputdir = &outputdir
				remoteLogArchiver.TempOutputdir = &tempdir

				err = remoteLogArchiver.Start()
				if err != nil {
					fmt.Println("Error in Remote Log Archive: ", err)
					return
				}

			}

		}
		//}
	} else {
		// localhost logic - when all nodes of cluster are on the same physical machine i.e. localhost
		/**
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

		ftdcarchive := ftdcarchiver.FTDCarchive{}
		ftdcarchive.Mongo.S = &cred
		ftdcarchive.Outputdir = &outputdir
		err = ftdcarchive.Start()
		if err != nil {
			fmt.Println("Error in FTDCArchive:", err)
			return
		}

		logarchive := mongologarchiver.MongoDLogarchive{}
		logarchive.Mongo.S = &cred
		logarchive.Outputdir = &outputdir
		err = logarchive.Start()
		if err != nil {
			fmt.Println("Error in LogArchive:", err)
			return
		}
		*/
		clustertopology := topologyfinder.TopologyFinder{}
		clustertopology.MongoshCapture.S = &cred
		err = clustertopology.GetAllNodes()
		if err != nil {
			fmt.Println("Error in Topology finding:", err)
			return
		}
		// if the nodes array is empty means its a standalone
		// if len(clustertopology.Allnodes.Nodes) != 0 {

		for _, host := range clustertopology.Allnodes.Nodes {

			fmt.Printf("host: %s, port: %d\n", host.Hostname, host.Port)
			// fmt.Printf("Seedhost: %s, Seedport: %s\n", cred.Seedmongodhost, cred.Seedmongodport)

			// if cred.Seedmongodhost != string(host.Hostname) ||
			// cred.Seedmongodport != strconv.Itoa(host.Port) {

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
			err = ftdcarchive.Start()
			if err != nil {
				fmt.Println("Error in FTDCArchive:", err)
				return
			}

			logarchive := mongologarchiver.MongoDLogarchive{}
			logarchive.Mongo.S = &cred
			logarchive.Outputdir = &outputdir
			err = logarchive.Start()
			if err != nil {
				fmt.Println("Error in LogArchive:", err)
				return
			}
			//}
		}
		//}
	}
}

func getListOfHostIPsForHostname(hostname string) ([]net.IP, error) {
	// We resolve the hostname to its IP address(es)
	listOfhostIPsForHostname, err := net.LookupIP(hostname)
	if err != nil {
		return nil, err
	}

	return listOfhostIPsForHostname, nil
}

func getLocalMachineInterfaces() ([]net.Interface, error) {
	localMachineInterfacesArray, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	return localMachineInterfacesArray, nil
}

func createAndInitializeArrayToHoldLocalIPAddresses() []net.IP {
	arrayOflocalIPsForMachine := make([]net.IP, 0)

	// add all possible loopback addresses to the list
	for i := 1; i <= 255; i++ {
		arrayOflocalIPsForMachine = append(arrayOflocalIPsForMachine, net.IPv4(127, 0, 0, byte(i)))
	}
	return arrayOflocalIPsForMachine
}

func determineLocalMachineAddresses(localMachineInterfaces []net.Interface) []net.IP {
	interfaceIPsArray := make([]net.IP, 0)

	for _, localMachineInterface := range localMachineInterfaces {
		interfaceAddresses, err := localMachineInterface.Addrs()
		if err != nil {
			continue
		}

		for _, interfaceAddress := range interfaceAddresses {
			var ip net.IP

			switch v := interfaceAddress.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip != nil && !ip.IsLoopback() {
				interfaceIPsArray = append(interfaceIPsArray, ip)
			}
		}
	}

	return interfaceIPsArray
}

func isHostnameALocalHost(hostname string) (bool, error) {
	resolvedIPsForHostname, err := getListOfHostIPsForHostname(hostname)
	if err != nil {
		return false, err
	}

	localMachineInterfaces, err := getLocalMachineInterfaces()
	if err != nil {
		return false, err
	}

	localIPsForMachine := createAndInitializeArrayToHoldLocalIPAddresses()

	localIPsForMachine = append(
		localIPsForMachine,
		determineLocalMachineAddresses(localMachineInterfaces)...)

	for _, resolvedIP := range resolvedIPsForHostname {
		for _, localIP := range localIPsForMachine {
			// fmt.Println("IP resolved for Hostname is ", resolvedIP, " localIp is ", localIP)
			if resolvedIP.Equal(localIP) {
				return true, nil
			}
		}
	}

	return false, nil
}
