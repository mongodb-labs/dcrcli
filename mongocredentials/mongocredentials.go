package mongocredentials

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"
	"syscall"

	"golang.org/x/term"
)

type Mongocredentials struct {
	Username string
	Mongouri string
	Password string
}

func validateMongoURIString(mongouri string) error {
	if mongouri == "" {
		return fmt.Errorf(
			"Error: Required MongoDB Connection String is missing. Further processing stopped.",
		)
	}

	isValidMongoDBURI, err := regexp.Match(`^mongodb://.*`, []byte(mongouri))
	if err != nil {
		return err
	}
	if !isValidMongoDBURI {
		return fmt.Errorf(
			"Error: Not a valid MongoDB Connectiong string. It should start with mongodb://",
		)
	}

	return nil
}

func setMongoConnectionString(s *Mongocredentials) error {
	reader := bufio.NewReader(os.Stdin)
	fmt.Println("Enter MongoURI(in format mongodb://...): ")

	mongouri, err := reader.ReadString('\n')
	if err != nil {
		return err
	}
	s.Mongouri = strings.TrimSuffix(mongouri, "\n")

	return validateMongoURIString(s.Mongouri)
}

func setMongoUsername(s *Mongocredentials) error {
	reader := bufio.NewReader(os.Stdin)
	fmt.Println("Enter Username: ")
	username, err := reader.ReadString('\n')
	if err != nil {
		return err
	}
	s.Username = strings.TrimSuffix(username, "\n")
	if s.Username == "" {
		println("WARNING: Username is empty")
	}

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
