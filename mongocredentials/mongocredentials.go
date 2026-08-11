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

package mongocredentials

import (
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"regexp"
	"strconv"
	"strings"
	"syscall"

	"golang.org/x/term"

	"dcrcli/dcrconfig"
	"dcrcli/dcrlogger"
	"dcrcli/termui"
)

type Mongocredentials struct {
	Username          string
	Mongouri          string
	Mongourioptions   string
	Password          string
	Seedmongodhost    string
	Seedmongodport    string
	Currentmongodhost string
	Currentmongodport string
	Clustername       string
	Dcrlog            *dcrlogger.DCRLogger
}

func checkStringLessThan16MB(s string) error {
	if len(s) > 16*1024*1024 {
		// The string is too long, so prevent the buffer overflow
		return errors.New("input too large beyond 16mb")
	}
	return nil
}

func checkValidListenerPort(s string) error {
	portnum, err := strconv.Atoi(s)
	if err != nil {
		return errors.New("invalid port number")
	}
	if portnum > 65535 {
		return errors.New("port number cannot exceed 65535")
	}

	return nil
}

func containsReplicaSet(str string) bool {
	return strings.Contains(str, "replicaSet")
}

// Options should be in format name1=value1&name2=value2
func (mcred *Mongocredentials) validationOfMongoConnectionURIoptions() error {
	re := regexp.MustCompile(
		`^[a-zA-Z0-9\-\.]+=[a-zA-Z0-9\-\.]+(&[a-zA-Z0-9\-\.]+=[a-zA-Z0-9\-\.]+)*$`,
	)
	isValidMongoDBURI := false
	isValidMongoDBURI = re.MatchString(mcred.Mongourioptions)

	if !isValidMongoDBURI {
		errmsg := "FATAL: mongo connection uri options should be in format name1=value1&name2=value2. File names can have dash(-) or dot(.)"
		return errors.New(errmsg)
	}

	if containsReplicaSet(mcred.Mongourioptions) {
		errmsg := "FATAL: do not enter replicaSet in options"
		return errors.New(errmsg)
	}

	return nil
}

func (s *Mongocredentials) askUserForMongoConnectionURIoptions(ui *termui.UI) error {
	ui.BeginStep("Extra connection options (optional)")
	ui.Note(
		"Only if your cluster needs extra MongoDB connection settings (e.g. tls=true).",
		"Most users can leave this blank. Do not include replicaSet.",
		"Format: name1=value1&name2=value2",
	)
	line, err := ui.AskInput("")
	if err != nil {
		return err
	}

	err = checkStringLessThan16MB(line)
	if err != nil {
		return err
	}

	s.Mongourioptions = line
	if s.Mongourioptions == "" {
		ui.Ok("No extra URI options")
		return nil
	}

	err = s.validationOfMongoConnectionURIoptions()
	if err != nil {
		return err
	}

	ui.Ok("URI options set")
	return nil
}

// should be called after setting Currentmongodhost and Currentmongodport
func (s *Mongocredentials) SetMongoURI() error {
	var err error
	s.Mongouri = "mongodb://" + s.Currentmongodhost + ":" + s.Currentmongodport + "/admin?directConnection=true&" + s.Mongourioptions
	err = checkStringLessThan16MB(s.Mongouri)
	if err != nil {
		return err
	}
	return nil
}

func (s *Mongocredentials) askUserForMongoConnectionUsername(ui *termui.UI) error {
	ui.BeginStep("MongoDB username")
	ui.Note(
		"Database user for connecting to the cluster (not your laptop or SSH login).",
		"Needs backup, readAnyDatabase, and clusterMonitor roles when auth is enabled.",
		"Leave blank if the cluster has no authentication.",
	)
	username, err := ui.AskInput("")
	if err != nil {
		return err
	}

	err = checkStringLessThan16MB(username)
	if err != nil {
		return err
	}

	s.Username = username
	if s.Username == "" {
		ui.Warn("No MongoDB username; assuming cluster without authentication")
	} else {
		ui.Ok("MongoDB username set")
	}

	return nil
}

