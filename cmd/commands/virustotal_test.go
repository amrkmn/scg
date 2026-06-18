package commands

import (
	"strings"
	"testing"
)

func TestSplitVTSupportedHash(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantAlgo string
		wantRaw  string
	}{
		{name: "sha256 explicit", input: "sha256:abc123", wantAlgo: "sha256", wantRaw: "abc123"},
		{name: "md5 inferred", input: "0123456789abcdef0123456789abcdef", wantAlgo: "md5", wantRaw: "0123456789abcdef0123456789abcdef"},
		{name: "unsupported", input: "sha512:abc", wantAlgo: "sha512", wantRaw: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotAlgo, gotRaw := splitVTSupportedHash(tt.input)
			if gotAlgo != tt.wantAlgo || gotRaw != tt.wantRaw {
				t.Fatalf("splitVTSupportedHash(%q) = (%q, %q), want (%q, %q)", tt.input, gotAlgo, gotRaw, tt.wantAlgo, tt.wantRaw)
			}
		})
	}
}

func TestFormatVirusTotalFileReportLine(t *testing.T) {
	safe := formatVirusTotalFileReportLine("git", "abc123", 0, 70)
	if !strings.Contains(safe, "git") || !strings.Contains(safe, "0/70") || !strings.Contains(safe, "/gui/file/abc123") {
		t.Fatalf("safe file line missing expected content: %q", safe)
	}
	unsafe := formatVirusTotalFileReportLine("git", "abc123", 3, 70)
	if !strings.Contains(unsafe, "3/70 unsafe") {
		t.Fatalf("unsafe file line missing expected content: %q", unsafe)
	}
}

func TestFormatVirusTotalURLReportLine(t *testing.T) {
	line := formatVirusTotalURLReportLine("git", "encoded-id")
	checks := []string{"git", "URL report found", "/gui/url/encoded-id"}
	for _, want := range checks {
		if !strings.Contains(line, want) {
			t.Fatalf("URL line missing %q: %q", want, line)
		}
	}
}
