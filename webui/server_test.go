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
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"dcrcli/dcrlogger"
	"dcrcli/mongosh"
	"dcrcli/topologyfinder"
)

func testApp(t *testing.T) *App {
	t.Helper()
	dir := t.TempDir()
	log := &dcrlogger.DCRLogger{OutputPrefix: dir + "/", FileName: "dcrwebui-test"}
	if err := log.Create(); err != nil {
		t.Fatal(err)
	}
	app := newApp(log)
	// Production starts with the Host guard on. Most tests use httptest's
	// default Host, so they opt out here; guard tests turn it back on.
	app.loopbackOnly = false
	return app
}

func TestAppStopCancelsAndWaitsForCollection(t *testing.T) {
	app := testApp(t)
	started := make(chan struct{})
	canceled := make(chan struct{})
	release := make(chan struct{})
	stopped := make(chan struct{})
	app.runCollection = func(ctx context.Context, _ collectJob) error {
		close(started)
		<-ctx.Done()
		close(canceled)
		<-release
		return ctx.Err()
	}
	app.running = true
	app.startCollection(collectJob{})
	<-started

	go func() {
		app.stop()
		close(stopped)
	}()
	<-canceled
	select {
	case <-stopped:
		t.Fatal("stop returned before collection goroutine completed")
	default:
	}
	close(release)
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("stop did not wait for collection goroutine")
	}
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
	for _, want := range []string{
		"select only getMongoData and Commands",
		"leave the SSH username blank",
		`aria-labelledby="ssh-title"`,
		`aria-describedby="ssh-context ssh-prompt ssh-note ssh-error"`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("page missing %q", want)
		}
	}
}

func TestStaticUIIncludesReviewGuards(t *testing.T) {
	css, err := staticFS.ReadFile("static/app.css")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(css), "[hidden] { display: none !important; }") {
		t.Fatal("CSS does not force hidden controls to stay hidden")
	}
	js, err := staticFS.ReadFile("static/app.js")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"promptReturnFocus", `e.key !== "Tab"`, `mode.id === "one-secondary"`} {
		if !strings.Contains(string(js), want) {
			t.Fatalf("app.js missing %q", want)
		}
	}
}

func TestDiscoverRejectsUnsafeClusterName(t *testing.T) {
	app := testApp(t)
	for _, name := range []string{"../tmp", ".", ".."} {
		rec := postJSON(app, "/api/discover", `{"cluster_name":"`+name+`","seed_host":"localhost","seed_port":"27017"}`)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("name %q: status %d body %s", name, rec.Code, rec.Body.String())
		}
		var body struct {
			Problem problem `json:"problem"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if body.Problem.Field != "cluster_name" {
			t.Fatalf("name %q: problem: %+v", name, body.Problem)
		}
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
	hint := loopbackListenHint(&net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 4041})
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
	hint = loopbackListenHint(&net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 18080})
	if !strings.Contains(hint, "http://127.0.0.1:18080") {
		t.Fatalf("port not reflected: %s", hint)
	}
	// The hint must name the address actually bound, including IPv6 loopback.
	hint = loopbackListenHint(&net.TCPAddr{IP: net.IPv6loopback, Port: 4041})
	if !strings.Contains(hint, "http://[::1]:4041") {
		t.Fatalf("bound address not reflected: %s", hint)
	}
}

func TestWebRunRejectsUnexpectedArgs(t *testing.T) {
	err := Run([]string{"extra"})
	if err == nil || !strings.Contains(err.Error(), "unexpected argument") {
		t.Fatalf("err = %v", err)
	}
}

func TestWebRunRejectsNonLoopbackBeforeListen(t *testing.T) {
	err := Run([]string{"-addr", "192.0.2.1:4041"})
	if err == nil || !strings.Contains(err.Error(), "loopback") {
		t.Fatalf("err = %v", err)
	}
}

func TestAskpassURLUsesActualListenerIP(t *testing.T) {
	for _, tc := range []struct {
		addr *net.TCPAddr
		want string
	}{
		{&net.TCPAddr{IP: net.ParseIP("127.0.0.2"), Port: 4041}, "http://127.0.0.2:4041/api/askpass"},
		{&net.TCPAddr{IP: net.ParseIP("::1"), Port: 4041}, "http://[::1]:4041/api/askpass"},
	} {
		if got := askpassURL(tc.addr); got != tc.want {
			t.Errorf("askpassURL(%v) = %q, want %q", tc.addr, got, tc.want)
		}
	}
}

func TestDiscoveryReservationConflicts(t *testing.T) {
	app := testApp(t)
	app.discovering = true
	app.discovered = true
	app.nodes = []topologyfinder.ClusterNode{{Hostname: "localhost", Port: 27017, ReplicaState: "PRIMARY"}}

	discover := postJSON(app, "/api/discover", `{"cluster_name":"safe","seed_host":"localhost","seed_port":"27017"}`)
	if discover.Code != http.StatusConflict {
		t.Fatalf("discover status %d: %s", discover.Code, discover.Body.String())
	}
	collect := postJSON(app, "/api/collect", `{"collect_nodes":"all-nodes","collect_data":"commands"}`)
	if collect.Code != http.StatusConflict {
		t.Fatalf("collect status %d: %s", collect.Code, collect.Body.String())
	}
	reset := postJSON(app, "/api/reset", `{}`)
	if reset.Code != http.StatusConflict {
		t.Fatalf("reset status %d: %s", reset.Code, reset.Body.String())
	}
}

func TestDiscoveryFailureReleasesReservationWithoutChangingGlobalMax(t *testing.T) {
	oldMax := mongosh.MaxCollections()
	if err := mongosh.SetMaxCollections(41); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = mongosh.SetMaxCollections(oldMax) })

	app := testApp(t)
	rec := postJSON(app, "/api/discover", `{"cluster_name":"safe","seed_host":"localhost","seed_port":"bad","max_collections":99}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	if app.discovering {
		t.Fatal("discovery reservation was not released")
	}
	if got := mongosh.MaxCollections(); got != 41 {
		t.Fatalf("discovery changed global max collections to %d", got)
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

func TestUnresolvableMembersReportsPartialFailure(t *testing.T) {
	old := lookupHost
	defer func() { lookupHost = old }()
	lookupHost = func(_ context.Context, host string) ([]string, error) {
		if host == "internal-b" {
			return nil, errors.New("no such host")
		}
		return []string{"10.0.0.1"}, nil
	}
	nodes := []topologyfinder.ClusterNode{
		{Hostname: "a.example", Port: 27017},
		{Hostname: "internal-b", Port: 27017},
		{Hostname: "internal-b", Port: 27018},
	}
	got := unresolvableMembers(context.Background(), nodes)
	if len(got) != 2 || got[0] != "internal-b:27017" || got[1] != "internal-b:27018" {
		t.Fatalf("unresolvable = %v", got)
	}
	lookupHost = func(context.Context, string) ([]string, error) { return []string{"10.0.0.1"}, nil }
	if got := unresolvableMembers(context.Background(), nodes); got != nil {
		t.Fatalf("expected none, got %v", got)
	}
}

func TestGuardSecurityHeadersAndOrigin(t *testing.T) {
	app := testApp(t)
	app.loopbackOnly = true
	h := app.handler()

	req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:4041/api/state", nil)
	req.Host = "127.0.0.1:4041"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("state = %d", rec.Code)
	}
	for k, want := range map[string]string{
		"Content-Security-Policy": "frame-ancestors 'none'",
		"X-Frame-Options":         "DENY",
		"X-Content-Type-Options":  "nosniff",
		"Cache-Control":           "no-store",
	} {
		if got := rec.Header().Get(k); got != want {
			t.Fatalf("%s = %q, want %q", k, got, want)
		}
	}

	// A foreign Origin is refused even when Host looks local.
	req = httptest.NewRequest(http.MethodPost, "http://127.0.0.1:4041/api/reset", strings.NewReader("{}"))
	req.Host = "127.0.0.1:4041"
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "https://evil.example")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("cross-origin = %d, want 403", rec.Code)
	}

	// The same origin, and requests without an Origin (askpass helper), pass.
	req = httptest.NewRequest(http.MethodGet, "http://127.0.0.1:4041/api/state", nil)
	req.Host = "127.0.0.1:4041"
	req.Header.Set("Origin", "http://127.0.0.1:4041")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("same-origin = %d", rec.Code)
	}

	// A non-loopback Host is refused by default (fail closed).
	req = httptest.NewRequest(http.MethodGet, "http://attacker.example/api/state", nil)
	req.Host = "attacker.example"
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("rebinding host = %d, want 403", rec.Code)
	}
}

