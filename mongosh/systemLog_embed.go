package mongosh

import (
	_ "embed"
)

//go:embed assets/logarchiver/systemLog.js
var GetSystemLogDBCommand string
