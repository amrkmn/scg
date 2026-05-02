package service

import (
	"os"
	"path/filepath"
	"strings"

	"go.noz.one/scg/internal/scoop"
)

// CleanupOptions configures a cleanup operation.
type CleanupOptions struct {
	Cache            bool
	DryRun           bool
	Verbose          bool
	SuppressWarnings bool
}

// VersionEntry describes an old version that was (or would be) removed.
type VersionEntry struct {
	Version string
	Size    int64
}

// FailedEntry describes a version that could not be removed.
type FailedEntry struct {
	Version string
	Error   error
}

// CacheEntry describes a cache file that was (or would be) removed.
type CacheEntry struct {
	Name string
	Size int64
}

// CleanupResult holds the outcome of cleaning a single app.
type CleanupResult struct {
	App            string
	Scope          scoop.InstallScope
	OldVersions    []VersionEntry
	FailedVersions []FailedEntry
	CacheFiles     []CacheEntry
}

// CleanupService removes old app versions and cache files.
type CleanupService struct {
	ctx AppContext
}

// NewCleanupService creates a CleanupService.
func NewCleanupService(ctx AppContext) *CleanupService {
	return &CleanupService{ctx: ctx}
}

// CleanupApp removes old versions (and optionally cache files) for a single app.
func (s *CleanupService) CleanupApp(appName string, scope scoop.InstallScope, opts CleanupOptions) CleanupResult {
	result := CleanupResult{App: appName, Scope: scope}
	paths := scoop.ResolvePaths(scope)
	appDir := filepath.Join(paths.Apps, appName)

	currentVersion := scoop.ResolveCurrentVersion(appName, scope)
	if currentVersion == "" {
		return result
	}

	// List all subdirectories of the app directory.
	entries, err := os.ReadDir(appDir)
	if err != nil {
		return result
	}

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		ver := e.Name()
		if ver == "current" || ver == currentVersion {
			continue
		}

		versionDir := filepath.Join(appDir, ver)
		size := getDirectorySize(versionDir)

		if opts.DryRun {
			result.OldVersions = append(result.OldVersions, VersionEntry{Version: ver, Size: size})
			continue
		}

		if err := os.RemoveAll(versionDir); err != nil {
			result.FailedVersions = append(result.FailedVersions, FailedEntry{Version: ver, Error: err})
		} else {
			result.OldVersions = append(result.OldVersions, VersionEntry{Version: ver, Size: size})
		}
	}

	// Cache cleanup - remove outdated cache files and .download temp files for this app.
	if opts.Cache {
		cacheDir := paths.Cache
		cacheEntries, err := os.ReadDir(cacheDir)
		if err == nil {
			prefix := strings.ToLower(appName) + "#"
			prefixLen := len(prefix)
			appDownloadName := strings.ToLower(appName) + ".download"
			for _, e := range cacheEntries {
				if e.IsDir() {
					continue
				}
				nameLower := strings.ToLower(e.Name())

				// Match .download temp files (e.g., "git.download")
				if nameLower == appDownloadName {
					downloadFile := filepath.Join(cacheDir, e.Name())
					var size int64
					if fi, err := e.Info(); err == nil {
						size = fi.Size()
					}
					if opts.DryRun {
						result.CacheFiles = append(result.CacheFiles, CacheEntry{Name: e.Name(), Size: size})
						continue
					}
					if err := os.Remove(downloadFile); err == nil {
						result.CacheFiles = append(result.CacheFiles, CacheEntry{Name: e.Name(), Size: size})
					}
					continue
				}

				// Match regular cache files (e.g., "git#2.43.0#hash.zip")
				if !strings.HasPrefix(nameLower, prefix) {
					continue
				}
				// Verify this is actually our app's cache file, not a prefix match
				// (e.g., "git#" should NOT match "git-lfs#3.2.0").
				// After the prefix, there should be a version number or '#'.
				if prefixLen < len(nameLower) {
					nextChar := nameLower[prefixLen]
					// If next char is a letter, this is a different app (e.g., git-lfs)
					if nextChar >= 'a' && nextChar <= 'z' {
						continue
					}
				}
				// Keep cache file for current version.
				if strings.Contains(e.Name(), "#"+currentVersion+"#") || strings.HasSuffix(e.Name(), "#"+currentVersion) {
					continue
				}
				cacheFile := filepath.Join(cacheDir, e.Name())
				var size int64
				if fi, err := e.Info(); err == nil {
					size = fi.Size()
				}
				if opts.DryRun {
					result.CacheFiles = append(result.CacheFiles, CacheEntry{Name: e.Name(), Size: size})
					continue
				}
				if err := os.Remove(cacheFile); err == nil {
					result.CacheFiles = append(result.CacheFiles, CacheEntry{Name: e.Name(), Size: size})
				}
			}
		}
	}

	return result
}

// CleanupAll removes .download temp files for all apps.
// This is called after cleanup to ensure temp files are removed.
func (s *CleanupService) CleanupAll(scope scoop.InstallScope) error {
	paths := scoop.ResolvePaths(scope)
	cacheDir := paths.Cache

	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		return err
	}

	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		// Remove all .download temp files.
		if strings.HasSuffix(e.Name(), ".download") {
			downloadFile := filepath.Join(cacheDir, e.Name())
			_ = os.Remove(downloadFile) // Best effort
		}
	}

	return nil
}

// getDirectorySize computes the total size of all files in a directory tree.
func getDirectorySize(dirPath string) int64 {
	var total int64
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return 0
	}
	for _, e := range entries {
		if e.IsDir() {
			path := filepath.Join(dirPath, e.Name())
			total += getDirectorySize(path)
		} else {
			if fi, err := e.Info(); err == nil {
				total += fi.Size()
			}
		}
	}
	return total
}
