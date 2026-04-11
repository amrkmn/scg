package install

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIsSevenZipArchive(t *testing.T) {
	tests := []struct {
		ext  string
		want bool
	}{
		{".7z", true},
		{".tar", true},
		{".gz", true},
		{".bz2", true},
		{".xz", true},
		{".tgz", true},
		{".lzma", true},
		{".lz4", true},
		{".zst", true},
		{".zip", false},
		{".msi", false},
		{".txt", false},
		{".exe", true},
		{".tar.gz", true},
		{".tar.bz2", true},
		{".tar.xz", true},
	}

	for _, tt := range tests {
		t.Run(tt.ext, func(t *testing.T) {
			got := isSevenZipArchive(tt.ext)
			if got != tt.want {
				t.Errorf("isSevenZipArchive(%q) = %v, want %v", tt.ext, got, tt.want)
			}
		})
	}
}

func TestShellQuote(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{`C:\test`, `C:\test`},
		{`C:\Program Files\test`, `"C:\Program Files\test"`},
		{`C:\Users\test`, `C:\Users\test`},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := shellQuote(tt.input)
			if got != tt.want {
				t.Errorf("shellQuote(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestContainsSpace(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"nospaces", false},
		{"has space", true},
		{"", false},
		{"  ", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := containsSpace(tt.input)
			if got != tt.want {
				t.Errorf("containsSpace(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestExtractExtensionTarVariants(t *testing.T) {
	ext := strings.ToLower(".tar.zst")
	if !isSevenZipArchive(ext) {
		t.Errorf("isSevenZipArchive(%q) = false, want true (compound tar extension)", ext)
	}
}

func TestLooksLikeTar(t *testing.T) {
	tmp := t.TempDir()

	tarLike := filepath.Join(tmp, "payload")
	buf := make([]byte, 262)
	copy(buf[257:262], []byte("ustar"))
	if err := os.WriteFile(tarLike, buf, 0o644); err != nil {
		t.Fatalf("write tar-like file: %v", err)
	}

	isTar, err := looksLikeTar(tarLike)
	if err != nil {
		t.Fatalf("looksLikeTar(tarLike) error: %v", err)
	}
	if !isTar {
		t.Fatalf("looksLikeTar(tarLike) = false, want true")
	}

	notTar := filepath.Join(tmp, "plain.bin")
	if err := os.WriteFile(notTar, []byte("hello"), 0o644); err != nil {
		t.Fatalf("write non-tar file: %v", err)
	}
	isTar, err = looksLikeTar(notTar)
	if err != nil {
		t.Fatalf("looksLikeTar(notTar) error: %v", err)
	}
	if isTar {
		t.Fatalf("looksLikeTar(notTar) = true, want false")
	}
}
