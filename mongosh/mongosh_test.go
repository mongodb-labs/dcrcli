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

package mongosh

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"strings"
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

func TestFormatMongoShellErrorIncludesShellOutputAndHint(t *testing.T) {
	out := []byte("MongoServerSelectionError: connect ECONNREFUSED 127.0.0.1:27017")
	err := formatMongoShellError("MongoDB shell (db.runCommand({hello:1}).hosts)", errors.New("exit status 1"), out)
	msg := err.Error()
	if !strings.Contains(msg, "Shell output:") || !strings.Contains(msg, "ECONNREFUSED") {
		t.Fatalf("expected shell output in error: %s", msg)
	}
	if !strings.Contains(msg, "Likely cause:") || !strings.Contains(msg, "accepting connections") {
		t.Fatalf("expected connection hint: %s", msg)
	}
}

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

func TestMongoShellArgsKeepsCredentialsAndEvalAsArgv(t *testing.T) {
	eval := RsConfCommand
	cgm := CaptureGetMongoData{
		S: &mongocredentials.Mongocredentials{
			Username: "diag",
			Password: "s3cret;rm -rf",
			Mongouri: "mongodb://localhost:27017",
		},
		CurrentBin:     mongoshBin,
		CurrentCommand: &eval,
	}

	args := cgm.mongoShellArgs(true)
	foundPass := false
	for i, a := range args {
		if a == "-p" && i+1 < len(args) && args[i+1] == "s3cret;rm -rf" {
			foundPass = true
		}
	}
	if !foundPass {
		t.Fatalf("password must be a separate argv element: %#v", args)
	}
	if args[len(args)-1] != "--json=canonical" {
		t.Fatalf("expected --json=canonical for mongosh JSON eval, got %#v", args)
	}

	plain := cgm.mongoShellArgs(false)
	for _, a := range plain {
		if strings.HasPrefix(a, "--json") {
			t.Fatalf("plain eval must not use --json (print helpers): %#v", plain)
		}
	}
}

func TestMaxCollectionsIsSharedSafelimit(t *testing.T) {
	t.Cleanup(func() { _ = SetMaxCollections(DefaultMaxCollections) })

	if MaxCollections() != DefaultMaxCollections {
		t.Fatalf("default MaxCollections: got %d want %d", MaxCollections(), DefaultMaxCollections)
	}
	prefix := fmt.Sprintf("var _maxCollections = %d;\n", DefaultMaxCollections)
	if got := WithMaxCollections("script"); got != prefix+"script" {
		t.Fatalf("WithMaxCollections: %q", got)
	}
	if err := SetMaxCollections(0); err == nil {
		t.Fatal("expected SetMaxCollections(0) to fail")
	}
	if err := SetMaxCollections(10000); err != nil {
		t.Fatal(err)
	}
	if MaxCollections() != 10000 {
		t.Fatalf("after SetMaxCollections: got %d", MaxCollections())
	}
	if !strings.HasPrefix(WithMaxCollections(UniqueIndexesCommand), "var _maxCollections = 10000;\n") {
		t.Fatal("unique indexes eval must use the active safelimit")
	}
	if !strings.HasPrefix(WithMaxCollections(ListCatalogTimeSeriesCommand), "var _maxCollections = 10000;\n") {
		t.Fatal("timeseries eval must use the active safelimit")
	}
}

func TestDiagCommandEmbedsAreStaticHelpers(t *testing.T) {
	if !strings.Contains(RsConfCommand, "rs.conf()") {
		t.Fatalf("rs.conf embed: %q", RsConfCommand)
	}
	if !strings.Contains(RsStatusCommand, "rs.status()") {
		t.Fatalf("rs.status embed: %q", RsStatusCommand)
	}
	if !strings.Contains(PrintReplicationInfoCommand, "printReplicationInfo") {
		t.Fatalf("printReplicationInfo embed: %q", PrintReplicationInfoCommand)
	}
	if !strings.Contains(PrintSecondaryReplicationInfoCommand, "printSecondaryReplicationInfo") {
		t.Fatalf("printSecondaryReplicationInfo embed: %q", PrintSecondaryReplicationInfoCommand)
	}
	if !strings.Contains(ShStatusCommand, "sh.status()") {
		t.Fatalf("sh.status embed: %q", ShStatusCommand)
	}
	if !strings.Contains(GetDbPathCommand, "dbPath") {
		t.Fatalf("dbPath embed: %q", GetDbPathCommand)
	}
	if !strings.Contains(ListCatalogTimeSeriesCommand, "$listCatalog") ||
		!strings.Contains(ListCatalogTimeSeriesCommand, `^system\\.buckets\\.`) ||
		!strings.Contains(ListCatalogTimeSeriesCommand, "getSiblingDB('admin')") ||
		!strings.Contains(ListCatalogTimeSeriesCommand, "2500") ||
		!strings.Contains(ListCatalogTimeSeriesCommand, "ERROR:") {
		t.Fatalf("listCatalog embed: %q", ListCatalogTimeSeriesCommand)
	}
	if !strings.Contains(ShardedIndexConsistencyCommand, "shardedIndexConsistency") {
		t.Fatalf("shardedIndexConsistency embed: %q", ShardedIndexConsistencyCommand)
	}
	if !strings.Contains(UniqueIndexesCommand, "getIndexes") ||
		!strings.Contains(UniqueIndexesCommand, "idx.unique") ||
		!strings.Contains(UniqueIndexesCommand, "$collStats") ||
		!strings.Contains(UniqueIndexesCommand, "formatVersion") ||
		!strings.Contains(UniqueIndexesCommand, "listed.ok") ||
		!strings.Contains(UniqueIndexesCommand, "getCollectionInfos") ||
		!strings.Contains(UniqueIndexesCommand, "errors.length") ||
		!strings.Contains(UniqueIndexesCommand, "2500") ||
		strings.Contains(UniqueIndexesCommand, "validate(") {
		t.Fatalf("uniqueIndexes embed: %q", UniqueIndexesCommand)
	}
}
