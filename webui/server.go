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

// Package webui serves a local web UI for dcr.
// (Data Capture Request tool). The terminal UI remains the default.
// Start the UI with `dcr web` (dcrcli is a supported binary alias).
package webui

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"dcrcli/collectdata"
	"dcrcli/collectnodes"
	"dcrcli/dcrconfig"
	"dcrcli/dcrlogger"
	"dcrcli/mongocredentials"
	"dcrcli/mongosh"
	"dcrcli/termui"
	"dcrcli/topologyfinder"
)

//go:embed static
var staticFS embed.FS

// defaultWebAddr is the listen address used when `dcrcli web` is started
// without -addr. Loopback keeps the unauthenticated PoC UI off the network.
const defaultWebAddr = "127.0.0.1:4041"

const maxEvents = 500

// Event is one server-sent update for the collection view.
type Event struct {
	Type      string      `json:"type"`
	Time      string      `json:"time"`
	Level     string      `json:"level,omitempty"`
	Message   string      `json:"message,omitempty"`
	Status    string      `json:"status,omitempty"`
	Percent   int         `json:"percent,omitempty"`
	OutputDir string      `json:"output_dir,omitempty"`
	Nodes     []nodeJSON  `json:"nodes,omitempty"`
	Prompt    *promptJSON `json:"prompt,omitempty"`
}

type nodeJSON struct {
	Hostname string     `json:"hostname"`
	Port     int        `json:"port"`
	Role     string     `json:"role"`
	Shard    string     `json:"shard_role,omitempty"`
	State    string     `json:"state,omitempty"`
	Tasks    []taskJSON `json:"tasks,omitempty"`
}

type taskJSON struct {
	Name  string `json:"name"`
	State string `json:"state"`
	Note  string `json:"note,omitempty"`
}

type connectionView struct {
	ClusterName    string `json:"cluster_name"`
	SeedHost       string `json:"seed_host"`
	SeedPort       string `json:"seed_port"`
	Username       string `json:"username"`
	HasPassword    bool   `json:"has_password"`
	URIOptions     string `json:"uri_options"`
	MaxCollections int    `json:"max_collections"`
}

type discoverRequest struct {
	ClusterName    string `json:"cluster_name"`
	SeedHost       string `json:"seed_host"`
	SeedPort       string `json:"seed_port"`
	Username       string `json:"username"`
	Password       string `json:"password"`
	URIOptions     string `json:"uri_options"`
	MaxCollections int    `json:"max_collections"`
}

type collectRequest struct {
	CollectNodes    string `json:"collect_nodes"`
	CollectData     string `json:"collect_data"`
	SSHUsername     string `json:"ssh_username"`
	AllowStandalone bool   `json:"allow_standalone_primary"`
}

// App is the in-memory session for one local dcrcli web process.
type App struct {
	mu             sync.Mutex
	log            *dcrlogger.DCRLogger
	cred           mongocredentials.Mongocredentials
	nodes          []topologyfinder.ClusterNode
	maxCollections int
	discovered     bool
	running        bool
	finished       bool
	runErr         string
	outputDir      string
	events         []Event
	subs           map[chan Event]struct{}
	askToken       string
	prompts        map[string]*pendingPrompt
	sshContext     string
	loopbackOnly   bool
}

func newApp(log *dcrlogger.DCRLogger) *App {
	return &App{
		log:            log,
		cred:           mongocredentials.Mongocredentials{Dcrlog: log},
		subs:           map[chan Event]struct{}{},
		prompts:        map[string]*pendingPrompt{},
		maxCollections: mongosh.DefaultMaxCollections,
	}
}

