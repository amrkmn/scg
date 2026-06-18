package service

import (
	"reflect"
	"testing"

	"go.noz.one/scg/internal/scoop"
)

func TestBulkUpdateTargetsForScope(t *testing.T) {
	installed := []InstalledApp{
		{Name: "git", Scope: scoop.ScopeUser},
		{Name: "7zip", Scope: scoop.ScopeGlobal},
		{Name: "curl", Scope: scoop.ScopeUser},
	}

	userTargets := bulkUpdateTargetsForScope(installed, "")
	if got, want := appNames(userTargets), []string{"git", "curl"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("bulkUpdateTargetsForScope(user) = %v, want %v", got, want)
	}

	globalTargets := bulkUpdateTargetsForScope(installed, scoop.ScopeGlobal)
	if got, want := appNames(globalTargets), []string{"7zip"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("bulkUpdateTargetsForScope(global) = %v, want %v", got, want)
	}
}

func TestFilterBulkUpdateCandidates_SkipsHeldAndNonOutdated(t *testing.T) {
	targets := []InstalledApp{
		{Name: "git", Scope: scoop.ScopeUser},
		{Name: "7zip", Scope: scoop.ScopeUser},
		{Name: "nodejs", Scope: scoop.ScopeUser},
		{Name: "python", Scope: scoop.ScopeUser},
	}

	statusResults := []AppStatusResult{
		{Name: "git", Scope: scoop.ScopeUser, Installed: "1.0", Latest: "2.0", Outdated: true},
		{Name: "7zip", Scope: scoop.ScopeUser, Installed: "1.0", Latest: "2.0", Outdated: true, Held: true},
		{Name: "nodejs", Scope: scoop.ScopeUser, Failed: true},
		{Name: "python", Scope: scoop.ScopeUser, MissingDeps: []string{"vcredist"}},
	}

	filtered, held := filterBulkUpdateCandidates(targets, updateNeedsAttentionMap(statusResults, false))
	if got, want := appNames(filtered), []string{"git"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("filterBulkUpdateCandidates() filtered = %v, want %v", got, want)
	}
	if len(held) != 1 || held[0].Name != "7zip" {
		t.Fatalf("filterBulkUpdateCandidates() held = %#v, want only 7zip", held)
	}
	for _, app := range filtered {
		if app.Name == "nodejs" || app.Name == "python" {
			t.Fatalf("unexpected repair-style bulk update candidate: %s", app.Name)
		}
	}
	for _, sr := range held {
		if sr.Name == "nodejs" || sr.Name == "python" {
			t.Fatalf("unexpected held result: %#v", sr)
		}
	}
}

func appNames(apps []InstalledApp) []string {
	names := make([]string, 0, len(apps))
	for _, app := range apps {
		names = append(names, app.Name)
	}
	return names
}
