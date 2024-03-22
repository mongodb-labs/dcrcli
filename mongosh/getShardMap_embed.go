package mongosh

import (
	_ "embed"
)

//go:embed assets/topologyFinder/getShardMap.js
var GetShardMapScriptCode string
