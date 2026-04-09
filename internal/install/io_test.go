package install

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteInstallInfo(t *testing.T) {
	tmpDir := t.TempDir()
	info := &InstallInfo{
		Architecture: "64bit",
		URL:          "https://example.com/app.json",
		Bucket:       "main",
	}

	path := filepath.Join(tmpDir, "install.json")
	if err := WriteInstallInfo(path, info); err != nil {
		t.Fatalf("WriteInstallInfo error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile error: %v", err)
	}

	var read InstallInfo
	if err := json.Unmarshal(data, &read); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if read.Architecture != "64bit" {
		t.Errorf("Architecture = %q, want %q", read.Architecture, "64bit")
	}
	if read.URL != "https://example.com/app.json" {
		t.Errorf("URL = %q, want %q", read.URL, "https://example.com/app.json")
	}
	if read.Bucket != "main" {
		t.Errorf("Bucket = %q, want %q", read.Bucket, "main")
	}
}

func TestWriteManifest(t *testing.T) {
	tmpDir := t.TempDir()
	manifest := map[string]any{
		"version":     "1.0.0",
		"description": "Test app",
	}

	path := filepath.Join(tmpDir, "manifest.json")
	if err := WriteManifest(path, manifest); err != nil {
		t.Fatalf("WriteManifest error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile error: %v", err)
	}

	if !json.Valid(data) {
		t.Error("WriteManifest produced invalid JSON")
	}
}
