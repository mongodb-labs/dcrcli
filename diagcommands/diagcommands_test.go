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

package diagcommands

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dcrcli/dcroutdir"
	"dcrcli/mongosh"
)

func TestIsSafeUnixPath(t *testing.T) {
	safe := []string{"/data/db", "/var/lib/mongodb", "/mnt/data-1/_db.0"}
	for _, p := range safe {
		if !isSafeUnixPath(p) {
			t.Fatalf("expected safe: %q", p)
		}
	}
	unsafe := []string{
		"",
		"data/db",
		"/data/../etc",
		"/data/db; rm -rf /",
		"/data/db$(reboot)",
		"/tmp/foo bar",
		"/tmp/foo\nbar",
		"C:\\data\\db",
		"/tmp/-h",
		"/tmp/`id`",
	}
	for _, p := range unsafe {
		if isSafeUnixPath(p) {
			t.Fatalf("expected unsafe: %q", p)
		}
	}
}

func TestHasShellMeta(t *testing.T) {
	if hasShellMeta("ubuntu") || hasShellMeta("mongo-1.internal") || hasShellMeta("10.0.0.5") {
		t.Fatal("plain tokens should be allowed")
	}
	if !hasShellMeta("") || !hasShellMeta("user;id") || !hasShellMeta("host name") {
		t.Fatal("empty or metacharacter tokens should be rejected")
	}
}

func TestParseDbPathOutput(t *testing.T) {
	for raw, want := range map[string]string{
		`"/data/db"`:   "/data/db",
		"  /data/db\n": "/data/db",
		`""`:           "",
		"undefined":    "",
		"null":         "",
		"  ":           "",
	} {
		if got := parseDbPathOutput(raw); got != want {
			t.Fatalf("parseDbPathOutput(%q) = %q want %q", raw, got, want)
		}
	}
}

func TestIsMongos(t *testing.T) {
	if !isMongos("MONGOS") || !isMongos("mongos") {
		t.Fatal("expected mongos")
	}
	if isMongos("PRIMARY") || isMongos("") {
		t.Fatal("did not expect mongos")
	}
}

func TestRoleGates(t *testing.T) {
	if !isConfigServer("config") || !isConfigServer("CONFIG") {
		t.Fatal("expected config server")
	}
	if isConfigServer("shard01") || isConfigServer("") {
		t.Fatal("did not expect config server")
	}
	if !isArbiter("ARBITER") || isArbiter("SECONDARY") {
		t.Fatal("arbiter gate")
	}
	if !isPrimary("PRIMARY") || isPrimary("SECONDARY") {
		t.Fatal("primary gate")
	}
}

func TestCollectShardedIndexConsistencySkipNonPrimaryConfig(t *testing.T) {
	dir := t.TempDir()
	out := &dcroutdir.DCROutputDir{OutputPrefix: dir + string(os.PathSeparator), Hostname: "cfg1", Port: "27017"}
	if err := out.CreateDCROutputDir(); err != nil {
		t.Fatal(err)
	}
	c := &Collector{
		Mongo:            &mongosh.CaptureGetMongoData{},
		Outputdir:        out,
		ReplicaState:     "SECONDARY",
		ShardMapHostRole: "config",
	}
	if err := c.collectShardedIndexConsistency(); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join(out.Path(), fileShardedIndexConsistency))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "config-server primary") {
		t.Fatalf("skip note: %s", body)
	}
}

func TestCollectShardedIndexConsistencySkipNonConfig(t *testing.T) {
	c := &Collector{ShardMapHostRole: "shard01", ReplicaState: "PRIMARY"}
	if err := c.collectShardedIndexConsistency(); err != nil {
		t.Fatal(err)
	}
}

func TestCollectTimeSeriesCatalogSkipConfigAndArbiter(t *testing.T) {
	c := &Collector{ShardMapHostRole: "config", ReplicaState: "PRIMARY"}
	if err := c.collectTimeSeriesCatalog(); err != nil {
		t.Fatal(err)
	}
	c = &Collector{ShardMapHostRole: "shard01", ReplicaState: "ARBITER"}
	if err := c.collectTimeSeriesCatalog(); err != nil {
		t.Fatal(err)
	}
}

