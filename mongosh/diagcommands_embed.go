// Copyright 2023 MongoDB Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package mongosh

import (
	_ "embed"
)

//go:embed assets/diagcommands/rsConf.js
var RsConfCommand string

//go:embed assets/diagcommands/rsStatus.js
var RsStatusCommand string

//go:embed assets/diagcommands/printReplicationInfo.js
var PrintReplicationInfoCommand string

//go:embed assets/diagcommands/printSecondaryReplicationInfo.js
var PrintSecondaryReplicationInfoCommand string

//go:embed assets/diagcommands/shStatus.js
var ShStatusCommand string

//go:embed assets/diagcommands/dbPath.js
var GetDbPathCommand string

//go:embed assets/diagcommands/listCatalogTimeSeries.js
var ListCatalogTimeSeriesCommand string

//go:embed assets/diagcommands/shardedIndexConsistency.js
var ShardedIndexConsistencyCommand string
