package service

import "testing"

func TestParseArchEnvSet(t *testing.T) {
	arch := map[string]any{
		"64bit": map[string]any{
			"env_set": map[string]any{
				"FOO": "bar",
				"NUM": 1,
			},
		},
	}
	got := parseArchEnvSet(arch, "64bit")
	if got["FOO"] != "bar" {
		t.Fatalf("FOO = %q", got["FOO"])
	}
	if _, ok := got["NUM"]; ok {
		t.Fatalf("expected non-string entry to be skipped")
	}
}

func TestMergeEnvSet(t *testing.T) {
	base := map[string]string{"A": "1", "B": "2"}
	arch := map[string]string{"B": "3", "C": "4"}
	got := mergeEnvSet(base, arch)
	if got["A"] != "1" || got["B"] != "3" || got["C"] != "4" {
		t.Fatalf("unexpected merge: %#v", got)
	}
}