func (s *Mongocredentials) askUserForMongoConnectionPassword(ui *termui.UI) error {
	ui.BeginStep("MongoDB password")
	ui.Note(
		"Password for the MongoDB database user above (not your SSH password).",
		"Leave blank if the cluster has no authentication.",
	)
	bytePassword, err := ui.AskPassword("MongoDB password")
	if err != nil {
		return err
	}

	err = checkStringLessThan16MB(bytePassword)
	if err != nil {
		return err
	}

	s.Password = strings.TrimSuffix(bytePassword, "\n")
	if s.Password == "" {
		if s.Username == "" {
			ui.Warn("No MongoDB password; assuming cluster without authentication")
		} else {
			ui.Warn("MongoDB password left empty")
		}
	} else {
		ui.Ok("MongoDB password received")
	}

	return nil
}

func (s *Mongocredentials) askUserForSeedMongodHostname(ui *termui.UI) error {
	ui.BeginStep("Seed node (cluster entry point)")
	ui.Note(
		"One reachable mongod/mongos (local or remote). dcrcli discovers other members via hello/getShardMap.",
		"For sharded clusters, use a mongos when possible (e.g. localhost, rs0-mongo1.example.com).",
	)
	seedmongodhost, err := ui.AskInput("")
	if err != nil {
		return err
	}

	err = checkStringLessThan16MB(seedmongodhost)
	if err != nil {
		return err
	}

	s.Seedmongodhost = seedmongodhost
	if s.Seedmongodhost == "" {
		ui.Warn("Hostname left empty; using localhost")
		s.Dcrlog.Debug("mongod host not provided defaulting to localhost")
		s.Seedmongodhost = "localhost"
	} else {
		ui.Ok("Seed host: " + s.Seedmongodhost)
	}

	return nil
}

func (s *Mongocredentials) askUserForClustername(ui *termui.UI) error {
	ui.BeginStep("Cluster name")
	ui.Note("Used as the output directory name. Leave blank to generate a unique name.")
	clustername, err := ui.AskInput("")
	if err != nil {
		return err
	}
	s.Dcrlog.Debug(
		fmt.Sprintf(
			"Clustername entered is: %s", clustername,
		),
	)

	err = checkStringLessThan16MB(clustername)
	if err != nil {
		return err
	}

	s.Clustername = clustername
	if s.Clustername == "" {
		ui.Warn("Cluster name left empty; generating unique name")
		s.Dcrlog.Debug("cluster name empty will generate unique random name")
		s.generateUniqueName()
		ui.Ok("Generated cluster name: " + s.Clustername)
	} else {
		ui.Ok("Cluster name: " + s.Clustername)
	}

	return nil
}

func (s *Mongocredentials) generateUniqueName() {
	letter := []rune("abcdefghijklmnopqrstuvwxyz")
	namebuffer := make([]rune, 10)
	max := big.NewInt(int64(len(letter)))
	for i := range namebuffer {
		n, _ := rand.Int(rand.Reader, max)
		namebuffer[i] = letter[n.Int64()]
	}
	s.Clustername = string(namebuffer)
	s.Dcrlog.Debug(fmt.Sprintf("generate unique name: %s", s.Clustername))
}

func (s *Mongocredentials) askUserForSeedMongoDport(ui *termui.UI) error {
	ui.BeginStep("Seed port")
	ui.Note(
		"TCP port of the seed mongod/mongos listener.",
		"Leave blank for default port 27017.",
	)
	seedmongodport, err := ui.AskInput("")
	if err != nil {
		return err
	}

	err = checkStringLessThan16MB(seedmongodport)
	if err != nil {
		return err
	}

	s.Seedmongodport = seedmongodport
	if s.Seedmongodport == "" {
		ui.Warn("Port left empty; using default port 27017")
		s.Dcrlog.Debug("mongod port not provided defaulting to 27017")
		s.Seedmongodport = "27017"
	}

	err = checkValidListenerPort(s.Seedmongodport)
	if err != nil {
		return err
	}

	ui.Ok("Seed port: " + s.Seedmongodport)
	return nil
}