// Run starts the local web UI and blocks until the process is interrupted.
// With -via, it only opens an SSH local port forward to a remote loopback
// listener (the remote web UI must already be running).
func Run(args []string) error {
	bin := productName()
	fs := flag.NewFlagSet("web", flag.ContinueOnError)
	addr := fs.String("addr", defaultWebAddr, "listen address (default loopback; keep loopback for SSH -L remote access)")
	via := fs.String("via", "", "SSH destination for a local port-forward tunnel only (user@host or Host alias); does not start the remote web UI")
	localPort := fs.Int("local-port", 0, "local port for -via forwarding (default: port from -addr)")
	fs.SetOutput(os.Stderr)
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage: %s web [-addr %s] [-via user@host] [-local-port N]\n\n", bin, defaultWebAddr)
		fmt.Fprintf(fs.Output(), "Start the MongoDB Data Collector web UI.\n")
		fmt.Fprintf(fs.Output(), "Terminal UI remains the default: run %s with no subcommand.\n\n", bin)
		fmt.Fprintf(fs.Output(), "Usual case: run %s web on a host that can reach MongoDB (direct network\n", bin)
		fmt.Fprintf(fs.Output(), "or VPN), then open the printed URL in a browser on that same machine.\n")
		fmt.Fprintf(fs.Output(), "Only if the browser is on a different machine than the collector, use\n")
		fmt.Fprintf(fs.Output(), "-via / SSH -L (do not bind 0.0.0.0; this UI has no login). See README.\n\n")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("unexpected argument %q", fs.Arg(0))
	}
	if *via != "" {
		spec, err := newSSHForwardSpec(*via, *addr, *localPort)
		if err != nil {
			return err
		}
		return runSSHTunnel(spec)
	}

	dcrlog := &dcrlogger.DCRLogger{OutputPrefix: "./", FileName: "dcrwebui"}
	if err := dcrlog.Create(); err != nil {
		return fmt.Errorf("create web UI log: %w", err)
	}
	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		return err
	}
	tcpAddr := ln.Addr().(*net.TCPAddr)
	askHost := "127.0.0.1"
	if !tcpAddr.IP.IsUnspecified() && !tcpAddr.IP.IsLoopback() {
		askHost = tcpAddr.IP.String()
	}

	app := newApp(dcrlog)
	app.askToken = randomHex(32)
	app.loopbackOnly = isLoopbackAddr(*addr)
	askURL := "http://" + net.JoinHostPort(askHost, strconv.Itoa(tcpAddr.Port)) + "/api/askpass"
	if err := setupAskpassEnv(askURL, app.askToken); err != nil {
		ln.Close()
		return err
	}
	srv := &http.Server{
		Handler:           app.handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	fmt.Printf("MongoDB Data Collector web UI\n")
	fmt.Printf("  http://%s\n", ln.Addr().String())
	fmt.Printf("  log: %s\n", dcrlog.Path())
	fmt.Printf("  SSH password and host-key prompts appear in the browser.\n")
	if !app.loopbackOnly {
		fmt.Fprintf(os.Stderr, "warning: %s is not a loopback address. This UI has no login;\n", *addr)
		fmt.Fprintf(os.Stderr, "         do not expose it on a customer network. Prefer the default loopback\n")
		fmt.Fprintf(os.Stderr, "         bind and SSH local port forwarding instead.\n")
	} else {
		fmt.Print(loopbackListenHint(tcpAddr.Port))
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	errCh := make(chan error, 1)
	go func() {
		err := srv.Serve(ln)
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()
	select {
	case <-ctx.Done():
		shutCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		fmt.Println("\nstopping web UI")
		return srv.Shutdown(shutCtx)
	case err := <-errCh:
		return err
	}
}

func (a *App) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/discover", a.handleDiscover)
	mux.HandleFunc("/api/state", a.handleState)
	mux.HandleFunc("/api/preview", a.handlePreview)
	mux.HandleFunc("/api/collect", a.handleCollect)
	mux.HandleFunc("/api/events", a.handleEvents)
	mux.HandleFunc("/api/reset", a.handleReset)
	mux.HandleFunc("/api/askpass", a.handleAskpass)
	mux.HandleFunc("/api/ssh-answer", a.handleSSHAnswer)
	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		panic(err)
	}
	mux.Handle("/", http.FileServer(http.FS(sub)))
	return a.guard(mux)
}

