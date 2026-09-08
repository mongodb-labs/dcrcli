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

// Package termui provides a consistent interactive terminal experience for dcrcli.
//
// Prompts follow the common Go CLI pattern used by survey, promptui, and gum:
// a short label on the same line as input (no decorative boxes). Libraries like
// charmbracelet/huh offer richer TUI forms (arrow-key selects, themes) but add
// weight and complexity; dcrcli keeps prompts lightweight for scripting and -config.
package termui

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode/utf8"

	"github.com/fatih/color"
	"golang.org/x/term"
)

const (
	headerWidth      = 62
	progressBarWidth = 32
	// progressCompactNodeThreshold switches to bar + active node + counter (not full node list).
	progressCompactNodeThreshold = 8
	collectionTasksPerNode       = 3
)

var collectionTaskLabels = [collectionTasksPerNode]string{"getMongoData", "FTDC", "logs"}

// ansiEscape matches SGR/CSI sequences so progress lines can be width-limited
// without counting color codes toward the terminal column count.
var ansiEscape = regexp.MustCompile(`\x1b\[[0-9;]*[A-Za-z]`)

// READMEPrerequisitesURL points to SSH and environment setup instructions.
const READMEPrerequisitesURL = "https://github.com/mongodb-labs/dcrcli#prerequisites"

// UI renders styled prompts and messages to a terminal.
type UI struct {
	in           io.Reader
	out          io.Writer
	reader       *bufio.Reader
	step         int
	stepTotal    int
	useColor     bool
	taskAvg      [3]time.Duration
	taskAvgCount [3]int
}

// New creates a UI bound to the given input and output streams.
func New(in io.Reader, out io.Writer) *UI {
	reader, ok := in.(*bufio.Reader)
	if !ok {
		reader = bufio.NewReader(in)
	}
	useColor := false
	if f, ok := out.(*os.File); ok {
		useColor = term.IsTerminal(int(f.Fd()))
	}
	return &UI{
		in:       in,
		out:      out,
		reader:   reader,
		useColor: useColor,
	}
}

// NewDefault creates a UI using os.Stdin and os.Stdout.
func NewDefault() *UI {
	return New(os.Stdin, os.Stdout)
}

// SetStepTotal enables [n/total] labels on BeginStep calls.
func (u *UI) SetStepTotal(total int) {
	u.stepTotal = total
}

// EndStepSession clears numbered step labels so the next prompts start a new section.
func (u *UI) EndStepSession() {
	u.step = 0
	u.stepTotal = 0
}

func (u *UI) paint(c *color.Color, s string) string {
	if !u.useColor {
		return s
	}
	return c.Sprint(s)
}

func (u *UI) println(s string) {
	_, _ = fmt.Fprintln(u.out, s)
}

func (u *UI) print(s string) {
	_, _ = fmt.Fprint(u.out, s)
}

func (u *UI) dim(s string) string {
	return u.paint(color.New(color.FgHiBlack), s)
}

func (u *UI) promptPrefix() string {
	return u.paint(color.New(color.FgGreen, color.Bold), "› ")
}

func (u *UI) readLine() (string, error) {
	line, err := u.reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r")), nil
}

// Header prints a top-level section banner.
func (u *UI) Header(title string) {
	line := strings.Repeat("═", headerWidth)
	u.println("")
	u.println(u.paint(color.New(color.FgCyan, color.Bold), line))
	u.println(u.paint(color.New(color.FgCyan, color.Bold), fmt.Sprintf("  %s", title)))
	u.println(u.paint(color.New(color.FgCyan, color.Bold), line))
	u.println("")
}

// Tip prints secondary guidance in a dim style.
func (u *UI) Tip(msg string) {
	u.println(u.dim("  " + msg))
	u.println("")
}

