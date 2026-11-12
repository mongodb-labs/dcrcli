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

package webui

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"dcrcli/collectdata"
	"dcrcli/dcroutdir"
	"dcrcli/diagcommands"
	"dcrcli/fscopy"
	"dcrcli/ftdcarchiver"
	"dcrcli/mongocredentials"
	"dcrcli/mongologarchiver"
	"dcrcli/mongosh"
	"dcrcli/sshctl"
	"dcrcli/topologyfinder"
)

const collectionTasksPerNode = 4

var collectionTaskNames = [collectionTasksPerNode]string{"getMongoData", "FTDC", "logs", "commands"}

var prepareSSHControlPath = sshctl.PreparePath

type collectJob struct {
	cred           mongocredentials.Mongocredentials
	allNodes       []topologyfinder.ClusterNode
	targets        []topologyfinder.ClusterNode
	data           collectdata.Selection
	sshUser        string
	maxCollections int
}

type runProgress struct {
	nodes     []nodeJSON
	done      int
	total     int
	outputDir string
}

func (p *runProgress) percent() int {
	if p.total <= 0 {
		return 0
	}
	pct := p.done * 100 / p.total
	if pct > 100 {
		pct = 100
	}
	return pct
}

func (p *runProgress) setTask(nodeIdx, taskIdx int, state, note string) {
	if nodeIdx < 0 || nodeIdx >= len(p.nodes) || taskIdx < 0 || taskIdx >= collectionTasksPerNode {
		return
	}
	p.nodes[nodeIdx].Tasks[taskIdx].State = state
	p.nodes[nodeIdx].Tasks[taskIdx].Note = note
}

func (p *runProgress) derive(nodeIdx int) {
	if nodeIdx < 0 || nodeIdx >= len(p.nodes) {
		return
	}
	hasFail, hasOK, hasSkip, attempted := false, false, false, false
	for _, t := range p.nodes[nodeIdx].Tasks {
		switch t.State {
		case "failed":
			hasFail = true
			attempted = true
		case "skipped":
			hasSkip = true
			attempted = true
		case "ok":
			hasOK = true
			attempted = true
		}
	}
	state := "pending"
	switch {
	case !attempted:
		state = "pending"
	case (hasFail || hasSkip) && hasOK:
		state = "partial"
	case hasFail || hasSkip:
		state = "failed"
	default:
		state = "done"
	}
	p.nodes[nodeIdx].State = state
}

func newRunProgress(targets []topologyfinder.ClusterNode, outputDir string) runProgress {
	nodes := make([]nodeJSON, len(targets))
	for i, t := range targets {
		tasks := make([]taskJSON, collectionTasksPerNode)
		for n := 0; n < collectionTasksPerNode; n++ {
			tasks[n] = taskJSON{Name: collectionTaskNames[n], State: "pending"}
		}
		nodes[i] = nodeJSON{
			Hostname: t.Hostname,
			Port:     t.Port,
			Role:     t.ReplicaState,
			Shard:    t.ShardMapHostRole,
			State:    "pending",
			Tasks:    tasks,
		}
	}
	return runProgress{
		nodes:     nodes,
		total:     len(targets) * collectionTasksPerNode,
		outputDir: outputDir,
	}
}

func (a *App) emitProgress(p *runProgress, status, message string) {
	a.emit(Event{
		Type:      "snapshot",
		Status:    status,
		Message:   message,
		Percent:   p.percent(),
		OutputDir: p.outputDir,
		Nodes:     cloneNodes(p.nodes),
	})
}

func (a *App) runCollect(job collectJob) error {
	return a.runCollectContext(context.Background(), job)
}

