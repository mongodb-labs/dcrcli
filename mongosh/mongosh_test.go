package mongosh

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"testing"

	"dcrcli/mongocredentials"
)

func TestRunCurrentDBCommand(t *testing.T) {
	cred := mongocredentials.Mongocredentials{
		Username: "",
		Mongouri: "mongodb://localhost:27017",
		Password: "",
	}

	c := CaptureGetMongoData{
		S:                   &cred,
		Getparsedjsonoutput: &bytes.Buffer{},
		CurrentBin:          "",
		ScriptPath:          "",
		Unixts:              "",
		FilePathOnDisk:      "",
		CurrentCommand:      &HelloDBCommand,
	}

	err := c.detectMongoShellType()
	if err != nil {
		t.Error(err.Error())
	}
	err = c.RunCurrentDBCommand()
	if err != nil {
		t.Error(err.Error())
	}
	fmt.Println(c.Getparsedjsonoutput.Bytes())
}

// ### START 5 tests for detectMongoShellType function
// ### Note: The binPath and legacybinPath functions just call exec.LookPath standard
// library function. Their functionality is covered by these tests.
// - No shell
// - Only Legacy mongo shell, No mongosh shell
// - Only New mongosh shell, No legacy mongo shell
// - New mongosh shell in the PATH first, then legacy mongo shell
// - Legacy mongo shell in the PATH first, then new mongosh shell

func TestDetectMongoShellTypeWithNoShell(t *testing.T) {
	os.Setenv("PATH", "/tmp")

	c := CaptureGetMongoData{
		S:                   nil,
		Getparsedjsonoutput: nil,
		CurrentBin:          "",
		ScriptPath:          "",
		Unixts:              "",
		FilePathOnDisk:      "",
	}

	err := c.detectMongoShellType()
	if err == nil {
		t.Error(err.Error())
	}
}

func TestDetectMongoShellTypeMongoShell(t *testing.T) {
	// This path generally has legacy mongo shell installed
	// If your test setup is different directly mention that path
	// Ensure mongosh is not in the same path

	os.Setenv("PATH", "/Users/nishant/.local/bin/")
	c := CaptureGetMongoData{
		S:                   nil,
		Getparsedjsonoutput: nil,
		CurrentBin:          "",
		ScriptPath:          "",
		Unixts:              "",
		FilePathOnDisk:      "",
	}

	err := c.detectMongoShellType()
	if err != nil {
		t.Error(err.Error())
	}

	if c.CurrentBin != "mongo" {
		t.Error("legacy mongo shell not detected even when legacy mongo shell was provided")
	}
}

func TestDetectMongoShellTypeMongoshShell(t *testing.T) {
	// This path generally has legacy mongo shell installed
	// If your test setup is different directly mention that path
	// Ensure mongosh is not in the same path

	os.Setenv("PATH", "/opt/homebrew/bin/")
	c := CaptureGetMongoData{
		S:                   nil,
		Getparsedjsonoutput: nil,
		CurrentBin:          "",
		ScriptPath:          "",
		Unixts:              "",
		FilePathOnDisk:      "",
	}

	err := c.detectMongoShellType()
	if err != nil {
		t.Error(err.Error())
	}

	if c.CurrentBin != "mongosh" {
		t.Error("New mongosh shell not detected even when PATH was provided")
	}
}

func TestDetectMongoShellTypeMongoshFirstInPATH(t *testing.T) {
	// If your test setup is different directly mention that path
	// Irrespective of the position in path if mongosh is present the detect function should return mongosh

	os.Setenv("PATH", "/opt/homebrew/bin:/Users/nishant/.local/bin")
	c := CaptureGetMongoData{
		S:                   nil,
		Getparsedjsonoutput: nil,
		CurrentBin:          "",
		ScriptPath:          "",
		Unixts:              "",
		FilePathOnDisk:      "",
	}

	err := c.detectMongoShellType()
	if err != nil {
		t.Error(err.Error())
	}

	if c.CurrentBin != "mongosh" {
		t.Error("New mongosh shell not detected even when it was first in the PATH provided")
	}
}

func TestDetectMongoShellTypeLegacyMongoFirstInPATH(t *testing.T) {
	// If your test setup is different directly mention that path
	// Irrespective of the position in path if mongosh is present the detect function should return mongosh

	os.Setenv("PATH", "/Users/nishant/.local/bin:/opt/homebrew/bin")
	c := CaptureGetMongoData{
		S:                   nil,
		Getparsedjsonoutput: nil,
		CurrentBin:          "",
		ScriptPath:          "",
		Unixts:              "",
		FilePathOnDisk:      "",
	}

	err := c.detectMongoShellType()
	if err != nil {
		t.Error(err.Error())
	}
	if c.CurrentBin != "mongosh" {
		t.Error(
			"New mongo shell not detected even when it was present in the PATH provided but second",
		)
	}
}

