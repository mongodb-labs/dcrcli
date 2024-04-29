package mongosh

import (
	_ "embed"
)

//go:embed assets/ftdcarchiver/diagnosticDataCollectionDirectoryPath.js
var GetCommandDiagnosticDataCollectionDirectoryPath string
