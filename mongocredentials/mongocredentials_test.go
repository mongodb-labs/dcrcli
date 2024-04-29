package mongocredentials

import (
	"testing"
)

//###START TESTS for validateMongoURIString function
/**
func TestValidateMongoURIStringEmptyMongoURI(t *testing.T) {
	mongouri := ""
	err := validateMongoURIString(mongouri)
	if err == nil {
		t.Error("want error for empty mongouri string")
	}
}

func TestValidateMongoURIStringNonEmptyMongoURINotStartsWithMongodb(t *testing.T) {
	mongouri := "dummy://"
	err := validateMongoURIString(mongouri)
	if err == nil {
		t.Error("want error for non mongodb uri string")
	}
}

func TestValidateMongoURIStringValidMongoDBString(t *testing.T) {
	mongouri := "mongodb://localhost:27017"
	err := validateMongoURIString(mongouri)
	if err != nil {
		t.Error("expect nil but got error", err)
	}
}

func TestValidateMongoURIStringInvalidMongoDBURIString(t *testing.T) {
	mongouri := "mongodb:localhost"
	err := validateMongoURIString(mongouri)
	if err == nil {
		t.Error("want error for invalid mongodb uri string")
	}
}
**/

//###END TESTS for validateMongoURIString function

//###START TESTS for validateMongoURIString function

func TestValidationofMongoConnectionURIEmptyMongoURI(t *testing.T) {
	s := Mongocredentials{}
	s.Username = "user"
	s.Password = "pass"
	s.Mongouri = ""

	err := s.validationOfMongoConnectionURI()
	if err == nil {
		t.Error("want error for empty mongouri string")
	}
}

func TestValidationofMongoConnectionURINonEmptyMongoURINotStartsWithMongodb(t *testing.T) {
	s := Mongocredentials{}
	s.Username = "user"
	s.Password = "pass"
	s.Mongouri = "dummy://"

	err := s.validationOfMongoConnectionURI()
	if err == nil {
		t.Error("want error for non mongodb uri string")
	}
}

func TestValidationofMongoConnectionURIValidMongoDBString(t *testing.T) {
	s := Mongocredentials{}
	s.Username = "user"
	s.Password = "pass"
	s.Mongouri = "mongodb://localhost:27017"

	err := s.validationOfMongoConnectionURI()
	if err != nil {
		t.Error("expect nil but got error", err)
	}
}

func TestValidationofMongoConnectionURIInvalidMongoDBURIString(t *testing.T) {
	s := Mongocredentials{}
	s.Username = "user"
	s.Password = "pass"
	s.Mongouri = "mongodb:localhost"

	err := s.validationOfMongoConnectionURI()
	if err == nil {
		t.Error("want error for invalid mongodb uri string")
	}
}

//###END TESTS for validateMongoURIString function
