package mongocredentials

import (
	"bufio"
	"fmt"
	"math/rand"
	"os"
	"regexp"
	"strings"
	"syscall"

	"golang.org/x/term"
)

type Mongocredentials struct {
	Username          string
	Mongouri          string
	Password          string
	Seedmongodhost    string
	Seedmongodport    string
	Currentmongodhost string
	Currentmongodport string
	Clustername       string
}

// OBSOLETE
func (mcred *Mongocredentials) validationOfMongoConnectionURI() error {
	isValidMongoDBURI, err := regexp.Match(`^mongodb://.*`, []byte(mcred.Mongouri))
	if err != nil {
		return fmt.Errorf("Regex matching failed for mongo connection uri %s", err)
	}

	if !isValidMongoDBURI {
		return fmt.Errorf(
			"Error: Not a valid MongoDB Connectiong string. It should start with mongodb://",
		)
	}

	return nil
}

// OBSOLETE
func (s *Mongocredentials) askUserForMongoConnectionURI() error {
	reader := bufio.NewReader(os.Stdin)
	fmt.Println("Enter MongoURI(in format mongodb://...): ")

	mongouri, err := reader.ReadString('\n')
	if err != nil {
		return err
	}
	s.Mongouri = strings.TrimSuffix(mongouri, "\n")

	return s.validationOfMongoConnectionURI()
}

// should be called after setting Currentmongodhost and Currentmongodport
func (s *Mongocredentials) SetMongoURI() {
	s.Mongouri = "mongodb://" + s.Currentmongodhost + ":" + s.Currentmongodport + "/admin"
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
	s.Seedmongodhost = strings.TrimSuffix(seedmongodhost, "\n")
	if s.Seedmongodhost == "" {
		println("WARNING: Seed Mongod/Mongos hostname left empty assuming localhost")
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
	s.Clustername = strings.TrimSuffix(clustername, "\n")
	if s.Clustername == "" {
		println("WARNING: Clustername left empty generating unique name")
		s.generateUniqueName()
	}

	return nil
}

func (s *Mongocredentials) generateUniqueName() {
	letter := []rune("abcdefghijklmnopqrstuvwxyz")
	namebuffer := make([]rune, 10)
	for i := range namebuffer {
		namebuffer[i] = letter[rand.Intn(len(letter))]
	}
	s.Clustername = string(namebuffer)
}

func (s *Mongocredentials) askUserForSeedMongoDport() error {
	reader := bufio.NewReader(os.Stdin)
	fmt.Println("Enter Port number of Seed Mongod/Mongos instance: ")
	seedmongodport, err := reader.ReadString('\n')
	if err != nil {
		return err
	}
	s.Seedmongodport = strings.TrimSuffix(seedmongodport, "\n")
	if s.Seedmongodport == "" {
		println("WARNING: Seed Mongod/Mongos port left empty assuming 27017")
		s.Seedmongodport = "27017"
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

	// set current host and port before setting Mongouri
	s.Currentmongodhost = s.Seedmongodhost
	s.Currentmongodport = s.Seedmongodport
	s.SetMongoURI()

	return nil
}
