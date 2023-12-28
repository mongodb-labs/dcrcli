package mongocredentials

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"syscall"

	"golang.org/x/term"
)

type Mongocredentials struct {
	Username string
	Mongouri string
	Password string
}

func setMongoConnectionString(s *Mongocredentials) error {
	reader := bufio.NewReader(os.Stdin)
	fmt.Println("Enter MongoURI: ")

	mongouri, err := reader.ReadString('\n')
	if err != nil {
		return err
	}
	s.Mongouri = strings.TrimSuffix(mongouri, "\n")
	return nil
}

func setMongoUsername(s *Mongocredentials) error {
	reader := bufio.NewReader(os.Stdin)
	fmt.Println("Enter Username: ")
	username, err := reader.ReadString('\n')
	if err != nil {
		return err
	}
	s.Username = strings.TrimSuffix(username, "\n")

	return nil
}

func setMongoPassword(s *Mongocredentials) error {
	fmt.Println("Enter Password: ")
	bytePassword, err := term.ReadPassword(syscall.Stdin)
	if err != nil {
		return err
	}
	s.Password = strings.TrimSuffix(string(bytePassword), "\n")

	return nil
}

// We do not handle passwords, instead let mongo/mongosh ask for password directly from the operator
func Get(s *Mongocredentials) error {
	var err error

	err = setMongoConnectionString(s)
	if err != nil {
		return err
	}

	err = setMongoUsername(s)
	if err != nil {
		return err
	}

	err = setMongoPassword(s)
	if err != nil {
		return err
	}

	return nil
}
