package dcroutdir

import (
	"os"
	"path/filepath"
	"testing"
)

// inTempCwd runs the test from an empty directory, because staging paths are
// relative to the working directory.
func inTempCwd(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(old) })
	return dir
}

func TestResetStagingDropsFilesFromAnEarlierRun(t *testing.T) {
	inTempCwd(t)
	od := DCROutputDir{OutputPrefix: TempRoot + "/cluster1/", Hostname: "h1", Port: "27017"}
	if err := od.CreateDCROutputDir(); err != nil {
		t.Fatal(err)
	}
	stale := filepath.Join(od.Path(), "metrics.2020-01-01T00-00-00Z-00000")
	if err := os.WriteFile(stale, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := od.ResetStaging(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Fatalf("stale file survived: %v", err)
	}
	if st, err := os.Stat(od.Path()); err != nil || !st.IsDir() {
		t.Fatalf("staging dir not recreated: %v", err)
	}
}

func TestResetStagingLeavesOtherHostsAlone(t *testing.T) {
	inTempCwd(t)
	a := DCROutputDir{OutputPrefix: TempRoot + "/c/", Hostname: "a", Port: "1"}
	b := DCROutputDir{OutputPrefix: TempRoot + "/c/", Hostname: "b", Port: "1"}
	for _, od := range []DCROutputDir{a, b} {
		if err := od.CreateDCROutputDir(); err != nil {
			t.Fatal(err)
		}
	}
	keep := filepath.Join(b.Path(), "keep")
	if err := os.WriteFile(keep, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := a.ResetStaging(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(keep); err != nil {
		t.Fatalf("other host's files were touched: %v", err)
	}
}

func TestStagingRefusesPathsOutsideTempRoot(t *testing.T) {
	dir := inTempCwd(t)
	precious := filepath.Join(dir, "precious")
	if err := os.WriteFile(precious, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	for name, od := range map[string]DCROutputDir{
		"parent cluster name": {OutputPrefix: TempRoot + "/../", Hostname: "x", Port: "1"},
		"deep escape":         {OutputPrefix: TempRoot + "/a/../../../", Hostname: "x", Port: "1"},
		"outside entirely":    {OutputPrefix: "./other/", Hostname: "x", Port: "1"},
		"the root itself":     {OutputPrefix: TempRoot, Hostname: "", Port: ""},
	} {
		od := od
		if err := od.ResetStaging(); err == nil {
			t.Errorf("%s: ResetStaging should refuse %q", name, od.Path())
		}
		if err := od.RemoveStaging(); err == nil {
			t.Errorf("%s: RemoveStaging should refuse %q", name, od.Path())
		}
	}
	if _, err := os.Stat(precious); err != nil {
		t.Fatalf("file outside the temp root was removed: %v", err)
	}
}

func TestRemoveStagingDeletesTheDirectory(t *testing.T) {
	inTempCwd(t)
	od := DCROutputDir{OutputPrefix: TempRoot + "/c/", Hostname: "h", Port: "1"}
	if err := od.ResetStaging(); err != nil {
		t.Fatal(err)
	}
	if err := od.RemoveStaging(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(od.Path()); !os.IsNotExist(err) {
		t.Fatalf("staging dir still there: %v", err)
	}
}
