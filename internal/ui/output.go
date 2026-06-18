package ui

import (
	"bufio"
	"fmt"
	"io"
)

// Output is a testable stream-based output writer. It wraps
// Configurable io.Writer destinations and color/no-color state.
type Output struct {
	out     io.Writer
	err     io.Writer
	verbose bool
	quiet   bool
	noColor bool
}

// OutputOption configures an Output.
type OutputOption func(*Output)

// NewOutput creates an Output that writes to the given
// stdout and stderr writers. Pass nil for os.Stdout/os.Stderr defaults.
func NewOutput(stdout, stderr io.Writer, opts ...OutputOption) *Output {
	o := &Output{out: stdout, err: stderr}
	for _, opt := range opts {
		opt(o)
	}
	return o
}

// WithVerbose enables verbose-level messages.
func WithVerbose() OutputOption { return func(o *Output) { o.verbose = true } }

// WithQuiet suppresses non-essential output.
func WithQuiet() OutputOption { return func(o *Output) { o.quiet = true } }

// WithNoColor disables ANSI color codes.
func WithNoColor() OutputOption {
	return func(o *Output) {
		o.noColor = true
		SetColorEnabled(false)
	}
}

// Out returns the configured stdout writer.
func (o *Output) Out() io.Writer { return o.out }

// Err returns the configured stderr writer.
func (o *Output) Err() io.Writer { return o.err }

// IsVerbose reports whether verbose output is enabled.
func (o *Output) IsVerbose() bool { return o.verbose }

// IsQuiet reports whether quiet mode is active.
func (o *Output) IsQuiet() bool { return o.quiet }

// WriteHeading writes a "==> Title" line to stdout.
func (o *Output) WriteHeading(title string) {
	if o.quiet {
		return
	}
	_, _ = fmt.Fprintln(o.out, Heading(title))
}

// WriteDetail writes an indented detail line to stdout.
func (o *Output) WriteDetail(text string) {
	if o.quiet {
		return
	}
	_, _ = fmt.Fprintln(o.out, Detail(text))
}

// WriteDone writes a "  ✓ subject detail" line to stdout.
func (o *Output) WriteDone(subject, detail string) {
	if o.quiet {
		return
	}
	_, _ = fmt.Fprintln(o.out, Done(subject, detail))
}

// WriteSkip writes a "  - subject detail" line to stdout.
func (o *Output) WriteSkip(subject, detail string) {
	if o.quiet {
		return
	}
	_, _ = fmt.Fprintln(o.out, Skip(subject, detail))
}

// WriteDry writes a "  ~ subject detail" line to stdout.
func (o *Output) WriteDry(subject, detail string) {
	if o.quiet {
		return
	}
	_, _ = fmt.Fprintln(o.out, Dry(subject, detail))
}

// WriteWarn writes a "  ! msg" line to stderr.
func (o *Output) WriteWarn(msg string) {
	_, _ = fmt.Fprintln(o.err, WarnLine(msg))
}

// WriteError writes a "  ✗ msg" line to stderr.
func (o *Output) WriteError(msg string) {
	_, _ = fmt.Fprintln(o.err, FailLine(msg))
}

// WriteVerbose writes a dimmed line to stdout, only when verbose is enabled.
func (o *Output) WriteVerbose(msg string) {
	if !o.verbose || o.quiet {
		return
	}
	_, _ = fmt.Fprintln(o.out, Dim(msg))
}

// WriteLog writes a plain indented line to stdout.
func (o *Output) WriteLog(msg string) {
	if o.quiet {
		return
	}
	_, _ = fmt.Fprintln(o.out, Detail(msg))
}

// WriteInfo writes a "  i msg" line to stdout.
func (o *Output) WriteInfo(msg string) {
	if o.quiet {
		return
	}
	_, _ = fmt.Fprintln(o.out, NoteLine(msg))
}

// WriteSuccess writes a "  ✓ msg" line to stdout.
func (o *Output) WriteSuccess(msg string) {
	if o.quiet {
		return
	}
	_, _ = fmt.Fprintln(o.out, Done(msg, ""))
}

// WriteSummary writes a "==> Summary" heading followed by a status line.
func (o *Output) WriteSummary(kind StatusKind, subject, detail string) {
	if o.quiet {
		return
	}
	_, _ = fmt.Fprintln(o.out, RenderStatusSummary(kind, subject, detail))
}

// WriteTable writes a table with headers, rows, weights, and optional footer.
func (o *Output) WriteTable(headers []string, rows [][]string, weights []float64, footer string) {
	if o.quiet {
		return
	}
	_, _ = fmt.Fprintln(o.out, RenderTable(headers, rows, weights, footer))
}

// WriteKeyValues writes a key/value block with optional title.
func (o *Output) WriteKeyValues(title string, pairs []KeyValue) {
	if o.quiet {
		return
	}
	_, _ = fmt.Fprintln(o.out, RenderKeyValueBlock(title, pairs))
}

// WriteRaw writes a raw string as-is to stdout (no prefix, no Quiet check).
func (o *Output) WriteRaw(text string) {
	_, _ = fmt.Fprintln(o.out, text)
}

// WriteNewline writes a blank line to stdout.
func (o *Output) WriteNewline() {
	if o.quiet {
		return
	}
	_, _ = fmt.Fprintln(o.out)
}

// Write status line with options.
func (o *Output) WriteStatus(kind StatusKind, subject, detail string, opts StatusOptions) {
	if o.quiet {
		return
	}
	switch kind {
	case StatusWarn, StatusFail:
		_, _ = fmt.Fprintln(o.err, StatusWithOptions(kind, subject, detail, opts))
	default:
		_, _ = fmt.Fprintln(o.out, StatusWithOptions(kind, subject, detail, opts))
	}
}

// WriteString writes an arbitrary string to stdout (unfiltered by Quiet).
func (o *Output) WriteString(s string) {
	_, _ = fmt.Fprint(o.out, s)
}

// FlushHint flushes a buffered writer if the underlying writer supports it.
func (o *Output) FlushHint() {
	if bw, ok := o.out.(*bufio.Writer); ok {
		_ = bw.Flush()
	}
}
