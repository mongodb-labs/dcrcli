package ftdcarchiver

import (
	"testing"

	"dcrcli/mongosh"
)

type testOutputForextractPathFromGetMongodataOutput struct {
	diagnosticDataPath string
	ok                 bool
}

// Test  extractPathFromGetMongodataOutput
// below functions not required to be tested yet:
// - Run
// - TrimgQuote

// Test when diagnosticDataCollectionDirectoryPath document string exists
func TestExtractPathFromGetMongodataOutputWhenGetMongoDataHasDiagnosticDataPath(t *testing.T) {
	testJsonBuf := []byte(
		`           "diagnosticDataCollectionDirectoryPath": "/test/dbpath/diagnostic.data",`,
	)

	// prepare mongosh buffer variable because it is also used internally by this function
	mongosh.Getparsedjsonoutput.Reset()
	mongosh.Getparsedjsonoutput.Write(
		testJsonBuf,
	)

	want := testOutputForextractPathFromGetMongodataOutput{"/test/dbpath/diagnostic.data", true}
	resultPath, resultOk := extractPathFromGetMongodataOutput()

	if want.diagnosticDataPath != resultPath || want.ok != resultOk {
		t.Fatalf("Expected values not found")
	}
}

// Test when diagnosticDataCollectionDirectoryPath document string exists but ending comma is not there
func TestExtractPathFromGetMongodataOutputWhenGetMongoDataHasDiagnosticDataPathMissingComma(
	t *testing.T,
) {
	testJsonBuf := []byte(
		`           "diagnosticDataCollectionDirectoryPath": "/test/dbpath/diagnostic.data"`,
	)

	// prepare mongosh buffer variable because it is also used internally by this function
	mongosh.Getparsedjsonoutput.Reset()
	mongosh.Getparsedjsonoutput.Write(
		testJsonBuf,
	)

	want := testOutputForextractPathFromGetMongodataOutput{"", false}
	resultPath, resultOk := extractPathFromGetMongodataOutput()

	if want.diagnosticDataPath != resultPath || want.ok != resultOk {
		t.Fatalf("Expected values not found")
	}
}

// Test when diagnosticDataCollectionDirectoryPath document string does not exists
func TestExtractPathFromGetMongodataOutputWhenGetMongoDataHasDiagnosticDataPathNotPresent(
	t *testing.T,
) {
	testJsonBuf := []byte(
		`"key": "value"`,
	)

	// prepare mongosh buffer variable because it is also used internally by this function
	mongosh.Getparsedjsonoutput.Reset()
	mongosh.Getparsedjsonoutput.Write(
		testJsonBuf,
	)

	want := testOutputForextractPathFromGetMongodataOutput{"", false}
	resultPath, resultOk := extractPathFromGetMongodataOutput()

	if want.diagnosticDataPath != resultPath || want.ok != resultOk {
		t.Fatalf("Expected values not found")
	}
}
