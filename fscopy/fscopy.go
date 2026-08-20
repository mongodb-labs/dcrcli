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

package fscopy

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"dcrcli/dcrconfig"
	"dcrcli/dcrlogger"
	"dcrcli/termui"
)

// rsyncVanishedSources is rsync's "Partial transfer due to vanished source files".
// MongoDB rotates diagnostic.data (and sometimes logs) while we copy, so this is
// expected and the files that did transfer are still usable.
const rsyncVanishedSources = 24

func tolerateRsyncError(err error, log *dcrlogger.DCRLogger) error {
	if err == nil {
		return nil
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) && ee.ExitCode() == rsyncVanishedSources {
		if log != nil {
			log.Warn("rsync exit 24 (vanished source files); keeping copied FTDC/log files")
		}
		return nil
	}
	return err
}

type RemoteCred struct {
	Username  string
	Available bool
	Dcrlog    *dcrlogger.DCRLogger
}

func (rc *RemoteCred) Get(ui *termui.UI) error {
	ui.BeginStep("SSH username")
	ui.Note(
		"Only needed when MongoDB runs on other machines — your login there to copy FTDC and log files.",
		"Leave blank if every MongoDB node is on this machine. Help: "+termui.READMEPrerequisitesURL,
	)
	username, err := ui.AskInput("Username")
	if err != nil {
		return err
	}

	rc.Username = username
	rc.Dcrlog.Debug(fmt.Sprintf("passwordless ssh username %s:", rc.Username))

	if rc.Username == "" {
		ui.Warn("SSH username left empty; assuming all nodes are local")
		rc.Available = false
		rc.Dcrlog.Debug("passwordless ssh username left blank assuming all nodes local")
	} else {
		rc.Available = true
		rc.Dcrlog.Debug("passwordless ssh username provided")
		ui.Ok("SSH username: " + rc.Username)
	}

	ui.Blank()
	return nil
}

// GetFromConfig populates the SSH username from a config file instead of an interactive prompt.
func (rc *RemoteCred) GetFromConfig(c *dcrconfig.Config) {
	rc.Username = strings.TrimSpace(c.SSHUsername)
	if rc.Username == "" {
		rc.Available = false
		rc.Dcrlog.Debug("config ssh_username empty, assuming all nodes local")
	} else {
		rc.Available = true
		rc.Dcrlog.Debug(fmt.Sprintf("config ssh_username: %s", rc.Username))
	}
}

type SourceDir struct {
	IsLocal  bool
	Path     []byte
	Hostname []byte
	SyncPort uint
	Username []byte
}

type DestDir struct {
	Path []byte
}

// example rsync -a --include='mongod.log*' --exclude='*' --include='*/' ubuntu@ip-10-0-0-246:/home/ubuntu/testcluster/dbp/ .
type FSCopyJobWithPattern struct {
	CopyJobDetails  *FSCopyJob
	CurrentFileName string
	Dcrlog          *dcrlogger.DCRLogger
}

func (fcjwp *FSCopyJobWithPattern) StartCopyWithPattern() error {
	if fcjwp.CopyJobDetails.Src.IsLocal {
		return fcjwp.StartCopyLocalWithPattern()
	}
	return fcjwp.StartCopyRemoteWithPattern()
}

func (fcjwp *FSCopyJobWithPattern) StartCopyLocalWithPattern() error {
	return nil
}