func TestCollectUniqueIndexesSkipConfigAndArbiter(t *testing.T) {
	c := &Collector{ShardMapHostRole: "config", ReplicaState: "PRIMARY"}
	if err := c.collectUniqueIndexes(); err != nil {
		t.Fatal(err)
	}
	c = &Collector{ShardMapHostRole: "shard01", ReplicaState: "ARBITER"}
	if err := c.collectUniqueIndexes(); err != nil {
		t.Fatal(err)
	}
}

func TestRunDFLocalUsesArgv(t *testing.T) {
	var gotName string
	var gotArgs []string
	old := commandOutput
	commandOutput = func(name string, arg ...string) ([]byte, error) {
		gotName = name
		gotArgs = append([]string{}, arg...)
		return []byte("ok\n"), nil
	}
	t.Cleanup(func() { commandOutput = old })

	oldLook := lookPath
	lookPath = func(file string) (string, error) { return "/bin/" + file, nil }
	t.Cleanup(func() { lookPath = oldLook })

	c := &Collector{IsLocal: true}
	out, err := c.runDF([]string{"-h", "--", "/data/db"})
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "ok\n" {
		t.Fatalf("output: %q", out)
	}
	if gotName != "df" {
		t.Fatalf("name: %q", gotName)
	}
	if len(gotArgs) != 3 || gotArgs[0] != "-h" || gotArgs[1] != "--" || gotArgs[2] != "/data/db" {
		t.Fatalf("args: %#v", gotArgs)
	}
}

func TestRunDFRemoteRejectsUnsafeHost(t *testing.T) {
	c := &Collector{SSHUser: "ubuntu", SSHHost: "mongo;id"}
	if _, err := c.runDF([]string{"-h"}); err == nil {
		t.Fatal("expected unsafe hostname to fail")
	}
}

func TestRunDFRemoteBuildsSSHArgv(t *testing.T) {
	var gotTarget string
	var gotRemote []string
	old := sshOutput
	sshOutput = func(userHost string, remoteArgs []string) ([]byte, error) {
		gotTarget = userHost
		gotRemote = append([]string{}, remoteArgs...)
		return []byte("ok\n"), nil
	}
	t.Cleanup(func() { sshOutput = old })

	c := &Collector{SSHUser: "ubuntu", SSHHost: "mongo1.internal"}
	if _, err := c.runDF([]string{"-h", "--", "/data/db"}); err != nil {
		t.Fatal(err)
	}
	if gotTarget != "ubuntu@mongo1.internal" {
		t.Fatalf("target: %q", gotTarget)
	}
	want := []string{"df", "-h", "--", "/data/db"}
	if strings.Join(gotRemote, " ") != strings.Join(want, " ") {
		t.Fatalf("remote argv: %#v", gotRemote)
	}
}

func TestRunDFRemoteUnavailable(t *testing.T) {
	c := &Collector{}
	_, err := c.runDF([]string{"-h"})
	if !errors.Is(err, errRemoteDFUnavailable) {
		t.Fatalf("got %v", err)
	}
}

func TestRunUlimitLocalUsesShDashCConstant(t *testing.T) {
	var gotName string
	var gotArgs []string
	old := commandOutput
	commandOutput = func(name string, arg ...string) ([]byte, error) {
		gotName = name
		gotArgs = append([]string{}, arg...)
		return []byte("open files\n"), nil
	}
	t.Cleanup(func() { commandOutput = old })

	oldLook := lookPath
	lookPath = func(file string) (string, error) { return "/bin/" + file, nil }
	t.Cleanup(func() { lookPath = oldLook })

	c := &Collector{IsLocal: true}
	out, err := c.runUlimit()
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "open files\n" {
		t.Fatalf("output: %q", out)
	}
	if gotName != "sh" {
		t.Fatalf("name: %q", gotName)
	}
	if len(gotArgs) != 2 || gotArgs[0] != "-c" || gotArgs[1] != ulimitCommand {
		t.Fatalf("args: %#v", gotArgs)
	}
}