func (a *App) runCollectContext(ctx context.Context, job collectJob) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := mongosh.SetMaxCollections(job.maxCollections); err != nil {
		return err
	}
	cred := job.cred
	outputPrefix := checkEmptyDirectory("./outputs/" + cred.Clustername + "/")
	displayOutput, err := outputDisplayPath(outputPrefix)
	if err != nil {
		return fmt.Errorf("resolve output directory: %w", err)
	}
	prog := newRunProgress(job.targets, displayOutput)
	a.mu.Lock()
	a.outputDir = displayOutput
	a.mu.Unlock()

	a.logInfo(fmt.Sprintf("DCR outputs directory: %s", displayOutput))
	a.logInfo(fmt.Sprintf("Collect data: %s (%s)", job.data.String(), job.data.Description()))
	a.emitProgress(&prog, "running", fmt.Sprintf("Collecting from %d node(s)", len(job.targets)))

	if err := unhealthyErrorContext(ctx, job.allNodes, "pre-collection", a.log); err != nil {
		a.logErr(err.Error())
		a.emitProgress(&prog, "error", err.Error())
		return err
	}

	remote := fscopy.RemoteCred{Dcrlog: a.log, Username: job.sshUser, Available: job.sshUser != "" && job.data.NeedsSSH()}
	if !job.data.NeedsSSH() {
		remote.Available = false
		a.logInfo("SSH not required for the selected artifacts")
	}

	for i, host := range job.targets {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := unhealthyErrorContext(ctx, job.allNodes, "pre-iteration", a.log); err != nil {
			a.logErr(err.Error())
			a.emitProgress(&prog, "error", err.Error())
			return err
		}

		prog.nodes[i].State = "active"
		a.emitProgress(&prog, "running", fmt.Sprintf("Collecting %s:%d", host.Hostname, host.Port))
		a.logInfo(fmt.Sprintf("Collecting MongoDB node %s:%d (%s)", host.Hostname, host.Port, host.ReplicaState))

		free, err := hasFreeSpace()
		if err != nil {
			a.logWarn("Cannot check free space for data collection")
		} else if !free {
			msg := "not enough free space for data collection to continue (need about 1.1 GB free)"
			a.logErr(msg)
			a.emitProgress(&prog, "error", msg)
			return fmt.Errorf("%s", msg)
		}

		cred.Currentmongodhost = host.Hostname
		cred.Currentmongodport = strconv.Itoa(host.Port)
		if err := cred.SetMongoURI(); err != nil {
			a.logErr(err.Error())
			a.emitProgress(&prog, "error", err.Error())
			return err
		}

		outputdir := dcroutdir.DCROutputDir{
			OutputPrefix: outputPrefix,
			Hostname:     cred.Currentmongodhost,
			Port:         cred.Currentmongodport,
		}
		if err := outputdir.CreateDCROutputDir(); err != nil {
			a.logErr("Error creating output directory")
			a.emitProgress(&prog, "error", "Error creating output directory")
			return err
		}

		isAliveBefore, aliveErr := isMongoNodeAliveContext(ctx, host.Hostname, host.Port)
		if aliveErr != nil {
			a.logErr(fmt.Sprintf("Error checking if %s:%d is alive: %v", host.Hostname, host.Port, aliveErr))
		}

		if job.data.GetMongoData {
			prog.setTask(i, 0, "running", "")
			a.emitProgress(&prog, "running", fmt.Sprintf("getMongoData on %s:%d", host.Hostname, host.Port))
			c := mongosh.CaptureGetMongoData{Context: ctx, S: &cred, Outputdir: &outputdir}
			err = c.RunMongoShellWithEval()
			if err != nil {
				a.logErr(fmt.Sprintf("Error running getMongoData: %v", err))
				prog.setTask(i, 0, "failed", err.Error())
			} else {
				prog.setTask(i, 0, "ok", "")
			}
			prog.done++
			a.emitProgress(&prog, "running", "")

			isAliveAfter, afterErr := isMongoNodeAliveContext(ctx, host.Hostname, host.Port)
			if !isAliveAfter && isAliveBefore {
				msg := fmt.Sprintf("MongoDB node %s:%d is unreachable after getMongoData collection.", host.Hostname, host.Port)
				if afterErr != nil {
					msg += "\n" + afterErr.Error()
				}
				a.logErr(msg)
				a.emitProgress(&prog, "error", msg)
				return fmt.Errorf("%s", msg)
			}
		} else {
			a.logInfo("Skipping getMongoData (not selected)")
			prog.setTask(i, 0, "omitted", "not selected")
			prog.done++
		}

		nodeRemote := remote
		sshOpts, sshCleanup, sshErr := a.prepareSSHContext(ctx, host, remote)
		var cleanupOnce sync.Once
		cleanupSSH := func() { cleanupOnce.Do(sshCleanup) }
		defer cleanupSSH()
		if sshErr != nil {
			a.logErr("SSH connection failed: " + sshErr.Error())
			nodeRemote.Available = false
		}
		if sshErr != nil && (job.data.FTDC || job.data.Logs) {
			note := "SSH connection failed: " + sshErr.Error()
			for t, selected := range map[int]bool{1: job.data.FTDC, 2: job.data.Logs} {
				if selected {
					prog.setTask(i, t, "failed", note)
				} else {
					prog.setTask(i, t, "omitted", "not selected")
				}
			}
			prog.done += 2
		} else if err := a.collectFiles(ctx, &prog, i, &cred, &outputdir, host, nodeRemote, job.data, sshOpts); err != nil {
			a.emitProgress(&prog, "error", err.Error())
			return err
		}

		if job.data.Commands {
			prog.setTask(i, 3, "running", "")
			a.emitProgress(&prog, "running", fmt.Sprintf("commands on %s:%d", host.Hostname, host.Port))
			commandErr := commandTaskError(sshErr, a.collectCommands(ctx, &cred, &outputdir, host, nodeRemote, sshOpts))
			if commandErr != nil {
				a.logErr(fmt.Sprintf("Error collecting diagnostic commands: %v", commandErr))
				prog.setTask(i, 3, "failed", commandErr.Error())
			} else {
				prog.setTask(i, 3, "ok", "")
			}
		} else {
			a.logInfo("Skipping diagnostic commands (not selected)")
			prog.setTask(i, 3, "omitted", "not selected")
		}
		prog.done++
		prog.derive(i)
		cleanupSSH()
		a.emitProgress(&prog, "running", "")
	}

	prog.done = prog.total
	a.logInfo("Data collection complete")
	a.emitProgress(&prog, "done", "Data collection complete")
	return nil
}

