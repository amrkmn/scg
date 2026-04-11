package install

import (
	"fmt"
	"os"
	"path/filepath"
)

type InstallScope = string

func CreateJunction(link, target string) error {
	if _, err := os.Lstat(link); err == nil {
		nativeRemoveReadOnly(link)
		if err := os.RemoveAll(link); err != nil {
			return fmt.Errorf("failed to remove existing junction %s: %w", link, err)
		}
	}

	if err := os.MkdirAll(target, 0o755); err != nil {
		return fmt.Errorf("failed to create target directory %s: %w", target, err)
	}

	if err := nativeCreateJunction(link, target); err != nil {
		return fmt.Errorf("failed to create junction %s => %s: %w", link, target, err)
	}

	nativeSetReadOnly(link)
	return nil
}

func RemoveJunction(link string) error {
	nativeRemoveReadOnly(link)
	return os.RemoveAll(link)
}

func ResolveCurrentDir(appName string, scope InstallScope, useJunction bool) string {
	paths := ResolvePathsHelper(scope)
	if useJunction {
		return filepath.Join(paths.Apps, appName, "current")
	}
	link := filepath.Join(paths.Apps, appName, "current")
	resolved, err := filepath.EvalSymlinks(link)
	if err != nil {
		return link
	}
	return resolved
}

type pathsHelper struct {
	Apps string
}

func ResolvePathsHelper(scope InstallScope) pathsHelper {
	return pathsHelper{Apps: filepath.Join(getUserRoot(), "apps")}
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
