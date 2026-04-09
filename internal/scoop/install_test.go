package scoop

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestReadManifest(t *testing.T) {
	tmpDir := t.TempDir()
	manifest := map[string]any{
		"version":     "2.43.0",
		"description": "Git for Windows",
		"homepage":    "https://git-scm.com",
		"bin":         "cmd/git.exe",
		"depends":     "7zip",
	}

	data, _ := json.Marshal(manifest)
	path := filepath.Join(tmpDir, "git.json")
	os.WriteFile(path, data, 0o644)

	m, err := ReadManifest(path)
	if err != nil {
		t.Fatalf("ReadManifest error: %v", err)
	}
	if m.Version != "2.43.0" {
		t.Errorf("Version = %q, want %q", m.Version, "2.43.0")
	}
	if m.Description != "Git for Windows" {
		t.Errorf("Description = %q, want %q", m.Description, "Git for Windows")
	}
}

func TestReadManifestNotFound(t *testing.T) {
	_, err := ReadManifest("nonexistent.json")
	if err == nil {
		t.Error("ReadManifest should return error for nonexistent file")
	}
}

func TestGetDependencies(t *testing.T) {
	tests := []struct {
		name  string
		input any
		want  []string
	}{
		{"nil", nil, nil},
		{"string", "7zip", []string{"7zip"}},
		{"array", []any{"7zip", "git"}, []string{"7zip", "git"}},
		{"array with non-strings", []any{"7zip", 123, "git"}, []string{"7zip", "git"}},
		{"empty array", []any{}, nil},
		{"other type", 42, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetDependencies(tt.input)
			if len(got) != len(tt.want) {
				t.Fatalf("GetDependencies() = %v, want %v", got, tt.want)
			}
			for i, v := range got {
				if v != tt.want[i] {
					t.Errorf("result[%d] = %q, want %q", i, v, tt.want[i])
				}
			}
		})
	}
}

func TestReadInstallInfo(t *testing.T) {
	tmpDir := t.TempDir()
	info := InstallInfo{
		Architecture: "64bit",
		Bucket:       "main",
		URL:          "https://github.com/ScoopInstaller/Main/blob/master/bucket/git.json",
	}

	data, _ := json.MarshalIndent(info, "", "  ")
	path := filepath.Join(tmpDir, "install.json")
	os.WriteFile(path, data, 0o644)

	read, err := ReadInstallInfo(path)
	if err != nil {
		t.Fatalf("ReadInstallInfo error: %v", err)
	}
	if read.Architecture != "64bit" {
		t.Errorf("Architecture = %q, want %q", read.Architecture, "64bit")
	}
}

func TestBucketFromURL(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{`C:\Users\test\scoop\buckets\main\bucket\git.json`, "main"},
		{`C:\Users\test\scoop\buckets\extras\bucket\7zip.json`, "extras"},
		{"https://github.com/ScoopInstaller/Main/blob/master/bucket/git.json", ""},
		{"no-bucket-path", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := bucketFromURL(tt.input)
			if got != tt.want {
				t.Errorf("bucketFromURL(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
