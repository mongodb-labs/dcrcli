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
	"errors"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"net"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/briandowns/spinner"
	"golang.org/x/term"

	"dcrcli/collectnodes"
	"dcrcli/dcrconfig"
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

// isMongoNodeAlive checks if a MongoDB node is reachable at the specified hostname and port.
// It attempts to establish a TCP connection to the given address within a 5-second timeout.
// Parameters:
// - hostname: The hostname or IP address of the MongoDB node.
// - port: The port number on which the MongoDB node is listening.
// Returns:
// - bool: True if the node is reachable, false otherwise.
// - error: An error object if the connection attempt fails.
func isMongoNodeAlive(hostname string, port int) (bool, error) {
	// Attempt to connect to the MongoDB host on the specified port
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(hostname, strconv.Itoa(port)), 5*time.Second)
	if err != nil {
		return false, err
	}
	conn.Close()
	return true, nil
}

// checkAllNodesAlive probes every supplied cluster node with isMongoNodeAlive and
// returns the subset of nodes that did not respond, along with the last connection
// error observed. Probes are sequential to match the conservative, low-footprint
// network behaviour of the rest of dcrcli.
// Each probe result is logged: Debug for reachable nodes, Error for unreachable.
// Parameters:
// - nodes: All cluster nodes discovered by the topology finder.
// - dcrlog: Logger used to record per-node probe results.
// Returns:
// - []topologyfinder.ClusterNode: Nodes that failed the TCP probe.
// - error: The last connection error observed while probing; informational only and may be nil even when unreachable nodes are present.
func checkAllNodesAlive(
	nodes []topologyfinder.ClusterNode,
	dcrlog *dcrlogger.DCRLogger,
) ([]topologyfinder.ClusterNode, error) {
	unhealthy := make([]topologyfinder.ClusterNode, 0)
	var lastErr error

	for _, n := range nodes {
		alive, err := isMongoNodeAlive(n.Hostname, n.Port)
		if !alive {
			if err != nil {
				lastErr = err
				dcrlog.Error(
					fmt.Sprintf(
						"Health check: MongoDB node %s:%d is unreachable: %v",
						n.Hostname, n.Port, err,
					),
				)
			} else {
				dcrlog.Error(
					fmt.Sprintf(
						"Health check: MongoDB node %s:%d is unreachable",
						n.Hostname, n.Port,
					),
				)
			}
			unhealthy = append(unhealthy, n)
			continue
		}
		dcrlog.Debug(
			fmt.Sprintf("Health check: MongoDB node %s:%d is reachable", n.Hostname, n.Port),
		)
	}

	return unhealthy, lastErr
}

// abortIfAnyNodeUnhealthy probes every cluster node and terminates the run when any
// node is unreachable. dcrcli collects diagnostic data via getMongoData against live
// (typically production) clusters; proceeding while a member is already down risks
// further degrading availability. The gate is invoked once before the per-target
// collection loop starts and again at the top of every iteration so that
// degradations occurring mid-run also stop the tool.
// Parameters:
// - nodes: All cluster nodes discovered by the topology finder.
// - phase: Short label included in log/console messages (e.g. "pre-collection", "pre-iteration") used to disambiguate where the gate fired.
// - dcrlog: Logger used to emit health-check progress and failure details.
func abortIfAnyNodeUnhealthy(
	nodes []topologyfinder.ClusterNode,
	phase string,
	dcrlog *dcrlogger.DCRLogger,
) {
	dcrlog.Info(
		fmt.Sprintf("Health check (%s): probing %d cluster node(s)", phase, len(nodes)),
	)

	unhealthy, lastErr := checkAllNodesAlive(nodes, dcrlog)
	if len(unhealthy) == 0 {
		dcrlog.Info(
			fmt.Sprintf("Health check (%s): all %d node(s) reachable", phase, len(nodes)),
		)
		return
	}

	fmt.Printf("\n")
	fmt.Println("######################################################################")
	fmt.Println("#                                 ERROR                              #")
	fmt.Println("######################################################################")
	fmt.Printf("\nCluster health check failed (%s).\n", phase)
	fmt.Println("The following MongoDB node(s) are unreachable:")
	for _, u := range unhealthy {
		fmt.Printf("  - %s:%d\n", u.Hostname, u.Port)
	}
	fmt.Println()
	fmt.Println(
		"dcrcli runs getMongoData against live clusters; refusing to proceed while any cluster node is down to avoid added production risk.",
	)
	fmt.Println("Verify all members are healthy (e.g. rs.status()) and retry.")
	if lastErr != nil {
		fmt.Printf("Last connection error: %v\n", lastErr)
	}
	fmt.Println()

	dcrlog.Error(
		fmt.Sprintf(
			"Terminating DCR-CLI execution: %d cluster node(s) unhealthy during %s health check",
			len(unhealthy), phase,
		),
	)
	os.Exit(1)
}

