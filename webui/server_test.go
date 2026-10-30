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
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"dcrcli/dcrlogger"
	"dcrcli/topologyfinder"
)

func testApp(t *testing.T) *App {
	t.Helper()
	dir := t.TempDir()
	log := &dcrlogger.DCRLogger{OutputPrefix: dir + "/", FileName: "dcrwebui-test"}
	if err := log.Create(); err != nil {
		t.Fatal(err)
	}
	return newApp(log)
}

func postJSON(app *App, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	app.handler().ServeHTTP(rec, req)
	return rec
}

func TestPageShowsWebUI(t *testing.T) {
	app := testApp(t)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	app.handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	body := rec.Body.String()
	if strings.Contains(body, "proof of concept") || !strings.Contains(body, "Discover topology") || !strings.Contains(body, "Commands") {
		t.Fatalf("page missing expected content")
	}
}

func TestDiscoverRejectsUnsafeClusterName(t *testing.T) {
	app := testApp(t)
	rec := postJSON(app, "/api/discover", `{"cluster_name":"../tmp","seed_host":"localhost","seed_port":"27017"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Problem problem `json:"problem"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Problem.Field != "cluster_name" {
		t.Fatalf("problem: %+v", body.Problem)
	}
}

func TestPostWithoutJSONContentTypeIsRejected(t *testing.T) {
	app := testApp(t)
	req := httptest.NewRequest(http.MethodPost, "/api/reset", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "text/plain")
	rec := httptest.NewRecorder()
	app.handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestGuardRejectsForeignHost(t *testing.T) {
	app := testApp(t)
	app.loopbackOnly = true
	req := httptest.NewRequest(http.MethodGet, "/api/state", nil)
	req.Host = "evil.example:4041"
	rec := httptest.NewRecorder()
	app.handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status %d", rec.Code)
	}
	req.Host = "127.0.0.1:4041"
	rec = httptest.NewRecorder()
	app.handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("loopback status %d", rec.Code)
	}
}

func TestDefaultWebAddrIsLoopback(t *testing.T) {
	if defaultWebAddr != "127.0.0.1:4041" {
		t.Fatalf("defaultWebAddr = %q", defaultWebAddr)
	}
	if !isLoopbackAddr(defaultWebAddr) {
		t.Fatalf("%q should be treated as loopback", defaultWebAddr)
	}
}

func TestIsLoopbackAddr(t *testing.T) {
	cases := []struct {
		addr string
		want bool
	}{
		{"127.0.0.1:4041", true},
		{"localhost:4041", true},
		{"LOCALHOST:9999", true},
		{"[::1]:4041", true},
		{"0.0.0.0:4041", false},
		{"[::]:4041", false},
		{"192.168.1.10:4041", false},
		{"jump.example:4041", false},
		{"not-an-addr", false},
	}
	for _, tc := range cases {
		if got := isLoopbackAddr(tc.addr); got != tc.want {
			t.Errorf("isLoopbackAddr(%q) = %v, want %v", tc.addr, got, tc.want)
		}
	}
}

func TestGuardAllowsForwardedLocalPorts(t *testing.T) {
	// ssh -L 18080:127.0.0.1:4041 makes the browser use Host: 127.0.0.1:18080
	// (or localhost:18080). The Host guard must still accept those.
	app := testApp(t)
	app.loopbackOnly = true
	for _, host := range []string{"127.0.0.1:18080", "localhost:18080", "[::1]:18080"} {
		req := httptest.NewRequest(http.MethodGet, "/api/state", nil)
		req.Host = host
		rec := httptest.NewRecorder()
		app.handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("host %q: status %d", host, rec.Code)
		}
	}
}

func TestListenDefaultLoopback(t *testing.T) {
	ln, err := net.Listen("tcp", defaultWebAddr)
	if err != nil {
		// Port may already be in use; fall back to an ephemeral loopback port
		// but still assert the default address string is loopback-only.
		ln, err = net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
	}
	defer ln.Close()
	tcpAddr := ln.Addr().(*net.TCPAddr)
	if !tcpAddr.IP.IsLoopback() {
		t.Fatalf("listen IP %v is not loopback", tcpAddr.IP)
	}
}

