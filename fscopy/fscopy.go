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

// can be local or remote
type SourceDir struct {
	IsLocal  bool
	Path     []byte
	Hostname []byte
	SyncPort uint
	Username []byte
}

// this is always local
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

// Copy with pattern
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

	// rsync automatically considers all filenames no need for giving ^
	filepattern := `'` + fcj.CurrentFileName + `*` + `'`
	excludepattern := `'` + `*` + `'`

	fmt.Println(fmt.Sprintf(
		`Inside StartCopyRemote %s@%s:%s and Dst is %s and file pattern is %s`,
		fcj.CopyJobDetails.Src.Username,
		fcj.CopyJobDetails.Src.Hostname,
		fcj.CopyJobDetails.Src.Path,
		fcj.CopyJobDetails.Dst.Path,
		filepattern,
	))

	// note trailing slash is added to only copy directory contents not the directory itself
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

	fmt.Println("StartCopyRemoteWithPattern: Starting the copy Command with pattern")
	err = cmd.Start()
	if err != nil {
		return err
	}

	// wait for the rsync command to complete
	output, err := io.ReadAll(stderr)
	if err != nil {
		_ = cmd.Process.Kill()
		fmt.Println("StartCopyRemoteWithPattern: Error reading from stderr pipe", err)
		return err
	}

	fmt.Println(fmt.Sprintf("StartCopyRemoteWithPattern: remote copy job stderr: %s", output))
	fmt.Println(
		fmt.Sprintf(
			"StartCopyRemoteWithPattern: remote copy job stdout: %s",
			fcj.CopyJobDetails.Output.Bytes(),
		),
	)

	err = cmd.Wait()
	if err != nil {
		fmt.Println("StartCopyRemoteWithPattern: error doing remote copy job wait", err)
		fmt.Println(fmt.Sprintf("StartCopyRemoteWithPattern: remote copy job stderr: %s", output))
		fmt.Println(
			fmt.Sprintf(
				"StartCopyRemoteWithPattern: remote copy job stdout: %s",
				fcj.CopyJobDetails.Output.Bytes(),
			),
		)
		return err
	}

	return nil
}

// currently only run for remote source directories
func (fcj *FSCopyJob) StartCopyRemote() error {
	var cmd *exec.Cmd
	// use rsync or scp
	/**cmd = exec.Command(
		"scp", "-r",
		fmt.Sprintf(`%s@%s:%s`,
			fcj.Src.Username,
			fcj.Src.Hostname,
			fcj.Src.Path,
		),
		fmt.Sprintf(`%s`,
			fcj.Dst.Path),
	)*/
	fmt.Println(fmt.Sprintf(
		`Inside StartCopyRemote %s@%s:%s and Dst is %s`,
		fcj.Src.Username,
		fcj.Src.Hostname,
		fcj.Src.Path,
		fcj.Dst.Path,
	))

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
	// fmt.Println(cmd.String())
	cmd.Stdout = fcj.Output
	// cmd.Stderr = fcj.Output

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	fmt.Println("Starting the copy Command")
	err = cmd.Start()
	if err != nil {
		return err
	}

	// wait for the rsync command to complete
	output, err := io.ReadAll(stderr)
	if err != nil {
		_ = cmd.Process.Kill()
		fmt.Println("Error reading from stderr pipe", err)
		return err
	}

	fmt.Println(fmt.Sprintf("remote copy job stderr: %s", output))
	fmt.Println(fmt.Sprintf("remote copy job stdout: %s", fcj.Output.Bytes()))

	err = cmd.Wait()
	if err != nil {
		fmt.Println("error doing remote copy job wait", err)
		fmt.Println(fmt.Sprintf("remote copy job stderr: %s", output))
		fmt.Println(fmt.Sprintf("remote copy job stdout: %s", fcj.Output.Bytes()))
		return err
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