// ### END tests for detectMongoShellType function

// ### START 5 tests for detect function
// ### Note: The binPath and legacybinPath functions just call exec.LookPath standard
// library function. Their functionality is covered by these tests.
// - No shell
// - Only Legacy mongo shell, No mongosh shell
// - Only New mongosh shell, No legacy mongo shell
// - New mongosh shell in the PATH first, then legacy mongo shell
// - Legacy mongo shell in the PATH first, then new mongosh shell

/**func TestDetectNoShell(t *testing.T) {
	os.Setenv("PATH", "/tmp")
	var currentBin string
	var scriptPath string

	err := detect(&currentBin, &scriptPath)
	if err == nil {
		t.Error(err.Error())
	}
}

func TestDetectMongoShell(t *testing.T) {
	// This path generally has legacy mongo shell installed
	// If your test setup is different directly mention that path
	// Ensure mongosh is not in the same path

	os.Setenv("PATH", "/Users/nishant/.local/bin/")
	var currentBin string
	var scriptPath string

	err := detect(&currentBin, &scriptPath)
	if err != nil {
		t.Error(err.Error())
	}

	if currentBin != "mongo" {
		t.Error("legacy mongo shell not detected even when legacy mongo shell was provided")
	}
}

func TestDetectMongoshShell(t *testing.T) {
	// This path generally has legacy mongo shell installed
	// If your test setup is different directly mention that path
	// Ensure mongosh is not in the same path

	os.Setenv("PATH", "/opt/homebrew/bin/")
	var currentBin string
	var scriptPath string

	err := detect(&currentBin, &scriptPath)
	if err != nil {
		t.Error(err.Error())
	}

	if currentBin != "mongosh" {
		t.Error("New mongosh shell not detected even when PATH was provided")
	}
}

func TestDetectMongoshFirstInPATH(t *testing.T) {
	// If your test setup is different directly mention that path
	// Irrespective of the position in path if mongosh is present the detect function should return mongosh

	os.Setenv("PATH", "/opt/homebrew/bin:/Users/nishant/.local/bin")
	var currentBin string
	var scriptPath string

	err := detect(&currentBin, &scriptPath)
	if err != nil {
		t.Error(err.Error())
	}

	if currentBin != "mongosh" {
		t.Error("New mongosh shell not detected even when it was first in the PATH provided")
	}
}

func TestDetectLegacyMongoFirstInPATH(t *testing.T) {
	// If your test setup is different directly mention that path
	// Irrespective of the position in path if mongosh is present the detect function should return mongosh

	os.Setenv("PATH", "/Users/nishant/.local/bin:/opt/homebrew/bin")
	var currentBin string
	var scriptPath string

	err := detect(&currentBin, &scriptPath)
	if err != nil {
		t.Error(err.Error())
	}

	if currentBin != "mongosh" {
		t.Error(
			"New mongo shell not detected even when it was present in the PATH provided but second",
		)
	}
}

// ### END tests for detect function
*/

// ### START TEST printErrorIfNotNil
func TestPrintErrorIfNotNilWithNilErrorInput(t *testing.T) {
	// For nil error return nil

	err := printErrorIfNotNil(nil, "This is a dummy message")
	if err != nil {
		t.Error(err.Error())
	}
}

func TestPrintErrorIfNotNilWithNonNilErrorInput(t *testing.T) {
	// For not nil error return not nil error

	fmt.Printf("IGNORE : ")
	err := printErrorIfNotNil(errors.New("dummy error"), "This is a dummy message")
	if err == nil {
		t.Error(err.Error())
	}
}

// ### END TEST printErrorIfNotNil

// ### START TEST runCommandAndCaptureOutputInVariable
// It calls standard exec.Command
// It also calls printErrorIfNotNil for which we have tests
// No other testing required
// ### END TEST runCommandAndCaptureOutputInVariable

// ### START TEST removeStaleOutputFiles
// We only call os.Remove - no further testing needed
// ### END TEST  removeStaleOutputFiles

// ### START TEST getMongoConnectionStringWithCredentials
// We only call mongocredentials which is already being tested - so skip testing
// ### END TEST getMongoConnectionStringWithCredentials

// ### START TEST writeOutputFromVariableToFile
// We only call os.WriteFile - so skip testing
// ### END TEST writeOutputFromVariableToFile

// ### START TEST RunShell
// All other sub functions covered and no addtional logic here so can be skipped
// ### END TEST RunShell
