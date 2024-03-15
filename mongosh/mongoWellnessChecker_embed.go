package mongosh

import (
	_ "embed"
)

//go:embed assets/mongoWellnessChecker/mongoWellnessChecker.js
var MongoWellnessCheckerScriptCode string
