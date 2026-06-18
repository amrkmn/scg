package app

import (
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

// ConsoleLogger implements Logger by delegating to ui.Output.
type ConsoleLogger struct {
	out *ui.Output
}

// NewConsoleLogger creates a ConsoleLogger that writes to os.Stdout/os.Stderr.
func NewConsoleLogger(verbose, noColor, quiet bool) *ConsoleLogger {
	var opts []ui.OutputOption
	if verbose {
		opts = append(opts, ui.WithVerbose())
	}
	if quiet {
		opts = append(opts, ui.WithQuiet())
	}
	if noColor {
		opts = append(opts, ui.WithNoColor())
	}
	return &ConsoleLogger{out: ui.NewOutput(os.Stdout, os.Stderr, opts...)}
}

// NewConsoleLoggerWithOutput creates a ConsoleLogger using the provided Output.
func NewConsoleLoggerWithOutput(out *ui.Output) *ConsoleLogger {
	return &ConsoleLogger{out: out}
}

// Output returns the underlying ui.Output.
func (l *ConsoleLogger) Output() *ui.Output { return l.out }

func (l *ConsoleLogger) Log(msg string)                 { l.out.WriteLog(msg) }
func (l *ConsoleLogger) Info(msg string)                { l.out.WriteInfo(msg) }
func (l *ConsoleLogger) Success(msg string)             { l.out.WriteSuccess(msg) }
func (l *ConsoleLogger) Warn(msg string)                { l.out.WriteWarn(msg) }
func (l *ConsoleLogger) Error(msg string)               { l.out.WriteError(msg) }
func (l *ConsoleLogger) Verbose(msg string)             { l.out.WriteVerbose(msg) }
func (l *ConsoleLogger) Header(msg string)              { l.out.WriteHeading(msg) }
func (l *ConsoleLogger) Detail(msg string)              { l.out.WriteDetail(msg) }
func (l *ConsoleLogger) Done(subject, detail string)    { l.out.WriteDone(subject, detail) }
func (l *ConsoleLogger) Skip(subject, detail string)    { l.out.WriteSkip(subject, detail) }
func (l *ConsoleLogger) Dry(subject, detail string)     { l.out.WriteDry(subject, detail) }
func (l *ConsoleLogger) Newline()                       { l.out.WriteNewline() }
