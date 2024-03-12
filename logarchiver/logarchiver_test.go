package logarchiver

import (
	"bytes"
	"testing"

	"dcrcli/mongosh"
)

//### START TESTING for estimateJsonIndexBoundsForFieldWithJSONValue
// - pass case where buffer is valid json document enclosing search field with its embedded document and search term is other than systemLog
// - pass case where buffer is valid json document enclosing search field with its embedded document and search term is systemLog
// - fail case where the search field is not present but the json document is valid

type JsonValueIndexBounds struct {
	rightCurlyIndex int
	leftCurlyIndex  int
}

// - pass case where buffer is valid json document enclosing search field with its embedded document and search term is other than systemLog
func TestEstimateJsonIndexBoundsForFieldWithJSONValueTestValidJsonWithSearchTermOtherThanSystemLog(
	t *testing.T,
) {
	//var Getparsedjsonoutput bytes.Buffer
	//{ "field" : { "subfield" : { "k1": "v1", "k2": "v2" }}}
	testJsonBuf := []byte(`{"f":{"s":{"k1":"v1","k2": "v2" }}}`)

	// prepare mongosh buffer variable because it is also used internally by this function
	mongosh.Getparsedjsonoutput.Reset()
	mongosh.Getparsedjsonoutput.Write(
		testJsonBuf,
	)

	fieldWithJsonDoc := `s`
	got := bytes.NewReader(testJsonBuf)

	want := JsonValueIndexBounds{11, 33}
	var result JsonValueIndexBounds
	result.rightCurlyIndex, result.leftCurlyIndex = estimateJsonIndexBoundsForFieldWithJSONValue(
		got,
		fieldWithJsonDoc,
	)
	if want.rightCurlyIndex != result.rightCurlyIndex ||
		want.leftCurlyIndex != result.leftCurlyIndex {
		t.Fatalf(
			`estimateJsonIndexBoundsForFieldWithJSONValue on document %s not matching bounds, got: (%d, %d), want:(%d,%d)`,
			string(testJsonBuf),
			result.rightCurlyIndex,
			result.leftCurlyIndex,
			want.rightCurlyIndex,
			want.leftCurlyIndex,
		)
	}
}

// - pass case where buffer is valid json document enclosing search field with its embedded document and search term is systemLog
func TestEstimateJsonIndexBoundsForFieldWithJSONValueWithSearchTermSystemLog(t *testing.T) {
	testJsonBuf := []byte(`{"f":{"systemLog":{"k1":"v1","k2": "v2" }}}`)

	// prepare mongosh buffer variable
	mongosh.Getparsedjsonoutput.Reset()
	mongosh.Getparsedjsonoutput.Write(
		testJsonBuf,
	)
	got := bytes.NewReader(mongosh.Getparsedjsonoutput.Bytes())
	fieldWithJsonDoc := `systemLog`

	want := JsonValueIndexBounds{19, 41}
	var result JsonValueIndexBounds
	result.rightCurlyIndex, result.leftCurlyIndex = estimateJsonIndexBoundsForFieldWithJSONValue(
		got,
		fieldWithJsonDoc,
	)
	if want.rightCurlyIndex != result.rightCurlyIndex ||
		want.leftCurlyIndex != result.leftCurlyIndex {
		t.Fatalf(
			`estimateJsonIndexBoundsForFieldWithJSONValue on document %s not matching bounds, got: (%d, %d), want:(%d,%d)`,
			string(testJsonBuf),
			result.rightCurlyIndex,
			result.leftCurlyIndex,
			want.rightCurlyIndex,
			want.leftCurlyIndex,
		)
	}
}

// - fail case where the search field is not present but the json document is valid
func TestEstimateJsonIndexBoundsForFieldWithJSONValueWithSearchTermMissing(t *testing.T) {
	testJsonBuf := []byte(`{"f":{"systemLog":{"k1":"v1","k2": "v2" }}}`)

	// prepare mongosh buffer variable
	mongosh.Getparsedjsonoutput.Reset()
	mongosh.Getparsedjsonoutput.Write(
		testJsonBuf,
	)
	got := bytes.NewReader(mongosh.Getparsedjsonoutput.Bytes())
	fieldWithJsonDoc := `missingVl`

	want := JsonValueIndexBounds{-1, -1}
	var result JsonValueIndexBounds
	result.rightCurlyIndex, result.leftCurlyIndex = estimateJsonIndexBoundsForFieldWithJSONValue(
		got,
		fieldWithJsonDoc,
	)
	if want.rightCurlyIndex != result.rightCurlyIndex ||
		want.leftCurlyIndex != result.leftCurlyIndex {
		t.Fatalf(
			`estimateJsonIndexBoundsForFieldWithJSONValue on document %s not matching bounds, got: (%d, %d), want:(%d,%d)`,
			string(testJsonBuf),
			result.rightCurlyIndex,
			result.leftCurlyIndex,
			want.rightCurlyIndex,
			want.leftCurlyIndex,
		)
	}
}

// Below functions don't need testing yet
// extractJSONfromBufferIntoMap
// estimateLogPath
// Run
// trimQuote