// BeginStep starts a numbered prompt step.
func (u *UI) BeginStep(title string) {
	u.step++
	u.println("")
	if u.stepTotal > 0 {
		u.println(
			u.paint(
				color.New(color.FgHiCyan, color.Bold),
				fmt.Sprintf("  [%d/%d] %s", u.step, u.stepTotal, title),
			),
		)
		return
	}
	u.println(u.paint(color.New(color.FgHiCyan, color.Bold), "  "+title))
}

// Note prints helper text under a step.
func (u *UI) Note(lines ...string) {
	for _, line := range lines {
		u.println(u.dim("  " + line))
	}
}

// AskInput reads a line. label is shown on the same line as the cursor (survey-style).
// Pass an empty label when BeginStep already names the field.
func (u *UI) AskInput(label string) (string, error) {
	u.print("  ")
	u.print(u.promptPrefix())
	if label != "" {
		u.print(label)
		if !strings.HasSuffix(label, ":") && !strings.HasSuffix(label, "?") {
			u.print(":")
		}
		u.print(" ")
	}
	return u.readLine()
}

// AskPassword reads a hidden password on the same line as the prompt.
func (u *UI) AskPassword(label string) (string, error) {
	u.print("  ")
	u.print(u.promptPrefix())
	if label == "" {
		label = "Password"
	}
	u.print(label + ": ")

	stdinFd := int(syscall.Stdin)
	if f, ok := u.in.(*os.File); ok {
		stdinFd = int(f.Fd())
	}
	bytePassword, err := term.ReadPassword(stdinFd)
	if err != nil {
		return "", err
	}
	u.println("")
	return string(bytePassword), nil
}

// Menu prints a numbered list of options.
func (u *UI) Menu(items []string) {
	for i, item := range items {
		u.println(fmt.Sprintf("    %d) %s", i+1, item))
	}
}

// AskChoice reads a menu selection on the same line as the prompt.
func (u *UI) AskChoice(hint string) (string, error) {
	if hint == "" {
		hint = "Choice [1]"
	}
	u.print("  ")
	u.print(u.promptPrefix())
	u.print(hint + ": ")
	return u.readLine()
}

// AskYesNo prompts for confirmation on the same line as the prompt.
func (u *UI) AskYesNo(hint string, defaultNo bool) (bool, error) {
	defaultLabel := "y/N"
	if !defaultNo {
		defaultLabel = "Y/n"
	}
	prompt := hint
	if prompt == "" {
		prompt = "Continue"
	}
	u.print("  ")
	u.print(u.promptPrefix())
	u.print(fmt.Sprintf("%s [%s]: ", prompt, defaultLabel))

	line, err := u.readLine()
	if err != nil {
		return false, err
	}

	answer := strings.ToLower(line)
	switch answer {
	case "":
		return !defaultNo, nil
	case "y", "yes":
		return true, nil
	case "n", "no":
		return false, nil
	default:
		return false, fmt.Errorf("invalid answer %q: enter y/yes or n/no", answer)
	}
}

// Warn prints a warning message.
func (u *UI) Warn(msg string) {
	u.println(u.paint(color.New(color.FgYellow), "  ! "+msg))
}

// Ok prints a success message.
func (u *UI) Ok(msg string) {
	u.println(u.paint(color.New(color.FgGreen), "  ✓ "+msg))
}

// Section prints a sub-heading between flows.
func (u *UI) Section(title string) {
	u.println("")
	u.println(u.paint(color.New(color.FgHiCyan, color.Bold), "  "+title))
}

// Info prints a neutral informational line.
func (u *UI) Info(msg string) {
	u.println("  " + msg)
}

// KeyValue prints an aligned label/value pair.
func (u *UI) KeyValue(key, value string) {
	u.println(fmt.Sprintf("  %-14s %s", key+":", value))
}

// ErrorBanner prints a prominent error block.
func (u *UI) ErrorBanner(title string, lines ...string) {
	u.println("")
	line := strings.Repeat("═", headerWidth)
	u.println(u.paint(color.New(color.FgRed, color.Bold), line))
	u.println(u.paint(color.New(color.FgRed, color.Bold), fmt.Sprintf("  %s", title)))
	u.println(u.paint(color.New(color.FgRed, color.Bold), line))
	u.println("")
	for _, msg := range lines {
		u.println("  " + msg)
	}
	u.println("")
}

