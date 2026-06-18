package commands

import (
	"strings"
	"testing"
	"time"

	"go.noz.one/scg/internal/scoop"
	"go.noz.one/scg/internal/service"
	"go.noz.one/scg/internal/ui"
)

func TestBuildInfoPairs_SortsDeterministicFields(t *testing.T) {
	fields := service.InfoFields{
		Name:             "git",
		InstalledVersion: "2.44.0",
		LatestVersion:    "2.45.0",
		UpdateAvailable:  true,
		Homepage:         "https://git-scm.com",
		License:          "GPL-2.0-only",
		Deprecated:       true,
		ReplacedBy:       "git-with-openssh",
		InstallDate:      time.Date(2026, 6, 17, 12, 0, 0, 0, time.UTC),
	}
	m := &scoop.Manifest{
		Architecture: map[string]any{"arm64": map[string]any{}, "64bit": map[string]any{}},
		Depends:      []any{"lessmsi", "7zip"},
		Suggest:      map[string]any{"delta": "delta", "openssh": "openssh"},
		Bin:          []any{"git.exe", "gitk.exe"},
		EnvAddPath:   []any{"cmd", "usr\\bin"},
		EnvSet:       map[string]string{"B": "2", "A": "1"},
		Persist:      []any{"etc", "home"},
		Notes:        []any{"note one", "note two"},
		Comments:     []any{"comment one"},
	}
	installed := &service.FoundManifest{Bucket: "main", Scope: scoop.ScopeUser}

	pairs := buildInfoPairs(fields, m, installed, nil, false)
	block := ui.RenderKeyValueBlock("", pairs)

	checks := []string{
		"Name", "Version", "Homepage", "License", "Installed", "Bucket",
		"Architecture", "64bit, arm64",
		"Suggestions", "delta, openssh",
		"Environment", "A=1, B=2",
		"DEPRECATED", "git-with-openssh",
		"Install date", "2026-06-17 12:00:00",
	}
	for _, want := range checks {
		if !strings.Contains(block, want) {
			t.Fatalf("info block missing %q\n%s", want, block)
		}
	}
}
