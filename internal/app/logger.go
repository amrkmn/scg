package app

import (
	"fmt"
	"os"

	"go.noz.one/scg/internal/ui"
)

// Logger defines the logging interface used throughout scg.
type Logger interface {
	Log(msg string)
	Info(msg string)
	Success(msg string)
	Warn(msg string)
	Error(msg string)
	Verbose(msg string)
	Header(msg string)
	Detail(msg string)
	Done(subject, detail string)
	Skip(subject, detail string)
	Dry(subject, detail string)
	Newline()
}

// ConsoleLogger implements Logger by writing to stdout/stderr with colour.
type ConsoleLogger struct {
	verbose bool
}

// NewConsoleLogger creates a ConsoleLogger. When verbose is false, Verbose() is a no-op.
func NewConsoleLogger(verbose bool) *ConsoleLogger {
	return &ConsoleLogger{verbose: verbose}
}

func (l *ConsoleLogger) Log(msg string) {
	_, _ = fmt.Fprintln(os.Stdout, ui.Detail(msg))
}

func (l *ConsoleLogger) Info(msg string) {
	_, _ = fmt.Fprintln(os.Stdout, ui.NoteLine(msg))
}

func (l *ConsoleLogger) Success(msg string) {
	_, _ = fmt.Fprintln(os.Stdout, ui.Done(msg, ""))
}

func (l *ConsoleLogger) Warn(msg string) {
	_, _ = fmt.Fprintln(os.Stderr, ui.WarnLine(msg))
}

func (l *ConsoleLogger) Error(msg string) {
	_, _ = fmt.Fprintln(os.Stderr, ui.FailLine(msg))
}

func (l *ConsoleLogger) Verbose(msg string) {
	if l.verbose {
		_, _ = fmt.Fprintln(os.Stdout, ui.Dim(msg))
	}
}

func (l *ConsoleLogger) Header(msg string) {
	_, _ = fmt.Fprintln(os.Stdout, ui.Heading(msg))
}

func (l *ConsoleLogger) Detail(msg string) {
	_, _ = fmt.Fprintln(os.Stdout, ui.Detail(msg))
}

func (l *ConsoleLogger) Done(subject, detail string) {
	_, _ = fmt.Fprintln(os.Stdout, ui.Done(subject, detail))
}

func (l *ConsoleLogger) Skip(subject, detail string) {
	_, _ = fmt.Fprintln(os.Stdout, ui.Skip(subject, detail))
}

func (l *ConsoleLogger) Dry(subject, detail string) {
	_, _ = fmt.Fprintln(os.Stdout, ui.Dry(subject, detail))
}

func (l *ConsoleLogger) Newline() {
	_, _ = fmt.Fprintln(os.Stdout)
}
