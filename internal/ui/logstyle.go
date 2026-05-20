package ui

import (
	"fmt"
	"strings"
)

func Heading(title string) string {
	return fmt.Sprintf("%s %s", BoldCyan("==>"), Bold(title))
}

func Detail(text string) string {
	return "  " + text
}

func Status(kind, subject, detail string) string {
	parts := []string{}
	if subject != "" {
		parts = append(parts, subject)
	}
	if detail != "" {
		parts = append(parts, detail)
	}
	return Detail(colorStatus(kind, strings.Join(parts, " ")))
}

func Done(subject, detail string) string { return Status("done", subject, detail) }
func Skip(subject, detail string) string { return Status("skip", subject, detail) }
func Dry(subject, detail string) string  { return Status("dry", subject, detail) }
func WarnLine(msg string) string         { return Status("warn", msg, "") }
func FailLine(msg string) string         { return Status("fail", msg, "") }
func NoteLine(msg string) string         { return Status("note", msg, "") }

func VersionChange(name, oldVersion, newVersion string) string {
	return fmt.Sprintf("%s %s -> %s", Cyan(name), oldVersion, newVersion)
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

func colorStatus(kind, text string) string {
	switch kind {
	case "done", "ok":
		return Green(text)
	case "warn":
		return Yellow(text)
	case "fail", "error":
		return Red(text)
	case "dry":
		return Yellow(text)
	case "skip", "note":
		return Dim(text)
	default:
		return Dim(text)
	}
}
