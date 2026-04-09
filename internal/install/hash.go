package install

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"
)

// HashFormat represents a parsed hash specification.
type HashFormat struct {
	Algorithm string // "sha256", "sha512", "sha1", or "md5"
	Value     string // hex-encoded hash
}

// ParseHash parses a hash string that may be prefixed with "sha256:", "sha512:",
// "sha1:", or "md5:". If no prefix is given, SHA-256 is assumed.
func ParseHash(hash string) (*HashFormat, error) {
	if hash == "" {
		return nil, nil
	}

	hash = strings.TrimSpace(hash)

	lower := strings.ToLower(hash)
	if strings.HasPrefix(lower, "sha256:") {
		return &HashFormat{Algorithm: "sha256", Value: hash[7:]}, nil
	}
	if strings.HasPrefix(lower, "sha512:") {
		return &HashFormat{Algorithm: "sha512", Value: hash[7:]}, nil
	}
	if strings.HasPrefix(lower, "sha1:") {
		return &HashFormat{Algorithm: "sha1", Value: hash[5:]}, nil
	}
	if strings.HasPrefix(lower, "md5:") {
		return &HashFormat{Algorithm: "md5", Value: hash[4:]}, nil
	}

	// Infer algorithm from hash length (matching Scoop's format_hash behavior).
	plain := strings.ToLower(hash)
	switch len(plain) {
	case 32:
		return &HashFormat{Algorithm: "md5", Value: plain}, nil
	case 40:
		return &HashFormat{Algorithm: "sha1", Value: plain}, nil
	case 64:
		return &HashFormat{Algorithm: "sha256", Value: plain}, nil
	case 128:
		return &HashFormat{Algorithm: "sha512", Value: plain}, nil
	default:
		return &HashFormat{Algorithm: "sha256", Value: plain}, nil
	}
}

// VerifyHash computes the hash of a file and compares it against the expected value.
func VerifyHash(filePath string, expected *HashFormat) error {
	if expected == nil || expected.Value == "" {
		return nil
	}

	var computed string
	var err error

	switch expected.Algorithm {
	case "sha256":
		computed, err = ComputeSHA256(filePath)
	case "sha512":
		computed, err = ComputeSHA512(filePath)
	case "sha1":
		computed, err = ComputeSHA1(filePath)
	case "md5":
		computed, err = ComputeMD5(filePath)
	default:
		return fmt.Errorf("unsupported hash algorithm: %s", expected.Algorithm)
	}

	if err != nil {
		return fmt.Errorf("failed to compute %s hash: %w", expected.Algorithm, err)
	}

	if !strings.EqualFold(computed, expected.Value) {
		return fmt.Errorf("%s hash mismatch for %s:\n  expected: %s\n  actual:   %s",
			expected.Algorithm, filePath, expected.Value, computed)
	}

	return nil
}

// ComputeSHA256 computes the SHA-256 hash of a file and returns the hex encoding.
func ComputeSHA256(filePath string) (string, error) {
	return computeHash(filePath, sha256.New())
}

// ComputeSHA512 computes the SHA-512 hash of a file and returns the hex encoding.
func ComputeSHA512(filePath string) (string, error) {
	return computeHash(filePath, sha512.New())
}

// ComputeSHA1 computes the SHA-1 hash of a file and returns the hex encoding.
func ComputeSHA1(filePath string) (string, error) {
	return computeHash(filePath, sha1.New())
}

// ComputeMD5 computes the MD5 hash of a file and returns the hex encoding.
func ComputeMD5(filePath string) (string, error) {
	return computeHash(filePath, md5.New())
}

func computeHash(filePath string, h interface {
	Sum([]byte) []byte
	Write([]byte) (int, error)
}) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
