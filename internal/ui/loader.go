package ui

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

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

var spinnerFrames = []string{"   ", ".  ", ".. ", "..."}

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
	s.ticker = time.NewTicker(150 * time.Millisecond)
	go func() {
		for {
			select {
			case <-s.done:
				return
			case <-s.ticker.C:
				s.mu.Lock()
				f := spinnerFrames[s.frame%len(spinnerFrames)]
				s.frame++
				msg := s.message
				s.mu.Unlock()
				fmt.Fprintf(os.Stdout, "%s%s%s", clearLine, msg, f)
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
	fmt.Fprint(os.Stdout, clearLine)
}

// Succeed stops the spinner and prints a success message.
func (s *Spinner) Succeed(msg string) {
	s.stop()
	if msg != "" {
		fmt.Fprintln(os.Stdout, Green("✓")+" "+msg)
	}
}

// Fail stops the spinner and prints a failure message.
func (s *Spinner) Fail(msg string) {
	s.stop()
	if msg != "" {
		fmt.Fprintln(os.Stdout, Red("✗")+" "+msg)
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
	fmt.Fprint(os.Stdout, clearLine)
}

// Complete prints the bar at 100% and moves to a new line.
func (p *ProgressBar) Complete() {
	p.mu.Lock()
	p.current = p.total
	p.mu.Unlock()
	p.render()
	fmt.Fprintln(os.Stdout)
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
	barW := p.barW
	p.mu.Unlock()

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
	fmt.Fprint(os.Stdout, line)
}
