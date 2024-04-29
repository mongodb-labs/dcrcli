package mongosh

import (
	_ "embed"
)

//go:embed assets/mongologarchiver/systemLog.js
var GetSystemLogDBCommand string
