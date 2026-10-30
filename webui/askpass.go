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
	"bytes"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

// OpenSSH runs SSH_ASKPASS instead of reading /dev/tty when it needs a password
// or a host-key answer. In web mode that program is this binary: it forwards the
// prompt to the running server, which shows it in the browser and waits for the
// answer.
const (
	envAskpassURL   = "DCRCLI_ASKPASS_URL"
	envAskpassToken = "DCRCLI_ASKPASS_TOKEN"
	askpassTimeout  = 5 * time.Minute
	askpassHeader   = "X-Dcrcli-Askpass-Token"
)

type promptJSON struct {
	ID      string `json:"id"`
	Text    string `json:"text,omitempty"`
	Kind    string `json:"kind,omitempty"`
	Context string `json:"context,omitempty"`
	Since   string `json:"since,omitempty"`
}

type promptAnswer struct {
	text   string
	cancel bool
}

type pendingPrompt struct {
	info   promptJSON
	answer chan promptAnswer
}

// IsAskpass reports whether this process was started by ssh as the askpass helper.
func IsAskpass() bool {
	return os.Getenv(envAskpassURL) != "" && os.Getenv(envAskpassToken) != ""
}

// RunAskpass forwards one ssh prompt to the web UI and prints the answer for ssh.
// The exit code is non-zero when the user cancels or the server is unreachable.
func RunAskpass(args []string) int {
	if os.Getenv("SSH_ASKPASS_PROMPT") == "none" {
		return 0
	}
	body, err := json.Marshal(map[string]string{
		"prompt": strings.Join(args, " "),
		"hint":   os.Getenv("SSH_ASKPASS_PROMPT"),
	})
	if err != nil {
		return 1
	}
	req, err := http.NewRequest(http.MethodPost, os.Getenv(envAskpassURL), bytes.NewReader(body))
	if err != nil {
		return 1
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(askpassHeader, os.Getenv(envAskpassToken))
	client := &http.Client{Timeout: askpassTimeout + 30*time.Second}
	res, err := client.Do(req)
	if err != nil {
		fmt.Fprintln(os.Stderr, "dcrcli askpass: web UI unreachable:", err)
		return 1
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return 1
	}
	var out struct {
		Answer string `json:"answer"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		return 1
	}
	fmt.Fprintln(os.Stdout, out.Answer)
	return 0
}

func setupAskpassEnv(url, token string) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locate dcrcli binary for SSH prompts: %w", err)
	}
	vars := map[string]string{
		envAskpassURL:         url,
		envAskpassToken:       token,
		"SSH_ASKPASS":         exe,
		"SSH_ASKPASS_REQUIRE": "force",
	}
	if os.Getenv("DISPLAY") == "" {
		vars["DISPLAY"] = "dcrcli:0"
	}
	for k, v := range vars {
		if err := os.Setenv(k, v); err != nil {
			return err
		}
	}
	return nil
}

func randomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

func promptKind(text, hint string) string {
	lower := strings.ToLower(text)
	switch {
	case strings.Contains(lower, "(yes/no"):
		return "confirm"
	case hint == "confirm":
		return "confirm"
	default:
		return "secret"
	}
}

func (a *App) setSSHContext(ctx string) {
	a.mu.Lock()
	a.sshContext = ctx
	a.mu.Unlock()
}

func (a *App) pendingPrompts() []promptJSON {
	out := make([]promptJSON, 0, len(a.prompts))
	for _, p := range a.prompts {
		out = append(out, p.info)
	}
	return out
}

// handleAskpass is called by the askpass helper, never by the browser.
func (a *App) handleAskpass(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "POST required")
		return
	}
	got := r.Header.Get(askpassHeader)
	if a.askToken == "" || subtle.ConstantTimeCompare([]byte(got), []byte(a.askToken)) != 1 {
		writeErr(w, http.StatusForbidden, "forbidden")
		return
	}
	var req struct {
		Prompt string `json:"prompt"`
		Hint   string `json:"hint"`
	}
	if err := decodeJSON(w, r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	text := strings.TrimSpace(req.Prompt)
	if len(text) > 8192 {
		text = text[:8192]
	}

	p := &pendingPrompt{answer: make(chan promptAnswer, 1)}
	a.mu.Lock()
	p.info = promptJSON{
		ID:      randomHex(8),
		Text:    text,
		Kind:    promptKind(text, req.Hint),
		Context: a.sshContext,
		Since:   time.Now().Format(time.RFC3339),
	}
	a.prompts[p.info.ID] = p
	a.mu.Unlock()

	info := p.info
	if info.Kind == "confirm" {
		a.logWarn("SSH asks to confirm a host key. Answer the prompt in the browser.")
	} else {
		a.logInfo("SSH is waiting for a password. Answer the prompt in the browser.")
	}
	a.emit(Event{Type: "ssh_prompt", Prompt: &info})

	defer func() {
		a.mu.Lock()
		delete(a.prompts, info.ID)
		a.mu.Unlock()
		a.emit(Event{Type: "ssh_prompt_done", Prompt: &promptJSON{ID: info.ID}})
	}()

	select {
	case ans := <-p.answer:
		if ans.cancel {
			a.logWarn("SSH prompt cancelled in the browser")
			writeErr(w, http.StatusConflict, "cancelled")
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"answer": ans.text})
	case <-r.Context().Done():
	case <-time.After(askpassTimeout):
		a.logWarn("SSH prompt timed out after 5 minutes without an answer")
		writeErr(w, http.StatusGatewayTimeout, "timed out")
	}
}

// handleSSHAnswer receives the browser's answer to a pending SSH prompt.
func (a *App) handleSSHAnswer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "POST required")
		return
	}
	var req struct {
		ID     string `json:"id"`
		Answer string `json:"answer"`
		Cancel bool   `json:"cancel"`
	}
	if err := decodeJSON(w, r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if strings.ContainsAny(req.Answer, "\r\n") {
		writeErr(w, http.StatusBadRequest, "answer must be a single line")
		return
	}
	a.mu.Lock()
	p, ok := a.prompts[req.ID]
	a.mu.Unlock()
	if !ok {
		writeErr(w, http.StatusNotFound, "this SSH prompt is no longer waiting")
		return
	}
	select {
	case p.answer <- promptAnswer{text: req.Answer, cancel: req.Cancel}:
	default:
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "sent"})
}