// Blank prints a single empty line.
func (u *UI) Blank() {
	u.println("")
}

func (u *UI) progressBar(filled, width int) string {
	if filled < 0 {
		filled = 0
	}
	if filled > width {
		filled = width
	}
	return u.paint(color.New(color.FgGreen), strings.Repeat("█", filled)) +
		u.dim(strings.Repeat("░", width-filled))
}

// CollectionHost identifies a node shown in the collection progress display.
type CollectionHost struct {
	Hostname string
	Port     int
}

// SSHTarget identifies the host rsync/ssh connects to when leaving the progress view.
type SSHTarget struct {
	User      string
	Host      string // SSH/rsync hostname
	Purpose   string // e.g. "FTDC data", "mongod logs"
	MongoHost string // MongoDB node being collected (for display)
	MongoPort int
}

type nodeState int

const (
	nodePending nodeState = iota
	nodeActive
	nodeDone
	nodePartial
	nodeFailed
)

type taskOutcome int

const (
	taskPending taskOutcome = iota
	taskOK
	taskFailed
	taskSkipped     // selected but could not run (e.g. remote node without SSH)
	taskNotSelected // not part of the collect-data selection
)

type taskStatus struct {
	outcome taskOutcome
	note    string
}

type collectionNodeStatus struct {
	host  CollectionHost
	state nodeState
	tasks [collectionTasksPerNode]taskStatus
}

// CollectionStart prints the data-collection header and a live progress view for the full run.
func (u *UI) CollectionStart(hosts []CollectionHost, tasksPerNode int) *CollectionProgress {
	nodeTotal := len(hosts)
	if nodeTotal < 1 {
		nodeTotal = 1
	}
	if tasksPerNode < 1 {
		tasksPerNode = 1
	}
	nodes := make([]collectionNodeStatus, len(hosts))
	for i, h := range hosts {
		nodes[i] = collectionNodeStatus{host: h, state: nodePending}
	}
	u.Header("Data collection")
	var display io.Writer = os.Stdout
	if !writerTTY(display) {
		display = u.out
	}
	cp := &CollectionProgress{
		ui:         u,
		display:    display,
		nodes:      nodes,
		totalTasks: nodeTotal * tasksPerNode,
		compact:    nodeTotal > progressCompactNodeThreshold,
	}
	activeCollectionProgress = cp
	cp.writeProgress()
	return cp
}

// CollectionProgress tracks one live progress bar across all nodes and tasks.
type CollectionProgress struct {
	mu              sync.Mutex
	ui              *UI
	display         io.Writer
	nodes           []collectionNodeStatus
	totalTasks      int
	tasksDone       int
	taskStart       time.Time
	currentTaskIdx  int
	currentIdx      int
	activeSpinner   bool
	compact         bool
	initialized     bool
	compactLineOpen bool // cursor is on the progress bar row
	onHintLine      bool // cursor is on the reusable SSH hint row under the bar
	linesOnScreen   int
}

var activeCollectionProgress *CollectionProgress

// LeaveForSubprocess is a no-op; RunTask leaves the progress view when usesTerminal is true.
func LeaveForSubprocess(string) {}

func (u *UI) defaultTaskEstimate(taskIdx int) time.Duration {
	if taskIdx >= 0 && taskIdx < len(u.taskAvg) && u.taskAvg[taskIdx] > 0 {
		return u.taskAvg[taskIdx]
	}
	switch taskIdx {
	case 0:
		return 800 * time.Millisecond
	case 1:
		return 1500 * time.Millisecond
	default:
		return 1200 * time.Millisecond
	}
}

