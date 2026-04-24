package install

import (
	"fmt"
	"os"
	"path/filepath"

	"go.noz.one/scg/internal/scoop"
)

// SetupPersistData creates persistence junctions for an app.
// For each item in the persist list, it creates a directory in the persist
// directory and a junction from the app's current directory to it.
func SetupPersistData(appName string, persistItems []string, appCurrentDir string, scope scoop.InstallScope) error {
	paths := scoop.ResolvePaths(scope)
	persistDir := filepath.Join(paths.Root, "persist", appName)

	// Create the persist directory if it doesn't exist.
	if err := os.MkdirAll(persistDir, 0o755); err != nil {
		return fmt.Errorf("failed to create persist directory %s: %w", persistDir, err)
	}

	for _, item := range persistItems {
		targetInApp := filepath.Join(appCurrentDir, item)
		persistPath := filepath.Join(persistDir, item)

		// Determine if this item is a file or directory.
		isDir := true
		if fi, err := os.Stat(persistPath); err == nil {
			isDir = fi.IsDir()
		} else if fi, err := os.Stat(targetInApp); err == nil {
			isDir = fi.IsDir()
		} else if filepath.Ext(item) != "" {
			// Heuristic: items with extensions are treated as files.
			isDir = false
		}

		if isDir {
			// If the data already exists in the app directory but not in persist,
			// move it to persist first (so it survives updates).
			if fi, err := os.Stat(targetInApp); err == nil && fi.IsDir() {
				if _, err := os.Stat(persistPath); err != nil {
					if err := os.Rename(targetInApp, persistPath); err != nil {
						return fmt.Errorf("failed to move %s to persist: %w", item, err)
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
						return fmt.Errorf("failed to move %s to persist: %w", item, err)
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
				return fmt.Errorf("failed to create persist junction for %s: %w", item, err)
			}
		} else {
			// Create a hardlink from the app directory to the persist file.
			if err := os.Link(persistPath, targetInApp); err != nil {
				// Fall back to symlink to preserve persistence semantics.
				if symlinkErr := os.Symlink(persistPath, targetInApp); symlinkErr != nil {
					return fmt.Errorf("failed to create persist link for %s (hardlink: %v, symlink: %w)", item, err, symlinkErr)
				}
			}
		}
	}

	return nil
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
	if persist == nil {
		return nil
	}

	var items []string

	switch v := persist.(type) {
	case string:
		items = append(items, v)
	case []any:
		for _, item := range v {
			switch iv := item.(type) {
			case string:
				items = append(items, iv)
			case []any:
				// Format: ["sourceDir", "persistName"]
				// Use the source directory name.
				if len(iv) > 0 {
					if s, ok := iv[0].(string); ok {
						items = append(items, s)
					}
				}
			}
		}
	}

	return items
}
