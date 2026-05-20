package install

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"go.noz.one/scg/internal/scoop"
)

// PersistItem maps an app-relative source path to its persist directory name.
type PersistItem struct {
	Source string
	Target string
}

// SetupPersistData creates persistence junctions for an app.
// For each item in the persist list, it creates a directory in the persist
// directory and a junction from the app's current directory to it.
func SetupPersistData(appName string, persistItems []PersistItem, appCurrentDir string, scope scoop.InstallScope) error {
	paths := scoop.ResolvePaths(scope)
	persistDir := filepath.Join(paths.Root, "persist", appName)

	// Create the persist directory if it doesn't exist.
	if err := os.MkdirAll(persistDir, 0o755); err != nil {
		return fmt.Errorf("failed to create persist directory %s: %w", persistDir, err)
	}

	for _, item := range persistItems {
		if item.Target == "" {
			item.Target = item.Source
		}
		targetInApp := filepath.Join(appCurrentDir, item.Source)
		persistPath := filepath.Join(persistDir, item.Target)

		// Determine if this item is a file or directory.
		isDir := true
		if fi, err := os.Stat(persistPath); err == nil {
			isDir = fi.IsDir()
		} else if fi, err := os.Stat(targetInApp); err == nil {
			isDir = fi.IsDir()
		} else if filepath.Ext(item.Source) != "" {
			// Heuristic: items with extensions are treated as files.
			isDir = false
		}

		if isDir {
			// If the data already exists in the app directory but not in persist,
			// move it to persist first (so it survives updates).
			if fi, err := os.Stat(targetInApp); err == nil && fi.IsDir() {
				if _, err := os.Stat(persistPath); err != nil {
					if err := os.Rename(targetInApp, persistPath); err != nil {
						return fmt.Errorf("failed to move %s to persist: %w", item.Source, err)
					}
				}
			}

			if err := os.MkdirAll(persistPath, 0o755); err != nil {
				return fmt.Errorf("failed to create persist directory %s: %w", persistPath, err)
			}
		} else {
			// Ensure parent directory exists for files.
			parentDir := filepath.Dir(persistPath)
			if err := os.MkdirAll(parentDir, 0o755); err != nil {
				return fmt.Errorf("failed to create persist parent %s: %w", parentDir, err)
			}

			// If file data exists in app but not in persist, move it first.
			if fi, err := os.Stat(targetInApp); err == nil && !fi.IsDir() {
				if _, err := os.Stat(persistPath); os.IsNotExist(err) {
					if err := os.Rename(targetInApp, persistPath); err != nil {
						return fmt.Errorf("failed to move %s to persist: %w", item.Source, err)
					}
				}
			}

			// Ensure the persist file exists (create empty file if needed).
			f, err := os.OpenFile(persistPath, os.O_CREATE|os.O_RDWR, 0o644)
			if err != nil {
				return fmt.Errorf("failed to create persist file %s: %w", persistPath, err)
			}
			if err := f.Close(); err != nil {
				return fmt.Errorf("failed to close persist file %s: %w", persistPath, err)
			}
		}

		// Remove the existing item in the app directory (could be a junction, directory, or file).
		_ = os.RemoveAll(targetInApp)

		if err := os.MkdirAll(filepath.Dir(targetInApp), 0o755); err != nil {
			return fmt.Errorf("failed to create app parent path %s: %w", filepath.Dir(targetInApp), err)
		}

		if isDir {
			// Create a junction from the app directory to the persist directory.
			if err := CreateJunction(targetInApp, persistPath); err != nil {
				return fmt.Errorf("failed to create persist junction for %s: %w", item.Source, err)
			}
		} else {
			// Create a hardlink from the app directory to the persist file.
			if err := os.Link(persistPath, targetInApp); err != nil {
				// Fall back to symlink to preserve persistence semantics.
				if symlinkErr := os.Symlink(persistPath, targetInApp); symlinkErr != nil {
					return fmt.Errorf("failed to create persist link for %s (hardlink: %v, symlink: %w)", item.Source, err, symlinkErr)
				}
			}
		}
	}

	if err := EnsurePersistACL(scope); err != nil {
		return err
	}

	return nil
}

// EnsurePersistACL grants normal users write access to global persisted data.
// This mirrors Scoop's persist_permission behavior and is only needed for global
// installs, where persisted files live under C:\ProgramData.
func EnsurePersistACL(scope scoop.InstallScope) error {
	if scope != scoop.ScopeGlobal || !isRunningAsAdmin() {
		return nil
	}

	paths := scoop.ResolvePaths(scope)
	persistRoot := filepath.Join(paths.Root, "persist")
	if err := os.MkdirAll(persistRoot, 0o755); err != nil {
		return fmt.Errorf("failed to create persist directory %s: %w", persistRoot, err)
	}

	cmd := exec.Command("icacls", persistRoot, "/grant", "*S-1-5-32-545:(OI)(W)")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to set persist ACL on %s: %w (%s)", persistRoot, err, strings.TrimSpace(string(out)))
	}

	return nil
}

func isRunningAsAdmin() bool {
	psPath := FindPowerShell()
	if psPath == "" {
		return false
	}
	out, err := exec.Command(psPath, "-NoProfile", "-Command", "([Security.Principal.WindowsPrincipal] [Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)").Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) == "True"
}

// RemovePersistData removes the persist data for an app.
// If purge is true, the actual persist directory is removed.
func RemovePersistData(appName string, scope scoop.InstallScope, purge bool) error {
	paths := scoop.ResolvePaths(scope)
	persistDir := filepath.Join(paths.Root, "persist", appName)

	if purge {
		if err := os.RemoveAll(persistDir); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove persist directory %s: %w", persistDir, err)
		}
	}

	return nil
}

// ParsePersistField parses the manifest's persist field into a list of item names.
// The persist field can be a string, []any (where items can be strings or arrays),
// or nil.
func ParsePersistField(persist any) []string {
	return ParsePersistFieldFromItems(ParsePersistItems(persist))
}

// ParsePersistFieldFromItems returns app-relative source paths from parsed persist items.
func ParsePersistFieldFromItems(items []PersistItem) []string {
	if len(items) == 0 {
		return nil
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, item.Source)
	}
	return out
}

// ParsePersistItems parses the manifest's persist field including rename pairs.
func ParsePersistItems(persist any) []PersistItem {
	if persist == nil {
		return nil
	}

	var items []PersistItem

	switch v := persist.(type) {
	case string:
		items = append(items, PersistItem{Source: v, Target: v})
	case []any:
		for _, item := range v {
			switch iv := item.(type) {
			case string:
				items = append(items, PersistItem{Source: iv, Target: iv})
			case []any:
				// Format: ["sourceDir", "persistName"]
				if len(iv) > 0 {
					if s, ok := iv[0].(string); ok {
						p := PersistItem{Source: s, Target: s}
						if len(iv) > 1 {
							if target, ok := iv[1].(string); ok && target != "" {
								p.Target = target
							}
						}
						items = append(items, p)
					}
				}
			}
		}
	}

	return items
}