func TestEventStreamEndsOnShutdown(t *testing.T) {
	app := testApp(t)
	srv := httptest.NewServer(app.handler())
	defer srv.Close()
	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/events", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	done := make(chan struct{})
	go func() {
		buf := make([]byte, 4096)
		for {
			if _, err := resp.Body.Read(buf); err != nil {
				close(done)
				return
			}
		}
	}()
	app.cancelWork()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("event stream stayed open after shutdown")
	}
}

func TestNoWorkStartsAfterShutdown(t *testing.T) {
	app := testApp(t)
	app.stop()
	if app.startCollection(collectJob{}) {
		t.Fatal("collection started after shutdown")
	}
}

func TestNewAppFailsClosed(t *testing.T) {
	dir := t.TempDir()
	log := &dcrlogger.DCRLogger{OutputPrefix: dir + "/", FileName: "dcrwebui-test"}
	if err := log.Create(); err != nil {
		t.Fatal(err)
	}
	if !newApp(log).loopbackOnly {
		t.Fatal("a new App must enforce the loopback Host guard by default")
	}
}

// TestTopologyScriptClassifiesStandalone runs the real topology.js under Node
// (skipped when Node is not installed) instead of grepping its source.
func TestTopologyScriptClassifiesStandalone(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not installed")
	}
	src, err := staticFS.ReadFile("static/topology.js")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "topology.js"), src, 0o600); err != nil {
		t.Fatal(err)
	}
	check := `
const vm = require("vm"), fs = require("fs"), assert = require("assert");
const sandbox = { window: {}, document: {} };
vm.createContext(sandbox);
vm.runInContext(fs.readFileSync(process.argv[1] + "/topology.js", "utf8"), sandbox);
const T = sandbox.window.Topology;
assert.strictEqual(T.roleOf({ role: "STANDALONE" }), "STANDALONE");
assert.strictEqual(T.roleOf({ role: "bogus" }), "UNKNOWN");
const one = [{ hostname: "h", port: 27017, role: "STANDALONE" }];
assert.strictEqual(T.summarize(one).type, "Standalone");
const rs = [
  { hostname: "a", port: 1, role: "PRIMARY" },
  { hostname: "b", port: 2, role: "SECONDARY" },
];
const sum = T.summarize(rs);
assert.strictEqual(sum.type, "Replica set");
assert.strictEqual(sum.primaries, 1);
assert.strictEqual(sum.secondaries, 1);
`
	out, err := exec.Command(node, "-e", check, dir).CombinedOutput()
	if err != nil {
		t.Fatalf("topology.js check failed: %v\n%s", err, out)
	}
}
