// Package install provides the core installation logic for scg,
// implementing a Scoop-compatible install workflow in native Go.
package install

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"go.noz.one/scg/internal/scoop"
)

// FindHelper locates an external tool by checking Scoop's apps directory first,
// then falling back to the system PATH.
//
// name is the Scoop app name (e.g. "7zip", "aria2", "lessmsi").
// exe is the relative executable path within the app (e.g. "7z.exe", "aria2c.exe").
func FindHelper(name, exe string) (string, error) {
	// 1. Check user scope Scoop apps directory.
	for _, paths := range scoop.BothScopes() {
		candidate := filepath.Join(paths.Apps, name, "current", exe)
		if fi, err := os.Stat(candidate); err == nil && !fi.IsDir() {
			return candidate, nil
		}
	}

	// 2. Fall back to system PATH.
	path, err := exec.LookPath(exe)
	if err == nil {
		return path, nil
	}

	return "", fmt.Errorf("helper %q not found (tried scoop apps and PATH)", name)
}

// Find7zip returns the path to 7z.exe, checking Scoop's apps first.
func Find7zip() (string, error) {
	return FindHelper("7zip", "7z.exe")
}

// FindAria2 returns the path to aria2c.exe, checking Scoop's apps first.
func FindAria2() (string, error) {
	return FindHelper("aria2", "aria2c.exe")
}

// FindLessmsi returns the path to lessmsi.exe, checking Scoop's apps first.
func FindLessmsi() (string, error) {
	return FindHelper("lessmsi", "lessmsi.exe")
}

// FindInnounp returns the path to innounp.exe, checking Scoop's apps first.
func FindInnounp() (string, error) {
	// Try unicode variant first.
	if p, err := FindHelper("innounp-unicode", "innounp.exe"); err == nil {
		return p, nil
	}
	return FindHelper("innounp", "innounp.exe")
}

// FindPowerShell returns the path to pwsh.exe (PowerShell Core) if available,
// falling back to powershell.exe (Windows PowerShell).
func FindPowerShell() string {
	if p, err := exec.LookPath("pwsh.exe"); err == nil {
		return p
	}
	if p, err := exec.LookPath("powershell.exe"); err == nil {
		return p
	}
	return ""
}

// HelperAvailable is a convenience function that returns true if the named helper can be found.
func HelperAvailable(name, exe string) bool {
	_, err := FindHelper(name, exe)
	return err == nil
}

// IsScoopInstalled checks whether a Scoop installation exists on the system.
func IsScoopInstalled() bool {
	return scoop.ScopeExists(scoop.ScopeUser) || scoop.ScopeExists(scoop.ScopeGlobal)
}

// EnsureScoopInstalled returns an error if Scoop is not installed.
func EnsureScoopInstalled() error {
	if !IsScoopInstalled() {
		return fmt.Errorf("scoop installation not found; please install scoop first: https://scoop.sh")
	}
	return nil
}

// ExtractExtension returns the lower-cased file extension of a path.
func ExtractExtension(path string) string {
	return strings.ToLower(filepath.Ext(path))
}

// IsArchive returns true if the file extension looks like a supported archive format.
func IsArchive(path string) bool {
	ext := ExtractExtension(path)
	switch ext {
	case ".zip", ".7z", ".tar", ".gz", ".bz2", ".xz", ".tgz",
		".lzma", ".lz4", ".zst",
		".msi":
		return true
	default:
		if strings.HasSuffix(ext, ".tar") {
			return true
		}
		return false
	}
}
