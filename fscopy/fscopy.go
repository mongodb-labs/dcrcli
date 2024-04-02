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
	}
	rc.Available = true

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
		"-a",
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