// prepareSSH opens one shared SSH connection to a remote node. Any password or
// host-key prompt is answered in the browser at this point, once per node.
func (a *App) prepareSSH(host topologyfinder.ClusterNode, remote fscopy.RemoteCred) (opts []string, cleanup func(), err error) {
	return a.prepareSSHContext(context.Background(), host, remote)
}

func (a *App) prepareSSHContext(ctx context.Context, host topologyfinder.ClusterNode, remote fscopy.RemoteCred) (opts []string, cleanup func(), err error) {
	cleanup = func() {}
	if remote.Username == "" || !remote.Available {
		return nil, cleanup, nil
	}
	isLocalHost, lookErr := isHostnameALocalHost(host.Hostname)
	if lookErr != nil {
		a.logErr(fmt.Sprintf("Error determining if hostname is local. Assuming remote node: %v", lookErr))
	}
	if isLocalHost {
		return nil, cleanup, nil
	}
	path, muxCleanup, muxErr := prepareSSHControlPath()
	if muxErr != nil {
		a.logWarn("SSH connection sharing unavailable: " + muxErr.Error())
		return nil, cleanup, nil
	}
	if path == "" {
		return nil, cleanup, nil
	}
	mux := sshctl.MuxArgs(path)
	a.setSSHContext(fmt.Sprintf("%s@%s, for FTDC, logs, and host commands on %s:%d", remote.Username, host.Hostname, host.Hostname, host.Port))
	defer a.setSSHContext("")
	a.logInfo(fmt.Sprintf("Opening SSH connection to %s@%s", remote.Username, host.Hostname))
	if err := openSSHMasterContext(ctx, remote.Username, host.Hostname, mux); err != nil {
		exitCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		sshctl.ExitContext(exitCtx, remote.Username, host.Hostname, path)
		cancel()
		if muxCleanup != nil {
			muxCleanup()
		}
		return nil, cleanup, err
	}
	a.logInfo("SSH connected. FTDC, logs, and host commands reuse this connection.")
	return mux, func() {
		exitCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		sshctl.ExitContext(exitCtx, remote.Username, host.Hostname, path)
		if muxCleanup != nil {
			muxCleanup()
		}
	}, nil
}

