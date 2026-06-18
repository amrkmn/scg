package ui

import (
	"bytes"
	"strings"
	"testing"
)

func TestOutputHeading(t *testing.T) {
	SetColorEnabled(false)
	t.Cleanup(func() { SetColorEnabled(true) })

	var buf bytes.Buffer
	out := NewOutput(&buf, nil)
	out.WriteHeading("Installing git 2.45.2 [64bit] from main")

	got := strings.TrimRight(buf.String(), "\r\n")
	if got != "==> Installing git 2.45.2 [64bit] from main" {
		t.Fatalf("WriteHeading() = %q", got)
	}
}

func TestOutputDone(t *testing.T) {
	SetColorEnabled(false)
	t.Cleanup(func() { SetColorEnabled(true) })

	var buf bytes.Buffer
	out := NewOutput(&buf, nil)
	out.WriteDone("cache", "git-2.45.2.7z")

	got := strings.TrimRight(buf.String(), "\r\n")
	if got != "  ✓ cache git-2.45.2.7z" {
		t.Fatalf("WriteDone() = %q", got)
	}
}

func TestOutputSkip(t *testing.T) {
	SetColorEnabled(false)
	t.Cleanup(func() { SetColorEnabled(true) })

	var buf bytes.Buffer
	out := NewOutput(&buf, nil)
	out.WriteSkip("app", "already installed")

	got := strings.TrimRight(buf.String(), "\r\n")
	if got != "  - app already installed" {
		t.Fatalf("WriteSkip() = %q", got)
	}
}

func TestOutputDry(t *testing.T) {
	SetColorEnabled(false)
	t.Cleanup(func() { SetColorEnabled(true) })

	var buf bytes.Buffer
	out := NewOutput(&buf, nil)
	out.WriteDry("update", "3 would update")

	got := strings.TrimRight(buf.String(), "\r\n")
	if got != "  ~ update 3 would update" {
		t.Fatalf("WriteDry() = %q", got)
	}
}

func TestOutputWarn(t *testing.T) {
	SetColorEnabled(false)
	t.Cleanup(func() { SetColorEnabled(true) })

	var stderr bytes.Buffer
	out := NewOutput(nil, &stderr)
	out.WriteWarn("aria2 failed; falling back to HTTP")

	got := strings.TrimRight(stderr.String(), "\r\n")
	if got != "  ! aria2 failed; falling back to HTTP" {
		t.Fatalf("WriteWarn() = %q", got)
	}
}

func TestOutputError(t *testing.T) {
	SetColorEnabled(false)
	t.Cleanup(func() { SetColorEnabled(true) })

	var stderr bytes.Buffer
	out := NewOutput(nil, &stderr)
	out.WriteError("hash verification failed for git.7z")

	got := strings.TrimRight(stderr.String(), "\r\n")
	if got != "  ✗ hash verification failed for git.7z" {
		t.Fatalf("WriteError() = %q", got)
	}
}

func TestOutputVerbose(t *testing.T) {
	SetColorEnabled(false)
	t.Cleanup(func() { SetColorEnabled(true) })

	var buf bytes.Buffer
	out := NewOutput(&buf, nil, WithVerbose())
	out.WriteVerbose("duration: 1.234s")

	got := strings.TrimRight(buf.String(), "\r\n")
	if got != "duration: 1.234s" {
		t.Fatalf("WriteVerbose() = %q", got)
	}
}

func TestOutputVerboseSuppressed(t *testing.T) {
	SetColorEnabled(false)
	t.Cleanup(func() { SetColorEnabled(true) })

	var buf bytes.Buffer
	out := NewOutput(&buf, nil) // no WithVerbose
	out.WriteVerbose("should not appear")

	got := strings.TrimRight(buf.String(), "\r\n")
	if got != "" {
		t.Fatalf("WriteVerbose() without verbose = %q, want empty", got)
	}
}

func TestOutputQuiet(t *testing.T) {
	SetColorEnabled(false)
	t.Cleanup(func() { SetColorEnabled(true) })

	var buf, stderr bytes.Buffer
	out := NewOutput(&buf, &stderr, WithQuiet())

	out.WriteHeading("Should not appear")
	out.WriteDone("done", "should not appear")
	out.WriteSkip("skip", "should not appear")
	out.WriteDry("dry", "should not appear")
	out.WriteDetail("should not appear")
	out.WriteLog("should not appear")
	out.WriteInfo("should not appear")
	out.WriteSuccess("should not appear")
	out.WriteSummary(StatusDone, "done", "should not appear")
	out.WriteVerbose("should not appear")

	if buf.Len() != 0 {
		t.Fatalf("quiet mode should suppress stdout, got %q", buf.String())
	}

	// Warnings and errors should still appear even in quiet mode
	out.WriteWarn("should appear")
	if stderr.Len() == 0 {
		t.Fatal("quiet mode should not suppress stderr warnings")
	}
}