func main() {
	var err error

	collectNodesFlag := flag.String(
		"collect-nodes",
		"",
		`Which members to collect from: "one-secondary" (default when non-interactive; one SECONDARY only), "all-secondaries" (every SECONDARY; if sharded, also one mongos and one config server), or "all-nodes" (every discovered host: all mongods, all mongos, all config). If omitted and stdin is a terminal, you are prompted.`,
	)
	configFile := flag.String(
		"config",
		"",
		"Path to a JSON config file with connection details. Use -generate-config to create a sample.",
	)
	generateConfig := flag.String(
		"generate-config",
		"",
		"Write a sample config file to the given path and exit. Example: ./dcrcli -generate-config dcrcli.config.json",
	)
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Discover MongoDB cluster nodes from a seed and collect diagnostic data (getMongoData, FTDC, logs).\n")
		fmt.Fprintf(os.Stderr, "By default a single SECONDARY is collected only; use -collect-nodes for all-secondaries (adds one mongos + one config when sharded) or all-nodes.\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if *generateConfig != "" {
		if err := dcrconfig.GenerateSample(*generateConfig); err != nil {
			log.Fatal("Failed to write sample config file:", err)
		}
		fmt.Println("Sample config written to:", *generateConfig)
		fmt.Println()
		fmt.Println("Fields:")
		fmt.Println("  cluster_name   — display name for the output directory")
		fmt.Println("  seed_host      — hostname or IP of a seed mongod/mongos")
		fmt.Println("  seed_port      — port of the seed node (default 27017)")
		fmt.Println("  username       — MongoDB admin username (blank = no auth)")
		fmt.Println("  password       — MongoDB admin password (blank = no auth)")
		fmt.Println("  uri_options    — extra URI options e.g. tls=true (no replicaSet)")
		fmt.Println("  ssh_username   — OS user for passwordless SSH to remote nodes (blank = all local)")
		fmt.Println("  collect_nodes  — one-secondary | all-secondaries | all-nodes (blank = prompt)")
		os.Exit(0)
	}

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

	remoteCred := fscopy.RemoteCred{}
	remoteCred.Dcrlog = &dcrlog

	// collectModeStr merges the -collect-nodes flag with any value from the config file.
	// A CLI flag always wins; config value is used when no flag is given.
	collectModeStr := *collectNodesFlag

	if *configFile != "" {
		cfg, err := dcrconfig.Load(*configFile)
		if err != nil {
			dcrlog.Error(err.Error())
			log.Fatal("Failed to load config file:", err)
		}

		fmt.Println("Loading config from:", *configFile)
		fmt.Printf("  cluster_name:  %s\n", cfg.ClusterName)
		fmt.Printf("  seed_host:     %s\n", cfg.SeedHost)
		fmt.Printf("  seed_port:     %s\n", cfg.SeedPort)
		if cfg.Username != "" {
			fmt.Printf("  username:      %s\n", cfg.Username)
		} else {
			fmt.Println("  username:      (none — no-auth cluster)")
		}
		if cfg.Username != "" {
			fmt.Println("  password:      [will prompt interactively]")
		} else {
			fmt.Println("  password:      (none — no-auth cluster)")
		}
		if cfg.URIOptions != "" {
			fmt.Printf("  uri_options:   %s\n", cfg.URIOptions)
		} else {
			fmt.Println("  uri_options:   (none)")
		}
		if cfg.SSHUsername != "" {
			fmt.Printf("  ssh_username:  %s\n", cfg.SSHUsername)
		} else {
			fmt.Println("  ssh_username:  (none — all nodes treated as local)")
		}
		if cfg.CollectNodes != "" {
			fmt.Printf("  collect_nodes: %s\n", cfg.CollectNodes)
		} else {
			fmt.Println("  collect_nodes: (will prompt interactively)")
		}
		fmt.Println()

		if err := cred.GetFromConfig(cfg); err != nil {
			dcrlog.Error(err.Error())
			fmt.Println()
			fmt.Println("Config validation failed:", err)
			fmt.Println("Fix the value in", *configFile, "and re-run.")
			os.Exit(1)
		}

		remoteCred.GetFromConfig(cfg)

		if collectModeStr == "" {
			collectModeStr = cfg.CollectNodes
		}
	} else {
		err = cred.Get()
		if err != nil {
			dcrlog.Error(err.Error())
			log.Fatal("Error while getting DB credentials aborting!")
		}
		remoteCred.Get()
	}

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
	clustertopology.Dcrlog = &dcrlog

	clustertopology.MongoshCapture.S = &cred

	// discover all nodes of cluster
	err = clustertopology.GetAllNodes()
	if err != nil {
		dcrlog.Error(fmt.Sprintf("Error in Topology finding: %s", err.Error()))
		log.Fatal("Error in Topology finding cannot proceed aborting:", err)
	}

	// dedup any mongo node entries because public/private hostnames point to same ip
	err = clustertopology.KeepUniqueNodes()
	if err != nil {
		dcrlog.Warn(fmt.Sprintf("Unable to filter for unique hostnames non-fatal: %s", err.Error()))
	}

	err = clustertopology.ResolveReplicaStates()
	if err != nil {
		dcrlog.Warn(fmt.Sprintf("Could not fully resolve replica roles (collection may be limited): %s", err.Error()))
	}

	// Stop spinner so terminal echo and the collect-nodes prompt are visible (spinner redraw would hide input).
	s.Stop()
	fmt.Println()

	isTerm := term.IsTerminal(int(syscall.Stdin))
	collectMode, err := collectnodes.ResolveMode(collectModeStr, isTerm, os.Stdin, os.Stdout)
	if err != nil {
		dcrlog.Error(err.Error())
		log.Fatal("Invalid collection scope:", err)
	}

	collectTargets, err := collectnodes.Select(clustertopology.Allnodes.Nodes, collectMode)
	if err != nil {
		nodes := clustertopology.Allnodes.Nodes
		if errors.Is(err, collectnodes.ErrNoSecondaries) &&
			collectMode != collectnodes.ModeAllNodes &&
			isTerm &&
			strings.TrimSpace(collectModeStr) == "" &&
			collectnodes.LooksLikeStandaloneMongod(nodes) {
			fmt.Println()
			fmt.Println("WARNING: A single MongoDB node was discovered and it is not a secondary (typical standalone).")
			fmt.Println("Options 1 and 2 normally avoid primaries; standalone has no secondary to collect from.")
			ok, perr := collectnodes.PromptStandaloneCollectPrimary(os.Stdin, os.Stdout)
			if perr != nil {
				dcrlog.Error(perr.Error())
				log.Fatal(perr)
			}
			if ok {
				collectTargets = collectnodes.StandalonePrimaryTargets(nodes)
				dcrlog.Info("User confirmed collection from standalone primary")
				fmt.Println("Proceeding: data will be collected from this primary (standalone).")
				fmt.Println()
			} else {
				log.Fatal("Aborted. For standalone use option 3 (all nodes), or pass --collect-nodes=all-nodes, or add a replica set secondary.")
			}
		} else {
			dcrlog.Error(err.Error())
			if errors.Is(err, collectnodes.ErrNoSecondaries) &&
				strings.TrimSpace(collectModeStr) != "" &&
				collectnodes.LooksLikeStandaloneMongod(nodes) {
				log.Fatal("No secondaries (standalone?). Use --collect-nodes=all-nodes, or run interactively without -collect-nodes to confirm primary collection.")
			}
			if errors.Is(err, collectnodes.ErrNoSecondaries) &&
				!isTerm &&
				collectnodes.LooksLikeStandaloneMongod(nodes) {
				log.Fatal("No secondaries (standalone?). Non-interactive run: use --collect-nodes=all-nodes.")
			}
			log.Fatal(err)
		}
	}

	dcrlog.Info(
		fmt.Sprintf(
			"Collect scope: mode=%s, %d target node(s) (from -collect-nodes or interactive prompt)",
			collectMode.String(),
			len(collectTargets),
		),
	)
	fmt.Printf(
		"\nCollecting from %d node(s); scope %s — %s\n",
		len(collectTargets),
		collectMode.String(),
		collectMode.Description(),
	)
	for _, t := range collectTargets {
		dcrlog.Info(fmt.Sprintf("Collection target: %s:%d (%s)", t.Hostname, t.Port, t.ReplicaState))
	}

	// Pre-collection cluster-wide health gate: refuse to start data collection if any
	// member of the discovered topology is already unreachable. getMongoData is run
	// against live (typically production) clusters, so taking on additional risk while
	// a node is down is unacceptable.
	abortIfAnyNodeUnhealthy(clustertopology.Allnodes.Nodes, "pre-collection", &dcrlog)

	s.Start()

	for _, host := range collectTargets {

		// Per-iteration cluster-wide health gate: re-probe every node before moving on
		// to the next collection target so we never stack additional load on a cluster
		// that has degraded mid-run.
		abortIfAnyNodeUnhealthy(clustertopology.Allnodes.Nodes, "pre-iteration", &dcrlog)

		dcrlog.Info(fmt.Sprintf("Collecting logs for MongoDB node - host: %s, port: %d", host.Hostname, host.Port))
		fmt.Printf("\nCollecting logs for MongoDB node %s:%d\n", host.Hostname, host.Port)
		// determine if the data collection should abort due to not enough free space
		// we keep approx 1GB as limit
		fsHasFreeSpace, err := hasFreeSpace()
		if err != nil {
			dcrlog.Warn("Warning cannot check free space for data collection.")
			fmt.Println(
				"WARNING: Cannot check free space for data collection monitor free space e.g. df -h output",
			)
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

		isAliveBefore, err := isMongoNodeAlive(host.Hostname, host.Port)
		if err != nil {
			dcrlog.Error(fmt.Sprintf("Error checking if host: %s, port: %d is alive: \n %v", host.Hostname, host.Port, err))
		}

		c := mongosh.CaptureGetMongoData{}
		c.S = &cred
		c.Outputdir = &outputdir

		dcrlog.Info("Running getMongoData/mongoWellnessChecker")
		err = c.RunMongoShellWithEval()
		if err != nil {
			dcrlog.Error(fmt.Sprintf("Error Running getMongoData %v", err))
		}

		isAliveAfter, err := isMongoNodeAlive(host.Hostname, host.Port)

		if !isAliveAfter && isAliveBefore {
			dcrlog.Error(fmt.Sprintf("MongoDB node %s:%d became unreachable after collecting getMongoData.\n %v", host.Hostname, host.Port, err))

			fmt.Printf("\n")
			fmt.Println("######################################################################")
			fmt.Println("#                                 ERROR                              #")
			fmt.Println("######################################################################")
			fmt.Printf("\nMongoDB node %s:%d is unreachable post getMongoData collection.\nTerminating the execution!\n\n", host.Hostname, host.Port)

			dcrlog.Error("Terminating DCR-CLI execution")
			os.Exit(1)

		} else {
			dcrlog.Info(fmt.Sprintf("MongoDB node %s:%d is reachable after collecting getMongoData...", host.Hostname, host.Port))
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
			// log.Fatal("Error determining if Hostname is a LocalHost or not :", errtest)
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
				// log.Fatal("Error in FTDCArchive: ", err)
			}

			dcrlog.Info("Running mongo log Archiving")
			logarchive := mongologarchiver.MongoDLogarchive{}
			logarchive.Mongo.S = &cred
			logarchive.Outputdir = &outputdir
			logarchive.Dcrlog = &dcrlog
			err = logarchive.Start()
			if err != nil {
				dcrlog.Error(fmt.Sprintf("Error in LogArchive: %v", err))
				// log.Fatal("Error in LogArchive:", err)
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
					// log.Fatal("Error in Remote FTDC Archive: ", err)
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
					// log.Fatal("Error in Remote Log Archive: ", err)
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