func commandTaskError(sshErr, commandErr error) error {
	if sshErr == nil {
		return commandErr
	}
	return errors.Join(fmt.Errorf("SSH connection failed: %w", sshErr), commandErr)
}

func outputDisplayPath(path string) (string, error) {
	return filepath.Abs(path)
}

func (a *App) collectFiles(
	ctx context.Context,
	prog *runProgress,
	idx int,
	cred *mongocredentials.Mongocredentials,
	outputdir *dcroutdir.DCROutputDir,
	host topologyfinder.ClusterNode,
	remote fscopy.RemoteCred,
	data collectdata.Selection,
	sshOpts []string,
) error {
	isLocalHost, err := isHostnameALocalHost(host.Hostname)
	if err != nil {
		a.logErr(fmt.Sprintf("Error determining if hostname is local. Assuming remote node: %v", err))
	}

	if !data.FTDC && !data.Logs {
		a.logInfo("Skipping FTDC and mongod logs (not selected)")
		prog.setTask(idx, 1, "omitted", "not selected")
		prog.setTask(idx, 2, "omitted", "not selected")
		prog.done += 2
		return nil
	}

	if isLocalHost {
		a.logInfo(fmt.Sprintf("%s is a local hostname. Performing local copy.", host.Hostname))
		if data.FTDC {
			prog.setTask(idx, 1, "running", "")
			a.emitProgress(prog, "running", fmt.Sprintf("FTDC on %s:%d", host.Hostname, host.Port))
			ftdcarchive := ftdcarchiver.FTDCarchive{Mongo: mongosh.CaptureGetMongoData{Context: ctx, S: cred}, Outputdir: outputdir}
			if err := ftdcarchive.Start(); err != nil {
				a.logErr(fmt.Sprintf("Error in FTDC archive: %v", err))
				prog.setTask(idx, 1, "failed", err.Error())
			} else {
				prog.setTask(idx, 1, "ok", "")
			}
		} else {
			prog.setTask(idx, 1, "omitted", "not selected")
		}
		prog.done++

		if data.Logs {
			prog.setTask(idx, 2, "running", "")
			a.emitProgress(prog, "running", fmt.Sprintf("logs on %s:%d", host.Hostname, host.Port))
			logarchive := mongologarchiver.MongoDLogarchive{
				Mongo:     mongosh.CaptureGetMongoData{Context: ctx, S: cred},
				Outputdir: outputdir,
				Dcrlog:    a.log,
			}
			if err := logarchive.Start(); err != nil {
				a.logErr(fmt.Sprintf("Error in log archive: %v", err))
				prog.setTask(idx, 2, "failed", err.Error())
			} else {
				prog.setTask(idx, 2, "ok", "")
			}
		} else {
			prog.setTask(idx, 2, "omitted", "not selected")
		}
		prog.done++
		return nil
	}

	if !remote.Available {
		skipped := remoteCopySkippedLabels(data.FTDC, data.Logs)
		a.logWarn(fmt.Sprintf("%s is not on this host and no SSH username was set; skipping %s", host.Hostname, skipped))
		if data.FTDC {
			prog.setTask(idx, 1, "skipped", "node is not local and no SSH user was set")
		} else {
			prog.setTask(idx, 1, "omitted", "not selected")
		}
		if data.Logs {
			prog.setTask(idx, 2, "skipped", "node is not local and no SSH user was set")
		} else {
			prog.setTask(idx, 2, "omitted", "not selected")
		}
		prog.done += 2
		return nil
	}

	a.logInfo(fmt.Sprintf("%s is not a local hostname. Proceeding with remote copy.", host.Hostname))
	a.logInfo("Remote copy uses passwordless SSH. The web UI does not prompt for an SSH password.")

	var buffer bytes.Buffer
	tempdir := dcroutdir.DCROutputDir{
		OutputPrefix: "./outputs/temp/" + cred.Clustername + "/",
		Hostname:     cred.Currentmongodhost,
		Port:         cred.Currentmongodport,
	}
	if err := tempdir.CreateDCROutputDir(); err != nil {
		a.logErr("Error creating temp output directory")
		return fmt.Errorf("error creating temp output directory: %w", err)
	}

	remotecopyJob := fscopy.FSCopyJob{
		Context:          ctx,
		Dcrlog:           a.log,
		SSHClientOptions: sshOpts,
		Output:           &buffer,
	}
	remotecopyJob.Src.IsLocal = false
	remotecopyJob.Src.Username = []byte(remote.Username)
	remotecopyJob.Src.Hostname = []byte(cred.Currentmongodhost)
	remotecopyJob.Dst.Path = []byte(tempdir.Path())

	if data.FTDC {
		prog.setTask(idx, 1, "running", "")
		a.emitProgress(prog, "running", fmt.Sprintf("FTDC on %s:%d via SSH", host.Hostname, host.Port))
		remoteFTDC := ftdcarchiver.RemoteFTDCarchive{
			RemoteCopyJob: &remotecopyJob,
			Mongo:         mongosh.CaptureGetMongoData{Context: ctx, S: cred},
			Outputdir:     outputdir,
			TempOutputdir: &tempdir,
		}
		if err := remoteFTDC.Start(); err != nil {
			a.logErr(fmt.Sprintf("Error in remote FTDC archive: %v", err))
			prog.setTask(idx, 1, "failed", err.Error())
		} else {
			prog.setTask(idx, 1, "ok", "")
		}
		buffer.Reset()
	} else {
		prog.setTask(idx, 1, "omitted", "not selected")
	}
	prog.done++

	if data.Logs {
		prog.setTask(idx, 2, "running", "")
		a.emitProgress(prog, "running", fmt.Sprintf("logs on %s:%d via SSH", host.Hostname, host.Port))
		remotecopyJobWithPattern := fscopy.FSCopyJobWithPattern{
			Dcrlog:         a.log,
			CopyJobDetails: &remotecopyJob,
		}
		remoteLog := mongologarchiver.RemoteMongoDLogarchive{
			RemoteCopyJob: &remotecopyJobWithPattern,
			Mongo:         mongosh.CaptureGetMongoData{Context: ctx, S: cred},
			Outputdir:     outputdir,
			TempOutputdir: &tempdir,
			Dcrlog:        a.log,
		}
		if err := remoteLog.Start(); err != nil {
			a.logErr(fmt.Sprintf("Error in remote log archive: %v", err))
			prog.setTask(idx, 2, "failed", err.Error())
		} else {
			prog.setTask(idx, 2, "ok", "")
		}
		buffer.Reset()
	} else {
		prog.setTask(idx, 2, "omitted", "not selected")
	}
	prog.done++
	return nil
}

func (a *App) collectCommands(
	ctx context.Context,
	cred *mongocredentials.Mongocredentials,
	outputdir *dcroutdir.DCROutputDir,
	host topologyfinder.ClusterNode,
	remote fscopy.RemoteCred,
	sshOpts []string,
) error {
	isLocalHost, err := isHostnameALocalHost(host.Hostname)
	if err != nil {
		a.logErr(fmt.Sprintf("Error determining if hostname is local. Assuming remote node: %v", err))
	}
	mongo := mongosh.CaptureGetMongoData{Context: ctx, S: cred, Outputdir: outputdir}
	col := diagcommands.Collector{
		Context:          ctx,
		Mongo:            &mongo,
		Outputdir:        outputdir,
		ReplicaState:     host.ReplicaState,
		ShardMapHostRole: host.ShardMapHostRole,
		IsLocal:          isLocalHost,
		Dcrlog:           a.log,
		SSHClientOptions: sshOpts,
	}
	if !isLocalHost && remote.Available {
		col.SSHUser = remote.Username
		col.SSHHost = cred.Currentmongodhost
	}
	return col.Collect()
}
