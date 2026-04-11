package install

import (
	"os"
	"path/filepath"
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

	t.Run("scoop rename fragment uses target extension", func(t *testing.T) {
		got := dm.CachePath("docker-compose", "5.1.2", "https://example.com/download?id=1#/docker-compose.exe")
		if !strings.HasSuffix(strings.ToLower(got), ".exe") {
			t.Errorf("CachePath extension = %s, want .exe", got)
		}
	})

	t.Run("uses final extension for tar.gz", func(t *testing.T) {
		got := dm.CachePath("nrfutil", "8.1.1", "https://example.com/nrfutil-8.1.1.tar.gz")
		if !strings.HasSuffix(strings.ToLower(got), ".gz") {
			t.Errorf("CachePath extension = %s, want .gz", got)
		}
	})
}

func TestFindCachedPathPrefersCurrentThenLegacy(t *testing.T) {
	cacheDir := t.TempDir()
	dm := &DownloadManager{cacheDir: cacheDir, verbose: false}
	url := "https://example.com/nrfutil-8.1.1.tar.gz"

	legacy := dm.legacyCachePath("nrfutil", "8.1.1", url)
	if err := os.WriteFile(legacy, []byte("legacy"), 0o644); err != nil {
		t.Fatalf("write legacy cache: %v", err)
	}

	found, ok := dm.FindCachedPath("nrfutil", "8.1.1", url)
	if !ok {
		t.Fatal("FindCachedPath did not find legacy cache")
	}
	if found != legacy {
		t.Fatalf("FindCachedPath = %q, want legacy %q", found, legacy)
	}

	current := dm.CachePath("nrfutil", "8.1.1", url)
	if err := os.WriteFile(current, []byte("current"), 0o644); err != nil {
		t.Fatalf("write current cache: %v", err)
	}

	found, ok = dm.FindCachedPath("nrfutil", "8.1.1", url)
	if !ok {
		t.Fatal("FindCachedPath did not find current cache")
	}
	if filepath.Clean(found) != filepath.Clean(current) {
		t.Fatalf("FindCachedPath = %q, want current %q", found, current)
	}
}

func TestDownloadFileName(t *testing.T) {
	t.Run("uses scoop rename fragment", func(t *testing.T) {
		got := DownloadFileName("docker-compose", "https://example.com/file.bin#/docker-compose.exe")
		if got != "docker-compose.exe" {
			t.Fatalf("DownloadFileName rename = %q, want %q", got, "docker-compose.exe")
		}
	})

	t.Run("falls back to url path base", func(t *testing.T) {
		got := DownloadFileName("git", "https://example.com/PortableGit.7z")
		if got != "PortableGit.7z" {
			t.Fatalf("DownloadFileName base = %q, want %q", got, "PortableGit.7z")
		}
	})

	t.Run("falls back to app name when no filename", func(t *testing.T) {
		got := DownloadFileName("python", "https://example.com/")
		if got != "python" {
			t.Fatalf("DownloadFileName fallback = %q, want %q", got, "python")
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

	t.Run("7 characters", func(t *testing.T) {
		if len(h1) != 7 {
			t.Errorf("cacheHash length = %d, want 7", len(h1))
		}
	})
}

func TestBoolFromConfig(t *testing.T) {
	cases := []struct {
		name string
		in   any
		def  bool
		want bool
	}{
		{name: "bool true", in: true, def: false, want: true},
		{name: "bool false", in: false, def: true, want: false},
		{name: "string true", in: "true", def: false, want: true},
		{name: "string false", in: "false", def: true, want: false},
		{name: "number zero", in: float64(0), def: true, want: false},
		{name: "number non-zero", in: float64(1), def: false, want: true},
		{name: "invalid string", in: "nope", def: true, want: true},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := boolFromConfig(tc.in, tc.def)
			if got != tc.want {
				t.Fatalf("boolFromConfig(%v, %v) = %v, want %v", tc.in, tc.def, got, tc.want)
			}
		})
	}
}

func TestFormatAria2Summary(t *testing.T) {
	raw := strings.Join([]string{
		"04/10 20:28:21 [NOTICE] Downloading 1 item(s)",
		"[#dc76b5 2.7MiB/2.8MiB(94%) CN:1 DL:1.1MiB]",
		"",
		"Download Results:",
		"gid   |stat|avg speed  |path/URI",
		"======+====+===========+=======================================================",
		"dc76b5|OK  |   1.2MiB/s|C:/Users/User/scoop/cache/x265#4.1+239-8be7dbf#20cf57df.7z",
		"",
		"Status Legend:",
		"(OK):download completed.",
	}, "\n")

	got := formatAria2Summary(raw)
	if len(got) != 6 {
		t.Fatalf("formatAria2Summary length = %d, want 6; got=%v", len(got), got)
	}
	if got[0] != "Download Results:" {
		t.Fatalf("first summary line = %q, want %q", got[0], "Download Results:")
	}
	if got[4] != "Status Legend:" {
		t.Fatalf("status line = %q, want %q", got[4], "Status Legend:")
	}
}
