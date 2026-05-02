package scoop

import (
	"os"
	"path/filepath"
)

// InstallScope represents whether an app/bucket is installed for the current user or globally.
type InstallScope string

const (
	ScopeUser   InstallScope = "user"
	ScopeGlobal InstallScope = "global"
)

// ScoopPaths holds the resolved directory paths for a given Scoop installation scope.
type ScoopPaths struct {
	Scope   InstallScope
	Root    string
	Apps    string
	Shims   string
	Buckets string
	Cache   string
}

// GetUserRoot returns the user-scoped Scoop root directory (%USERPROFILE%\scoop).
// Falls back to \scoop if USERPROFILE and HOME are both empty (unusual on Windows).
func GetUserRoot() string {
	profile := os.Getenv("USERPROFILE")
	if profile == "" {
		profile = os.Getenv("HOME")
	}
	if profile == "" {
		// Fallback for unusual environments; operations may fail later
		return filepath.Join(string(os.PathSeparator), "scoop")
	}
	return filepath.Join(profile, "scoop")
}

// GetGlobalRoot returns the global Scoop root directory (C:\ProgramData\scoop).
func GetGlobalRoot() string {
	return `C:\ProgramData\scoop`
}

// ResolvePaths builds a ScoopPaths struct for the given scope.
func ResolvePaths(scope InstallScope) ScoopPaths {
	var root string
	if scope == ScopeGlobal {
		root = GetGlobalRoot()
	} else {
		root = GetUserRoot()
	}
	return ScoopPaths{
		Scope:   scope,
		Root:    root,
		Apps:    filepath.Join(root, "apps"),
		Shims:   filepath.Join(root, "shims"),
		Buckets: filepath.Join(root, "buckets"),
		Cache:   filepath.Join(root, "cache"),
	}
}

// BothScopes returns ScoopPaths for both user and global scopes.
func BothScopes() []ScoopPaths {
	return []ScoopPaths{
		ResolvePaths(ScopeUser),
		ResolvePaths(ScopeGlobal),
	}
}

// ScopeExists reports whether the Scoop root for the given scope exists on disk.
func ScopeExists(scope InstallScope) bool {
	p := ResolvePaths(scope)
	_, err := os.Stat(p.Root)
	return err == nil
}

// ResolveCurrentDir resolves the current version directory for an installed app.
// It follows Scoop's Select-CurrentVersion logic:
//  1. Read the current/ junction target via os.Readlink (primary)
//  2. If the junction can't be read (NO_JUNCTION or regular directory),
//     fall back to reading manifest.json from the current/ directory
//
// Returns the resolved directory path or an error if the version cannot be determined.
func ResolveCurrentDir(appName string, scope InstallScope) (string, error) {
	paths := ResolvePaths(scope)
	currentLink := filepath.Join(paths.Apps, appName, "current")

	resolved, err := os.Readlink(currentLink)
	if err == nil {
		return resolved, nil
	}

	manifestPath := filepath.Join(currentLink, "manifest.json")
	if m, err := ReadManifest(manifestPath); err == nil && m.Version != "" {
		return filepath.Join(paths.Apps, appName, m.Version), nil
	}

	return "", os.ErrNotExist
}

// ResolveCurrentVersion resolves the current version string for an installed app.
// Returns the version string (e.g., "24.12.0") or empty string if not determinable.
func ResolveCurrentVersion(appName string, scope InstallScope) string {
	dir, err := ResolveCurrentDir(appName, scope)
	if err != nil {
		return ""
	}
	return filepath.Base(dir)
}