func (u *UI) recordTaskDuration(taskIdx int, d time.Duration) {
	if taskIdx < 0 || taskIdx >= len(u.taskAvg) {
		return
	}
	n := u.taskAvgCount[taskIdx]
	if n == 0 {
		u.taskAvg[taskIdx] = d
	} else {
		u.taskAvg[taskIdx] = (u.taskAvg[taskIdx]*time.Duration(n) + d) / time.Duration(n+1)
	}
	u.taskAvgCount[taskIdx]++
}

func (cp *CollectionProgress) fraction() float64 {
	if cp.totalTasks <= 0 {
		return 1
	}
	// Keep progress discrete (completed steps only) to avoid noisy percentage creep.
	return float64(cp.tasksDone) / float64(cp.totalTasks)
}

func (cp *CollectionProgress) barLine() string {
	pct := int(cp.fraction() * 100)
	if pct > 100 {
		pct = 100
	}
	filled := int(cp.fraction() * float64(progressBarWidth))
	if filled > progressBarWidth {
		filled = progressBarWidth
	}
	return fmt.Sprintf("  %s  %3d%%", cp.ui.progressBar(filled, progressBarWidth), pct)
}

func (cp *CollectionProgress) nodeLine(i int) string {
	n := cp.nodes[i]
	switch n.state {
	case nodeDone:
		return cp.ui.paint(
			color.New(color.FgGreen),
			fmt.Sprintf("  ✓ %s:%d", n.host.Hostname, n.host.Port),
		)
	case nodeFailed:
		return cp.ui.paint(
			color.New(color.FgYellow),
			fmt.Sprintf("  ! %s:%d", n.host.Hostname, n.host.Port),
		)
	case nodeActive:
		return cp.ui.paint(
			color.New(color.FgHiCyan),
			fmt.Sprintf("  › %s:%d", n.host.Hostname, n.host.Port),
		)
	case nodePartial:
		return cp.ui.paint(
			color.New(color.FgHiYellow),
			fmt.Sprintf("  ~ %s:%d", n.host.Hostname, n.host.Port),
		)
	default:
		return cp.ui.dim(fmt.Sprintf("  · %s:%d", n.host.Hostname, n.host.Port))
	}
}

func (cp *CollectionProgress) nodeCounts() (done, partial, failed, total int) {
	total = len(cp.nodes)
	for _, n := range cp.nodes {
		switch n.state {
		case nodeDone:
			done++
		case nodePartial:
			partial++
		case nodeFailed:
			failed++
		}
	}
	return done, partial, failed, total
}

func (cp *CollectionProgress) deriveNodeState(i int) nodeState {
	if i < 0 || i >= len(cp.nodes) {
		return nodePending
	}
	hasFail, hasOK, hasUnavailable, attempted := false, false, false, false
	for _, t := range cp.nodes[i].tasks {
		switch t.outcome {
		case taskFailed:
			hasFail = true
			attempted = true
		case taskSkipped:
			hasUnavailable = true
			attempted = true
		case taskNotSelected:
			// Omitted from the collect-data selection — does not affect node outcome.
		case taskOK:
			hasOK = true
			attempted = true
		}
	}
	if !attempted {
		return nodePending
	}
	if hasFail {
		return nodeFailed
	}
	if hasUnavailable && !hasOK {
		return nodeFailed
	}
	if hasUnavailable && hasOK {
		return nodePartial
	}
	return nodeDone
}

func (cp *CollectionProgress) taskSummaryPart(taskIdx int, t taskStatus) string {
	label := collectionTaskLabels[taskIdx]
	switch t.outcome {
	case taskOK:
		return cp.ui.paint(color.New(color.FgGreen), "✓") + " " + label
	case taskFailed:
		return cp.ui.paint(color.New(color.FgYellow), "!") + " " + label
	case taskSkipped:
		return cp.ui.dim("− " + label)
	default:
		return cp.ui.dim("· " + label)
	}
}