func TestLoopbackListenHint(t *testing.T) {
	hint := loopbackListenHint(4041)
	for _, want := range []string{
		"http://127.0.0.1:4041",
		"browser on this machine",
		"loopback only",
		"-via",
	} {
		if !strings.Contains(hint, want) {
			t.Fatalf("hint missing %q:\n%s", want, hint)
		}
	}
	// Same-machine startup must not dump jump-host instructions.
	for _, ban := range []string{"JUMP_HOST", "ssh -L", "LocalForward"} {
		if strings.Contains(hint, ban) {
			t.Fatalf("hint should not emphasize remote setup with %q:\n%s", ban, hint)
		}
	}
	hint = loopbackListenHint(18080)
	if !strings.Contains(hint, "http://127.0.0.1:18080") {
		t.Fatalf("port not reflected: %s", hint)
	}
}

func TestWebRunRejectsUnexpectedArgs(t *testing.T) {
	err := Run([]string{"extra"})
	if err == nil || !strings.Contains(err.Error(), "unexpected argument") {
		t.Fatalf("err = %v", err)
	}
}

func TestCollectRequiresDiscovery(t *testing.T) {
	app := testApp(t)
	rec := postJSON(app, "/api/collect", `{"collect_nodes":"one-secondary","collect_data":"commands"}`)
	if rec.Code != http.StatusConflict {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
}

func TestPreviewModesPicksOneSecondary(t *testing.T) {
	nodes := []topologyfinder.ClusterNode{
		{Hostname: "b", Port: 27017, ReplicaState: "SECONDARY"},
		{Hostname: "a", Port: 27017, ReplicaState: "SECONDARY"},
		{Hostname: "p", Port: 27017, ReplicaState: "PRIMARY"},
	}
	modes := previewModes(nodes)
	if len(modes) != 3 {
		t.Fatalf("modes: %d", len(modes))
	}
	if modes[0].ID != "one-secondary" || len(modes[0].Targets) != 1 || modes[0].Targets[0].Hostname != "a" {
		t.Fatalf("one-secondary: %+v", modes[0])
	}
	if len(modes[1].Targets) != 2 || len(modes[2].Targets) != 3 {
		t.Fatalf("modes: %+v", modes)
	}
}

func TestPreviewModesStandaloneNeedsConfirm(t *testing.T) {
	nodes := []topologyfinder.ClusterNode{{Hostname: "solo", Port: 27017, ReplicaState: "PRIMARY"}}
	modes := previewModes(nodes)
	if !modes[0].Standalone || modes[0].Error == "" {
		t.Fatalf("expected standalone warning, got %+v", modes[0])
	}
	if len(modes[2].Targets) != 1 || modes[2].Error != "" {
		t.Fatalf("all-nodes should include the standalone: %+v", modes[2])
	}
	if targets, err := resolveTargets(nodes, 0, false); err == nil || len(targets) != 0 {
		t.Fatalf("secondary scope without confirm should fail, targets=%v err=%v", targets, err)
	}
	if targets, err := resolveTargets(nodes, 0, true); err != nil || len(targets) != 1 {
		t.Fatalf("confirmed standalone: targets=%v err=%v", targets, err)
	}
}

func TestUnresolvedMembersListsHosts(t *testing.T) {
	before := []topologyfinder.ClusterNode{
		{Hostname: "psmdb2", Port: 27017},
		{Hostname: "psmdb1", Port: 27017},
	}
	hosts := unresolvedMembers(before, nil)
	if strings.Join(hosts, ",") != "psmdb1:27017,psmdb2:27017" {
		t.Fatalf("hosts: %v", hosts)
	}
	if hosts := unresolvedMembers(before, before[:1]); hosts != nil {
		t.Fatalf("partial result should stay: %v", hosts)
	}
	p := unresolvedProblem(hosts)
	if p.Code != "unresolved_members" || !strings.Contains(unresolvedProblem([]string{"psmdb1:27017"}).Details, "psmdb1") {
		t.Fatalf("problem: %+v", p)
	}
}

func TestClassifyMissingPassword(t *testing.T) {
	err := errors.New("Failed MongoDB shell: exit status 1\nShell output:\nWarning: Could not access file at x\nMongoServerError: Authentication failed.")
	p := classifyDiscoverError(err, discoverRequest{Username: "diag", SeedHost: "localhost"}, false)
	if p.Code != "password_missing" || p.Field != "password" {
		t.Fatalf("problem: %+v", p)
	}
	if strings.Contains(p.Details, "Could not access file") {
		t.Fatalf("mongosh noise kept: %q", p.Details)
	}
	p = classifyDiscoverError(err, discoverRequest{Username: "diag"}, true)
	if p.Code != "auth_failed" {
		t.Fatalf("problem: %+v", p)
	}
	p = classifyDiscoverError(errors.New("connect ECONNREFUSED 127.0.0.1:27018"), discoverRequest{SeedPort: "27018"}, false)
	if p.Code != "connection_refused" || p.Field != "seed_port" {
		t.Fatalf("problem: %+v", p)
	}
}

func TestStateOmitsPassword(t *testing.T) {
	app := testApp(t)
	app.mu.Lock()
	app.cred.Password = "secret-value"
	app.cred.Username = "diag"
	app.mu.Unlock()

	req := httptest.NewRequest(http.MethodGet, "/api/state", nil)
	rec := httptest.NewRecorder()
	app.handler().ServeHTTP(rec, req)
	if strings.Contains(rec.Body.String(), "secret-value") {
		t.Fatalf("password leaked in state: %s", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"has_password":true`) {
		t.Fatalf("state: %s", rec.Body.String())
	}
}

func TestAskpassRoundTrip(t *testing.T) {
	app := testApp(t)
	app.askToken = "tok"

	forbidden := postJSON(app, "/api/askpass", `{"prompt":"Password:"}`)
	if forbidden.Code != http.StatusForbidden {
		t.Fatalf("askpass without token: %d", forbidden.Code)
	}

	done := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		req := httptest.NewRequest(http.MethodPost, "/api/askpass", strings.NewReader(`{"prompt":"(renato@psmdb1) Password:"}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set(askpassHeader, "tok")
		rec := httptest.NewRecorder()
		app.handler().ServeHTTP(rec, req)
		done <- rec
	}()

	var id string
	for i := 0; i < 100 && id == ""; i++ {
		app.mu.Lock()
		for k, p := range app.prompts {
			id = k
			if p.info.Kind != "secret" {
				t.Errorf("kind %q", p.info.Kind)
			}
		}
		app.mu.Unlock()
		time.Sleep(10 * time.Millisecond)
	}
	if id == "" {
		t.Fatal("prompt never registered")
	}
	if rec := postJSON(app, "/api/ssh-answer", `{"id":"`+id+`","answer":"hunter2"}`); rec.Code != http.StatusOK {
		t.Fatalf("answer: %d %s", rec.Code, rec.Body.String())
	}
	rec := <-done
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "hunter2") {
		t.Fatalf("askpass reply: %d %s", rec.Code, rec.Body.String())
	}
	for _, ev := range app.events {
		if strings.Contains(ev.Message, "hunter2") {
			t.Fatal("password leaked into events")
		}
	}
}

func TestPromptKind(t *testing.T) {
	if promptKind("Are you sure you want to continue connecting (yes/no/[fingerprint])?", "") != "confirm" {
		t.Fatal("host key prompt should be confirm")
	}
	if promptKind("(renato@psmdb1) Password:", "") != "secret" {
		t.Fatal("password prompt should be secret")
	}
}
