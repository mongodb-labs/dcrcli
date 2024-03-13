package archiver

import (
	"testing"
)

// Test TarWithPatternMatch - for handling non-existent directory source
func TestTarWithPatternMatchWithNonExistentSource(t *testing.T) {
	got := "/dummy/src/folder"

	err := TarWithPatternMatch(got, "metrics.*")
	if err == nil {
		t.Fatalf("Should error out on non-existent source folder")
	}
}
