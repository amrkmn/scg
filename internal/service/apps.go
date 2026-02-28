package service

import (
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"go.noz.one/scg/internal/scoop"
)

// AppContext is a minimal interface for what services need from app.Context.
// This avoids circular imports between app and service packages.
type AppContext interface {
	GetLogger() Logger
	GetVerbose() bool
}

// Logger is the subset of app.Logger used by services.
type Logger interface {
	Log(msg string)
	Info(msg string)
	Success(msg string)
	Warn(msg string)
	Error(msg string)
	Verbose(msg string)
	Header(msg string)
	Newline()
}

// InstalledApp holds information about a single installed Scoop application.
type InstalledApp struct {
	Name        string
	Scope       scoop.InstallScope
	AppDir      string
	CurrentPath string
	Version     string
	Bucket      string
	Updated     time.Time
	Held        bool
}

// appsCache is the module-level cache for ListInstalled.
var appsCache struct {
	mu      sync.Mutex
	entries []InstalledApp
	expiry  time.Time
}

const appsCacheTTL = 30 * time.Second

// AppsService provides operations on installed Scoop applications.
type AppsService struct {
	ctx AppContext
}

// NewAppsService creates an AppsService.
func NewAppsService(ctx AppContext) *AppsService {
	return &AppsService{ctx: ctx}
}

// ListInstalled returns all installed apps, optionally filtered by name substring.
// Results are cached for 30 seconds.
func (s *AppsService) ListInstalled(filter string) ([]InstalledApp, error) {
	appsCache.mu.Lock()
	if time.Now().Before(appsCache.expiry) && filter == "" {
		result := make([]InstalledApp, len(appsCache.entries))
		copy(result, appsCache.entries)
		appsCache.mu.Unlock()
		return result, nil
	}
	appsCache.mu.Unlock()

	var all []InstalledApp
	for _, paths := range scoop.BothScopes() {
		apps, err := scanAppsDir(paths)
		if err != nil {
			continue
		}
		all = append(all, apps...)
	}

	// Sort: name ascending, user scope before global.
	sort.Slice(all, func(i, j int) bool {
		if all[i].Name != all[j].Name {
			return all[i].Name < all[j].Name
		}
		return all[i].Scope == scoop.ScopeUser
	})

	// Cache unfiltered results.
	if filter == "" {
		appsCache.mu.Lock()
		appsCache.entries = all
		appsCache.expiry = time.Now().Add(appsCacheTTL)
		appsCache.mu.Unlock()
		return all, nil
	}

	// Apply filter.
	var filtered []InstalledApp
	lf := filter
	for _, a := range all {
		if containsFold(a.Name, lf) {
			filtered = append(filtered, a)
		}
	}
	return filtered, nil
}

// GetAppPrefix returns the resolved path of the app's current/ directory.
func (s *AppsService) GetAppPrefix(name string, scope scoop.InstallScope) (string, error) {
	paths := scoop.ResolvePaths(scope)
	currentLink := filepath.Join(paths.Apps, name, "current")
	resolved, err := filepath.EvalSymlinks(currentLink)
	if err != nil {
		return "", err
	}
	return resolved, nil
}

// InvalidateCache clears the apps cache.
func (s *AppsService) InvalidateCache() {
	appsCache.mu.Lock()
	appsCache.expiry = time.Time{}
	appsCache.mu.Unlock()
}

// scanAppsDir reads all installed apps in a single scope's apps directory.
func scanAppsDir(paths scoop.ScoopPaths) ([]InstalledApp, error) {
	entries, err := os.ReadDir(paths.Apps)
	if err != nil {
		return nil, err
	}

	var apps []InstalledApp
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if name == "scoop" {
			continue // skip scoop itself
		}
		app, err := readAppInfo(name, paths)
		if err != nil {
			continue
		}
		apps = append(apps, app)
	}
	return apps, nil
}

// readAppInfo reads metadata for a single installed app.
func readAppInfo(name string, paths scoop.ScoopPaths) (InstalledApp, error) {
	app := InstalledApp{Name: name, Scope: paths.Scope, AppDir: filepath.Join(paths.Apps, name)}

	// Resolve current/ junction/symlink.
	currentLink := filepath.Join(paths.Apps, name, "current")
	resolved, err := filepath.EvalSymlinks(currentLink)
	if err != nil {
		return app, err
	}
	app.CurrentPath = resolved
	app.Version = filepath.Base(resolved)

	// Read install.json.
	installPath := filepath.Join(resolved, "install.json")
	if info, err := scoop.ReadInstallInfo(installPath); err == nil {
		app.Bucket = info.Bucket
		app.Held = info.Hold
	}

	// Read mtime of the version directory.
	if fi, err := os.Stat(resolved); err == nil {
		app.Updated = fi.ModTime()
	}

	return app, nil
}

func containsFold(s, sub string) bool {
	if sub == "" {
		return true
	}
	sLower := toLower(s)
	subLower := toLower(sub)
	return contains(sLower, subLower)
}

func toLower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + 32
		}
	}
	return string(b)
}

func contains(s, sub string) bool {
	if len(sub) == 0 {
		return true
	}
	if len(sub) > len(s) {
		return false
	}
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