// guard rejects requests whose Host is not this machine when the server is
// bound to loopback, so another website cannot reach the API by DNS rebinding.
func (a *App) guard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if a.loopbackOnly && !isLoopbackAddr(hostWithPort(r.Host)) {
			writeErr(w, http.StatusForbidden, "forbidden host")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func hostWithPort(host string) string {
	if _, _, err := net.SplitHostPort(host); err == nil {
		return host
	}
	return net.JoinHostPort(strings.Trim(host, "[]"), "80")
}

func (a *App) handleDiscover(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "POST required")
		return
	}
	var req discoverRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	req.ClusterName = strings.TrimSpace(req.ClusterName)
	req.Username = strings.TrimSpace(req.Username)
	req.SeedHost = strings.TrimSpace(req.SeedHost)
	req.SeedPort = strings.TrimSpace(req.SeedPort)
	req.URIOptions = strings.TrimSpace(req.URIOptions)
	if err := validateClusterName(req.ClusterName); err != nil {
		writeProblem(w, http.StatusBadRequest, fieldProblem("cluster_name", "Check the cluster name", err.Error(),
			"It becomes the folder name under ./outputs, so keep it to letters, numbers, dot, underscore, and hyphen."))
		return
	}

	a.mu.Lock()
	if a.running {
		a.mu.Unlock()
		writeErr(w, http.StatusConflict, "collection is already running")
		return
	}
	reusedPassword := ""
	if req.Password == "" && req.Username == a.cred.Username && a.cred.Password != "" {
		reusedPassword = a.cred.Password
	}
	a.mu.Unlock()

	maxC := req.MaxCollections
	if maxC == 0 {
		maxC = mongosh.DefaultMaxCollections
	}
	if err := mongosh.SetMaxCollections(maxC); err != nil {
		writeProblem(w, http.StatusBadRequest, fieldProblem("max_collections", "Check max collections", err.Error()))
		return
	}

	cfg := &dcrconfig.Config{
		ClusterName: req.ClusterName,
		SeedHost:    req.SeedHost,
		SeedPort:    req.SeedPort,
		URIOptions:  req.URIOptions,
	}
	cred := mongocredentials.Mongocredentials{Dcrlog: a.log}
	ui := termui.New(bytes.NewReader(nil), io.Discard)
	if err := cred.GetFromConfig(ui, cfg); err != nil {
		writeProblem(w, http.StatusBadRequest, configProblem(err))
		return
	}
	cred.Username = req.Username
	if req.Password != "" {
		cred.Password = strings.TrimSuffix(req.Password, "\n")
	} else {
		cred.Password = reusedPassword
	}
	if len(cred.Username) > 16*1024*1024 || len(cred.Password) > 16*1024*1024 {
		writeErr(w, http.StatusBadRequest, "username or password is too large")
		return
	}

	a.logInfo(fmt.Sprintf("Discovering topology from %s:%s", cred.Seedmongodhost, cred.Seedmongodport))
	finder := topologyfinder.TopologyFinder{Dcrlog: a.log}
	finder.MongoshCapture.S = &cred
	if err := finder.GetAllNodes(); err != nil {
		a.logErr("Topology discovery failed: " + err.Error())
		writeProblem(w, http.StatusBadRequest, classifyDiscoverError(err, req, cred.Password != ""))
		return
	}
	found := append([]topologyfinder.ClusterNode(nil), finder.Allnodes.Nodes...)
	if err := finder.KeepUniqueNodes(); err != nil {
		a.logWarn("Unable to filter unique hostnames: " + err.Error())
	}
	if hosts := unresolvedMembers(found, finder.Allnodes.Nodes); len(hosts) > 0 {
		a.logErr("Discovered members do not resolve on this machine: " + strings.Join(hosts, ", "))
		writeProblem(w, http.StatusBadRequest, unresolvedProblem(hosts))
		return
	}
	if err := finder.ResolveReplicaStates(); err != nil {
		a.logWarn("Could not fully resolve replica roles: " + err.Error())
	}

	a.mu.Lock()
	a.cred = cred
	a.nodes = append([]topologyfinder.ClusterNode(nil), finder.Allnodes.Nodes...)
	a.maxCollections = maxC
	a.discovered = true
	a.finished = false
	a.runErr = ""
	a.outputDir = ""
	a.events = nil
	nodes := toNodeJSON(a.nodes)
	modes := previewModes(a.nodes)
	view := connectionFrom(cred, maxC)
	a.mu.Unlock()

	a.logInfo(fmt.Sprintf("Discovered %d node(s)", len(nodes)))
	writeJSON(w, http.StatusOK, map[string]any{
		"nodes":           nodes,
		"modes":           modes,
		"connection":      view,
		"max_collections": maxC,
	})
}

