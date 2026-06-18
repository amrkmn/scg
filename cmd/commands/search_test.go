package commands

import (
	"strings"
	"testing"

	"go.noz.one/scg/internal/service"
)

func TestRenderSearchResults(t *testing.T) {
	results := []service.SearchResult{
		{Name: "ripgrep", Version: "14.0.0", Bucket: "extras", Description: "fast grep", Binaries: []string{"rg.exe"}},
		{Name: "git", Version: "2.45.0", Bucket: "main", IsInstalled: true, Description: "distributed vcs", Binaries: []string{"git.exe", "gitk.exe"}},
		{Name: "gh", Version: "2.0.0", Bucket: "main"},
	}

	rendered := renderSearchResults(results, true)
	checks := []string{
		"==> Search results",
		"extras:",
		"main:",
		"Package",
		"Version",
		"Status",
		"Details",
		"git",
		"installed",
		"distributed vcs | bin: git.exe, gitk.exe",
		"fast grep | bin: rg.exe",
		"3 package(s)",
	}
	for _, want := range checks {
		if !strings.Contains(rendered, want) {
			t.Fatalf("renderSearchResults() missing %q\nrendered:\n%s", want, rendered)
		}
	}
	if strings.Index(rendered, "git") > strings.Index(rendered, "gh") {
		// installed entries should sort ahead of non-installed entries within a bucket.
		t.Fatalf("renderSearchResults() did not keep installed entries first\nrendered:\n%s", rendered)
	}
}
