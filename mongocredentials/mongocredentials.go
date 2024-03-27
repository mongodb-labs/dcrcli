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

func (s *Mongocredentials) askUserForMongoConnectionUsername() error {
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

func (s *Mongocredentials) askUserForMongoConnectionPassword() error {
	fmt.Println("Enter Password: ")
	bytePassword, err := term.ReadPassword(syscall.Stdin)
	if err != nil {
		return err
	}

	s.Password = strings.TrimSuffix(string(bytePassword), "\n")

	return nil
}

func (s *Mongocredentials) Get() error {
	var err error

	err = s.askUserForMongoConnectionURI()
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

	return nil
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
