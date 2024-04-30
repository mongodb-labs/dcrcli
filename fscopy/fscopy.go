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
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

type RemoteCred struct {
	Username  string
	Available bool
}

func (rc *RemoteCred) Get() error {
	reader := bufio.NewReader(os.Stdin)
	fmt.Println(
		"Enter PasswordLess ssh User for remote copy. Leave Blank for cluster without remote nodes): ",
	)
	username, err := reader.ReadString('\n')
	if err != nil {
		return err
	}
	rc.Username = strings.TrimSuffix(username, "\n")
	if rc.Username == "" {
		println("WARNING: PasswordLess SSH Username is empty assuming all nodes local")
		rc.Available = false
	} else {
		rc.Available = true
	}

	return nil
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
}

// example rsync -a --include='mongod.log*' --exclude='*' --include='*/' ubuntu@ip-10-0-0-246:/home/ubuntu/testcluster/dbp/ .
type FSCopyJobWithPattern struct {
	CopyJobDetails  *FSCopyJob
	CurrentFileName string
}

func (fcj *FSCopyJobWithPattern) StartCopyWithPattern() error {
	if fcj.CopyJobDetails.Src.IsLocal {
		return fcj.StartCopyLocalWithPattern()
	}
	return fcj.StartCopyRemoteWithPattern()
}

func (fcj *FSCopyJobWithPattern) StartCopyLocalWithPattern() error {
	return nil
}

func (fcj *FSCopyJobWithPattern) StartCopyRemoteWithPattern() error {
	var cmd *exec.Cmd

	filepattern := `'` + fcj.CurrentFileName + `*` + `'`
	excludepattern := `'` + `*` + `'`

	// we invoke bash shell because the wildcards are interpretted by bash shell not the rsync program
	cmd = exec.Command(
		"bash",
		"-c",
		fmt.Sprintf(
			"rsync -az --include=%s --exclude=%s --info=progress2 %s@%s:%s/ %s",
			filepattern,
			excludepattern,
			fcj.CopyJobDetails.Src.Username,
			fcj.CopyJobDetails.Src.Hostname,
			fcj.CopyJobDetails.Src.Path,
			fcj.CopyJobDetails.Dst.Path,
		))

	cmd.Stdout = fcj.CopyJobDetails.Output

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	err = cmd.Start()
	if err != nil {
		return err
	}

	_, err = io.ReadAll(stderr)
	if err != nil {
		_ = cmd.Process.Kill()
		return fmt.Errorf("StartCopyRemoteWithPattern: Error reading from stderr pipe %w", err)
	}

	err = cmd.Wait()
	if err != nil {
		return fmt.Errorf("StartCopyRemoteWithPattern: error doing remote copy job wait %w", err)
	}
	return nil
}

// currently only run for remote source directories
func (fcj *FSCopyJob) StartCopyRemote() error {
	var cmd *exec.Cmd

	cmd = exec.Command(
		"rsync",
		"-az",
		"--info=progress2",
		fmt.Sprintf(`%s@%s:%s`,
			fcj.Src.Username,
			fcj.Src.Hostname,
			fcj.Src.Path),
		fmt.Sprintf(`%s`,
			fcj.Dst.Path),
	)

	cmd.Stdout = fcj.Output

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	err = cmd.Start()
	if err != nil {
		return err
	}

	_, err = io.ReadAll(stderr)
	if err != nil {
		_ = cmd.Process.Kill()
		return fmt.Errorf("Error reading from stderr pipe %w", err)
	}

	err = cmd.Wait()
	if err != nil {
		return fmt.Errorf("error doing remote copy job wait %w", err)
	}

	return nil
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