func (cp *CollectionProgress) nodeResultLine(i int) string {
	if i < 0 || i >= len(cp.nodes) {
		return ""
	}
	n := cp.nodes[i]
	host := fmt.Sprintf("%s:%d", n.host.Hostname, n.host.Port)

	var prefix string
	switch n.state {
	case nodeFailed:
		prefix = cp.ui.paint(color.New(color.FgYellow), "  ! "+host)
	case nodePartial:
		prefix = cp.ui.paint(color.New(color.FgHiYellow), "  ~ "+host)
	default:
		prefix = cp.ui.paint(color.New(color.FgGreen), "  ✓ "+host)
	}

	var parts []string
	for t := 0; t < collectionTasksPerNode; t++ {
		outcome := n.tasks[t].outcome
		if outcome == taskNotSelected || outcome == taskPending {
			continue
		}
		parts = append(parts, cp.taskSummaryPart(t, n.tasks[t]))
	}
	if len(parts) == 0 {
		return prefix
	}
	return prefix + "  " + strings.Join(parts, "  ")
}

func (cp *CollectionProgress) collectionTotalsLine() string {
	done, partial, failed, total := cp.nodeCounts()
	if total == 0 {
		return ""
	}
	parts := []string{fmt.Sprintf("%d/%d nodes complete", done+partial+failed, total)}
	if partial > 0 {
		parts = append(parts, fmt.Sprintf("%d partial", partial))
	}
	if failed > 0 {
		parts = append(parts, fmt.Sprintf("%d failed", failed))
	}
	return cp.ui.dim("  " + strings.Join(parts, " · "))
}

func (cp *CollectionProgress) progressLines() []string {
	return []string{cp.compactStatusLine()}
}

func (cp *CollectionProgress) compactStatusLine() string {
	line := cp.barLine()
	for i := range cp.nodes {
		if cp.nodes[i].state == nodeActive {
			n := cp.nodes[i]
			line += cp.ui.paint(
				color.New(color.FgHiCyan),
				fmt.Sprintf("  › %s:%d", n.host.Hostname, n.host.Port),
			)
			break
		}
	}
	done, partial, failed, total := cp.nodeCounts()
	if total > 0 {
		completed := done + partial + failed
		line += cp.ui.dim(fmt.Sprintf("  %d/%d nodes", completed, total))
	}
	return line
}

// summaryLines is the final on-screen progress block after collection.
func (cp *CollectionProgress) summaryLines() []string {
	lines := []string{cp.barLine()}
	if !cp.compact {
		for i := range cp.nodes {
			lines = append(lines, cp.nodeResultLine(i))
		}
	}
	return lines
}

func writerTTY(w io.Writer) bool {
	f, ok := w.(*os.File)
	return ok && term.IsTerminal(int(f.Fd()))
}

func stripANSI(s string) string {
	return ansiEscape.ReplaceAllString(s, "")
}

func visibleLen(s string) int {
	return utf8.RuneCountInString(stripANSI(s))
}

func (cp *CollectionProgress) termSize() (w, h int) {
	w, h = 80, 24
	f, ok := cp.display.(*os.File)
	if !ok {
		return w, h
	}
	tw, th, err := term.GetSize(int(f.Fd()))
	if err != nil {
		return w, h
	}
	if tw >= 20 {
		w = tw
	}
	if th >= 3 {
		h = th
	}
	return w, h
}

func (cp *CollectionProgress) termWidth() int {
	w, _ := cp.termSize()
	return w
}

// fitLine truncates s to at most width visible columns so the terminal will
// not wrap the live progress line (wrapping would make \r start a new row).
func fitLine(s string, width int) string {
	if width < 1 {
		return ""
	}
	if visibleLen(s) <= width {
		return s
	}
	plain := []rune(stripANSI(s))
	if width == 1 {
		return string(plain[:1])
	}
	return string(plain[:width-1]) + "…"
}

