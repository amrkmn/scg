package install

import (
	"strings"
	"testing"
)

func TestCachePath(t *testing.T) {
	dm := &DownloadManager{cacheDir: `C:\Users\test\scoop\cache`, verbose: false}

	t.Run("basic URL", func(t *testing.T) {
		got := dm.CachePath("git", "2.43.0", "https://github.com/git-for-windows/git/releases/download/v2.43.0.windows.1/PortableGit-2.43.0.windows.1.7z")
		if !strings.Contains(got, "git") {
			t.Errorf("CachePath missing app name: %s", got)
		}
		if !strings.Contains(got, "2.43.0") {
			t.Errorf("CachePath missing version: %s", got)
		}
		if !strings.Contains(got, ".7z") {
			t.Errorf("CachePath missing extension: %s", got)
		}
	})

	t.Run("lowercase app name", func(t *testing.T) {
		got := dm.CachePath("Git", "2.43.0", "https://example.com/git.7z")
		if !strings.Contains(strings.ToLower(got), "git") {
			t.Errorf("CachePath should lowercase app name: %s", got)
		}
	})
}

func TestCacheHash(t *testing.T) {
	h1 := cacheHash("https://example.com/file1.zip")
	h2 := cacheHash("https://example.com/file1.zip")
	h3 := cacheHash("https://example.com/file2.zip")

	t.Run("deterministic", func(t *testing.T) {
		if h1 != h2 {
			t.Errorf("cacheHash not deterministic: %s != %s", h1, h2)
		}
	})

	t.Run("different URLs produce different hashes", func(t *testing.T) {
		if h1 == h3 {
			t.Errorf("cacheHash collision for different URLs: %s == %s", h1, h3)
		}
	})

	t.Run("8 characters", func(t *testing.T) {
		if len(h1) != 8 {
			t.Errorf("cacheHash length = %d, want 8", len(h1))
		}
	})
}
