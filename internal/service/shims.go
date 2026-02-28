package service

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/amrkmn/scg/internal/scoop"
)

// ShimService finds executables in Scoop shims and PATH.
type ShimService struct {
	ctx AppContext
}

// NewShimService creates a ShimService.
func NewShimService(ctx AppContext) *ShimService {
	return &ShimService{ctx: ctx}
}

// FindExecutable looks for an executable by name, searching:
// 1. Scoop shim directories (both scopes)
// 2. PATH directories
// 3. where.exe fallback
func (s *ShimService) FindExecutable(name string) ([]string, error) {
	candidates := candidatesForName(name)
	var found []string

	// 1. Shim directories.
	for _, paths := range scoop.BothScopes() {
		for _, candidate := range candidates {
			if path, ok := fileExistsCaseInsensitive(paths.Shims, candidate); ok {
				// Try to resolve the shim to its actual binary.
				resolved := resolveShimTarget(path, paths)
				if resolved != "" {
					found = append(found, resolved)
				} else {
					found = append(found, path)
				}
			}
		}
	}
	if len(found) > 0 {
		return found, nil
	}

	// 2. PATH directories.
	pathDirs := strings.Split(os.Getenv("PATH"), string(os.PathListSeparator))
	for _, dir := range pathDirs {
		if dir == "" {
			continue
		}
		for _, candidate := range candidates {
			if path, ok := fileExistsCaseInsensitive(dir, candidate); ok {
				found = append(found, path)
			}
		}
	}
	if len(found) > 0 {
		return found, nil
	}

	// 3. Fallback: where.exe
	out, err := exec.Command("where.exe", name).Output()
	if err == nil {
		lines := strings.Split(strings.TrimSpace(string(out)), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line != "" {
				found = append(found, line)
			}
		}
	}

	if len(found) == 0 {
		return nil, os.ErrNotExist
	}
	return found, nil
}

// resolveShimTarget attempts to find the actual binary that a shim points to.
// Scoop shims live in <root>/shims/ and the real binary is in apps/<app>/current/.
func resolveShimTarget(shimPath string, paths scoop.ScoopPaths) string {
	// Scoop shim files (.shim or .cmd) contain the target path.
	shimFile := shimPath
	// Try reading a .shim file (Scoop 3+).
	shimContent := shimPath[:len(shimPath)-len(filepath.Ext(shimPath))] + ".shim"
	if data, err := os.ReadFile(shimContent); err == nil {
		// Format: path = "C:\path\to\binary.exe"
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "path") {
				parts := strings.SplitN(line, "=", 2)
				if len(parts) == 2 {
					target := strings.Trim(strings.TrimSpace(parts[1]), `"`)
					if _, err := os.Stat(target); err == nil {
						return target
					}
				}
			}
		}
	}

	// Walk apps/<app>/current/ directories looking for the binary name.
	binName := filepath.Base(shimFile)
	entries, err := os.ReadDir(paths.Apps)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		currentPath := filepath.Join(paths.Apps, e.Name(), "current")
		resolved, err := filepath.EvalSymlinks(currentPath)
		if err != nil {
			continue
		}
		if path, ok := fileExistsCaseInsensitive(resolved, binName); ok {
			return path
		}
		// Also check one level of subdirectories.
		subEntries, _ := os.ReadDir(resolved)
		for _, se := range subEntries {
			if se.IsDir() {
				if path, ok := fileExistsCaseInsensitive(filepath.Join(resolved, se.Name()), binName); ok {
					return path
				}
			}
		}
	}
	return ""
}

// candidatesForName returns the set of filenames to search for a given command name.
// If name already has an extension, only that name is returned.
// Otherwise, name + each PATHEXT extension is returned.
func candidatesForName(name string) []string {
	if filepath.Ext(name) != "" {
		return []string{name}
	}
	exts := getPathExts()
	candidates := make([]string, 0, len(exts))
	for _, ext := range exts {
		candidates = append(candidates, name+ext)
	}
	return candidates
}

// fileExistsCaseInsensitive checks whether a file named `file` exists in `dir`.
// It first tries an exact match, then falls back to a case-insensitive directory scan.
func fileExistsCaseInsensitive(dir, file string) (string, bool) {
	exact := filepath.Join(dir, file)
	if _, err := os.Stat(exact); err == nil {
		return exact, true
	}
	// Case-insensitive scan.
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", false
	}
	lower := strings.ToLower(file)
	for _, e := range entries {
		if strings.ToLower(e.Name()) == lower {
			return filepath.Join(dir, e.Name()), true
		}
	}
	return "", false
}

// getPathExts returns the list of executable extensions from %PATHEXT%.
func getPathExts() []string {
	pathext := os.Getenv("PATHEXT")
	if pathext == "" {
		pathext = ".COM;.EXE;.BAT;.CMD;.VBS;.VBE;.JS;.JSE;.WSF;.WSH;.MSC"
	}
	parts := strings.Split(pathext, ";")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, strings.ToLower(p))
		}
	}
	return result
}
