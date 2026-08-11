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
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/fatih/color"
	"golang.org/x/term"
)

const (
	headerWidth      = 62
	progressBarWidth = 32
)

// READMEPrerequisitesURL points to SSH and environment setup instructions.
const READMEPrerequisitesURL = "https://github.com/mongodb-labs/dcrcli#prerequisites"

// UI renders styled prompts and messages to a terminal.
type UI struct {
	in        io.Reader
	out       io.Writer
	reader    *bufio.Reader
	step      int
	stepTotal int
	useColor  bool
	taskAvg   [3]time.Duration
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

type nodeState int

const (
	nodePending nodeState = iota
	nodeActive
	nodeDone
	nodeFailed
)

type collectionNodeStatus struct {
	host  CollectionHost
	state nodeState
}

// CollectionStart prints the data-collection header and a single live progress bar for the full run.
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
	cp := &CollectionProgress{
		ui:         u,
		nodes:      nodes,
		totalTasks: nodeTotal * tasksPerNode,
	}
	cp.writeProgress()
	return cp
}

// CollectionProgress tracks one live progress bar across all nodes and tasks.
type CollectionProgress struct {
	mu             sync.Mutex
	ui             *UI
	nodes          []collectionNodeStatus
	totalTasks     int
	tasksDone      int
	taskStart      time.Time
	currentTaskIdx int
	currentIdx     int
	activeSpinner  bool
	nodeFailed     bool
	initialized    bool
}

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
	base := float64(cp.tasksDone) / float64(cp.totalTasks)
	if !cp.activeSpinner {
		return base
	}
	est := cp.ui.defaultTaskEstimate(cp.currentTaskIdx)
	if est <= 0 {
		est = time.Second
	}
	partial := float64(time.Since(cp.taskStart)) / float64(est)
	if partial > 0.95 {
		partial = 0.95
	}
	f := base + partial/float64(cp.totalTasks)
	if f > 1 {
		f = 1
	}
	return f
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
	default:
		return cp.ui.dim(fmt.Sprintf("  · %s:%d", n.host.Hostname, n.host.Port))
	}
}

func (cp *CollectionProgress) allLines() []string {
	lines := make([]string, 0, 1+len(cp.nodes))
	lines = append(lines, cp.barLine())
	for i := range cp.nodes {
		lines = append(lines, cp.nodeLine(i))
	}
	return lines
}

func (cp *CollectionProgress) writeProgress() {
	if !cp.ui.isTTY() {
		return
	}

	cp.mu.Lock()
	lines := cp.allLines()
	cp.mu.Unlock()

	if len(lines) == 0 {
		return
	}

	if !cp.initialized {
		for i, line := range lines {
			if i < len(lines)-1 {
				cp.ui.println(line)
			} else {
				cp.ui.print(line)
			}
		}
		cp.initialized = true
		return
	}

	fmt.Fprintf(cp.ui.out, "\033[%dA", len(lines)-1)
	for i, line := range lines {
		if i > 0 {
			fmt.Fprint(cp.ui.out, "\n")
		}
		fmt.Fprintf(cp.ui.out, "\r\033[K%s", line)
	}
}

// BeginNode marks which node is currently being collected.
func (cp *CollectionProgress) BeginNode(idx int) {
	cp.mu.Lock()
	cp.currentIdx = idx
	cp.nodeFailed = false
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
		if cp.nodeFailed {
			cp.nodes[cp.currentIdx].state = nodeFailed
		} else {
			cp.nodes[cp.currentIdx].state = nodeDone
		}
	}
	cp.mu.Unlock()
	cp.writeProgress()
}

// RunTask runs a collection step and updates the global progress bar.
func (cp *CollectionProgress) RunTask(taskIdx int, _ string, fn func() error) error {
	cp.currentTaskIdx = taskIdx
	cp.taskStart = time.Now()
	cp.activeSpinner = true

	stop := make(chan struct{})
	var wg sync.WaitGroup

	if cp.ui.isTTY() {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ticker := time.NewTicker(100 * time.Millisecond)
			defer ticker.Stop()
			for {
				select {
				case <-stop:
					return
				case <-ticker.C:
					cp.writeProgress()
				}
			}
		}()
	}

	err := fn()
	close(stop)
	wg.Wait()
	cp.activeSpinner = false

	if err != nil {
		cp.mu.Lock()
		cp.nodeFailed = true
		cp.mu.Unlock()
	}

	cp.ui.recordTaskDuration(taskIdx, time.Since(cp.taskStart))
	cp.tasksDone++
	cp.writeProgress()
	return err
}

// SkipTask records a skipped step and advances the global progress bar.
func (cp *CollectionProgress) SkipTask(taskIdx int, _, _ string) {
	_ = taskIdx
	cp.tasksDone++
	cp.writeProgress()
}

// Finish completes the progress bar and moves to the next output line.
func (cp *CollectionProgress) Finish() {
	cp.mu.Lock()
	cp.activeSpinner = false
	cp.tasksDone = cp.totalTasks
	cp.mu.Unlock()

	if cp.ui.isTTY() {
		cp.writeProgress()
		cp.ui.println("")
		return
	}
	cp.ui.println(cp.barLine())
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

// SSHAuthNotice tells the user to enter an SSH password when rsync prompts for it.
func SSHAuthNotice() {
	ui := NewDefault()
	ui.Section("SSH authentication")
	ui.Note("Enter your SSH password when rsync prompts for it.")
}