func (a *App) handleState(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, http.StatusMethodNotAllowed, "GET required")
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	body := map[string]any{
		"discovered":      a.discovered,
		"running":         a.running,
		"finished":        a.finished,
		"error":           a.runErr,
		"output_dir":      a.outputDir,
		"max_collections": a.maxCollections,
		"connection":      connectionFrom(a.cred, a.maxCollections),
		"nodes":           toNodeJSON(a.nodes),
		"prompts":         a.pendingPrompts(),
	}
	if a.discovered {
		body["modes"] = previewModes(a.nodes)
	}
	writeJSON(w, http.StatusOK, body)
}

func (a *App) handlePreview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, http.StatusMethodNotAllowed, "GET required")
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.discovered {
		writeErr(w, http.StatusConflict, "discover a cluster before choosing a scope")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"modes": previewModes(a.nodes)})
}

func (a *App) handleCollect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "POST required")
		return
	}
	var req collectRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	mode, err := collectnodes.ParseMode(req.CollectNodes)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	sel, err := collectdata.Parse(req.CollectData)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	sshUser := strings.TrimSpace(req.SSHUsername)
	if strings.ContainsAny(sshUser, " \t\r\n") {
		writeErr(w, http.StatusBadRequest, "ssh_username must be a single token")
		return
	}

	a.mu.Lock()
	if !a.discovered {
		a.mu.Unlock()
		writeErr(w, http.StatusConflict, "discover a cluster before collecting")
		return
	}
	if a.running {
		a.mu.Unlock()
		writeErr(w, http.StatusConflict, "collection is already running")
		return
	}
	targets, err := resolveTargets(a.nodes, mode, req.AllowStandalone)
	if err != nil {
		a.mu.Unlock()
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if len(targets) == 0 {
		a.mu.Unlock()
		writeErr(w, http.StatusBadRequest, "no collection targets")
		return
	}
	job := collectJob{
		cred:     a.cred,
		allNodes: append([]topologyfinder.ClusterNode(nil), a.nodes...),
		targets:  targets,
		data:     sel,
		sshUser:  sshUser,
	}
	a.running = true
	a.finished = false
	a.runErr = ""
	a.outputDir = ""
	a.events = nil
	a.mu.Unlock()

	a.logInfo(fmt.Sprintf("Collect scope: %s, %d target node(s)", mode.String(), len(targets)))
	go func() {
		err := a.runCollect(job)
		a.mu.Lock()
		a.running = false
		a.finished = true
		if err != nil {
			a.runErr = err.Error()
		}
		a.mu.Unlock()
	}()

	writeJSON(w, http.StatusAccepted, map[string]any{
		"status":  "running",
		"targets": toNodeJSON(targets),
		"scope":   modeLabel(mode.String()),
		"data":    sel.Description(),
	})
}

func (a *App) handleReset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "POST required")
		return
	}
	if err := requireJSON(r); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.running {
		writeErr(w, http.StatusConflict, "collection is still running")
		return
	}
	a.cred = mongocredentials.Mongocredentials{Dcrlog: a.log}
	a.nodes = nil
	a.discovered = false
	a.finished = false
	a.runErr = ""
	a.outputDir = ""
	a.events = nil
	a.maxCollections = mongosh.DefaultMaxCollections
	writeJSON(w, http.StatusOK, map[string]string{"status": "reset"})
}

