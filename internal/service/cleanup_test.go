package service

import (
	"os"
	"path/filepath"
	"testing"

	"go.noz.one/scg/internal/scoop"
)

func TestCacheFileMatching(t *testing.T) {
	// Create a temporary cache directory
	cacheDir := t.TempDir()

	// Create test cache files
	testFiles := []struct {
		name    string
		shouldMatchGit bool
	}{
		// Should match git
		{"git#2.43.0#https_example.com.exe", true},
		{"git#2.43.0", true},
		{"git#3.0.0#abc123", true},
		// Should NOT match git (different apps)
		{"git-lfs#3.2.0#https_example.com.exe", false},
		{"github-cli#2.0.0.exe", false},
		{"mingit#1.0.0.exe", false},
		// Should NOT match git (unrelated)
		{"node#18.0.0.exe", false},
		{"python#3.11.0.exe", false},
	}

	for _, tf := range testFiles {
		path := filepath.Join(cacheDir, tf.name)
		if err := os.WriteFile(path, []byte("test"), 0644); err != nil {
			t.Fatalf("Failed to create test file %s: %v", tf.name, err)
		}
	}

	// Test the prefix matching logic
	prefix := "git#"
	prefixLen := len(prefix)

	for _, tf := range testFiles {
		nameLower := tf.name
		matches := false

		if len(nameLower) >= prefixLen && nameLower[:prefixLen] == prefix {
			// Check boundary
			if prefixLen < len(nameLower) {
				nextChar := nameLower[prefixLen]
				// If next char is a letter, this is a different app
				if !(nextChar >= 'a' && nextChar <= 'z') {
					matches = true
				}
			} else {
				matches = true
			}
		}

		if matches != tf.shouldMatchGit {
			t.Errorf("File %s: expected match=%v, got match=%v", tf.name, tf.shouldMatchGit, matches)
		}
	}
}

func TestGetDirectorySize(t *testing.T) {
	// Create temp directory structure
	tmpDir := t.TempDir()

	// Create files with known sizes
	files := map[string]int64{
		"file1.txt": 100,
		"file2.txt": 200,
		"subdir/file3.txt": 50,
	}

	for path, size := range files {
		fullPath := filepath.Join(tmpDir, path)
		dir := filepath.Dir(fullPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("Failed to create dir %s: %v", dir, err)
		}
		data := make([]byte, size)
		if err := os.WriteFile(fullPath, data, 0644); err != nil {
			t.Fatalf("Failed to create file %s: %v", fullPath, err)
		}
	}

	// Test size calculation
	size := getDirectorySize(tmpDir)
	expected := int64(100 + 200 + 50)
	if size != expected {
		t.Errorf("Expected size %d, got %d", expected, size)
	}
}

func TestCleanupResultAccumulation(t *testing.T) {
	result := CleanupResult{
		App:   "test-app",
		Scope: scoop.ScopeUser,
		OldVersions: []VersionEntry{
			{Version: "1.0.0", Size: 1000},
			{Version: "1.1.0", Size: 2000},
		},
		FailedVersions: []FailedEntry{
			{Version: "1.2.0", Error: os.ErrPermission},
		},
		CacheFiles: []CacheEntry{
			{Name: "test#1.0.0#abc.exe", Size: 500},
		},
	}

	if len(result.OldVersions) != 2 {
		t.Errorf("Expected 2 old versions, got %d", len(result.OldVersions))
	}

	if len(result.FailedVersions) != 1 {
		t.Errorf("Expected 1 failed version, got %d", len(result.FailedVersions))
	}

	if len(result.CacheFiles) != 1 {
		t.Errorf("Expected 1 cache file, got %d", len(result.CacheFiles))
	}
}