// writeProgressLine redraws the bar on a single row. A second row is reserved
// for the current SSH hint and is overwritten, so large clusters do not log
// one SSH line per node.
func (cp *CollectionProgress) writeProgressLine(line string) {
	line = fitLine(line, cp.termWidth()-1)
	if !cp.initialized {
		fmt.Fprintf(cp.display, "\r\033[2K%s\n\033[2K\033[1A", line)
		cp.initialized = true
	} else {
		if cp.onHintLine {
			fmt.Fprint(cp.display, "\033[1A")
			cp.onHintLine = false
		}
		fmt.Fprintf(cp.display, "\r\033[2K%s", line)
	}
	cp.compactLineOpen = true
	cp.linesOnScreen = 1
}

func (cp *CollectionProgress) printNodeReport() {
	cp.mu.Lock()
	n := len(cp.nodes)
	cp.mu.Unlock()
	if n == 0 {
		return
	}

	fmt.Fprintln(cp.display, cp.ui.dim("  Node results:"))
	for i := 0; i < n; i++ {
		cp.mu.Lock()
		line := cp.nodeResultLine(i)
		cp.mu.Unlock()
		if line != "" {
			fmt.Fprintln(cp.display, line)
		}
	}
	if totals := cp.collectionTotalsLine(); totals != "" {
		fmt.Fprintln(cp.display, totals)
	}
}

func (cp *CollectionProgress) writeProgress() {
	if !writerTTY(cp.display) {
		return
	}
	cp.mu.Lock()
	line := cp.compactStatusLine()
	cp.mu.Unlock()
	cp.writeProgressLine(line)
}

// BeginNode marks which node is currently being collected.
func (cp *CollectionProgress) BeginNode(idx int) {
	cp.mu.Lock()
	cp.currentIdx = idx
	if idx >= 0 && idx < len(cp.nodes) {
		cp.nodes[idx].state = nodeActive
	}
	cp.mu.Unlock()
	cp.writeProgress()
}

// FinishNode marks the active node as done or failed.
func (cp *CollectionProgress) FinishNode() {
	cp.mu.Lock()
	if cp.currentIdx >= 0 && cp.currentIdx < len(cp.nodes) {
		cp.nodes[cp.currentIdx].state = cp.deriveNodeState(cp.currentIdx)
	}
	cp.mu.Unlock()
	cp.writeProgress()
}

func sshHandoffMessage(ssh *SSHTarget) string {
	if ssh == nil {
		return "SSH/rsync — enter password or confirm host key if prompted"
	}
	if ssh.User == "" || ssh.Host == "" {
		return "SSH/rsync — enter password or confirm host key if prompted"
	}

	node := strings.TrimSpace(ssh.MongoHost)
	if node == "" {
		node = ssh.Host
	}
	if ssh.MongoPort > 0 {
		node = fmt.Sprintf("%s:%d", node, ssh.MongoPort)
	}

	purpose := strings.TrimSpace(ssh.Purpose)
	if purpose != "" {
		return fmt.Sprintf(
			"SSH to %s@%s — copying %s for %s (enter password or confirm host key if prompted)",
			ssh.User,
			ssh.Host,
			purpose,
			node,
		)
	}
	return fmt.Sprintf(
		"SSH to %s@%s — enter password or confirm host key if prompted",
		ssh.User,
		ssh.Host,
	)
}

func (cp *CollectionProgress) handoffForSubprocess(ssh *SSHTarget) {
	msg := fitLine(cp.ui.dim("  "+sshHandoffMessage(ssh)), cp.termWidth()-1)
	if cp.onHintLine {
		fmt.Fprintf(cp.display, "\r\033[2K%s", msg)
		return
	}
	fmt.Fprintf(cp.display, "\n\r\033[2K%s", msg)
	cp.onHintLine = true
	cp.compactLineOpen = false
}

func (cp *CollectionProgress) reconcileAfterSubprocess() {
	if cp.onHintLine {
		fmt.Fprint(cp.display, "\r\033[2K\033[1A")
		cp.onHintLine = false
		cp.compactLineOpen = true
	}
	cp.writeProgress()
}

