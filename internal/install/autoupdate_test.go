package install

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.noz.one/scg/internal/scoop"
)

func TestVersionSubstitutions(t *testing.T) {
	tests := []struct {
		version string
		want    map[string]string
	}{
		{
			version: "1.2.3",
			want: map[string]string{
				"version":           "1.2.3",
				"dotVersion":        "1.2.3",
				"underscoreVersion": "1_2_3",
				"dashVersion":       "1-2-3",
				"cleanVersion":      "123",
				"majorVersion":      "1",
				"minorVersion":      "2",
				"patchVersion":      "3",
				"buildVersion":      "",
				"preReleaseVersion": "1.2.3",
				"matchHead":         "1.2.3",
				"matchTail":         "",
			},
		},
		{
			version: "26.02",
			want: map[string]string{
				"version":           "26.02",
				"cleanVersion":      "2602",
				"dotVersion":        "26.02",
				"underscoreVersion": "26_02",
				"dashVersion":       "26-02",
				"majorVersion":      "26",
				"minorVersion":      "02",
				"patchVersion":      "",
				"preReleaseVersion": "26.02",
				"matchHead":         "26.02",
				"matchTail":         "",
			},
		},
		{
			version: "1.37.0-1",
			want: map[string]string{
				"version":           "1.37.0-1",
				"dotVersion":        "1.37.0.1",
				"underscoreVersion": "1_37_0_1",
				"dashVersion":       "1-37-0-1",
				"cleanVersion":      "13701",
				"majorVersion":      "1",
				"minorVersion":      "37",
				"patchVersion":      "0",
				"buildVersion":      "",
				"preReleaseVersion": "1",
				"matchHead":         "1.37.0",
				"matchTail":         "-1",
			},
		},
		{
			version: "1.2.3-beta.4",
			want: map[string]string{
				"dotVersion":        "1.2.3.beta.4",
				"underscoreVersion": "1_2_3_beta_4",
				"cleanVersion":      "123beta4",
				"preReleaseVersion": "beta.4",
				"matchHead":         "1.2.3",
				"matchTail":         "-beta.4",
			},
		},
	}

	for _, tt := range tests {
		got := VersionSubstitutions(tt.version)
		for k, want := range tt.want {
			if got[k] != want {
				t.Errorf("VersionSubstitutions(%q)[%s] = %q, want %q", tt.version, k, got[k], want)
			}
		}
	}
}

func TestSubstituteAutoupdate(t *testing.T) {
	// 7zip-style template with $version and $cleanVersion.
	vars := VersionSubstitutions("26.02")
	tmpl := "https://github.com/ip7z/7zip/releases/download/$version/7z$cleanVersion-x64.msi"
	want := "https://github.com/ip7z/7zip/releases/download/26.02/7z2602-x64.msi"
	if got := SubstituteAutoupdate(tmpl, vars); got != want {
		t.Errorf("SubstituteAutoupdate = %q, want %q", got, want)
	}
}