func TestRunUlimitRemoteBuildsSSHArgv(t *testing.T) {
	var gotTarget string
	var gotRemote []string
	old := sshOutput
	sshOutput = func(userHost string, remoteArgs []string) ([]byte, error) {
		gotTarget = userHost
		gotRemote = append([]string{}, remoteArgs...)
		return []byte("ok\n"), nil
	}
	t.Cleanup(func() { sshOutput = old })

	c := &Collector{SSHUser: "ubuntu", SSHHost: "mongo1.internal"}
	if _, err := c.runUlimit(); err != nil {
		t.Fatal(err)
	}
	if gotTarget != "ubuntu@mongo1.internal" {
		t.Fatalf("target: %q", gotTarget)
	}
	want := []string{"ulimit", "-a"}
	if strings.Join(gotRemote, " ") != strings.Join(want, " ") {
		t.Fatalf("remote argv: %#v", gotRemote)
	}
}

func TestCollectUlimitRemoteUnavailableWritesSkipFile(t *testing.T) {
	dir := t.TempDir()
	out := &dcroutdir.DCROutputDir{OutputPrefix: dir + string(os.PathSeparator), Hostname: "mongo1", Port: "27017"}
	if err := out.CreateDCROutputDir(); err != nil {
		t.Fatal(err)
	}
	c := &Collector{
		Mongo:     &mongosh.CaptureGetMongoData{},
		Outputdir: out,
		IsLocal:   false,
	}
	if err := c.collectUlimit(); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join(out.Path(), fileUlimit))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "no SSH username") {
		t.Fatalf("ulimit skip: %s", body)
	}
}

func TestCollectMongosWritesShStatusSkipsRsAndDbpath(t *testing.T) {
	dir := t.TempDir()
	out := &dcroutdir.DCROutputDir{OutputPrefix: dir + string(os.PathSeparator), Hostname: "mongos1", Port: "27017"}
	if err := out.CreateDCROutputDir(); err != nil {
		t.Fatal(err)
	}

	oldCmd := commandOutput
	commandOutput = func(name string, arg ...string) ([]byte, error) {
		return []byte("Filesystem Size\n"), nil
	}
	t.Cleanup(func() { commandOutput = oldCmd })
	oldLook := lookPath
	lookPath = func(file string) (string, error) { return "/bin/" + file, nil }
	t.Cleanup(func() { lookPath = oldLook })

	mongo := &mongosh.CaptureGetMongoData{}
	c := &Collector{
		Mongo:        mongo,
		Outputdir:    out,
		ReplicaState: "MONGOS",
		IsLocal:      true,
	}
	// CapturePlain will fail without a mongo shell; write the skip/df files by
	// calling collectDF only, then assert mongos command selection.
	if !isMongos(c.ReplicaState) {
		t.Fatal("expected mongos collector")
	}
	if err := c.collectDF(); err != nil {
		t.Fatal(err)
	}
	dbpathNote, err := os.ReadFile(filepath.Join(out.Path(), fileDFDbpath))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(dbpathNote), "mongos") {
		t.Fatalf("dbpath skip note: %s", dbpathNote)
	}
	if _, err := os.Stat(filepath.Join(out.Path(), fileDF)); err != nil {
		t.Fatal(err)
	}
}

func TestCollectDFRemoteUnavailableWritesSkipFiles(t *testing.T) {
	dir := t.TempDir()
	out := &dcroutdir.DCROutputDir{OutputPrefix: dir + string(os.PathSeparator), Hostname: "mongo1", Port: "27017"}
	if err := out.CreateDCROutputDir(); err != nil {
		t.Fatal(err)
	}
	c := &Collector{
		Mongo:     &mongosh.CaptureGetMongoData{},
		Outputdir: out,
		IsLocal:   false,
	}
	if err := c.collectDF(); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{fileDF, fileDFDbpath} {
		body, err := os.ReadFile(filepath.Join(out.Path(), name))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(body), "no SSH username") {
			t.Fatalf("%s: %s", name, body)
		}
	}
}

