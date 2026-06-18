package commands

import (
	"testing"

	"go.noz.one/scg/internal/scoop"
	"go.noz.one/scg/internal/service"
)

func TestCacheRemoveSummaryDetail(t *testing.T) {
	result := service.CacheResult{FilesRemoved: 3, BytesFreed: 1024}

	if got := cacheRemoveSummaryDetail(result, false); got != "3 file(s), 1.00 KB freed" {
		t.Fatalf("cacheRemoveSummaryDetail() = %q", got)
	}

	if got := cacheRemoveSummaryDetail(result, true); got != "3 file(s), 1.00 KB would be freed" {
		t.Fatalf("cacheRemoveSummaryDetail() dry-run = %q", got)
	}
}

func TestFormatHoldHeading(t *testing.T) {
	if got := formatHoldHeading("Holding", "git", scoop.ScopeUser); got != "Holding git" {
		t.Fatalf("formatHoldHeading() user = %q", got)
	}

	if got := formatHoldHeading("Holding", "git", scoop.ScopeGlobal); got != "Holding git [global]" {
		t.Fatalf("formatHoldHeading() global = %q", got)
	}
}

func TestCacheRemoveHeading(t *testing.T) {
	if got := cacheRemoveHeading(scoop.ScopeUser); got != "Removing cache" {
		t.Fatalf("cacheRemoveHeading() user = %q", got)
	}

	if got := cacheRemoveHeading(scoop.ScopeGlobal); got != "Removing cache [global]" {
		t.Fatalf("cacheRemoveHeading() global = %q", got)
	}
}

func TestJoinStrings(t *testing.T) {
	if got := joinStrings([]string{"one", "two", "three"}); got != "one, two, three" {
		t.Fatalf("joinStrings() = %q", got)
	}
}

func TestFormatInstallHeading(t *testing.T) {
	if got := formatInstallHeading([]string{"git"}, scoop.ScopeUser); got != "Installing git" {
		t.Fatalf("single user = %q", got)
	}
	if got := formatInstallHeading([]string{"git"}, scoop.ScopeGlobal); got != "Installing git [global]" {
		t.Fatalf("single global = %q", got)
	}
	if got := formatInstallHeading([]string{"git", "nodejs"}, scoop.ScopeUser); got != "Installing 2 apps" {
		t.Fatalf("multi user = %q", got)
	}
}

func TestFormatUpdateHeading(t *testing.T) {
	if got := formatUpdateHeading([]string{"git"}, scoop.ScopeUser); got != "Updating git" {
		t.Fatalf("single user = %q", got)
	}
	if got := formatUpdateHeading([]string{"git"}, scoop.ScopeGlobal); got != "Updating git [global]" {
		t.Fatalf("single global = %q", got)
	}
	if got := formatUpdateHeading([]string{}, scoop.ScopeUser); got != "Updating all apps" {
		t.Fatalf("all user = %q", got)
	}
	if got := formatUpdateHeading([]string{"git", "nodejs"}, scoop.ScopeUser); got != "Updating 2 apps" {
		t.Fatalf("multi user = %q", got)
	}
}

func TestFormatUninstallHeading(t *testing.T) {
	if got := formatUninstallHeading([]string{"git"}, scoop.ScopeUser); got != "Uninstalling git" {
		t.Fatalf("single user = %q", got)
	}
	if got := formatUninstallHeading([]string{"git"}, scoop.ScopeGlobal); got != "Uninstalling git [global]" {
		t.Fatalf("single global = %q", got)
	}
	if got := formatUninstallHeading([]string{"git", "nodejs"}, scoop.ScopeUser); got != "Uninstalling 2 apps" {
		t.Fatalf("multi user = %q", got)
	}
}

func TestFormatResetHeading(t *testing.T) {
	if got := formatResetHeading([]string{"git"}, scoop.ScopeUser, false); got != "Resetting git" {
		t.Fatalf("single user = %q", got)
	}
	if got := formatResetHeading([]string{"git"}, scoop.ScopeGlobal, false); got != "Resetting git [global]" {
		t.Fatalf("single global = %q", got)
	}
	if got := formatResetHeading(nil, scoop.ScopeUser, true); got != "Resetting all apps" {
		t.Fatalf("all user = %q", got)
	}
	if got := formatResetHeading([]string{"git", "nodejs"}, scoop.ScopeUser, false); got != "Resetting 2 apps" {
		t.Fatalf("multi user = %q", got)
	}
}

func TestFormatDownloadHeading(t *testing.T) {
	if got := formatDownloadHeading([]string{"git"}, scoop.ScopeUser); got != "Downloading git" {
		t.Fatalf("single user = %q", got)
	}
	if got := formatDownloadHeading([]string{"git"}, scoop.ScopeGlobal); got != "Downloading git [global]" {
		t.Fatalf("single global = %q", got)
	}
	if got := formatDownloadHeading([]string{"git", "nodejs"}, scoop.ScopeUser); got != "Downloading 2 apps" {
		t.Fatalf("multi user = %q", got)
	}
}