func TestFormatHash(t *testing.T) {
	sha256Hex := strings.Repeat("ab", 32)
	tests := []struct {
		in   string
		want string
	}{
		{sha256Hex, sha256Hex},
		{"sha256:" + sha256Hex, sha256Hex},
		{strings.Repeat("ab", 16), "md5:" + strings.Repeat("ab", 16)},
		{strings.Repeat("ab", 20), "sha1:" + strings.Repeat("ab", 20)},
		{strings.Repeat("ab", 64), "sha512:" + strings.Repeat("ab", 64)},
		{"not-a-hash", ""},
	}
	for _, tt := range tests {
		if got := formatHash(tt.in); got != tt.want {
			t.Errorf("formatHash(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestGenerateManifestForVersionSameVersion(t *testing.T) {
	m := &scoop.Manifest{Version: "1.0.0"}
	got, err := GenerateManifestForVersion(m, "app", "1.0.0", scoop.ScopeUser, "", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != m {
		t.Error("expected the same manifest when the requested version matches")
	}
}

func TestGenerateManifestForVersionNoAutoupdate(t *testing.T) {
	m := &scoop.Manifest{Version: "1.0.0"}
	_, err := GenerateManifestForVersion(m, "app", "0.9.0", scoop.ScopeUser, "", false)
	if err == nil || !strings.Contains(err.Error(), "autoupdate") {
		t.Fatalf("expected autoupdate capability error, got: %v", err)
	}
}

func TestGenerateManifestForVersionEmptyVersion(t *testing.T) {
	m := &scoop.Manifest{Version: "1.0.0", Autoupdate: map[string]any{"url": "x"}}
	if _, err := GenerateManifestForVersion(m, "app", "", scoop.ScopeUser, "", false); err == nil {
		t.Fatal("expected an error for an empty version")
	}
}

func TestGenerateManifestForVersionExtractHash(t *testing.T) {
	orig := autoupdateFetchURL
	autoupdateFetchURL = func(target, proxy string) ([]byte, error) {
		return []byte("9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08  file.zip\n"), nil
	}
	defer func() { autoupdateFetchURL = orig }()

	m := &scoop.Manifest{
		Version: "1.0.0",
		Architecture: map[string]any{
			"64bit": map[string]any{
				"url":  "https://example.com/file.zip",
				"hash": "oldhash",
			},
		},
		Autoupdate: map[string]any{
			"url": map[string]any{
				"64bit": "https://example.com/v$version/file.zip",
			},
			"hash": map[string]any{
				"url":  "https://example.com/v$version/SHA256SUMS",
				"find": `$sha256\s+file.zip`,
			},
		},
	}

	got, err := GenerateManifestForVersion(m, "app", "1.0.1", scoop.ScopeUser, "", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Version != "1.0.1" {
		t.Errorf("version = %q, want 1.0.1", got.Version)
	}
	section, ok := got.Architecture["64bit"].(map[string]any)
	if !ok {
		t.Fatalf("architecture section missing: %#v", got.Architecture)
	}
	if url := section["url"]; url != "https://example.com/v1.0.1/file.zip" {
		t.Errorf("arch url = %v, want substituted URL", url)
	}
	wantHash := "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08"
	if hash := section["hash"]; hash != wantHash {
		t.Errorf("arch hash = %v, want %s", hash, wantHash)
	}
}

func TestGenerateManifestForVersionComputeHash(t *testing.T) {
	orig := autoupdateDownload
	autoupdateDownload = func(appName, version, rawURL string, scope scoop.InstallScope, proxy string, verbose bool) (string, error) {
		p := filepath.Join(t.TempDir(), "payload.bin")
		if err := os.WriteFile(p, []byte("hello"), 0o644); err != nil {
			return "", err
		}
		return p, nil
	}
	defer func() { autoupdateDownload = orig }()

	m := &scoop.Manifest{
		Version: "1.0.0",
		URL:     "https://example.com/file.zip",
		Hash:    "oldhash",
		Autoupdate: map[string]any{
			"url": "https://example.com/v$version/file.zip",
		},
	}

	got, err := GenerateManifestForVersion(m, "app", "1.0.1", scoop.ScopeUser, "", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if url := got.URL; url != "https://example.com/v1.0.1/file.zip" {
		t.Errorf("url = %v, want substituted URL", url)
	}
	// sha256("hello")
	want := "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"
	if hash := got.Hash; hash != want {
		t.Errorf("hash = %v, want %s", hash, want)
	}
}

func TestFindHashInTextfileGzip(t *testing.T) {
	orig := autoupdateFetchURL
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	_, _ = zw.Write([]byte("prefix\n9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08  file.zip\n"))
	_ = zw.Close()
	autoupdateFetchURL = func(target, proxy string) ([]byte, error) {
		return buf.Bytes(), nil
	}
	defer func() { autoupdateFetchURL = orig }()

	hash, err := findHashInTextfile("https://example.com/SHA256SUMS.gz", `$sha256\s+file.zip`, VersionSubstitutions("1.0.0"), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08"
	if hash != want {
		t.Errorf("hash = %q, want %q", hash, want)
	}
}

func TestFindHashInTextfileFilenameFallback(t *testing.T) {
	orig := autoupdateFetchURL
	autoupdateFetchURL = func(target, proxy string) ([]byte, error) {
		return []byte("9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08  file.zip\n"), nil
	}
	defer func() { autoupdateFetchURL = orig }()

	// The find regex doesn't match; the filename-based fallback should locate the hash.
	vars := VersionSubstitutions("1.0.0")
	vars["basename"] = "file.zip"
	hash, err := findHashInTextfile("https://example.com/SHA256SUMS", "$sha256\\s+other.bin", vars, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08"
	if hash != want {
		t.Errorf("hash = %q, want %q", hash, want)
	}
}

func TestDecodeBase64Hash(t *testing.T) {
	// 32 zero bytes base64-encoded decode to 64 hex zeros.
	encoded := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0}, 32))
	if got := decodeBase64Hash(encoded); got != strings.Repeat("0", 64) {
		t.Errorf("decodeBase64Hash(%q) = %q, want %d zeros", encoded, got, 64)
	}
	// Pure hex hashes of standard lengths pass through unchanged.
	hex64 := strings.Repeat("ab", 32)
	if got := decodeBase64Hash(hex64); got != hex64 {
		t.Errorf("hex hash was modified: got %q", got)
	}
	// Non-base64-shaped input passes through.
	if got := decodeBase64Hash("not-a-hash"); got != "not-a-hash" {
		t.Errorf("unexpected transform: %q", got)
	}
}

func TestGenerateManifestForVersionMultiURL(t *testing.T) {
	orig := autoupdateFetchURL
	autoupdateFetchURL = func(target, proxy string) ([]byte, error) {
		return []byte(
			"1111111111111111111111111111111111111111111111111111111111111111  file1.zip\n" +
				"2222222222222222222222222222222222222222222222222222222222222222  file2.zip\n"), nil
	}
	defer func() { autoupdateFetchURL = orig }()

	m := &scoop.Manifest{
		Version: "1.0.0",
		URL:     []any{"https://example.com/file1.zip", "https://example.com/file2.zip"},
		Hash:    "oldhash",
		Autoupdate: map[string]any{
			"url": []any{"https://example.com/v$version/file1.zip", "https://example.com/v$version/file2.zip"},
			"hash": []any{
				map[string]any{"url": "https://example.com/SHA256SUMS", "find": `$sha256\s+file1.zip`},
				map[string]any{"url": "https://example.com/SHA256SUMS", "find": `$sha256\s+file2.zip`},
			},
		},
	}

	got, err := GenerateManifestForVersion(m, "app", "1.0.1", scoop.ScopeUser, "", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	hashes, ok := got.Hash.([]any)
	if !ok || len(hashes) != 2 {
		t.Fatalf("expected 2 hashes, got %#v", got.Hash)
	}
	if hashes[0] != "1111111111111111111111111111111111111111111111111111111111111111" {
		t.Errorf("hash[0] = %v", hashes[0])
	}
	if hashes[1] != "2222222222222222222222222222222222222222222222222222222222222222" {
		t.Errorf("hash[1] = %v", hashes[1])
	}
	urls, ok := got.URL.([]any)
	if !ok || len(urls) != 2 || urls[0] != "https://example.com/v1.0.1/file1.zip" || urls[1] != "https://example.com/v1.0.1/file2.zip" {
		t.Errorf("urls = %#v", got.URL)
	}
}

func TestGenerateManifestForVersionArchExtractDir(t *testing.T) {
	m := &scoop.Manifest{
		Version: "1.0.0",
		Architecture: map[string]any{
			"64bit": map[string]any{
				"url":         "https://example.com/file.zip",
				"hash":        "oldhash",
				"extract_dir": "old-dir",
			},
		},
		Autoupdate: map[string]any{
			"architecture": map[string]any{
				"64bit": map[string]any{
					"url":         "https://example.com/v$version/file.zip",
					"extract_dir": "dir-$version",
				},
			},
		},
	}

	// Arch has a hash but no autoupdate hash config: generation computes it, so stub the download.
	orig := autoupdateDownload
	autoupdateDownload = func(appName, version, rawURL string, scope scoop.InstallScope, proxy string, verbose bool) (string, error) {
		p := filepath.Join(t.TempDir(), "payload.bin")
		if err := os.WriteFile(p, []byte("data"), 0o644); err != nil {
			return "", err
		}
		return p, nil
	}
	defer func() { autoupdateDownload = orig }()

	got, err := GenerateManifestForVersion(m, "app", "1.0.1", scoop.ScopeUser, "", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	section := got.Architecture["64bit"].(map[string]any)
	if ed := section["extract_dir"]; ed != "dir-1.0.1" {
		t.Errorf("extract_dir = %v, want dir-1.0.1", ed)
	}
	if u := section["url"]; u != "https://example.com/v1.0.1/file.zip" {
		t.Errorf("url = %v", u)
	}
}

func TestGenerateManifestForVersionKeepsOriginalOnUnrelatedFields(t *testing.T) {
	orig := autoupdateFetchURL
	autoupdateFetchURL = func(target, proxy string) ([]byte, error) {
		return []byte("9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08  file.zip\n"), nil
	}
	defer func() { autoupdateFetchURL = orig }()

	m := &scoop.Manifest{
		Version:     "1.0.0",
		Description: "desc",
		Homepage:    "https://example.com",
		URL:         "https://example.com/file.zip",
		Hash:        "oldhash",
		Bin:         []any{"file.exe"},
		Autoupdate: map[string]any{
			"url": "https://example.com/v$version/file.zip",
			"hash": map[string]any{
				"url": "https://example.com/SHA256SUMS",
			},
		},
	}

	got, err := GenerateManifestForVersion(m, "app", "1.0.1", scoop.ScopeUser, "", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Description != m.Description || got.Homepage != m.Homepage {
		t.Error("unrelated manifest fields should be preserved")
	}
	if len(got.Bin.([]any)) != 1 {
		t.Error("bin field should be preserved")
	}
	if got.URL != "https://example.com/v1.0.1/file.zip" {
		t.Errorf("url = %v", got.URL)
	}
}