func (fcjwp *FSCopyJobWithPattern) StartCopyRemoteWithPattern() error {
	var cmd *exec.Cmd

	filepattern := `'` + fcjwp.CurrentFileName + `*` + `'`
	excludepattern := `'` + `*` + `'`

	// we invoke bash shell because the wildcards are interpretted by bash shell not the rsync program
	fcjwp.Dcrlog.Debug(
		fmt.Sprintf(
			"preparing command rsync -az --include=%s --exclude=%s %s@%s:%s/ %s",
			filepattern,
			excludepattern,
			fcjwp.CopyJobDetails.Src.Username,
			fcjwp.CopyJobDetails.Src.Hostname,
			fcjwp.CopyJobDetails.Src.Path,
			fcjwp.CopyJobDetails.Dst.Path,
		),
	)

	cmd = exec.Command(
		"bash",
		"-c",
		fmt.Sprintf(
			"rsync -az --include=%s --exclude=%s %s@%s:%s/ %s",
			filepattern,
			excludepattern,
			fcjwp.CopyJobDetails.Src.Username,
			fcjwp.CopyJobDetails.Src.Hostname,
			fcjwp.CopyJobDetails.Src.Path,
			fcjwp.CopyJobDetails.Dst.Path,
		))

	//commenting out the cmd.Stdout because it is being used to capture the output below.
	//cmd.Stdout = fcjwp.CopyJobDetails.Output

	//Allow user to provide input if needed.
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	//Executing the rsync command
	fcjwp.Dcrlog.Debug("rsync command start")
	err := tolerateRsyncError(cmd.Run(), fcjwp.Dcrlog)
	if err != nil {
		fcjwp.Dcrlog.Debug(
			fmt.Sprintf("StartCopyRemoteWithPattern: error doing remote copy job wait %v", err),
		)
		return fmt.Errorf("StartCopyRemoteWithPattern: error doing remote copy job wait %w", err)
	}
	return nil

	// Removing stderr pipe for now. cmd.StderrPipe() The error produced by the command will appear in real time in the terminal.
	/*	stderr, err := cmd.StderrPipe()
		if err != nil {
			return err
		}

		fcjwp.Dcrlog.Debug("rsync command start")
		err = cmd.Start()
		if err != nil {
			return err
		}

		_, err = io.ReadAll(stderr)
		if err != nil {
			_ = cmd.Process.Kill()
			fcjwp.Dcrlog.Debug(
				fmt.Sprintf("StartCopyRemoteWithPattern: Error reading from stderr pipe %w", err),
			)
			return fmt.Errorf("StartCopyRemoteWithPattern: Error reading from stderr pipe %w", err)
		}

		err = cmd.Wait()
		if err != nil {
			fcjwp.Dcrlog.Debug(
				fmt.Sprintf("StartCopyRemoteWithPattern: error doing remote copy job wait %w", err),
			)
			return fmt.Errorf("StartCopyRemoteWithPattern: error doing remote copy job wait %w", err)
		}
		return nil
	*/
}

// Copy job
// state:
// N - not started
// P - Progressing
// A - Aborted
// C - Completed successfully
type FSCopyJob struct {
	Src    SourceDir
	Dst    DestDir
	State  string
	Output *bytes.Buffer
	Dcrlog *dcrlogger.DCRLogger
}

// currently only run for remote source directories
func (fcj *FSCopyJob) StartCopyRemote() error {
	// var cmd *exec.Cmd

	fcj.Dcrlog.Debug(fmt.Sprintf("preparing command rsync -az %s@%s:%s/ %s",
		fcj.Src.Username,
		fcj.Src.Hostname,
		fcj.Src.Path,
		fcj.Dst.Path))

	cmd := exec.Command(
		"rsync",
		"-az",
		fmt.Sprintf(`%s@%s:%s`,
			fcj.Src.Username,
			fcj.Src.Hostname,
			fcj.Src.Path),
		fmt.Sprintf(`%s`,
			fcj.Dst.Path),
	)

	//cmd.Stdout = fcj.Output
	// Allow user to provide input if needed
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	fcj.Dcrlog.Debug("starting rsync command")
	err := tolerateRsyncError(cmd.Run(), fcj.Dcrlog)
	if err != nil {
		fcj.Dcrlog.Debug(fmt.Sprintf("error doing remote copy job wait %v", err))
		return fmt.Errorf("error doing remote copy job wait %w", err)
	}
	return nil

	/*	stderr, err := cmd.StderrPipe()
		if err != nil {
			return err
		}

		fcj.Dcrlog.Debug("starting rsync command")
		err = cmd.Start()
		if err != nil {
			return err
		}

		_, err = io.ReadAll(stderr)
		if err != nil {
			_ = cmd.Process.Kill()
			fcj.Dcrlog.Debug(fmt.Sprintf("error reading from stderr pipe %w", err))
			return fmt.Errorf("error reading from stderr pipe %w", err)
		}

		err = cmd.Wait()
		if err != nil {
			fcj.Dcrlog.Debug(fmt.Sprintf("error doing remote copy job wait %w", err))
			return fmt.Errorf("error doing remote copy job wait %w", err)
		}

		return nil
	*/

}

func (fcj *FSCopyJob) StartCopyLocal() error {
	return nil
}

func (fcj *FSCopyJob) StartCopy() error {
	if fcj.Src.IsLocal {
		return fcj.StartCopyLocal()
	}
	return fcj.StartCopyRemote()
}
