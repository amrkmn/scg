package ui

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"golang.org/x/term"
)

// IsTTY returns true if stdout is connected to a terminal.
func IsTTY() bool {
	return term.IsTerminal(int(os.Stdout.Fd()))
}

// clearLine clears the current terminal line and returns the cursor to column 0.
const clearLine = "\r\x1b[2K"

// Spinner displays an animated "message..." indicator.
type Spinner struct {
	message string
	ticker  *time.Ticker
	done    chan struct{}
	mu      sync.Mutex
	running bool
	frame   int
}

// SpinnerFrames are the animation frames used by Spinner, exported for reuse.
var SpinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// SpinnerInterval is the tick interval used by Spinner, exported for reuse.
const SpinnerInterval = 150 * time.Millisecond

// NewSpinner creates a new Spinner with the given message.
func NewSpinner(message string) *Spinner {
	return &Spinner{message: message}
}

// Start begins the spinner animation.
func (s *Spinner) Start() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running {
		return
	}
	s.running = true
	s.done = make(chan struct{})
	s.ticker = time.NewTicker(SpinnerInterval)
	go func() {
		for {
			select {
			case <-s.done:
				return
			case <-s.ticker.C:
				s.mu.Lock()
				f := SpinnerFrames[s.frame%len(SpinnerFrames)]
				s.frame++
				msg := s.message
				s.mu.Unlock()
				_, _ = fmt.Fprintf(os.Stdout, "%s%s %s", clearLine, Cyan(f), msg)
			}
		}
	}()
}

// stop halts the animation without printing a final message.
func (s *Spinner) stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.running {
		return
	}
	s.running = false
	s.ticker.Stop()
	close(s.done)
	_, _ = fmt.Fprint(os.Stdout, clearLine)
}

// Succeed stops the spinner and prints a success message.
func (s *Spinner) Succeed(msg string) {
	s.stop()
	if msg != "" {
		_, _ = fmt.Fprintln(os.Stdout, Done(msg, ""))
	}
}

// Fail stops the spinner and prints a failure message.
func (s *Spinner) Fail(msg string) {
	s.stop()
	if msg != "" {
		_, _ = fmt.Fprintln(os.Stdout, FailLine(msg))
	}
}

// Stop halts the spinner silently.
func (s *Spinner) Stop() {
	s.stop()
}

// SetMessage updates the spinner message while it is running.
func (s *Spinner) SetMessage(msg string) {
	s.mu.Lock()
	s.message = msg
	s.mu.Unlock()
}

// ProgressBar renders a visual progress bar: [====    ] current/total message.
type ProgressBar struct {
	total   int
	current int
	message string
	step    string
	barW    int
	mu      sync.Mutex
	started bool
	stopped bool
}

// NewProgressBar creates a ProgressBar with the given total and message.
func NewProgressBar(total int, message string) *ProgressBar {
	return &ProgressBar{
		total:   total,
		message: message,
		barW:    30,
	}
}

// Start prints the initial bar.
func (p *ProgressBar) Start() {
	p.mu.Lock()
	p.started = true
	p.mu.Unlock()
	p.render()
}

// Increment advances progress by n (default 1).
func (p *ProgressBar) Increment(n int) {
	p.mu.Lock()
	p.current += n
	if p.current > p.total {
		p.current = p.total
	}
	p.mu.Unlock()
	p.render()
}

// SetProgress sets the current progress value and optional step label.
func (p *ProgressBar) SetProgress(current int, stepLabel string) {
	p.mu.Lock()
	p.current = current
	if p.current > p.total {
		p.current = p.total
	}
	if stepLabel != "" {
		p.step = stepLabel
	}
	p.mu.Unlock()
	p.render()
}

// SetStep sets the step label without changing the current progress.
func (p *ProgressBar) SetStep(stepLabel string) {
	p.mu.Lock()
	p.step = stepLabel
	p.mu.Unlock()
	p.render()
}

// Reset reinitializes the bar with a new total and optional message.
func (p *ProgressBar) Reset(newTotal int, msg string) {
	p.mu.Lock()
	p.total = newTotal
	p.current = 0
	if msg != "" {
		p.message = msg
	}
	p.step = ""
	p.mu.Unlock()
	p.render()
}

// Stop halts the bar and clears the line.
func (p *ProgressBar) Stop() {
	p.mu.Lock()
	p.stopped = true
	p.mu.Unlock()
	_, _ = fmt.Fprint(os.Stdout, clearLine)
}

// Complete prints the bar at 100% and moves to a new line.
func (p *ProgressBar) Complete() {
	p.mu.Lock()
	p.current = p.total
	p.mu.Unlock()
	p.render()
	_, _ = fmt.Fprintln(os.Stdout)
}

func (p *ProgressBar) render() {
	p.mu.Lock()
	if p.stopped {
		p.mu.Unlock()
		return
	}
	current := p.current
	total := p.total
	msg := p.message
	step := p.step
	p.mu.Unlock()

	barW := p.renderWidth(current, total, msg, step)

	var pct float64
	if total > 0 {
		pct = float64(current) / float64(total)
	}
	filled := int(pct * float64(barW))
	if filled > barW {
		filled = barW
	}
	bar := strings.Repeat("=", filled) + strings.Repeat(" ", barW-filled)

	suffix := ""
	if step != "" {
		suffix = " " + Dim(step)
	}

	line := fmt.Sprintf("%s[%s] %d/%d %s%s", clearLine, bar, current, total, msg, suffix)
	_, _ = fmt.Fprint(os.Stdout, line)
}

