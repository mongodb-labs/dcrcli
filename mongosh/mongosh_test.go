package mongosh

import (
	"errors"
	"fmt"
	"os"
	"testing"
)

// 5 tests for detect function
// - No shell
// - Only Legacy mongo shell, No mongosh shell
// - Only New mongosh shell, No legacy mongo shell
// - New mongosh shell in the PATH first, then legacy mongo shell
// - Legacy mongo shell in the PATH first, then new mongosh shell

func TestDetectNoShell(t *testing.T) {
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

// TEST printErrorIfNotNil

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
