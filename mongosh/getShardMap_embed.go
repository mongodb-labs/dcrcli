package mongosh

import (
	_ "embed"
)

//go:embed assets/topologyFinder/getShardMap.js
var GetShardMapScriptCode string

//go:embed assets/topologyFinder/helloCommand.js
var HelloDBCommand string