func (p *ProgressBar) renderWidth(current, total int, msg, step string) int {
	width, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || width <= 0 {
		return p.barW
	}

	suffix := ""
	if step != "" {
		suffix = " " + step
	}
	fixed := len(fmt.Sprintf("[] %d/%d %s%s", current, total, msg, suffix))
	barW := width - fixed - 1
	if barW < 8 {
		barW = 8
	}
	if barW > 50 {
		barW = 50
	}
	return barW
}

type statusLine struct {
	name   string
	status string
	err    error
}

// StatusLines renders a set of named items with animated status updates.
// It prints one line per item and, on a terminal, reprints in-place with spinner frames.
// Thread-safe for concurrent updates via SetStatus.
type StatusLines struct {
	lines     []statusLine
	index     map[string]int
	mu        sync.Mutex
	frame     int
	ticker    *time.Ticker
	done      chan struct{}
	started   bool
	stopped   bool
	tty       bool
	nameWidth int
}

// NewStatusLines creates a StatusLines for the given item names. All items
// start in "updating" state.
func NewStatusLines(names []string) *StatusLines {
	nameW := 0
	for _, n := range names {
		if len(n) > nameW {
			nameW = len(n)
		}
	}
	lines := make([]statusLine, len(names))
	index := make(map[string]int, len(names))
	for i, name := range names {
		lines[i] = statusLine{name: name, status: "updating"}
		index[name] = i
	}
	return &StatusLines{lines: lines, index: index, nameWidth: nameW, tty: IsTTY()}
}

// Start prints the initial state and, on a terminal, starts the spinner ticker.
func (sl *StatusLines) Start() {
	sl.mu.Lock()
	defer sl.mu.Unlock()
	if sl.started {
		return
	}
	sl.started = true
	if sl.tty {
		sl.print()
		sl.startTicker()
	}
}

// SetStatus updates the status of a named item. On a terminal it triggers
// a reprint of all lines. On non-TTY, it only updates internal state so the
// caller can produce final output after Stop().
func (sl *StatusLines) SetStatus(name, status string, err error) {
	sl.mu.Lock()
	defer sl.mu.Unlock()
	idx, ok := sl.index[name]
	if !ok {
		return
	}
	sl.lines[idx].status = status
	sl.lines[idx].err = err
	if sl.tty {
		sl.reprint()
	}
}

// Stop halts the spinner ticker, clears the animated lines, and leaves the
// terminal ready for subsequent output.
func (sl *StatusLines) Stop() {
	sl.mu.Lock()
	defer sl.mu.Unlock()
	if sl.stopped {
		return
	}
	sl.stopped = true
	if sl.ticker != nil {
		sl.ticker.Stop()
	}
	if sl.done != nil {
		close(sl.done)
	}
	if sl.tty {
		for range sl.lines {
			_, _ = fmt.Fprintf(os.Stdout, "\r\x1b[2K\x1b[A")
		}
		_, _ = fmt.Fprint(os.Stdout, "\r\x1b[2K")
	}
}

func (sl *StatusLines) print() {
	for i := range sl.lines {
		_, _ = fmt.Fprintln(os.Stdout, sl.formatLine(i))
	}
}

func (sl *StatusLines) reprint() {
	_, _ = fmt.Fprintf(os.Stdout, "\x1b[%dA", len(sl.lines))
	for i := range sl.lines {
		_, _ = fmt.Fprintf(os.Stdout, "\r\x1b[2K%s\n", sl.formatLine(i))
	}
}

// NameWidth returns the maximum name width used for padding when formatting lines.
func (sl *StatusLines) NameWidth() int {
	return sl.nameWidth
}

func (sl *StatusLines) formatLine(idx int) string {
	s := sl.lines[idx]
	name := PadRight(s.name, sl.nameWidth)
	var spinner string
	if sl.tty {
		spinner = SpinnerFrames[sl.frame%len(SpinnerFrames)]
	} else {
		spinner = " "
	}
	switch s.status {
	case "updating":
		return Detail(fmt.Sprintf("%s %s %s", Cyan(spinner), Bold(name), Cyan("updating")))
	case "updated":
		return Detail(fmt.Sprintf("  %s %s", Bold(name), Green("updated")))
	case "up-to-date":
		return Detail(Dim(fmt.Sprintf("  %s up-to-date", name)))
	case "failed":
		msg := "failed"
		if s.err != nil {
			msg = fmt.Sprintf("failed: %v", s.err)
		}
		return Detail(fmt.Sprintf("  %s %s", Bold(name), Red(msg)))
	default:
		return Detail(Dim("  " + name))
	}
}

func (sl *StatusLines) startTicker() {
	sl.done = make(chan struct{})
	sl.ticker = time.NewTicker(SpinnerInterval)
	go func() {
		for {
			select {
			case <-sl.done:
				return
			case <-sl.ticker.C:
				sl.mu.Lock()
				if sl.stopped {
					sl.mu.Unlock()
					return
				}
				anyUpdating := false
				for _, s := range sl.lines {
					if s.status == "updating" {
						anyUpdating = true
						break
					}
				}
				if anyUpdating {
					sl.frame++
					sl.reprint()
				}
				sl.mu.Unlock()
			}
		}
	}()
}
