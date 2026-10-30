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

package webui

import (
	"fmt"
	"net/http"
	"strings"
)

// problem is an error the page can explain: what happened, which field to fix,
// what to try, and the raw output for whoever needs it.
type problem struct {
	Code    string   `json:"code"`
	Title   string   `json:"title"`
	Message string   `json:"message"`
	Hints   []string `json:"hints,omitempty"`
	Field   string   `json:"field,omitempty"`
	Hosts   []string `json:"hosts,omitempty"`
	Details string   `json:"details,omitempty"`
}

func writeProblem(w http.ResponseWriter, code int, p problem) {
	writeJSON(w, code, map[string]any{"error": p.Message, "problem": p})
}

func fieldProblem(field, title, message string, hints ...string) problem {
	return problem{Code: "invalid_" + field, Title: title, Message: message, Field: field, Hints: hints}
}

// cleanShellOutput drops mongosh noise that is not about the failure.
func cleanShellOutput(s string) string {
	lines := strings.Split(s, "\n")
	kept := lines[:0]
	for _, line := range lines {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "Warning: Could not access file") {
			continue
		}
		kept = append(kept, line)
	}
	return strings.TrimSpace(strings.Join(kept, "\n"))
}

func classifyDiscoverError(err error, req discoverRequest, passwordSent bool) problem {
	raw := cleanShellOutput(err.Error())
	lower := strings.ToLower(raw)
	seed := fmt.Sprintf("%s:%s", orDefault(req.SeedHost, "localhost"), orDefault(req.SeedPort, "27017"))
	p := problem{Details: raw}

	switch {
	case strings.Contains(lower, "authentication failed") && req.Username != "" && !passwordSent:
		p.Code = "password_missing"
		p.Title = "MongoDB password is missing"
		p.Message = fmt.Sprintf("The cluster rejected %q because no password was sent.", req.Username)
		p.Field = "password"
		p.Hints = []string{
			"Enter the password for this database user.",
			"If the cluster has no authentication, clear the username instead.",
		}
	case strings.Contains(lower, "authentication failed"):
		p.Code = "auth_failed"
		p.Title = "MongoDB rejected these credentials"
		p.Message = fmt.Sprintf("Authentication failed for %q on %s.", req.Username, seed)
		p.Field = "password"
		p.Hints = []string{
			"Check the username and password (this is the database user, not your SSH login).",
			"If the user lives in another database, add authSource=<db> to URI options.",
			"The user needs the backup, readAnyDatabase, and clusterMonitor roles.",
		}
	case strings.Contains(lower, "requires authentication") || strings.Contains(lower, "unauthorized") || strings.Contains(lower, "not authorized"):
		p.Code = "auth_required"
		p.Title = "This cluster requires authentication"
		p.Message = fmt.Sprintf("%s refused to run discovery commands without credentials.", seed)
		p.Field = "username"
		p.Hints = []string{"Enter a database user with backup, readAnyDatabase, and clusterMonitor roles."}
	case strings.Contains(lower, "econnrefused") || strings.Contains(lower, "connection refused"):
		p.Code = "connection_refused"
		p.Title = "Nothing is listening on " + seed
		p.Message = "The seed host answered, but no mongod or mongos accepted the connection on that port."
		p.Field = "seed_port"
		p.Hints = []string{
			"Check the port. mongos and shard members often use 27017 to 27019.",
			"Make sure the mongod or mongos process is running.",
		}
	case strings.Contains(lower, "enotfound") || strings.Contains(lower, "getaddrinfo") || strings.Contains(lower, "no such host"):
		p.Code = "host_not_found"
		p.Title = "Seed host not found"
		p.Message = fmt.Sprintf("This machine cannot resolve %q.", orDefault(req.SeedHost, "localhost"))
		p.Field = "seed_host"
		p.Hints = []string{"Check the spelling, or add the host to DNS or /etc/hosts."}
	case strings.Contains(lower, "timed out") || strings.Contains(lower, "etimedout") || strings.Contains(lower, "server selection"):
		p.Code = "timeout"
		p.Title = "Connection timed out"
		p.Message = fmt.Sprintf("No answer from %s in time.", seed)
		p.Field = "seed_host"
		p.Hints = []string{
			"A firewall may be dropping traffic to the MongoDB port.",
			"If the cluster uses TLS, add tls=true to URI options.",
		}
	case strings.Contains(lower, "tls") || strings.Contains(lower, "ssl") || strings.Contains(lower, "certificate"):
		p.Code = "tls"
		p.Title = "TLS handshake failed"
		p.Message = "The connection failed while negotiating TLS."
		p.Field = "uri_options"
		p.Hints = []string{
			"Add tls=true to URI options if the cluster requires TLS.",
			"Self-signed certificates may need tlsCAFile=<path>.",
		}
	case strings.Contains(lower, "could not find the mongosh") || strings.Contains(lower, "executable file not found"):
		p.Code = "no_shell"
		p.Title = "mongosh is not installed"
		p.Message = "MongoDB Data Collector runs its discovery and collection through mongosh (or the legacy mongo shell)."
		p.Hints = []string{"Install the current mongosh and make sure it is on PATH, then restart dcrcli web."}
	default:
		p.Code = "discovery_failed"
		p.Title = "Topology discovery failed"
		p.Message = fmt.Sprintf("MongoDB Data Collector could not read the topology from %s.", seed)
		p.Hints = []string{"The technical details below have the shell output."}
	}
	return p
}

// configProblem maps the shared config validation errors to the field they name.
func configProblem(err error) problem {
	msg := err.Error()
	for _, field := range []string{"seed_port", "seed_host", "uri_options", "cluster_name", "username"} {
		if strings.Contains(msg, `"`+field+`"`) {
			p := fieldProblem(field, "Check "+strings.ReplaceAll(field, "_", " "), msg)
			if field == "uri_options" {
				p.Hints = []string{"Use name=value&name2=value2, for example tls=true&authSource=admin.", "Leave out replicaSet. dcrcli discovers the topology itself."}
			}
			if field == "seed_port" {
				p.Hints = []string{"Use a number from 1 to 65535. Leave it blank for 27017."}
			}
			return p
		}
	}
	return problem{Code: "invalid_config", Title: "Check the connection details", Message: msg}
}

func unresolvedProblem(hosts []string) problem {
	example := make([]string, 0, len(hosts))
	for _, h := range hosts {
		name := h
		if i := strings.LastIndex(h, ":"); i > 0 {
			name = h[:i]
		}
		example = append(example, "<ip-address>  "+name)
	}
	return problem{
		Code:    "unresolved_members",
		Title:   "Found the cluster, but can't reach its members by name",
		Message: fmt.Sprintf("The seed reported %d member(s). This machine cannot resolve any of their hostnames, so it cannot connect to them.", len(hosts)),
		Hosts:   hosts,
		Hints: []string{
			"Use the same hostnames rs.status() reports.",
			"Add each name to DNS, or to /etc/hosts on this machine.",
		},
		Details: "# /etc/hosts\n" + strings.Join(example, "\n"),
	}
}

func orDefault(s, def string) string {
	if strings.TrimSpace(s) == "" {
		return def
	}
	return s
}
