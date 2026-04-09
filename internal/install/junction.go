package install

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

// InstallScope is an alias for scoop.InstallScope for convenience.
type InstallScope = string

// CreateJunction creates a directory junction from link to target.
// On Windows, this uses the mklink /j command. The junction is created
// read-only (like scoop) to prevent accidental deletion of the junction target.
func CreateJunction(link, target string) error {
	// Remove existing link if it exists.
	if _, err := os.Lstat(link); err == nil {
		// Try to remove the read-only attribute first.
		removeReadOnly(link)
		if err := os.RemoveAll(link); err != nil {
			return fmt.Errorf("failed to remove existing junction %s: %w", link, err)
		}
	}

	// Ensure target exists.
	if err := os.MkdirAll(target, 0o755); err != nil {
		return fmt.Errorf("failed to create target directory %s: %w", target, err)
	}

	// Create the junction using cmd /c mklink /j.
	cmd := exec.Command("cmd", "/c", "mklink", "/j",
		shellQuote(link), shellQuote(target))
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to create junction %s => %s: %w\n%s", link, target, err, out)
	}

	// Set read-only attribute on the junction (like scoop).
	setReadOnly(link)

	return nil
}

// RemoveJunction removes a directory junction. It first removes the read-only
// attribute, then removes the junction itself.
func RemoveJunction(link string) error {
	removeReadOnly(link)
	return os.RemoveAll(link)
}

// removeReadOnly removes the read-only attribute from a file or directory.
func removeReadOnly(path string) {
	cmd := exec.Command("attrib", "-R", "/L", path)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	_ = cmd.Run()
}

// setReadOnly sets the read-only attribute on a file or directory.
func setReadOnly(path string) {
	cmd := exec.Command("attrib", "+R", "/L", path)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	_ = cmd.Run()
}

// shellQuote wraps a path in quotes if it contains spaces.
func shellQuote(path string) string {
	if containsSpace(path) {
		return `"` + path + `"`
	}
	return path
}

func containsSpace(s string) bool {
	return strings.Contains(s, " ")
}

// ResolveCurrentDir returns the path to the "current" directory for an app
// (or the resolved version directory if not using junctions).
func ResolveCurrentDir(appName string, scope InstallScope, useJunction bool) string {
	paths := ResolvePathsHelper(scope)
	if useJunction {
		return filepath.Join(paths.Apps, appName, "current")
	}
	// Resolve the junction to the actual version directory.
	link := filepath.Join(paths.Apps, appName, "current")
	resolved, err := filepath.EvalSymlinks(link)
	if err != nil {
		return link // Fall back to the link path.
	}
	return resolved
}

// PathsHelper returns a minimal paths struct for junction helpers.
func ResolvePathsHelper(scope InstallScope) struct {
	Apps string
} {
	return struct{ Apps string }{Apps: filepath.Join(getUserRoot(), "apps")}
}

func getUserRoot() string {
	profile := os.Getenv("USERPROFILE")
	if profile == "" {
		profile = os.Getenv("HOME")
	}
	if profile == "" {
		return filepath.Join(string(os.PathSeparator), "scoop")
	}
	return filepath.Join(profile, "scoop")
}
