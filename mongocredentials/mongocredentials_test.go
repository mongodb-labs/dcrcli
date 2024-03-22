package mongocredentials

import (
	"testing"
)

//###START TESTS for validateMongoURIString function

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

//###END TESTS for validateMongoURIString function

//###START TESTS for validateMongoURIString function

func TestValidationofMongoConnectionURIEmptyMongoURI(t *testing.T) {
	s := Mongocredentials{
		"user",
		"",
		"pass",
	}

	err := s.validationOfMongoConnectionURI()
	if err == nil {
		t.Error("want error for empty mongouri string")
	}
}

func TestValidationofMongoConnectionURINonEmptyMongoURINotStartsWithMongodb(t *testing.T) {
	s := Mongocredentials{
		"user",
		"dummy://",
		"pass",
	}

	err := s.validationOfMongoConnectionURI()
	if err == nil {
		t.Error("want error for non mongodb uri string")
	}
}

func TestValidationofMongoConnectionURIValidMongoDBString(t *testing.T) {
	s := Mongocredentials{
		"user",
		"mongodb://localhost:27017",
		"pass",
	}

	err := s.validationOfMongoConnectionURI()
	if err != nil {
		t.Error("expect nil but got error", err)
	}
}

func TestValidationofMongoConnectionURIInvalidMongoDBURIString(t *testing.T) {
	s := Mongocredentials{
		"user",
		"mongodb:localhost",
		"pass",
	}

	err := s.validationOfMongoConnectionURI()
	if err == nil {
		t.Error("want error for invalid mongodb uri string")
	}
}

//###END TESTS for validateMongoURIString function
