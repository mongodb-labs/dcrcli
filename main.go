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
	"time"

	"dcrcli/dcrlogger"
	"dcrcli/dcroutdir" //"os"
	"dcrcli/fscopy"
	"dcrcli/ftdcarchiver"
	"dcrcli/mongocredentials"
	"dcrcli/mongologarchiver"
	"dcrcli/mongosh"
	"dcrcli/topologyfinder"
)

func checkEmptyDirectory(OutputPrefix string) string {
	if isDirectoryExist(OutputPrefix) {
		// if the output directory exists then add a timestamp to the given directory and create it
		return OutputPrefix[:len(OutputPrefix)-1] + "_" + strconv.FormatInt(
			time.Now().Unix(),
			10,
		) + "/"
	}
	return OutputPrefix
}

func isDirectoryExist(OutputPrefix string) bool {
	_, err := os.Stat(OutputPrefix)
	if os.IsNotExist(err) {
		return false
	}
	return true
}

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
	fmt.Println("DCR Log file:", dcrlog.Path())

	cred := mongocredentials.Mongocredentials{}
	cred.Get()

	remoteCred := fscopy.RemoteCred{}
	remoteCred.Get()

	outputdir := dcroutdir.DCROutputDir{}
	outputdir.OutputPrefix = checkEmptyDirectory("./outputs/" + cred.Clustername + "/")

	dcrlog.Info(fmt.Sprintf("DCR outputs directory: %s", outputdir.OutputPrefix))
	// fmt.Println(os.Hostname())
	// We choose to pull local or remote based on whether remote cred is setup or not

	if remoteCred.Available == true {
		// fmt.Println("Entering remote node handling logic")
		dcrlog.Info("remote creds provided will handle both remote and local node")

		dcrlog.Info("Probing cluster topology")

		clustertopology := topologyfinder.TopologyFinder{}
		clustertopology.MongoshCapture.S = &cred
		err = clustertopology.GetAllNodes()
		if err != nil {
			dcrlog.Error(fmt.Sprintf("Error in Topology finding: %s", err.Error()))
			log.Fatal("Error in Topology finding:", err)
		}
		// if the nodes array is empty means its a standalone
		// if len(clustertopology.Allnodes.Nodes) != 0 {
		for _, host := range clustertopology.Allnodes.Nodes {

			// fmt.Printf("host: %s, port: %d\n", host.Hostname, host.Port)
			dcrlog.Info(fmt.Sprintf("host: %s, port: %d", host.Hostname, host.Port))

			cred.Currentmongodhost = host.Hostname
			cred.Currentmongodport = strconv.Itoa(host.Port)
			cred.SetMongoURI()

			outputdir.Hostname = cred.Currentmongodhost
			outputdir.Port = cred.Currentmongodport
			err = outputdir.CreateDCROutputDir()
			if err != nil {
				dcrlog.Error("Error creating output Directory for storing DCR outputs")
				log.Fatal("Error creating output Directory for storing DCR outputs")
			}

			c := mongosh.CaptureGetMongoData{}
			c.S = &cred
			c.Outputdir = &outputdir

			// fmt.Println("Running getMongoData/mongoWellnessChecker")
			dcrlog.Info("Running getMongoData/mongoWellnessChecker")
			err := c.RunMongoShellWithEval()
			if err != nil {
				// fmt.Println(err)
				dcrlog.Error(err.Error())
				return
			}

			isLocalHost := false
			var errtest error

			hostname := host.Hostname
			isLocalHost, errtest = isHostnameALocalHost(hostname)
			if errtest != nil {
				dcrlog.Error(
					fmt.Sprintf(
						"Error determining if Hostname is a LocalHost or not : %s",
						errtest.Error(),
					),
				)
				log.Fatal("Error determining if Hostname is a LocalHost or not :", errtest)
			}

			if isLocalHost {
				// fmt.Printf("%s is a local hostname. Performing Local Copying.\n", hostname)
				dcrlog.Info(
					fmt.Sprintf("%s is a local hostname. Performing Local Copying.", hostname),
				)
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

				dcrlog.Info("Running FTDC Archiving")
				ftdcarchive := ftdcarchiver.FTDCarchive{}
				ftdcarchive.Mongo.S = &cred
				ftdcarchive.Outputdir = &outputdir
				err = ftdcarchive.Start()
				if err != nil {
					fmt.Println("Error in FTDCArchive:", err)
					dcrlog.Error(fmt.Sprintf("Error in FTDCArchive: %s", err.Error()))
					return
				}

				dcrlog.Info("Running mongo log Archiving")
				logarchive := mongologarchiver.MongoDLogarchive{}
				logarchive.Mongo.S = &cred
				logarchive.Outputdir = &outputdir
				err = logarchive.Start()
				if err != nil {
					fmt.Println("Error in LogArchive:", err)
					dcrlog.Error(fmt.Sprintf("Error in LogArchive: %s", err.Error()))
					return
				}
				//}
			} else {
				dcrlog.Info(fmt.Sprintf("%s is not a local hostname. Proceeding with remote Copier.", hostname))
				// since we have remote cred so lets setup remote FTDC Archiver
				remotecopyJob := fscopy.FSCopyJob{}

				// now setup the ftdc archiver to archive remote files
				dcrlog.Info("Running FTDC Archiving")
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
					dcrlog.Error(
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
					dcrlog.Error(fmt.Sprintf("Error in Remote FTDC Archive: %s", err.Error()))
					return
				}

				// empty the old rsync command
				remotecopyJob.Output.Reset()

				remotecopyJobWithPattern := fscopy.FSCopyJobWithPattern{}
				remotecopyJobWithPattern.CopyJobDetails = &remotecopyJob

				dcrlog.Info("Running mongo log Archiving")
				remoteLogArchiver := mongologarchiver.RemoteMongoDLogarchive{}
				remoteLogArchiver.RemoteCopyJob = &remotecopyJobWithPattern
				remoteLogArchiver.Mongo.S = &cred
				remoteLogArchiver.Outputdir = &outputdir
				remoteLogArchiver.TempOutputdir = &tempdir

				err = remoteLogArchiver.Start()
				if err != nil {
					fmt.Println("Error in Remote Log Archive: ", err)
					dcrlog.Error(fmt.Sprintf("Error in Remote Log Archive: %s", err.Error()))
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

		dcrlog.Info("Remote cred not provided looking for nodes running locally")

		dcrlog.Info("Probing cluster topology")

		clustertopology := topologyfinder.TopologyFinder{}
		clustertopology.MongoshCapture.S = &cred
		err = clustertopology.GetAllNodes()
		if err != nil {
			fmt.Println("Error in Topology finding:", err)
			dcrlog.Error(fmt.Sprintf("Error in Topology finding: %s", err.Error()))
			return
		}
		// if the nodes array is empty means its a standalone
		// if len(clustertopology.Allnodes.Nodes) != 0 {

		for _, host := range clustertopology.Allnodes.Nodes {

			dcrlog.Info(fmt.Sprintf("host: %s, port: %d", host.Hostname, host.Port))
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
				dcrlog.Error("Error creating output Directory for storing DCR outputs")
				return
			}

			c := mongosh.CaptureGetMongoData{}
			c.S = &cred
			c.Outputdir = &outputdir

			dcrlog.Info("Running getMongoData/mongoWellnessChecker")
			err := c.RunMongoShellWithEval()
			if err != nil {
				fmt.Println(err)
				dcrlog.Error(err.Error())
				return
			}

			dcrlog.Info("Running FTDCArchiving")
			ftdcarchive := ftdcarchiver.FTDCarchive{}
			ftdcarchive.Mongo.S = &cred
			ftdcarchive.Outputdir = &outputdir
			err = ftdcarchive.Start()
			if err != nil {
				fmt.Println("Error in FTDCArchive:", err)
				dcrlog.Error(fmt.Sprintf("Error in FTDC Archive: %s", err.Error()))
				return
			}

			dcrlog.Info("Running mongo log archiving")
			logarchive := mongologarchiver.MongoDLogarchive{}
			logarchive.Mongo.S = &cred
			logarchive.Outputdir = &outputdir
			err = logarchive.Start()
			if err != nil {
				fmt.Println("Error in LogArchive:", err)
				dcrlog.Error(fmt.Sprintf("Error in LogArchive: %s", err.Error()))
				return
			}
			//}
		}
		//}
	}
	dcrlog.Info("---End of Script Execution----")
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
