package ui

import (
	"strings"
	"testing"
)

func TestStatusSymbol(t *testing.T) {
	tests := []struct {
		name  string
		kind  StatusKind
		ascii bool
		want  string
	}{
		{name: "running unicode", kind: StatusRunning, want: "•"},
		{name: "running ascii", kind: StatusRunning, ascii: true, want: ">"},
		{name: "done unicode", kind: StatusDone, want: "✓"},
		{name: "done ascii", kind: StatusDone, ascii: true, want: "+"},
		{name: "skip", kind: StatusSkip, want: "-"},
		{name: "warn", kind: StatusWarn, want: "!"},
		{name: "fail unicode", kind: StatusFail, want: "✗"},
		{name: "fail ascii", kind: StatusFail, ascii: true, want: "x"},
		{name: "dry", kind: StatusDry, want: "~"},
		{name: "note", kind: StatusNote, want: "i"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := StatusSymbol(tt.kind, tt.ascii); got != tt.want {
				t.Fatalf("StatusSymbol() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestStatusWithOptionsASCII(t *testing.T) {
	SetColorEnabled(false)
	t.Cleanup(func() { SetColorEnabled(true) })

	got := StatusWithOptions(StatusDone, "cache", "git.7z", StatusOptions{ASCII: true})
	if got != "  + cache git.7z" {
		t.Fatalf("StatusWithOptions() = %q", got)
	}
}

func TestRenderTableAlignment(t *testing.T) {
	SetColorEnabled(false)
	t.Cleanup(func() { SetColorEnabled(true) })

	got := RenderTable(
		[]string{"Name", "Source"},
		[][]string{{"main", "https://example.com/main"}, {"extras", "https://example.com/extras"}},
		[]float64{1, 3},
		"2 bucket(s) installed",
	)

	lines := strings.Split(got, "\n")
	if len(lines) < 4 {
		t.Fatalf("RenderTable() lines = %d, want at least 4", len(lines))
	}
	if !strings.Contains(lines[0], "Name") || !strings.Contains(lines[0], "Source") {
		t.Fatalf("header line = %q", lines[0])
	}
	if strings.Index(lines[1], "https://") <= strings.Index(lines[1], "main") {
		t.Fatalf("row alignment incorrect: %q", lines[1])
	}
	if lines[len(lines)-1] != "2 bucket(s) installed" {
		t.Fatalf("footer = %q", lines[len(lines)-1])
	}
	if lines[len(lines)-2] != "" {
		t.Fatalf("expected blank line before footer, got %q", lines[len(lines)-2])
	}
}

func TestRenderKeyValueBlock(t *testing.T) {
	SetColorEnabled(false)
	t.Cleanup(func() { SetColorEnabled(true) })

	got := RenderKeyValueBlock("Config", []KeyValue{{Key: "proxy", Value: "http://localhost:8080"}, {Key: "use_external_7zip", Value: "true"}})
	lines := strings.Split(got, "\n")
	if len(lines) != 3 {
		t.Fatalf("RenderKeyValueBlock() lines = %d, want 3", len(lines))
	}
	if lines[0] != "==> Config" {
		t.Fatalf("heading = %q", lines[0])
	}
	if !strings.Contains(lines[1], "proxy") || !strings.Contains(lines[1], " : http://localhost:8080") {
		t.Fatalf("first pair = %q", lines[1])
	}
	if !strings.Contains(lines[2], "use_external_7zip") || !strings.Contains(lines[2], " : true") {
		t.Fatalf("second pair = %q", lines[2])
	}
	if strings.Index(lines[1], ":") != strings.Index(lines[2], ":") {
		t.Fatalf("expected aligned separators: %q / %q", lines[1], lines[2])
	}
}

func TestRenderSummary(t *testing.T) {
	SetColorEnabled(false)
	t.Cleanup(func() { SetColorEnabled(true) })

	got := RenderSummary(StatusWithOptions(StatusDone, "bucket", "2 installed", StatusOptions{ASCII: true}))
	lines := strings.Split(got, "\n")
	if len(lines) != 2 {
		t.Fatalf("RenderSummary() lines = %d, want 2", len(lines))
	}
	if lines[0] != "==> Summary" {
		t.Fatalf("heading = %q", lines[0])
	}
	if lines[1] != "  + bucket 2 installed" {
		t.Fatalf("summary line = %q", lines[1])
	}
}

func TestRenderStatusSummary(t *testing.T) {
	SetColorEnabled(false)
	t.Cleanup(func() { SetColorEnabled(true) })

	got := RenderStatusSummary(StatusDry, "cleanup", "2 old version(s), 10 MB would be freed")
	lines := strings.Split(got, "\n")
	if len(lines) != 2 {
		t.Fatalf("RenderStatusSummary() lines = %d, want 2", len(lines))
	}
	if lines[0] != "==> Summary" {
		t.Fatalf("heading = %q", lines[0])
	}
	if lines[1] != "  ~ cleanup 2 old version(s), 10 MB would be freed" {
		t.Fatalf("summary line = %q", lines[1])
	}
}
