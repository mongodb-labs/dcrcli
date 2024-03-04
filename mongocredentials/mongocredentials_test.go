package mongocredentials

import (
	"testing"
)

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
