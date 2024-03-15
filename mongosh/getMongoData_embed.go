package mongosh

import (
	_ "embed"
)

//go:embed assets/getMongoData/getMongoData.js
var GetMongDataScriptCode string
