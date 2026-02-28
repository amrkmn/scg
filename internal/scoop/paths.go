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
func GetUserRoot() string {
	profile := os.Getenv("USERPROFILE")
	if profile == "" {
		profile = os.Getenv("HOME")
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
