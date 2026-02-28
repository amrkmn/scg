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
	fmt.Fprintln(os.Stdout, msg)
}

func (l *ConsoleLogger) Info(msg string) {
	fmt.Fprintln(os.Stdout, ui.Blue(msg))
}

func (l *ConsoleLogger) Success(msg string) {
	fmt.Fprintln(os.Stdout, ui.Green(msg))
}

func (l *ConsoleLogger) Warn(msg string) {
	fmt.Fprintln(os.Stderr, ui.Yellow(msg))
}

func (l *ConsoleLogger) Error(msg string) {
	fmt.Fprintln(os.Stderr, ui.Red(msg))
}

func (l *ConsoleLogger) Verbose(msg string) {
	if l.verbose {
		fmt.Fprintln(os.Stdout, ui.Dim(msg))
	}
}

func (l *ConsoleLogger) Header(msg string) {
	fmt.Fprintln(os.Stdout, ui.BoldCyan(msg))
}

func (l *ConsoleLogger) Newline() {
	fmt.Fprintln(os.Stdout)
}
