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

// Package diagcommands collects DCR command outputs (df, ulimit, rs.conf,
// rs.status, replication helpers, and sh.status on mongos) that getMongoData
// does not write as standalone files.
package diagcommands

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"unicode"

	"dcrcli/dcrlogger"
	"dcrcli/dcroutdir"
	"dcrcli/mongosh"
)

const (
	fileDF                      = "df-h.txt"
	fileDFDbpath                = "df-h-dbpath.txt"
	fileUlimit                  = "ulimit-a.txt"
	fileRsConf                  = "rs.conf.txt"
	fileRsStatus                = "rs.status.txt"
	filePrintRepl               = "rs.printReplicationInfo.txt"
	filePrintSecRepl            = "rs.printSecondaryReplicationInfo.txt"
	fileShStatus                = "sh.status.txt"
	fileListCatalogTimeSeries   = "listCatalog-system.buckets.txt"
	fileShardedIndexConsistency = "serverStatus.shardedIndexConsistency.txt"
	fileUniqueIndexes           = "uniqueIndexes.txt"
	fileWritePermission         = 0666
)

// commandOutput runs a local executable with argv (never a shell). Tests replace this.
var commandOutput = func(name string, arg ...string) ([]byte, error) {
	cmd := exec.Command(name, arg...)
	return cmd.CombinedOutput()
}

// sshOutput runs `ssh user@host <remoteArgs...>` with argv. OpenSSH concatenates
// remoteArgs into a remote command string, so callers must pass only literal
// tokens and a validated dbpath. Tests replace this.
var sshOutput = func(userHost string, remoteArgs []string) ([]byte, error) {
	args := append([]string{userHost}, remoteArgs...)
	cmd := exec.Command("ssh", args...)
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr
	return cmd.Output()
}

func (c *Collector) ssh(remoteArgs []string) ([]byte, error) {
	if hasShellMeta(c.SSHUser) || hasShellMeta(c.SSHHost) {
		return nil, fmt.Errorf("unsafe SSH username or hostname")
	}
	args := append([]string{}, c.SSHClientOptions...)
	args = append(args, c.SSHUser+"@"+c.SSHHost)
	args = append(args, remoteArgs...)
	return sshOutput(args[0], args[1:])
}

var lookPath = exec.LookPath

// Collector writes per-node diagnostic command outputs into Outputdir.
type Collector struct {
	Mongo            *mongosh.CaptureGetMongoData
	Outputdir        *dcroutdir.DCROutputDir
	ReplicaState     string
	ShardMapHostRole string
	IsLocal          bool
	SSHUser          string
	SSHHost          string
	// SSHClientOptions are extra ssh(1) args before user@host (ControlMaster).
	SSHClientOptions []string
	Dcrlog           *dcrlogger.DCRLogger
}

func (c *Collector) logInfo(msg string) {
	if c.Dcrlog != nil {
		c.Dcrlog.Info(msg)
	}
}

func (c *Collector) logWarn(msg string) {
	if c.Dcrlog != nil {
		c.Dcrlog.Warn(msg)
	}
}

func (c *Collector) dest(name string) string {
	return filepath.Join(c.Outputdir.Path(), name)
}

func (c *Collector) writeFile(name string, body []byte) error {
	return os.WriteFile(c.dest(name), body, fileWritePermission)
}

func (c *Collector) capturePlain(script *string, filename string) error {
	c.Mongo.CurrentCommand = script
	runErr := c.Mongo.RunPlainDBCommand()
	var body []byte
	if c.Mongo.Getparsedjsonoutput != nil {
		body = c.Mongo.Getparsedjsonoutput.Bytes()
	}
	if writeErr := c.writeFile(filename, body); writeErr != nil {
		return writeErr
	}
	return plainScriptError(body, runErr)
}

// plainScriptError keeps mongosh's non-zero exit, and also fails when the
// embedded helpers catch an exception and print "ERROR:" (mongosh still exits 0).
func plainScriptError(body []byte, runErr error) error {
	if runErr != nil {
		return runErr
	}
	if strings.Contains(string(body), "ERROR:") {
		return fmt.Errorf("command printed ERROR (shell exited 0)")
	}
	return nil
}

