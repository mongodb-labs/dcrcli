package mongocredentials

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

type Mongocredentials struct {
	Username string
	Mongouri string
}

// We do not handle passwords, instead let mongo/mongosh ask for password directly from the operator
func Get(s *Mongocredentials) error {
	reader := bufio.NewReader(os.Stdin)

	fmt.Println("Enter MongoURI: ")

	mongouri, err := reader.ReadString('\n')
	if err != nil {
		return err
	}
	s.Mongouri = mongouri

	fmt.Println("Enter Username: ")
	username, err := reader.ReadString('\n')
	if err != nil {
		return err
	}
	s.Username = strings.TrimSuffix(username, "\n")

	return nil
}
