package scoop

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolvePaths(t *testing.T) {
	t.Run("user scope", func(t *testing.T) {
		paths := ResolvePaths(ScopeUser)
		if paths.Scope != ScopeUser {
			t.Errorf("Scope = %q, want %q", paths.Scope, ScopeUser)
		}
		if !filepath.IsAbs(paths.Root) {
			t.Errorf("Root should be absolute: %s", paths.Root)
		}
		if paths.Apps == "" {
			t.Error("Apps should not be empty")
		}
		if paths.Shims == "" {
			t.Error("Shims should not be empty")
		}
		if paths.Buckets == "" {
			t.Error("Buckets should not be empty")
		}
		if paths.Cache == "" {
			t.Error("Cache should not be empty")
		}
	})

	t.Run("global scope", func(t *testing.T) {
		paths := ResolvePaths(ScopeGlobal)
		if paths.Scope != ScopeGlobal {
			t.Errorf("Scope = %q, want %q", paths.Scope, ScopeGlobal)
		}
		if paths.Root != `C:\ProgramData\scoop` {
			t.Errorf("Global root = %q, want %q", paths.Root, `C:\ProgramData\scoop`)
		}
	})
}

func TestBothScopes(t *testing.T) {
	scopes := BothScopes()
	if len(scopes) != 2 {
		t.Fatalf("BothScopes returned %d scopes, want 2", len(scopes))
	}
	if scopes[0].Scope != ScopeUser {
		t.Errorf("First scope = %q, want %q", scopes[0].Scope, ScopeUser)
	}
	if scopes[1].Scope != ScopeGlobal {
		t.Errorf("Second scope = %q, want %q", scopes[1].Scope, ScopeGlobal)
	}
}

func TestScopeExists(t *testing.T) {
	t.Run("global scope typically exists", func(t *testing.T) {
		// C:\ProgramData\scoop may or may not exist, just test no panic
		_ = ScopeExists(ScopeGlobal)
	})

	t.Run("invalid scope", func(t *testing.T) {
		// Verify it doesn't panic with a bad scope
		_ = ScopeExists("nonexistent")
	})
}

func TestGetGlobalRoot(t *testing.T) {
	root := GetGlobalRoot()
	if root != `C:\ProgramData\scoop` {
		t.Errorf("GetGlobalRoot() = %q, want %q", root, `C:\ProgramData\scoop`)
	}
}

func TestGetUserRoot(t *testing.T) {
	root := GetUserRoot()
	if !filepath.IsAbs(root) {
		t.Errorf("GetUserRoot() should return absolute path, got %q", root)
	}
	profile := os.Getenv("USERPROFILE")
	if profile != "" {
		expected := filepath.Join(profile, "scoop")
		if root != expected {
			t.Errorf("GetUserRoot() = %q, want %q", root, expected)
		}
	}
}
