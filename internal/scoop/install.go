package scoop

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// InstallInfo represents the install.json file stored in each app's current/ directory.
type InstallInfo struct {
	Bucket string `json:"bucket"`
	Hold   bool   `json:"hold"`
	URL    string `json:"url"`
}

// ReadInstallInfo reads and parses an install.json file from disk.
// If the bucket field is absent, it is derived from the url field path.
func ReadInstallInfo(path string) (*InstallInfo, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var info InstallInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return nil, err
	}
	if info.Bucket == "" && info.URL != "" {
		info.Bucket = bucketFromURL(info.URL)
	}
	return &info, nil
}

// bucketFromURL extracts the bucket name from a manifest URL path.
// Scoop stores the manifest path as: ...\buckets\<bucket-name>\bucket\<app>.json
// or ...\buckets\<bucket-name>\<app>.json
func bucketFromURL(url string) string {
	// Normalise to forward slashes for consistent splitting.
	url = filepath.ToSlash(url)
	const marker = "/buckets/"
	idx := strings.Index(strings.ToLower(url), marker)
	if idx == -1 {
		return ""
	}
	rest := url[idx+len(marker):]
	// rest is now "<bucket-name>/bucket/<app>.json" or "<bucket-name>/<app>.json"
	parts := strings.SplitN(rest, "/", 2)
	return parts[0]
}