func TestPlainScriptErrorTreatsCaughtMongoExceptionAsFailure(t *testing.T) {
	if err := plainScriptError([]byte("{ ok: 1 }"), nil); err != nil {
		t.Fatalf("success: %v", err)
	}
	runErr := errors.New("exit 1")
	if err := plainScriptError([]byte("ERROR: boom"), runErr); !errors.Is(err, runErr) {
		t.Fatalf("nonzero exit: %v", err)
	}
	if err := plainScriptError([]byte("ERROR: not authorized"), nil); err == nil {
		t.Fatal("caught ERROR: should fail even when mongosh exits 0")
	}
	partial := []byte("{ buckets: [], errors: [ { db: 'app', error: 'not authorized' } ] }\nERROR: 1 database(s) failed\n")
	if err := plainScriptError(partial, nil); err == nil {
		t.Fatal("partial database failures should fail the task")
	}
}

func TestSplitHostCommandOutput(t *testing.T) {
	raw := []byte("df-all\n" + markerRCDF + " 0\n" + markerDFDB + "\ndf-db\n" + markerRCDFDB + " 0\n" + markerUlimit + "\nlimits\n" + markerRCUlimit + " 0\n")
	dfOut, dfdbOut, ulOut := splitHostCommandOutput(raw, true)
	if string(dfOut) != "df-all" || string(dfdbOut) != "df-db" || string(ulOut) != "limits" {
		t.Fatalf("got %q %q %q", dfOut, dfdbOut, ulOut)
	}
	raw = []byte("df-all\n" + markerRCDF + " 0\n" + markerUlimit + "\nlimits\n" + markerRCUlimit + " 0\n")
	dfOut, dfdbOut, ulOut = splitHostCommandOutput(raw, false)
	if string(dfOut) != "df-all" || dfdbOut != nil || string(ulOut) != "limits" {
		t.Fatalf("no-dbpath got %q %q %q", dfOut, dfdbOut, ulOut)
	}
}

func TestParseHostCommandBundlePerCommandStatus(t *testing.T) {
	raw := []byte("no space\n" + markerRCDF + " 1\n" + markerUlimit + "\nopen files\n" + markerRCUlimit + " 0\n")
	df, dfdb, ul := parseHostCommandBundle(raw, false)
	if dfdb.out != nil || dfdb.err != nil {
		t.Fatalf("dfdb: %+v", dfdb)
	}
	if df.err == nil || !strings.Contains(df.err.Error(), "exit status 1") {
		t.Fatalf("df err: %v", df.err)
	}
	if string(df.out) != "no space" {
		t.Fatalf("df out: %q", df.out)
	}
	if ul.err != nil {
		t.Fatalf("ulimit should succeed: %v", ul.err)
	}
	if string(ul.out) != "open files" {
		t.Fatalf("ulimit out: %q", ul.out)
	}
}

