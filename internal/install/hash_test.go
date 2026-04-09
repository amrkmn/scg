package install

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func computeTestHashHex(content []byte, algo string) string {
	switch algo {
	case "sha1":
		h := sha1.Sum(content)
		return hex.EncodeToString(h[:])
	case "md5":
		h := md5.Sum(content)
		return hex.EncodeToString(h[:])
	case "sha256":
		h := sha256.Sum256(content)
		return hex.EncodeToString(h[:])
	case "sha512":
		h := sha512.Sum512(content)
		return hex.EncodeToString(h[:])
	default:
		return ""
	}
}

func TestParseHash(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantAlgo  string
		wantValue string
		wantNil   bool
	}{
		{"empty string", "", "", "", true},
		{"sha256 prefix", "sha256:abc123", "sha256", "abc123", false},
		{"SHA256 upper prefix", "SHA256:ABC123", "sha256", "ABC123", false},
		{"sha512 prefix", "sha512:deadbeef", "sha512", "deadbeef", false},
		{"sha1 prefix", "sha1:abc123", "sha1", "abc123", false},
		{"SHA1 upper prefix", "SHA1:ABC123", "sha1", "ABC123", false},
		{"md5 prefix", "md5:deadbeef", "md5", "deadbeef", false},
		{"MD5 upper prefix", "MD5:ABC123", "md5", "ABC123", false},
		{"plain hex 64 chars defaults to sha256", "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789", "sha256", "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789", false},
		{"plain hex 32 chars inferred as md5", "d41d8cd98f00b204e9800998ecf8427e", "md5", "d41d8cd98f00b204e9800998ecf8427e", false},
		{"plain hex 40 chars inferred as sha1", "da39a3ee5e6b4b0d3255bfef95601890afd80709", "sha1", "da39a3ee5e6b4b0d3255bfef95601890afd80709", false},
		{"plain hex 128 chars inferred as sha512", "cf83e1357eefb8bdf1542850d66d8007d620e4050b5715dc83f4a921d36ce9ce47d0d13c5d85f2b0ff8318d2877eec2f63b931bd47417a81a538327af927da3e", "sha512", "cf83e1357eefb8bdf1542850d66d8007d620e4050b5715dc83f4a921d36ce9ce47d0d13c5d85f2b0ff8318d2877eec2f63b931bd47417a81a538327af927da3e", false},
		{"whitespace trimmed", "  sha256:abc  ", "sha256", "abc", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseHash(tt.input)
			if err != nil {
				t.Fatalf("ParseHash(%q) error: %v", tt.input, err)
			}
			if tt.wantNil {
				if got != nil {
					t.Errorf("ParseHash(%q) = %v, want nil", tt.input, got)
				}
				return
			}
			if got == nil {
				t.Fatalf("ParseHash(%q) = nil, want non-nil", tt.input)
			}
			if got.Algorithm != tt.wantAlgo {
				t.Errorf("ParseHash(%q).Algorithm = %q, want %q", tt.input, got.Algorithm, tt.wantAlgo)
			}
			if got.Value != tt.wantValue {
				t.Errorf("ParseHash(%q).Value = %q, want %q", tt.input, got.Value, tt.wantValue)
			}
		})
	}
}

func TestVerifyHash(t *testing.T) {
	content := []byte("hello world")
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "testfile")
	if err := os.WriteFile(filePath, content, 0o644); err != nil {
		t.Fatal(err)
	}

	sha256Hash := sha256.Sum256(content)
	sha256Hex := hex.EncodeToString(sha256Hash[:])

	sha512Hash := sha512.Sum512(content)
	sha512Hex := hex.EncodeToString(sha512Hash[:])

	t.Run("sha256 match", func(t *testing.T) {
		err := VerifyHash(filePath, &HashFormat{Algorithm: "sha256", Value: sha256Hex})
		if err != nil {
			t.Errorf("VerifyHash sha256 match: %v", err)
		}
	})

	t.Run("sha256 case-insensitive match", func(t *testing.T) {
		err := VerifyHash(filePath, &HashFormat{Algorithm: "sha256", Value: strings.ToUpper(sha256Hex)})
		if err != nil {
			t.Errorf("VerifyHash sha256 upper: %v", err)
		}
	})

	t.Run("sha256 mismatch", func(t *testing.T) {
		err := VerifyHash(filePath, &HashFormat{Algorithm: "sha256", Value: "0000"})
		if err == nil {
			t.Error("VerifyHash sha256 mismatch: expected error")
		}
	})

	t.Run("sha512 match", func(t *testing.T) {
		err := VerifyHash(filePath, &HashFormat{Algorithm: "sha512", Value: sha512Hex})
		if err != nil {
			t.Errorf("VerifyHash sha512 match: %v", err)
		}
	})

	t.Run("sha1 match", func(t *testing.T) {
		err := VerifyHash(filePath, &HashFormat{Algorithm: "sha1", Value: computeTestHashHex(content, "sha1")})
		if err != nil {
			t.Errorf("VerifyHash sha1 match: %v", err)
		}
	})

	t.Run("md5 match", func(t *testing.T) {
		err := VerifyHash(filePath, &HashFormat{Algorithm: "md5", Value: computeTestHashHex(content, "md5")})
		if err != nil {
			t.Errorf("VerifyHash md5 match: %v", err)
		}
	})

	t.Run("nil hash skips verification", func(t *testing.T) {
		err := VerifyHash(filePath, nil)
		if err != nil {
			t.Errorf("VerifyHash nil: %v", err)
		}
	})

	t.Run("empty hash skips verification", func(t *testing.T) {
		err := VerifyHash(filePath, &HashFormat{Algorithm: "sha256", Value: ""})
		if err != nil {
			t.Errorf("VerifyHash empty value: %v", err)
		}
	})

	t.Run("unsupported algorithm", func(t *testing.T) {
		err := VerifyHash(filePath, &HashFormat{Algorithm: "ripemd160", Value: "abc"})
		if err == nil {
			t.Error("VerifyHash unsupported algo: expected error")
		}
	})

	t.Run("nonexistent file", func(t *testing.T) {
		err := VerifyHash(filepath.Join(tmpDir, "nonexistent"), &HashFormat{Algorithm: "sha256", Value: sha256Hex})
		if err == nil {
			t.Error("VerifyHash nonexistent file: expected error")
		}
	})
}

func TestComputeSHA256(t *testing.T) {
	content := []byte("test content")
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "testfile")
	if err := os.WriteFile(filePath, content, 0o644); err != nil {
		t.Fatal(err)
	}

	expected := sha256.Sum256(content)
	expectedHex := hex.EncodeToString(expected[:])

	got, err := ComputeSHA256(filePath)
	if err != nil {
		t.Fatal(err)
	}
	if got != expectedHex {
		t.Errorf("ComputeSHA256 = %s, want %s", got, expectedHex)
	}
}

func TestComputeSHA512(t *testing.T) {
	content := []byte("test content")
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "testfile")
	if err := os.WriteFile(filePath, content, 0o644); err != nil {
		t.Fatal(err)
	}

	expected := sha512.Sum512(content)
	expectedHex := hex.EncodeToString(expected[:])

	got, err := ComputeSHA512(filePath)
	if err != nil {
		t.Fatal(err)
	}
	if got != expectedHex {
		t.Errorf("ComputeSHA512 = %s, want %s", got, expectedHex)
	}
}