func (a *App) handleEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, http.StatusMethodNotAllowed, "GET required")
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeErr(w, http.StatusInternalServerError, "streaming is not supported")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	history, ch, cancel := a.subscribe()
	defer cancel()
	for _, ev := range history {
		if err := writeSSE(w, ev); err != nil {
			return
		}
	}
	flusher.Flush()

	ping := time.NewTicker(15 * time.Second)
	defer ping.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case ev := <-ch:
			if err := writeSSE(w, ev); err != nil {
				return
			}
			flusher.Flush()
		case <-ping.C:
			if _, err := fmt.Fprint(w, ": ping\n\n"); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

func (a *App) subscribe() ([]Event, chan Event, func()) {
	ch := make(chan Event, 256)
	a.mu.Lock()
	history := append([]Event(nil), a.events...)
	a.subs[ch] = struct{}{}
	a.mu.Unlock()
	cancel := func() {
		a.mu.Lock()
		delete(a.subs, ch)
		a.mu.Unlock()
	}
	return history, ch, cancel
}

func (a *App) emit(ev Event) {
	if ev.Time == "" {
		ev.Time = time.Now().Format(time.RFC3339)
	}
	a.mu.Lock()
	a.events = append(a.events, ev)
	if len(a.events) > maxEvents {
		a.events = append([]Event(nil), a.events[len(a.events)-maxEvents:]...)
	}
	subs := make([]chan Event, 0, len(a.subs))
	for ch := range a.subs {
		subs = append(subs, ch)
	}
	a.mu.Unlock()
	for _, ch := range subs {
		select {
		case ch <- ev:
		default:
		}
	}
}

func (a *App) logInfo(msg string) {
	a.log.Info(msg)
	a.emit(Event{Type: "log", Level: "info", Message: msg})
}

func (a *App) logWarn(msg string) {
	a.log.Warn(msg)
	a.emit(Event{Type: "log", Level: "warn", Message: msg})
}

func (a *App) logErr(msg string) {
	a.log.Error(msg)
	a.emit(Event{Type: "log", Level: "error", Message: msg})
}

func connectionFrom(cred mongocredentials.Mongocredentials, maxCollections int) connectionView {
	return connectionView{
		ClusterName:    cred.Clustername,
		SeedHost:       cred.Seedmongodhost,
		SeedPort:       cred.Seedmongodport,
		Username:       cred.Username,
		HasPassword:    cred.Password != "",
		URIOptions:     cred.Mongourioptions,
		MaxCollections: maxCollections,
	}
}

func cloneNodes(in []nodeJSON) []nodeJSON {
	out := make([]nodeJSON, len(in))
	for i := range in {
		out[i] = in[i]
		if in[i].Tasks != nil {
			out[i].Tasks = append([]taskJSON(nil), in[i].Tasks...)
		}
	}
	return out
}

// requireJSON makes cross-site form posts fail: a browser cannot send
// application/json to another origin without a CORS preflight this server never approves.
func requireJSON(r *http.Request) error {
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		return fmt.Errorf("Content-Type must be application/json")
	}
	return nil
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	if err := requireJSON(r); err != nil {
		return err
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return fmt.Errorf("invalid JSON body")
	}
	return nil
}

func writeJSON(w http.ResponseWriter, code int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(body)
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

func writeSSE(w http.ResponseWriter, ev Event) error {
	b, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "data: %s\n\n", b)
	return err
}

func isLoopbackAddr(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return false
	}
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// loopbackListenHint is printed for the common same-machine workflow.
// Remote browser / jump-host steps stay in README and -via help.
func loopbackListenHint(port int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "  open http://127.0.0.1:%d in a browser on this machine\n", port)
	fmt.Fprintf(&b, "  (loopback only; if the browser is on another host, see README: -via / SSH -L)\n")
	return b.String()
}