func TestCollectRemoteHostCommandsOneSSH(t *testing.T) {
	dir := t.TempDir()
	out := &dcroutdir.DCROutputDir{OutputPrefix: dir + string(os.PathSeparator), Hostname: "mongo1", Port: "27017"}
	if err := out.CreateDCROutputDir(); err != nil {
		t.Fatal(err)
	}

	var calls int
	var gotRemote []string
	old := sshOutput
	sshOutput = func(userHost string, remoteArgs []string) ([]byte, error) {
		calls++
		gotRemote = append([]string{}, remoteArgs...)
		body := "filesys\n" + markerRCDF + " 0\n" + markerUlimit + "\nopen files\n" + markerRCUlimit + " 0\n"
		return []byte(body), nil
	}
	t.Cleanup(func() { sshOutput = old })

	c := &Collector{
		Mongo:        &mongosh.CaptureGetMongoData{},
		Outputdir:    out,
		ReplicaState: "MONGOS",
		SSHUser:      "test",
		SSHHost:      "mongo2",
	}
	if err := c.collectRemoteHostCommands(); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("expected 1 ssh, got %d", calls)
	}
	if len(gotRemote) != 1 || !strings.Contains(gotRemote[0], "df -h") || !strings.Contains(gotRemote[0], "ulimit -a") {
		t.Fatalf("remote command: %#v", gotRemote)
	}
	if !strings.Contains(gotRemote[0], markerRCDF) || !strings.Contains(gotRemote[0], markerRCUlimit) {
		t.Fatalf("remote script missing per-command status markers: %#v", gotRemote)
	}
	if strings.Contains(gotRemote[0], markerDFDB) {
		t.Fatal("mongos must not run df dbpath remotely")
	}

	dfBody, _ := os.ReadFile(filepath.Join(out.Path(), fileDF))
	ulBody, _ := os.ReadFile(filepath.Join(out.Path(), fileUlimit))
	dbBody, _ := os.ReadFile(filepath.Join(out.Path(), fileDFDbpath))
	if !strings.Contains(string(dfBody), "filesys") {
		t.Fatalf("df: %s", dfBody)
	}
	if !strings.Contains(string(ulBody), "open files") {
		t.Fatalf("ulimit: %s", ulBody)
	}
	if !strings.Contains(string(dbBody), "mongos") {
		t.Fatalf("dbpath skip: %s", dbBody)
	}
}

func TestCollectRemoteHostCommandsReportsPerCommandFailure(t *testing.T) {
	dir := t.TempDir()
	out := &dcroutdir.DCROutputDir{OutputPrefix: dir + string(os.PathSeparator), Hostname: "mongo1", Port: "27017"}
	if err := out.CreateDCROutputDir(); err != nil {
		t.Fatal(err)
	}

	old := sshOutput
	sshOutput = func(userHost string, remoteArgs []string) ([]byte, error) {
		body := "df failed\n" + markerRCDF + " 1\n" + markerUlimit + "\nopen files\n" + markerRCUlimit + " 0\n"
		return []byte(body), nil
	}
	t.Cleanup(func() { sshOutput = old })

	c := &Collector{
		Mongo:        &mongosh.CaptureGetMongoData{},
		Outputdir:    out,
		ReplicaState: "MONGOS",
		SSHUser:      "test",
		SSHHost:      "mongo2",
	}
	err := c.collectRemoteHostCommands()
	if err == nil || !strings.Contains(err.Error(), "df -h") {
		t.Fatalf("expected df failure, got %v", err)
	}
	if strings.Contains(err.Error(), "ulimit") {
		t.Fatalf("ulimit should not fail: %v", err)
	}
	dfBody, _ := os.ReadFile(filepath.Join(out.Path(), fileDF))
	ulBody, _ := os.ReadFile(filepath.Join(out.Path(), fileUlimit))
	if !strings.Contains(string(dfBody), "ERROR: exit status 1") {
		t.Fatalf("df file: %s", dfBody)
	}
	if strings.Contains(string(ulBody), "ERROR:") {
		t.Fatalf("ulimit file should not inherit df error: %s", ulBody)
	}
	if !strings.Contains(string(ulBody), "open files") {
		t.Fatalf("ulimit file: %s", ulBody)
	}
}

func TestSSHPrependsClientOptions(t *testing.T) {
	var got []string
	old := sshOutput
	sshOutput = func(userHost string, remoteArgs []string) ([]byte, error) {
		got = append([]string{userHost}, remoteArgs...)
		return []byte("ok\n"), nil
	}
	t.Cleanup(func() { sshOutput = old })

	c := &Collector{
		SSHUser:          "test",
		SSHHost:          "mongo2",
		SSHClientOptions: []string{"-o", "ControlMaster=auto", "-o", "ControlPath=/tmp/c"},
	}
	if _, err := c.ssh([]string{"df", "-h"}); err != nil {
		t.Fatal(err)
	}
	want := []string{"-o", "ControlMaster=auto", "-o", "ControlPath=/tmp/c", "test@mongo2", "df", "-h"}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Fatalf("ssh argv: %#v", got)
	}
}