// RunTask runs a collection step and updates the global progress bar.
// Pass ssh when the task needs the main screen (e.g. SSH/rsync prompts).
func (cp *CollectionProgress) RunTask(taskIdx int, ssh *SSHTarget, fn func() error) error {
	cp.currentTaskIdx = taskIdx
	cp.taskStart = time.Now()
	cp.activeSpinner = true
	cp.writeProgress()

	if ssh != nil {
		cp.handoffForSubprocess(ssh)
	}

	err := fn()

	if ssh != nil {
		cp.reconcileAfterSubprocess()
	}

	cp.activeSpinner = false

	cp.mu.Lock()
	if cp.currentIdx >= 0 && cp.currentIdx < len(cp.nodes) && taskIdx >= 0 && taskIdx < collectionTasksPerNode {
		ts := taskStatus{outcome: taskOK}
		if err != nil {
			ts.outcome = taskFailed
		}
		cp.nodes[cp.currentIdx].tasks[taskIdx] = ts
	}
	cp.mu.Unlock()

	cp.ui.recordTaskDuration(taskIdx, time.Since(cp.taskStart))
	cp.tasksDone++
	cp.writeProgress()
	return err
}

// SkipTask records a selected artifact that could not run and advances the global progress bar.
func (cp *CollectionProgress) SkipTask(taskIdx int, _, reason string) {
	cp.mu.Lock()
	if cp.currentIdx >= 0 && cp.currentIdx < len(cp.nodes) && taskIdx >= 0 && taskIdx < collectionTasksPerNode {
		cp.nodes[cp.currentIdx].tasks[taskIdx] = taskStatus{outcome: taskSkipped, note: reason}
	}
	cp.tasksDone++
	cp.mu.Unlock()
	cp.writeProgress()
}

// SkipTaskNotSelected records an artifact type omitted from the collect-data selection.
func (cp *CollectionProgress) SkipTaskNotSelected(taskIdx int, _, reason string) {
	cp.mu.Lock()
	if cp.currentIdx >= 0 && cp.currentIdx < len(cp.nodes) && taskIdx >= 0 && taskIdx < collectionTasksPerNode {
		cp.nodes[cp.currentIdx].tasks[taskIdx] = taskStatus{outcome: taskNotSelected, note: reason}
	}
	cp.tasksDone++
	cp.mu.Unlock()
	cp.writeProgress()
}

// Finish completes the progress bar and leaves the final summary on the same screen.
func (cp *CollectionProgress) Finish() {
	cp.mu.Lock()
	cp.activeSpinner = false
	cp.tasksDone = cp.totalTasks
	bar := cp.barLine()
	cp.mu.Unlock()

	activeCollectionProgress = nil

	if !writerTTY(cp.display) {
		fmt.Fprintln(cp.display, bar)
		return
	}

	if cp.onHintLine {
		fmt.Fprint(cp.display, "\r\033[2K\033[1A")
		cp.onHintLine = false
	}
	fmt.Fprintf(cp.display, "\r\033[2K%s\n\033[2K\n", fitLine(bar, cp.termWidth()-1))
	cp.compactLineOpen = false
	cp.printNodeReport()
	fmt.Fprintln(cp.display)
}

func (u *UI) isTTY() bool {
	if f, ok := u.out.(*os.File); ok {
		return term.IsTerminal(int(f.Fd()))
	}
	return false
}

func (u *UI) clearLine() {
	if u.isTTY() {
		_, _ = fmt.Fprint(u.out, "\r\033[K")
	}
}

// CollectionTaskSkipped is a no-op; collection progress is shown only via CollectionProgress.
func (u *UI) CollectionTaskSkipped(_, _ int, _, _ string) {}

// SSHAuthNotice is a no-op; RunTask leaves the progress view before SSH/rsync subprocesses.
func SSHAuthNotice() {}
