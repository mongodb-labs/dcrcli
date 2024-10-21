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

package main

import (
	"bytes"
	"fmt"
	"log"
	"log/slog"
	"net"
	"os"
	"strconv"
	"syscall"
	"time"

	"github.com/briandowns/spinner"

	"dcrcli/dcrlogger"
	"dcrcli/dcroutdir"
	"dcrcli/fscopy"
	"dcrcli/ftdcarchiver"
	"dcrcli/mongocredentials"
	"dcrcli/mongologarchiver"
	"dcrcli/mongosh"
	"dcrcli/topologyfinder"
)

func checkEmptyDirectory(OutputPrefix string) string {
	if isDirectoryExist(OutputPrefix) {
		return OutputPrefix[:len(OutputPrefix)-1] + "_" + strconv.FormatInt(
			time.Now().Unix(),
			10,
		) + "/"
	}
	return OutputPrefix
}

func isDirectoryExist(OutputPrefix string) bool {
	_, err := os.Stat(OutputPrefix)
	return !os.IsNotExist(err)
}

func main() {
	var err error

	dcrcliDebugModeEnv, isEnvSet := os.LookupEnv("DCRCLI_DEBUG_MODE")
	dcrcliDebugMode := false
	if isEnvSet {
		dcrcliDebugMode, _ = strconv.ParseBool(dcrcliDebugModeEnv)
	}

	dcrlog := dcrlogger.DCRLogger{}
	dcrlog.OutputPrefix = "./"
	dcrlog.FileName = "dcrlogfile"

	err = dcrlog.Create()
	if err != nil {
		log.Fatal("Unable to create log file abormal Exit:", err)
	}

	if dcrcliDebugMode {
		dcrlog.SetLogLevel(slog.LevelDebug)
	}

	fmt.Println("DCR Log file:", dcrlog.Path())

	cred := mongocredentials.Mongocredentials{}
	cred.Dcrlog = &dcrlog

	err = cred.Get()
	if err != nil {
		dcrlog.Error(err.Error())
		log.Fatal("Error why getting DB credentials aborting!")
	}

	remoteCred := fscopy.RemoteCred{}
	remoteCred.Dcrlog = &dcrlog
	remoteCred.Get()

	s := spinner.New(spinner.CharSets[11], 100*time.Millisecond)
	s.Start()

	outputdir := dcroutdir.DCROutputDir{}
	outputdir.OutputPrefix = checkEmptyDirectory("./outputs/" + cred.Clustername + "/")

	dcrlog.Info(
		fmt.Sprintf(
			"Seed Host: %s, Seed Port: %s", cred.Seedmongodhost, cred.Seedmongodport,
		),
	)

	dcrlog.Info(
		fmt.Sprintf(
			"DCR outputs directory: %s", outputdir.OutputPrefix,
		),
	)

	dcrlog.Info(
		fmt.Sprintf(
			"remote creds: %v, %v", remoteCred.Username, remoteCred.Available,
		),
	)

	dcrlog.Info("Probing cluster topology")

	clustertopology := topologyfinder.TopologyFinder{}
	clustertopology.MongoshCapture.S = &cred
	err = clustertopology.GetAllNodes()
	if err != nil {
		dcrlog.Error(fmt.Sprintf("Error in Topology finding: %s", err.Error()))
		log.Fatal("Error in Topology finding cannot proceed aborting:", err)
	}

	for _, host := range clustertopology.Allnodes.Nodes {

		dcrlog.Info(fmt.Sprintf("host: %s, port: %d", host.Hostname, host.Port))

		//determine if the data collection should abort due to not enough free space
		//we keep approx 1GB as limit
		fsHasFreeSpace, err := hasFreeSpace()
		if err != nil {
			dcrlog.Warn("Warning cannot check free space for data collection.")
			fmt.Println("WARNING: Cannot check free space for data collection monitor free space e.g. df -h output")
		} else {
			if !fsHasFreeSpace {
				log.Fatal("aborting because not enough free space for data collection to continue")
			}
		}

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

		dcrlog.Info("Running getMongoData/mongoWellnessChecker")
		err = c.RunMongoShellWithEval()
		if err != nil {
			dcrlog.Error(fmt.Sprintf("Error Running getMongoData %v", err))
			//log.Fatal("Error Running getMongoData ", err)
		}

		isLocalHost := false
		var errtest error

		hostname := host.Hostname
		isLocalHost, errtest = isHostnameALocalHost(hostname)
		if errtest != nil {
			dcrlog.Error(
				fmt.Sprintf(
					"Error determining if Hostname is a LocalHost or not. Assuming Remote node: %v",
					errtest,
				),
			)
			//log.Fatal("Error determining if Hostname is a LocalHost or not :", errtest)
		}

		if isLocalHost {
			dcrlog.Info(
				fmt.Sprintf("%s is a local hostname. Performing Local Copying.", hostname),
			)

			dcrlog.Info("Running FTDC Archiving")
			ftdcarchive := ftdcarchiver.FTDCarchive{}
			ftdcarchive.Mongo.S = &cred
			ftdcarchive.Outputdir = &outputdir
			err = ftdcarchive.Start()
			if err != nil {
				dcrlog.Error(fmt.Sprintf("Error in FTDCArchive: %v", err))
				//log.Fatal("Error in FTDCArchive: ", err)
			}

			dcrlog.Info("Running mongo log Archiving")
			logarchive := mongologarchiver.MongoDLogarchive{}
			logarchive.Mongo.S = &cred
			logarchive.Outputdir = &outputdir
			logarchive.Dcrlog = &dcrlog
			err = logarchive.Start()
			if err != nil {
				dcrlog.Error(fmt.Sprintf("Error in LogArchive: %v", err))
				//log.Fatal("Error in LogArchive:", err)
			}

		} else {
			if remoteCred.Available {
				dcrlog.Info(fmt.Sprintf("%s is not a local hostname. Proceeding with remote Copier.", hostname))

				remotecopyJob := fscopy.FSCopyJob{}
				remotecopyJob.Dcrlog = &dcrlog

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
					dcrlog.Error(
						"Error creating temp output Directory for storing remote DCR outputs",
					)
					log.Fatal(
						"Error creating temp output Directory for storing remote DCR outputs",
					)
				}

				remoteFTDCArchiver.TempOutputdir = &tempdir
				remoteFTDCArchiver.RemoteCopyJob.Src.IsLocal = false
				remoteFTDCArchiver.RemoteCopyJob.Src.Username = []byte(remoteCred.Username)
				remoteFTDCArchiver.RemoteCopyJob.Src.Hostname = []byte(cred.Currentmongodhost)

				var buffer bytes.Buffer
				remoteFTDCArchiver.RemoteCopyJob.Output = &buffer

				remoteFTDCArchiver.RemoteCopyJob.Dst.Path = []byte(
					remoteFTDCArchiver.TempOutputdir.Path(),
				)

				err = remoteFTDCArchiver.Start()
				if err != nil {
					dcrlog.Error(fmt.Sprintf("Error in Remote FTDC Archive for this node: %v", err))
					//log.Fatal("Error in Remote FTDC Archive: ", err)
				}

				dcrlog.Debug(fmt.Sprintf("remote copy job output %s:", buffer.String()))
				remotecopyJob.Output.Reset()

				remotecopyJobWithPattern := fscopy.FSCopyJobWithPattern{}
				remotecopyJobWithPattern.Dcrlog = &dcrlog
				remotecopyJobWithPattern.CopyJobDetails = &remotecopyJob

				dcrlog.Info("Running mongo log Archiving")
				remoteLogArchiver := mongologarchiver.RemoteMongoDLogarchive{}
				remoteLogArchiver.RemoteCopyJob = &remotecopyJobWithPattern
				remoteLogArchiver.Mongo.S = &cred
				remoteLogArchiver.Outputdir = &outputdir
				remoteLogArchiver.TempOutputdir = &tempdir
				remoteLogArchiver.Dcrlog = &dcrlog

				err = remoteLogArchiver.Start()
				if err != nil {
					dcrlog.Error(fmt.Sprintf("Error in Remote Log Archive for this node: %v", err))
					//log.Fatal("Error in Remote Log Archive: ", err)
				}
				dcrlog.Debug(fmt.Sprintf("remote copy job output %s:", buffer.String()))
				remotecopyJob.Output.Reset()
			}

		}

	}

	s.Stop()

	fmt.Println("Data collection completed outputs directory location: ", outputdir.OutputPrefix)
	dcrlog.Info("---End of Script Execution----")
}

func hasFreeSpace() (bool, error) {
	processwd, err := os.Getwd()
	if err != nil {
		return false, err
	}

	var fsstat syscall.Statfs_t
	if err := syscall.Statfs(processwd, &fsstat); err != nil {
		return false, err
	}

	freeSpaceOnFSInGB := float64(fsstat.Bavail*uint64(fsstat.Bsize)) / (1024 * 1024 * 1024)

	if freeSpaceOnFSInGB < 1.1 {
		return false, nil
	}

	return true, nil

}

func getListOfHostIPsForHostname(hostname string) ([]net.IP, error) {
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
			if resolvedIP.Equal(localIP) {
				return true, nil
			}
		}
	}

	return false, nil
}