// Collect writes replica-set (or mongos) helper output and df listings.
// Individual command failures are recorded in the output files and joined into
// the returned error so collection of the remaining commands still runs.
func (c *Collector) Collect() error {
	if c.Mongo == nil || c.Outputdir == nil {
		return fmt.Errorf("diagcommands: Mongo and Outputdir must be set")
	}

	var errs []error
	if isMongos(c.ReplicaState) {
		c.logInfo("Collecting sh.status() (mongos)")
		if err := c.capturePlain(&mongosh.ShStatusCommand, fileShStatus); err != nil {
			errs = append(errs, fmt.Errorf("sh.status(): %w", err))
		}
	} else if isStandalone(c.ReplicaState) {
		if err := c.skipReplicaSetHelpers(); err != nil {
			errs = append(errs, err)
		}
		if err := c.collectTimeSeriesCatalog(); err != nil {
			errs = append(errs, err)
		}
		if err := c.collectUniqueIndexes(); err != nil {
			errs = append(errs, err)
		}
	} else {
		c.logInfo("Collecting rs.conf(), rs.status(), rs.printReplicationInfo(), rs.printSecondaryReplicationInfo()")
		for _, item := range []struct {
			script *string
			name   string
			label  string
		}{
			{&mongosh.RsConfCommand, fileRsConf, "rs.conf()"},
			{&mongosh.RsStatusCommand, fileRsStatus, "rs.status()"},
			{&mongosh.PrintReplicationInfoCommand, filePrintRepl, "rs.printReplicationInfo()"},
			{&mongosh.PrintSecondaryReplicationInfoCommand, filePrintSecRepl, "rs.printSecondaryReplicationInfo()"},
		} {
			if err := c.capturePlain(item.script, item.name); err != nil {
				errs = append(errs, fmt.Errorf("%s: %w", item.label, err))
			}
		}
		if err := c.collectTimeSeriesCatalog(); err != nil {
			errs = append(errs, err)
		}
		if err := c.collectUniqueIndexes(); err != nil {
			errs = append(errs, err)
		}
		if err := c.collectShardedIndexConsistency(); err != nil {
			errs = append(errs, err)
		}
	}

	if err := c.collectHostCommands(); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

func (c *Collector) collectHostCommands() error {
	if c.IsLocal {
		var errs []error
		if err := c.collectDF(); err != nil {
			errs = append(errs, err)
		}
		if err := c.collectUlimit(); err != nil {
			errs = append(errs, err)
		}
		return errors.Join(errs...)
	}
	return c.collectRemoteHostCommands()
}

func isMongos(state string) bool {
	return strings.EqualFold(strings.TrimSpace(state), "MONGOS")
}

func isStandalone(state string) bool {
	return strings.EqualFold(strings.TrimSpace(state), "STANDALONE")
}

func (c *Collector) skipReplicaSetHelpers() error {
	c.logInfo("Skipping rs.* helpers (standalone mongod, not a replica set)")
	skip := []byte("skipped: standalone mongod is not running with a replica set\n")
	var errs []error
	for _, name := range []string{fileRsConf, fileRsStatus, filePrintRepl, filePrintSecRepl} {
		if err := c.writeFile(name, skip); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func isArbiter(state string) bool {
	return strings.EqualFold(strings.TrimSpace(state), "ARBITER")
}

func isConfigServer(role string) bool {
	return strings.EqualFold(strings.TrimSpace(role), "config")
}

func isPrimary(state string) bool {
	return strings.EqualFold(strings.TrimSpace(state), "PRIMARY")
}

// collectTimeSeriesCatalog runs $listCatalog for system.buckets.* on data-bearing
// mongods (replica-set members and shard mongods). Skipped on mongos, arbiters,
// and config servers — those are not the DCR data-bearing targets for this check.
func (c *Collector) collectTimeSeriesCatalog() error {
	if isArbiter(c.ReplicaState) || isConfigServer(c.ShardMapHostRole) {
		c.logInfo("Skipping $listCatalog system.buckets (not a data-bearing shard/replica mongod)")
		return nil
	}
	c.logInfo("Collecting $listCatalog system.buckets (time series)")
	if err := c.capturePlain(&mongosh.ListCatalogTimeSeriesCommand, fileListCatalogTimeSeries); err != nil {
		return fmt.Errorf("$listCatalog system.buckets: %w", err)
	}
	return nil
}

// collectUniqueIndexes records WiredTiger formatVersion for non-_id unique
// indexes via $collStats (13 or 14 = post-4.2 / mongosync reverse-safe).
// Skipped on mongos, arbiters, and config servers. Does not run validate().
func (c *Collector) collectUniqueIndexes() error {
	if isArbiter(c.ReplicaState) || isConfigServer(c.ShardMapHostRole) {
		c.logInfo("Skipping unique indexes formatVersion (not a data-bearing shard/replica mongod)")
		return nil
	}
	c.logInfo("Collecting unique index formatVersion ($collStats on collections with unique indexes)")
	if err := c.capturePlain(&mongosh.UniqueIndexesCommand, fileUniqueIndexes); err != nil {
		return fmt.Errorf("unique indexes: %w", err)
	}
	return nil
}

// collectShardedIndexConsistency runs db.serverStatus().shardedIndexConsistency
// on the config-server primary only. It is not present on mongos or shard mongods.
func (c *Collector) collectShardedIndexConsistency() error {
	if !isConfigServer(c.ShardMapHostRole) {
		return nil
	}
	if isArbiter(c.ReplicaState) {
		return nil
	}
	if !isPrimary(c.ReplicaState) {
		role := strings.TrimSpace(c.ReplicaState)
		msg := "skipped: shardedIndexConsistency is collected from the config-server primary; this node's replica role is unknown\n"
		if role != "" {
			msg = fmt.Sprintf(
				"skipped: shardedIndexConsistency is collected from the config-server primary; this node is %s\n",
				role,
			)
		}
		c.logInfo("Skipping shardedIndexConsistency on non-primary config member")
		return c.writeFile(fileShardedIndexConsistency, []byte(msg))
	}
	c.logInfo("Collecting serverStatus().shardedIndexConsistency (config primary)")
	if err := c.capturePlain(&mongosh.ShardedIndexConsistencyCommand, fileShardedIndexConsistency); err != nil {
		return fmt.Errorf("shardedIndexConsistency: %w", err)
	}
	return nil
}

func (c *Collector) collectDF() error {
	c.logInfo("Collecting df -h")
	out, err := c.runDF([]string{"-h"})
	if errors.Is(err, errRemoteDFUnavailable) {
		msg := []byte("skipped: node is remote and no SSH username was set\n")
		c.logWarn("Skipping df collection; node is remote and no SSH username was set")
		if werr := c.writeFile(fileDF, msg); werr != nil {
			return werr
		}
		return c.writeFile(fileDFDbpath, msg)
	}
	if err != nil {
		_ = c.writeFile(fileDF, formatCommandResult("df -h", out, err))
		return fmt.Errorf("df -h: %w", err)
	}
	if werr := c.writeFile(fileDF, formatCommandResult("df -h", out, nil)); werr != nil {
		return werr
	}

	if isMongos(c.ReplicaState) {
		c.logInfo("Skipping df -h <dbpath> on mongos (no storage.dbPath)")
		return c.writeFile(fileDFDbpath, []byte("skipped: mongos has no storage.dbPath\n"))
	}

	dbpath, err := c.readDbPath()
	if err != nil {
		_ = c.writeFile(fileDFDbpath, []byte("skipped: could not read storage.dbPath: "+err.Error()+"\n"))
		c.logWarn("Could not read storage.dbPath for df -h <dbpath>: " + err.Error())
		return nil
	}
	if dbpath == "" {
		_ = c.writeFile(fileDFDbpath, []byte("skipped: storage.dbPath is empty\n"))
		c.logWarn("storage.dbPath is empty; skipping df -h <dbpath>")
		return nil
	}
	if !isSafeUnixPath(dbpath) {
		msg := fmt.Sprintf("skipped: storage.dbPath %q is not a safe Unix path for df\n", dbpath)
		_ = c.writeFile(fileDFDbpath, []byte(msg))
		c.logWarn(strings.TrimSpace(msg))
		return nil
	}

	c.logInfo("Collecting df -h -- " + dbpath)
	dbOut, dbErr := c.runDF([]string{"-h", "--", dbpath})
	label := "df -h -- " + dbpath
	if werr := c.writeFile(fileDFDbpath, formatCommandResult(label, dbOut, dbErr)); werr != nil {
		return werr
	}
	if dbErr != nil {
		return fmt.Errorf("df -h dbpath: %w", dbErr)
	}
	return nil
}

func (c *Collector) readDbPath() (string, error) {
	if err := c.Mongo.RunGetDbPathWithEval(); err != nil {
		return "", err
	}
	raw := ""
	if c.Mongo.Getparsedjsonoutput != nil {
		raw = c.Mongo.Getparsedjsonoutput.String()
	}
	return parseDbPathOutput(raw), nil
}

func (c *Collector) runDF(dfArgs []string) ([]byte, error) {
	if c.IsLocal {
		if _, err := lookPath("df"); err != nil {
			return nil, fmt.Errorf("df not found on PATH")
		}
		return commandOutput("df", dfArgs...)
	}
	if c.SSHUser == "" || c.SSHHost == "" {
		return nil, errRemoteDFUnavailable
	}
	return c.ssh(append([]string{"df"}, dfArgs...))
}

var errRemoteDFUnavailable = errors.New("node is remote and no SSH username was set")

const ulimitCommand = "ulimit -a"

func (c *Collector) collectUlimit() error {
	c.logInfo("Collecting ulimit -a")
	out, err := c.runUlimit()
	if errors.Is(err, errRemoteDFUnavailable) {
		msg := []byte("skipped: node is remote and no SSH username was set\n")
		c.logWarn("Skipping ulimit -a; node is remote and no SSH username was set")
		return c.writeFile(fileUlimit, msg)
	}
	if werr := c.writeFile(fileUlimit, formatCommandResult(ulimitCommand, out, err)); werr != nil {
		return werr
	}
	if err != nil {
		return fmt.Errorf("ulimit -a: %w", err)
	}
	return nil
}

func (c *Collector) runUlimit() ([]byte, error) {
	if c.IsLocal {
		// ulimit is a POSIX shell builtin, not a binary. The -c script is a
		// compile-time constant (never interpolated) so this is not injection.
		if _, err := lookPath("sh"); err != nil {
			return nil, fmt.Errorf("sh not found on PATH (needed for ulimit -a)")
		}
		return commandOutput("sh", "-c", ulimitCommand)
	}
	if c.SSHUser == "" || c.SSHHost == "" {
		return nil, errRemoteDFUnavailable
	}
	return c.ssh([]string{"ulimit", "-a"})
}

const (
	markerDFDB               = "===DCRCLI_DFDBPATH==="
	markerUlimit             = "===DCRCLI_ULIMIT==="
	markerRCDF               = "===DCRCLI_RC_DF==="
	markerRCDFDB             = "===DCRCLI_RC_DFDB==="
	markerRCUlimit           = "===DCRCLI_RC_ULIMIT==="
	remoteHostScriptNoDbpath = "df -h; printf '%s %s\\n' '" + markerRCDF + "' \"$?\"; printf '%s\\n' '" + markerUlimit + "'; ulimit -a; printf '%s %s\\n' '" + markerRCUlimit + "' \"$?\""
	sshSkipNote              = "skipped: node is remote and no SSH username was set\n"
)

func remoteHostScriptWithDbpath(dbpath string) string {
	return "df -h; printf '%s %s\\n' '" + markerRCDF + "' \"$?\"; printf '%s\\n' '" + markerDFDB + "'; df -h -- " + dbpath + "; printf '%s %s\\n' '" + markerRCDFDB + "' \"$?\"; printf '%s\\n' '" + markerUlimit + "'; ulimit -a; printf '%s %s\\n' '" + markerRCUlimit + "' \"$?\""
}

type hostCmdResult struct {
	out []byte
	err error
}

// collectRemoteHostCommands runs df -h, optional df -h <dbpath>, and ulimit -a
// in a single SSH session so password SSH prompts once for host commands.
func (c *Collector) collectRemoteHostCommands() error {
	c.logInfo("Collecting df -h, df -h <dbpath>, and ulimit -a over one SSH session")
	if c.SSHUser == "" || c.SSHHost == "" {
		msg := []byte(sshSkipNote)
		c.logWarn("Skipping remote host commands; node is remote and no SSH username was set")
		var errs []error
		for _, name := range []string{fileDF, fileDFDbpath, fileUlimit} {
			if werr := c.writeFile(name, msg); werr != nil {
				errs = append(errs, werr)
			}
		}
		return errors.Join(errs...)
	}

	dbpath := ""
	dbpathNote := ""
	if isMongos(c.ReplicaState) {
		dbpathNote = "skipped: mongos has no storage.dbPath\n"
		c.logInfo("Skipping df -h <dbpath> on mongos (no storage.dbPath)")
	} else {
		p, err := c.readDbPath()
		switch {
		case err != nil:
			dbpathNote = "skipped: could not read storage.dbPath: " + err.Error() + "\n"
			c.logWarn("Could not read storage.dbPath for df -h <dbpath>: " + err.Error())
		case p == "":
			dbpathNote = "skipped: storage.dbPath is empty\n"
			c.logWarn("storage.dbPath is empty; skipping df -h <dbpath>")
		case !isSafeUnixPath(p):
			dbpathNote = fmt.Sprintf("skipped: storage.dbPath %q is not a safe Unix path for df\n", p)
			c.logWarn(strings.TrimSpace(dbpathNote))
		default:
			dbpath = p
		}
	}

	script := remoteHostScriptNoDbpath
	if dbpath != "" {
		script = remoteHostScriptWithDbpath(dbpath)
	}
	out, sshErr := c.ssh([]string{script})
	dfRes, dfdbRes, ulRes := parseHostCommandBundle(out, dbpath != "")
	if sshErr != nil && !hostBundleHasStatus(out) {
		if dfRes.err == nil {
			dfRes.err = sshErr
		}
		if ulRes.err == nil {
			ulRes.err = sshErr
		}
		if dbpath != "" && dfdbRes.err == nil {
			dfdbRes.err = sshErr
		}
	}

	var errs []error
	if werr := c.writeFile(fileDF, formatCommandResult("df -h", dfRes.out, dfRes.err)); werr != nil {
		errs = append(errs, werr)
	}
	if dfRes.err != nil {
		errs = append(errs, fmt.Errorf("df -h: %w", dfRes.err))
	}
	if dbpathNote != "" {
		if werr := c.writeFile(fileDFDbpath, []byte(dbpathNote)); werr != nil {
			errs = append(errs, werr)
		}
	} else {
		label := "df -h -- " + dbpath
		if werr := c.writeFile(fileDFDbpath, formatCommandResult(label, dfdbRes.out, dfdbRes.err)); werr != nil {
			errs = append(errs, werr)
		}
		if dfdbRes.err != nil {
			errs = append(errs, fmt.Errorf("df -h dbpath: %w", dfdbRes.err))
		}
	}
	if werr := c.writeFile(fileUlimit, formatCommandResult(ulimitCommand, ulRes.out, ulRes.err)); werr != nil {
		errs = append(errs, werr)
	}
	if ulRes.err != nil {
		errs = append(errs, fmt.Errorf("ulimit -a: %w", ulRes.err))
	}
	return errors.Join(errs...)
}

func hostBundleHasStatus(out []byte) bool {
	s := string(out)
	return strings.Contains(s, markerRCDF) || strings.Contains(s, markerRCUlimit)
}

func parseHostCommandBundle(out []byte, withDbpath bool) (df, dfdb, ul hostCmdResult) {
	dfPart, ulPart := splitMarkedSection(out, markerUlimit)
	if withDbpath {
		dfSec, dfdbSec := splitMarkedSection(dfPart, markerDFDB)
		df = parseCmdStatus(dfSec, markerRCDF)
		dfdb = parseCmdStatus(dfdbSec, markerRCDFDB)
	} else {
		df = parseCmdStatus(dfPart, markerRCDF)
	}
	ul = parseCmdStatus(ulPart, markerRCUlimit)
	return df, dfdb, ul
}

func parseCmdStatus(in []byte, marker string) hostCmdResult {
	body, rc, ok := splitStatusSuffix(in, marker)
	if !ok {
		return hostCmdResult{out: body, err: fmt.Errorf("missing exit-status marker")}
	}
	if rc != 0 {
		return hostCmdResult{out: body, err: fmt.Errorf("exit status %d", rc)}
	}
	return hostCmdResult{out: body}
}

func splitStatusSuffix(in []byte, marker string) (body []byte, rc int, ok bool) {
	s := string(in)
	i := strings.Index(s, marker)
	if i < 0 {
		return bytesTrimRightNewline(in), 0, false
	}
	body = bytesTrimRightNewline([]byte(s[:i]))
	rest := strings.TrimSpace(s[i+len(marker):])
	n, err := fmt.Sscanf(rest, "%d", &rc)
	if err != nil || n != 1 {
		return body, 0, false
	}
	return body, rc, true
}

func splitHostCommandOutput(out []byte, withDbpath bool) (dfOut, dfdbOut, ulOut []byte) {
	df, dfdb, ul := parseHostCommandBundle(out, withDbpath)
	if withDbpath {
		return df.out, dfdb.out, ul.out
	}
	return df.out, nil, ul.out
}

func splitMarkedSection(in []byte, marker string) (before, after []byte) {
	s := string(in)
	i := strings.Index(s, marker)
	if i < 0 {
		return bytesTrimRightNewline(in), nil
	}
	return bytesTrimRightNewline([]byte(s[:i])), bytesTrimRightNewline(bytesTrimLeftNewline([]byte(s[i+len(marker):])))
}

func bytesTrimRightNewline(b []byte) []byte {
	return []byte(strings.TrimRight(string(b), "\n"))
}

func bytesTrimLeftNewline(b []byte) []byte {
	return []byte(strings.TrimLeft(string(b), "\n"))
}

func formatCommandResult(command string, out []byte, err error) []byte {
	var b strings.Builder
	b.WriteString("Command: ")
	b.WriteString(command)
	b.WriteByte('\n')
	b.WriteByte('\n')
	b.Write(out)
	if err != nil {
		if len(out) > 0 && !strings.HasSuffix(string(out), "\n") {
			b.WriteByte('\n')
		}
		b.WriteString("ERROR: ")
		b.WriteString(err.Error())
		b.WriteByte('\n')
	}
	return []byte(b.String())
}

func parseDbPathOutput(raw string) string {
	s := strings.TrimSpace(raw)
	s = trimMatchingQuotes(s)
	s = strings.TrimSpace(s)
	switch strings.ToLower(s) {
	case "", "undefined", "null":
		return ""
	}
	return s
}

func trimMatchingQuotes(s string) string {
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

// isSafeUnixPath allows only an absolute Unix path with no shell metacharacters.
// Required because OpenSSH joins remote argv into a string executed by the
// remote login shell — dbpath must never be able to inject extra commands.
func isSafeUnixPath(p string) bool {
	if p == "" || !strings.HasPrefix(p, "/") {
		return false
	}
	if strings.Contains(p, "..") {
		return false
	}
	if strings.HasPrefix(filepath.Base(p), "-") {
		return false
	}
	for _, r := range p {
		if r > unicode.MaxASCII || r < 32 || r == 127 {
			return false
		}
		if !(unicode.IsLetter(r) || unicode.IsDigit(r) || r == '/' || r == '.' || r == '_' || r == '-') {
			return false
		}
	}
	return true
}

func hasShellMeta(s string) bool {
	if s == "" {
		return true
	}
	return strings.ContainsAny(s, " \t\n\r\"'`$&|;<>(){}[]*?!~\\")
}
