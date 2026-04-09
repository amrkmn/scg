package install

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"go.noz.one/scg/internal/scoop"
)

// DownloadManager handles downloading files to the Scoop cache directory.
type DownloadManager struct {
	cacheDir string
	verbose  bool
}

// NewDownloadManager creates a DownloadManager for the given scope.
func NewDownloadManager(scope scoop.InstallScope, verbose bool) *DownloadManager {
	paths := scoop.ResolvePaths(scope)
	return &DownloadManager{
		cacheDir: paths.Cache,
		verbose:  verbose,
	}
}

// DownloadResult contains the result of a download operation.
type DownloadResult struct {
	CachePath  string // Full path to the cached file
	Downloaded bool   // Whether the file was newly downloaded (vs cache hit)
	Size       int64  // File size in bytes
}

// CachePath returns the expected cache path for a file, following Scoop's
// cache naming convention: <app>#<version>#<url_hash>
func (dm *DownloadManager) CachePath(app, version, downloadURL string) string {
	hash := cacheHash(downloadURL)
	filename := filepath.Base(downloadURL)

	// Handle URLs that end with a path but no filename.
	if filename == "" || filename == "." || filename == "/" {
		filename = app
	}

	// Strip query parameters from filename.
	if idx := strings.IndexAny(filename, "?#"); idx >= 0 {
		filename = filename[:idx]
	}

	return filepath.Join(dm.cacheDir, fmt.Sprintf("%s#%s#%s%s",
		strings.ToLower(app), version, hash, filepath.Ext(filename)))
}

// cacheHash computes a short hash from a URL for the cache filename.
func cacheHash(downloadURL string) string {
	h := sha256.Sum256([]byte(downloadURL))
	return hex.EncodeToString(h[:])[:8]
}

// Download downloads a file to the cache directory.
// If the file already exists in cache and useCache is true, it returns the cached path.
// If aria2 is available, it uses aria2 for multi-connection downloads.
func (dm *DownloadManager) Download(app, version, downloadURL string, useCache bool, proxy string) (*DownloadResult, error) {
	cachePath := dm.CachePath(app, version, downloadURL)

	// Check cache.
	if useCache {
		if fi, err := os.Stat(cachePath); err == nil {
			return &DownloadResult{
				CachePath:  cachePath,
				Downloaded: false,
				Size:       fi.Size(),
			}, nil
		}
	}

	// Ensure cache directory exists.
	if err := os.MkdirAll(dm.cacheDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	// Try aria2 first.
	if aria2Path, err := FindAria2(); err == nil {
		if dm.verbose {
			fmt.Printf("  Using aria2 for download: %s\n", downloadURL)
		}
		result, err := dm.downloadWithAria2(aria2Path, cachePath, downloadURL, proxy)
		if err == nil {
			return result, nil
		}
		// aria2 failed, fall back to HTTP.
		if dm.verbose {
			fmt.Printf("  aria2 download failed, falling back to HTTP: %v\n", err)
		}
	}

	// Fall back to HTTP download.
	if dm.verbose {
		fmt.Printf("  Downloading via HTTP: %s\n", downloadURL)
	}
	if err := dm.downloadHTTP(cachePath, downloadURL, proxy); err != nil {
		return nil, fmt.Errorf("failed to download %s: %w", downloadURL, err)
	}

	fi, err := os.Stat(cachePath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat downloaded file: %w", err)
	}

	return &DownloadResult{
		CachePath:  cachePath,
		Downloaded: true,
		Size:       fi.Size(),
	}, nil
}

// downloadHTTP downloads a file using Go's net/http package.
func (dm *DownloadManager) downloadHTTP(destPath, downloadURL, proxy string) error {
	client := &http.Client{
		Timeout: 30 * time.Minute,
	}
	if proxy != "" {
		proxyURL, err := url.Parse(proxy)
		if err != nil {
			return fmt.Errorf("invalid proxy URL: %w", err)
		}
		client.Transport = &http.Transport{
			Proxy: http.ProxyURL(proxyURL),
		}
	}

	resp, err := client.Get(downloadURL)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	f, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	_, err = io.Copy(f, resp.Body)
	return err
}

// downloadWithAria2 downloads a file using aria2c.
func (dm *DownloadManager) downloadWithAria2(aria2Path, destPath, downloadURL, proxy string) (*DownloadResult, error) {
	args := []string{
		"--continue=true",
		"--max-connection-per-server=5",
		"--split=5",
		"--min-split-size=5M",
		fmt.Sprintf("--dir=%s", filepath.Dir(destPath)),
		fmt.Sprintf("--out=%s", filepath.Base(destPath)),
		"--auto-file-renaming=false",
		"--allow-overwrite=true",
	}
	if proxy != "" {
		args = append(args, fmt.Sprintf("--all-proxy=%s", proxy))
	}
	args = append(args, downloadURL)

	cmd := exec.Command(aria2Path, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("aria2 failed: %w\n%s", err, out)
	}

	fi, err := os.Stat(destPath)
	if err != nil {
		return nil, fmt.Errorf("aria2 completed but file not found: %w", err)
	}

	return &DownloadResult{
		CachePath:  destPath,
		Downloaded: true,
		Size:       fi.Size(),
	}, nil
}