// Get collects MongoDB connection details through interactive prompts.
func (s *Mongocredentials) Get(ui *termui.UI) error {
	var err error

	// TSTOOLS-16661: future improvement to pass all connection fields via config and skip prompts.
	ui.Header("MongoDB connection setup")
	ui.Tip("Tip: use -config <file> to skip these prompts. See README: https://github.com/mongodb-labs/dcrcli#config-file-recommended")

	err = s.askUserForClustername(ui)
	if err != nil {
		return err
	}

	err = s.askUserForSeedMongodHostname(ui)
	if err != nil {
		return err
	}
	err = s.askUserForSeedMongoDport(ui)
	if err != nil {
		return err
	}

	err = s.askUserForMongoConnectionUsername(ui)
	if err != nil {
		return err
	}

	err = s.askUserForMongoConnectionPassword(ui)
	if err != nil {
		return err
	}

	err = s.askUserForMongoConnectionURIoptions(ui)
	if err != nil {
		return err
	}

	ui.Section("Connection ready")
	ui.Ok(fmt.Sprintf("mongodb://%s:%s/admin", s.Seedmongodhost, s.Seedmongodport))
	ui.Blank()

	// set current host and port before setting Mongouri
	s.Currentmongodhost = s.Seedmongodhost
	s.Currentmongodport = s.Seedmongodport
	s.SetMongoURI()

	return nil
}

// GetFromConfig populates credentials from a config file instead of interactive prompts.
// Any validation error names the offending config field so the user knows what to fix.
func (s *Mongocredentials) GetFromConfig(ui *termui.UI, c *dcrconfig.Config) error {
	s.Clustername = strings.TrimSpace(c.ClusterName)
	if s.Clustername == "" {
		s.generateUniqueName()
	}
	if err := checkStringLessThan16MB(s.Clustername); err != nil {
		return fmt.Errorf("config field \"cluster_name\": %w", err)
	}

	s.Seedmongodhost = strings.TrimSpace(c.SeedHost)
	if s.Seedmongodhost == "" {
		s.Seedmongodhost = "localhost"
		s.Dcrlog.Debug("config seed_host empty, defaulting to localhost")
	}
	if err := checkStringLessThan16MB(s.Seedmongodhost); err != nil {
		return fmt.Errorf("config field \"seed_host\": %w", err)
	}

	s.Seedmongodport = strings.TrimSpace(c.SeedPort)
	if s.Seedmongodport == "" {
		s.Seedmongodport = "27017"
		s.Dcrlog.Debug("config seed_port empty, defaulting to 27017")
	}
	if err := checkValidListenerPort(s.Seedmongodport); err != nil {
		return fmt.Errorf("config field \"seed_port\": %w", err)
	}

	s.Username = strings.TrimSpace(c.Username)
	if err := checkStringLessThan16MB(s.Username); err != nil {
		return fmt.Errorf("config field \"username\": %w", err)
	}

	// Password is never stored in the config file.
	// Prompt interactively when a username is set; skip for no-auth clusters.
	if s.Username != "" {
		if !term.IsTerminal(int(syscall.Stdin)) {
			return fmt.Errorf("config: cannot prompt for MongoDB password (stdin is not a terminal)")
		}
		ui.BeginStep("MongoDB password")
		ui.Note("Password for the MongoDB database user (not stored in the config file).")
		bytePassword, err := ui.AskPassword("MongoDB password")
		if err != nil {
			return fmt.Errorf("config: failed to read password interactively: %w", err)
		}
		s.Password = strings.TrimSuffix(string(bytePassword), "\n")
		s.Dcrlog.Debug("password entered interactively")
		if err := checkStringLessThan16MB(s.Password); err != nil {
			return fmt.Errorf("config: password input: %w", err)
		}
		ui.Ok("Password received")
	} else {
		s.Dcrlog.Debug("no username set, assuming no-auth cluster")
	}

	s.Mongourioptions = strings.TrimSpace(c.URIOptions)
	if s.Mongourioptions != "" {
		if err := s.validationOfMongoConnectionURIoptions(); err != nil {
			return fmt.Errorf("config field \"uri_options\": %w", err)
		}
	}

	s.Currentmongodhost = s.Seedmongodhost
	s.Currentmongodport = s.Seedmongodport
	return s.SetMongoURI()
}
