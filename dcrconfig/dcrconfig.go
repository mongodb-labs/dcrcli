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

package dcrconfig

import (
	"encoding/json"
	"fmt"
	"os"
)

// Config holds all connection and collection settings that dcrcli needs.
// Generate a sample with: ./dcrcli -generate-config dcrcli.config.json
type Config struct {
	// ClusterName is the display name used for the output directory.
	ClusterName string `json:"cluster_name"`

	// SeedHost is the hostname or IP of a seed mongod or mongos node.
	SeedHost string `json:"seed_host"`

	// SeedPort is the port of the seed node. Defaults to "27017" if empty.
	SeedPort string `json:"seed_port"`

	// Username is the MongoDB admin username.
	// Leave empty for clusters running without authentication.
	// Password is never stored in the config file — dcrcli always prompts for it
	// at startup when a username is set.
	Username string `json:"username"`

	// URIOptions are extra MongoDB URI connection options in name=value&name2=value2 format.
	// Do NOT include replicaSet here — dcrcli discovers topology itself.
	URIOptions string `json:"uri_options"`

	// SSHUsername is the OS user for passwordless SSH to remote cluster nodes.
	// Leave empty if all cluster nodes are on the same machine as dcrcli.
	SSHUsername string `json:"ssh_username"`

	// CollectNodes controls which nodes to collect diagnostic data from.
	// Valid values: "one-secondary" (default), "all-secondaries", "all-nodes".
	// Leave empty to be prompted interactively when running in a terminal.
	CollectNodes string `json:"collect_nodes"`
}

// Load reads and parses a JSON config file at the given path.
// On any parse or validation error it returns a message that includes the
// field name so the user knows exactly what to fix in the file.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read config file %q: %w", path, err)
	}
	var c Config
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("invalid JSON in config file %q: %w", path, err)
	}
	return &c, nil
}

// GenerateSample writes a sample config file with placeholder values to path.
func GenerateSample(path string) error {
	sample := Config{
		ClusterName:  "my-cluster",
		SeedHost:     "localhost",
		SeedPort:     "27017",
		Username:     "",
		URIOptions:   "",
		SSHUsername:  "",
		CollectNodes: "one-secondary",
	}
	data, err := json.MarshalIndent(sample, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0600)
}
