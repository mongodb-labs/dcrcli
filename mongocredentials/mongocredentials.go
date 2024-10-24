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
	"bufio"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"os"
	"regexp"
	"strconv"
	"strings"
	"syscall"

	"golang.org/x/term"

	"dcrcli/dcrlogger"
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

func (s *Mongocredentials) askUserForMongoConnectionURIoptions() error {
	reader := bufio.NewReader(os.Stdin)
	fmt.Println(
		"Enter MongoURI options for connecting to seed node without replicaSet option(in the format name1=value1&name2=value2): ",
	)

	Mongourioptions, err := reader.ReadString('\n')
	if err != nil {
		return err
	}

	err = checkStringLessThan16MB(Mongourioptions)
	if err != nil {
		return err
	}

	s.Mongourioptions = strings.TrimSuffix(Mongourioptions, "\n")
	if s.Mongourioptions == "" {
		return nil
	}

	err = s.validationOfMongoConnectionURIoptions()
	if err != nil {
		// println(err.Error())
		return err
	}

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

func (s *Mongocredentials) askUserForMongoConnectionUsername() error {
	reader := bufio.NewReader(os.Stdin)
	fmt.Println(
		"Enter Admin Username(A database user with minimum backup, readAnyDatabase, clusterMonitor roles. Leave Blank for cluster without authentication): ",
	)
	username, err := reader.ReadString('\n')
	if err != nil {
		return err
	}

	err = checkStringLessThan16MB(username)
	if err != nil {
		return err
	}

	s.Username = strings.TrimSuffix(username, "\n")
	if s.Username == "" {
		println("WARNING: Admin Username is empty assuming cluster without authentication")
	}

	return nil
}

func (s *Mongocredentials) askUserForMongoConnectionPassword() error {
	fmt.Println("Enter Admin Password(Leave blank for cluster without authentication): ")
	bytePassword, err := term.ReadPassword(syscall.Stdin)
	if err != nil {
		return err
	}

	err = checkStringLessThan16MB(string(bytePassword))
	if err != nil {
		return err
	}

	s.Password = strings.TrimSuffix(string(bytePassword), "\n")

	return nil
}

func (s *Mongocredentials) askUserForSeedMongodHostname() error {
	reader := bufio.NewReader(os.Stdin)
	fmt.Println("Due to privacy/security reasons the program does not scan all machine processes")
	fmt.Println(
		"Only the provided the seed mongod process is used to discover cluster nodes using mongo commands",
	)
	fmt.Println("Enter Hostname of Seed Mongod/Mongos: ")
	seedmongodhost, err := reader.ReadString('\n')
	if err != nil {
		return err
	}

	err = checkStringLessThan16MB(seedmongodhost)
	if err != nil {
		return err
	}

	s.Seedmongodhost = strings.TrimSuffix(seedmongodhost, "\n")
	if s.Seedmongodhost == "" {
		println("WARNING: Seed Mongod/Mongos hostname left empty assuming localhost")
		s.Dcrlog.Debug("mongod host not provided defaulting to localhost")
		s.Seedmongodhost = "localhost"
	}

	return nil
}

func (s *Mongocredentials) askUserForClustername() error {
	reader := bufio.NewReader(os.Stdin)
	fmt.Println("Enter Cluster Name: ")
	clustername, err := reader.ReadString('\n')
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

	s.Clustername = strings.TrimSuffix(clustername, "\n")
	if s.Clustername == "" {
		println("WARNING: Clustername left empty generating unique name")
		s.Dcrlog.Debug("cluster name empty will generate unique random name")
		s.generateUniqueName()
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

func (s *Mongocredentials) askUserForSeedMongoDport() error {
	reader := bufio.NewReader(os.Stdin)
	fmt.Println("Enter Port number of Seed Mongod/Mongos instance: ")
	seedmongodport, err := reader.ReadString('\n')
	if err != nil {
		return err
	}

	err = checkStringLessThan16MB(seedmongodport)
	if err != nil {
		return err
	}

	s.Seedmongodport = strings.TrimSuffix(seedmongodport, "\n")
	if s.Seedmongodport == "" {
		println("WARNING: Seed Mongod/Mongos port left empty assuming 27017")
		s.Dcrlog.Debug("mongod port not provided defaulting to 27017")
		s.Seedmongodport = "27017"
	}

	err = checkValidListenerPort(s.Seedmongodport)
	if err != nil {
		return err
	}

	return nil
}

func (s *Mongocredentials) Get() error {
	var err error

	err = s.askUserForClustername()
	if err != nil {
		return err
	}

	err = s.askUserForSeedMongodHostname()
	if err != nil {
		return err
	}
	err = s.askUserForSeedMongoDport()
	if err != nil {
		return err
	}

	err = s.askUserForMongoConnectionUsername()
	if err != nil {
		return err
	}

	err = s.askUserForMongoConnectionPassword()
	if err != nil {
		return err
	}

	err = s.askUserForMongoConnectionURIoptions()
	if err != nil {
		return err
	}

	// set current host and port before setting Mongouri
	s.Currentmongodhost = s.Seedmongodhost
	s.Currentmongodport = s.Seedmongodport
	s.SetMongoURI()

	return nil
}
