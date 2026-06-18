package ui

import (
	"fmt"
	"strings"
)

type StatusKind string

const (
	StatusRunning StatusKind = "running"
	StatusDone    StatusKind = "done"
	StatusSkip    StatusKind = "skip"
	StatusWarn    StatusKind = "warn"
	StatusFail    StatusKind = "fail"
	StatusDry     StatusKind = "dry"
	StatusNote    StatusKind = "note"
)

type StatusOptions struct {
	ASCII bool
}

func Heading(title string) string {
	return fmt.Sprintf("%s %s", BoldCyan("==>"), Bold(title))
}

func Detail(text string) string {
	return "  " + text
}

func Status(kind, subject, detail string) string {
	return StatusWithOptions(StatusKind(kind), subject, detail, StatusOptions{})
}

func StatusWithOptions(kind StatusKind, subject, detail string, opts StatusOptions) string {
	parts := []string{}
	if subject != "" {
		parts = append(parts, subject)
	}
	if detail != "" {
		parts = append(parts, detail)
	}
	text := strings.Join(parts, " ")
	if text == "" {
		return Detail(colorStatusSymbol(kind, opts.ASCII))
	}
	return Detail(fmt.Sprintf("%s %s", colorStatusSymbol(kind, opts.ASCII), text))
}

func Done(subject, detail string) string { return Status("done", subject, detail) }
func Skip(subject, detail string) string { return Status("skip", subject, detail) }
func Dry(subject, detail string) string  { return Status("dry", subject, detail) }
func WarnLine(msg string) string         { return Status("warn", msg, "") }
func FailLine(msg string) string         { return Status("fail", msg, "") }
func NoteLine(msg string) string         { return Status("note", msg, "") }

func VersionChange(name, oldVersion, newVersion string) string {
	return fmt.Sprintf("%s %s %s %s", BoldCyan(name), Dim(oldVersion), Dim("->"), Green(newVersion))
}

func JoinList(items []string) string {
	switch len(items) {
	case 0:
		return ""
	case 1:
		return items[0]
	case 2:
		return items[0] + " and " + items[1]
	default:
		return strings.Join(items[:len(items)-1], ", ") + " and " + items[len(items)-1]
	}
}

func StatusSymbol(kind StatusKind, ascii bool) string {
	switch kind {
	case StatusRunning:
		if ascii {
			return ">"
		}
		return "•"
	case StatusDone:
		if ascii {
			return "+"
		}
		return "✓"
	case StatusSkip:
		return "-"
	case StatusWarn:
		return "!"
	case StatusFail:
		if ascii {
			return "x"
		}
		return "✗"
	case StatusDry:
		return "~"
	case StatusNote:
		return "i"
	default:
		return "-"
	}
}

func colorStatusSymbol(kind StatusKind, ascii bool) string {
	symbol := StatusSymbol(kind, ascii)
	switch kind {
	case StatusDone:
		return Green(symbol)
	case StatusWarn, StatusDry:
		return Yellow(symbol)
	case StatusFail:
		return Red(symbol)
	case StatusSkip:
		return Dim(symbol)
	case StatusNote, StatusRunning:
		return Cyan(symbol)
	default:
		return symbol
	}
}
