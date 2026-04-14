package service

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"go.noz.one/scg/internal/scoop"
)

// CacheFileEntry represents a cached installer file.
type CacheFileEntry struct {
	Name    string
	App     string
	Version string
	Size    int64
	ModTime time.Time
	Scope   string // "user" or "global"
}

// CacheResult holds the result of a cache operation.
type CacheResult struct {
	Entries      []CacheFileEntry
	FilesFound   int
	TotalSize    int64
	FilesRemoved int
	BytesFreed   int64
	Errors       []string
}

// CacheService provides operations on Scoop's download cache.
type CacheService struct {
	ctx AppContext
}

// NewCacheService creates a CacheService.
func NewCacheService(ctx AppContext) *CacheService {
	return &CacheService{ctx: ctx}
}

// ListCache returns all cache entries matching the given app patterns.
// If apps is nil or empty, returns all entries.
func (s *CacheService) ListCache(apps []string) (CacheResult, error) {
	paths := scoop.ResolvePaths(scoop.ScopeUser)
	cacheDir := paths.Cache

	// Also check global cache
	globalPaths := scoop.ResolvePaths(scoop.ScopeGlobal)
	globalCacheDir := globalPaths.Cache

	var result CacheResult

	entries, err := s.scanCacheDir(cacheDir, apps)
	if err != nil && !os.IsNotExist(err) {
		result.Errors = append(result.Errors, "user cache: "+err.Error())
	}
	for _, e := range entries {
		e.Scope = "user"
		result.Entries = append(result.Entries, e)
	}

	if globalCacheDir != cacheDir {
		entries, err := s.scanCacheDir(globalCacheDir, apps)
		if err != nil && !os.IsNotExist(err) {
			result.Errors = append(result.Errors, "global cache: "+err.Error())
		}
		for _, e := range entries {
			e.Scope = "global"
			result.Entries = append(result.Entries, e)
		}
	}

	// Sort by app name, then version
	sort.Slice(result.Entries, func(i, j int) bool {
		if result.Entries[i].App != result.Entries[j].App {
			return result.Entries[i].App < result.Entries[j].App
		}
		return result.Entries[i].Version < result.Entries[j].Version
	})

	result.FilesFound = len(result.Entries)
	for _, e := range result.Entries {
		result.TotalSize += e.Size
	}

	return result, nil
}

// RemoveCache removes cache entries matching the given app patterns.
// Use "*" for all apps. Returns removed count and bytes freed.
func (s *CacheService) RemoveCache(apps []string, scope scoop.InstallScope, dryRun bool) (CacheResult, error) {
	paths := scoop.ResolvePaths(scope)
	cacheDir := paths.Cache

	var result CacheResult

	entries, err := s.scanCacheDir(cacheDir, apps)
	if err != nil && !os.IsNotExist(err) {
		return result, err
	}

	scopeLabel := string(scope)
	for _, e := range entries {
		e.Scope = scopeLabel
		if dryRun {
			result.Entries = append(result.Entries, e)
			result.FilesRemoved++
			result.BytesFreed += e.Size
		} else {
			cacheFile := filepath.Join(cacheDir, e.Name)
			if err := os.Remove(cacheFile); err != nil {
				result.Errors = append(result.Errors, e.Name+": "+err.Error())
			} else {
				// Also remove associated .txt manifest file if exists
				txtFile := cacheFile + ".txt"
				_ = os.Remove(txtFile) // Ignore error if doesn't exist

				result.Entries = append(result.Entries, e)
				result.FilesRemoved++
				result.BytesFreed += e.Size
			}
		}
	}

	result.FilesFound = len(entries)

	return result, nil
}

// scanCacheDir reads cache entries from a directory, filtering by app patterns.
func (s *CacheService) scanCacheDir(cacheDir string, apps []string) ([]CacheFileEntry, error) {
	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		return nil, err
	}

	var result []CacheFileEntry
	for _, e := range entries {
		if e.IsDir() {
			continue
		}

		name := e.Name()

		// Skip .txt manifest files - they're companion files
		if strings.HasSuffix(name, ".txt") {
			continue
		}

		entry, err := parseCacheEntry(name)
		if err != nil {
			// Skip files that don't match the expected pattern
			continue
		}

		// Apply filter
		if len(apps) > 0 && !matchesAnyApp(entry.App, apps) {
			continue
		}

		// Get file info
		if fi, err := e.Info(); err == nil {
			entry.Size = fi.Size()
			entry.ModTime = fi.ModTime()
		}

		result = append(result, entry)
	}

	return result, nil
}

// parseCacheEntry parses a cache filename into its components.
// Format: app#version#hash.ext or app#version#hash
var cacheEntryRegex = regexp.MustCompile(`^(.+?)#(.+?)#([^#]+)$`)

func parseCacheEntry(name string) (CacheFileEntry, error) {
	// Try with extension
	matches := cacheEntryRegex.FindStringSubmatch(name)
	if matches == nil {
		return CacheFileEntry{}, os.ErrInvalid
	}

	return CacheFileEntry{
		Name:    name,
		App:     matches[1],
		Version: matches[2],
	}, nil
}

// matchesAnyApp checks if the app name matches any of the patterns.
func matchesAnyApp(app string, patterns []string) bool {
	appLower := strings.ToLower(app)
	for _, pattern := range patterns {
		patternLower := strings.ToLower(pattern)

		// Wildcard matches everything
		if patternLower == "*" {
			return true
		}

		// Exact match
		if appLower == patternLower {
			return true
		}

		// Check if this is a different app that happens to start with the same prefix
		// e.g., "git" should not match "git-lfs"
		if strings.HasPrefix(appLower, patternLower+"-") {
			continue
		}

		// Simple substring match
		if strings.Contains(appLower, patternLower) {
			return true
		}
	}
	return false
}