func TestOutputSummary(t *testing.T) {
	SetColorEnabled(false)
	t.Cleanup(func() { SetColorEnabled(true) })

	var buf bytes.Buffer
	out := NewOutput(&buf, nil)
	out.WriteSummary(StatusDone, "install", "2 installed, 1 skipped")

	lines := strings.Split(strings.TrimRight(buf.String(), "\r\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("WriteSummary() lines = %d, want 2", len(lines))
	}
	if lines[0] != "==> Summary" {
		t.Fatalf("heading = %q", lines[0])
	}
	if lines[1] != "  ✓ install 2 installed, 1 skipped" {
		t.Fatalf("summary line = %q", lines[1])
	}
}

func TestOutputTable(t *testing.T) {
	SetColorEnabled(false)
	t.Cleanup(func() { SetColorEnabled(true) })

	var buf bytes.Buffer
	out := NewOutput(&buf, nil)
	out.WriteTable(
		[]string{"Name", "Version"},
		[][]string{{"git", "2.45.2"}, {"nodejs", "22.4.0"}},
		[]float64{1, 2},
		"2 app(s) installed",
	)

	lines := strings.Split(strings.TrimRight(buf.String(), "\r\n"), "\n")
	if len(lines) < 4 {
		t.Fatalf("WriteTable() lines = %d, want at least 4", len(lines))
	}
	if !strings.Contains(lines[0], "Name") || !strings.Contains(lines[0], "Version") {
		t.Fatalf("header line = %q", lines[0])
	}
	if lines[len(lines)-1] != "2 app(s) installed" {
		t.Fatalf("footer = %q", lines[len(lines)-1])
	}
}

func TestOutputKeyValues(t *testing.T) {
	SetColorEnabled(false)
	t.Cleanup(func() { SetColorEnabled(true) })

	var buf bytes.Buffer
	out := NewOutput(&buf, nil)
	out.WriteKeyValues("App git", []KeyValue{
		{Key: "Name", Value: "git"},
		{Key: "Version", Value: "2.45.2"},
		{Key: "Bucket", Value: "main"},
	})

	lines := strings.Split(strings.TrimRight(buf.String(), "\r\n"), "\n")
	if len(lines) != 4 {
		t.Fatalf("WriteKeyValues() lines = %d, want 4", len(lines))
	}
	if lines[0] != "==> App git" {
		t.Fatalf("heading = %q", lines[0])
	}
	if !strings.Contains(lines[1], "Name") || !strings.Contains(lines[1], " : git") {
		t.Fatalf("first pair = %q", lines[1])
	}
}

func TestOutputDetail(t *testing.T) {
	SetColorEnabled(false)
	t.Cleanup(func() { SetColorEnabled(true) })

	var buf bytes.Buffer
	out := NewOutput(&buf, nil)
	out.WriteDetail("C:\\Users\\User\\scoop\\apps\\git\\2.45.2")

	got := strings.TrimRight(buf.String(), "\r\n")
	if !strings.HasPrefix(got, "  C:") {
		t.Fatalf("WriteDetail() = %q", got)
	}
}

func TestOutputNewline(t *testing.T) {
	SetColorEnabled(false)
	t.Cleanup(func() { SetColorEnabled(true) })

	var buf bytes.Buffer
	out := NewOutput(&buf, nil)
	out.WriteNewline()

	got := strings.TrimRight(buf.String(), "\r\n")
	if got != "" {
		t.Fatalf("WriteNewline() = %q, want empty line", got)
	}
	if !strings.HasSuffix(buf.String(), "\n") {
		t.Fatal("WriteNewline() should end with newline")
	}
}

func TestOutputRaw(t *testing.T) {
	SetColorEnabled(false)
	t.Cleanup(func() { SetColorEnabled(true) })

	var buf bytes.Buffer
	out := NewOutput(&buf, nil)
	out.WriteRaw("v0.1.0")

	got := strings.TrimRight(buf.String(), "\r\n")
	if got != "v0.1.0" {
		t.Fatalf("WriteRaw() = %q", got)
	}
}

func TestOutputRawSurvivesQuiet(t *testing.T) {
	SetColorEnabled(false)
	t.Cleanup(func() { SetColorEnabled(true) })

	var buf bytes.Buffer
	out := NewOutput(&buf, nil, WithQuiet())
	out.WriteRaw("v0.1.0")

	got := strings.TrimRight(buf.String(), "\r\n")
	if got != "v0.1.0" {
		t.Fatalf("WriteRaw() in quiet mode = %q", got)
	}
}

func TestOutputWithNoColor(t *testing.T) {
	var buf bytes.Buffer
	out := NewOutput(&buf, nil, WithNoColor())
	out.WriteHeading("Test")

	got := strings.TrimRight(buf.String(), "\r\n")
	if got != "==> Test" {
		t.Fatalf("WriteHeading() with NoColor = %q", got)
	}
	if strings.Contains(got, "\x1b") {
		t.Fatalf("WriteHeading() with NoColor contains escape codes: %q", got)
	}

	t.Cleanup(func() { SetColorEnabled(true) })
}
