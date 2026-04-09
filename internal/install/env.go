package install

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"go.noz.one/scg/internal/scoop"
)

// AddToPath adds directory paths to the user or system PATH environment variable.
// It uses reg.exe and PowerShell to modify the Windows Registry and broadcast changes.
func AddToPath(paths []string, scope scoop.InstallScope) error {
	if len(paths) == 0 {
		return nil
	}

	// Get current PATH from registry.
	currentPath, err := getRegistryPath(scope)
	if err != nil {
		currentPath = ""
	}

	// Normalize the current path entries for comparison.
	currentEntries := splitPath(currentPath)
	var additions []string

	for _, p := range paths {
		normalized := strings.ToLower(filepath.Clean(p))
		found := false
		for _, existing := range currentEntries {
			if strings.ToLower(filepath.Clean(existing)) == normalized {
				found = true
				break
			}
		}
		if !found {
			additions = append(additions, p)
		}
	}

	if len(additions) == 0 {
		return nil // Nothing to add.
	}

	// Append new paths.
	newPath := currentPath
	if newPath != "" && !strings.HasSuffix(newPath, ";") {
		newPath += ";"
	}
	newPath += strings.Join(additions, ";")

	if err := setRegistryPath(newPath, scope); err != nil {
		return fmt.Errorf("failed to set PATH: %w", err)
	}

	broadcastEnvironmentChange()
	return nil
}

// RemoveFromPath removes directory paths from the user or system PATH environment variable.
func RemoveFromPath(paths []string, scope scoop.InstallScope) error {
	if len(paths) == 0 {
		return nil
	}

	currentPath, err := getRegistryPath(scope)
	if err != nil {
		return nil // PATH doesn't exist, nothing to remove.
	}

	// Build a set of paths to remove (normalized for comparison).
	removeSet := make(map[string]bool)
	for _, p := range paths {
		removeSet[strings.ToLower(filepath.Clean(p))] = true
	}

	// Filter the current path entries.
	entries := splitPath(currentPath)
	var kept []string
	for _, entry := range entries {
		if !removeSet[strings.ToLower(filepath.Clean(entry))] {
			kept = append(kept, entry)
		}
	}

	newPath := strings.Join(kept, ";")
	if err := setRegistryPath(newPath, scope); err != nil {
		return fmt.Errorf("failed to set PATH: %w", err)
	}

	broadcastEnvironmentChange()
	return nil
}

// SetEnvVar sets a persistent environment variable using PowerShell.
func SetEnvVar(keyStr, value string, scope scoop.InstallScope) error {
	psPath := FindPowerShell()
	if psPath == "" {
		return fmt.Errorf("no PowerShell found")
	}

	// Use [Environment]::SetEnvironmentVariable which writes to the registry.
	scopeStr := "'User'"
	if scope == scoop.ScopeGlobal {
		scopeStr = "'Machine'"
	}

	script := fmt.Sprintf("[Environment]::SetEnvironmentVariable('%s', '%s', %s)", keyStr, value, scopeStr)
	cmd := exec.Command(psPath, "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", script)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to set env var %s: %w\n%s", keyStr, err, out)
	}

	broadcastEnvironmentChange()
	return nil
}

// RemoveEnvVar removes a persistent environment variable using PowerShell.
func RemoveEnvVar(keyStr string, scope scoop.InstallScope) error {
	psPath := FindPowerShell()
	if psPath == "" {
		return fmt.Errorf("no PowerShell found")
	}

	scopeStr := "'User'"
	if scope == scoop.ScopeGlobal {
		scopeStr = "'Machine'"
	}

	script := fmt.Sprintf("[Environment]::SetEnvironmentVariable('%s', $null, %s)", keyStr, scopeStr)
	cmd := exec.Command(psPath, "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", script)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to remove env var %s: %w\n%s", keyStr, err, out)
	}

	broadcastEnvironmentChange()
	return nil
}

// getRegistryPath reads the PATH from the Windows Registry using reg.exe.
func getRegistryPath(scope scoop.InstallScope) (string, error) {
	var args []string
	if scope == scoop.ScopeGlobal {
		args = []string{"query", `HKLM\SYSTEM\CurrentControlSet\Control\Session Manager\Environment`, "/v", "PATH"}
	} else {
		args = []string{"query", `HKCU\Environment`, "/v", "PATH"}
	}

	cmd := exec.Command("reg", args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}

	// Parse reg.exe output.
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "PATH") || strings.HasPrefix(line, "Path") {
			// Format: "    PATH    REG_EXPAND_SZ    value"
			parts := strings.SplitN(line, "REG_", 2)
			if len(parts) == 2 {
				// Find the value after the registry type.
				idx := strings.Index(parts[1], "    ")
				if idx >= 0 {
					return strings.TrimSpace(parts[1][idx+4:]), nil
				}
				// Try space separator.
				idx = strings.Index(parts[1], " ")
				if idx >= 0 {
					return strings.TrimSpace(parts[1][idx+1:]), nil
				}
			}
		}
	}

	return "", fmt.Errorf("PATH not found in registry output")
}

// setRegistryPath writes the PATH to the Windows Registry using PowerShell.
func setRegistryPath(pathValue string, scope scoop.InstallScope) error {
	psPath := FindPowerShell()
	if psPath == "" {
		return fmt.Errorf("no PowerShell found")
	}

	scopeStr := "'User'"
	if scope == scoop.ScopeGlobal {
		scopeStr = "'Machine'"
	}

	// Escape single quotes in the path value.
	escapedPath := strings.ReplaceAll(pathValue, "'", "''")
	script := fmt.Sprintf("[Environment]::SetEnvironmentVariable('PATH', '%s', %s)", escapedPath, scopeStr)
	cmd := exec.Command(psPath, "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", script)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to set PATH: %w\n%s", err, out)
	}

	return nil
}

// broadcastEnvironmentChange sends WM_SETTINGCHANGE to notify all applications.
func broadcastEnvironmentChange() {
	// Use PowerShell to broadcast the change.
	psPath := FindPowerShell()
	if psPath == "" {
		return
	}

	script := `Add-Type -TypeDefinition 'using System; using System.Runtime.InteropServices; public class Win32 { [DllImport(\"user32.dll\", SetLastError=true, CharSet=CharSet.Auto)] public static extern IntPtr SendMessageTimeout(IntPtr hWnd, uint Msg, IntPtr wParam, string lParam, uint fuFlags, uint uTimeout, out IntPtr lpdwResult); }'; [Win32]::SendMessageTimeout([IntPtr]0xffff, 0x001A, [IntPtr]::Zero, 'Environment', 2, 5000, [ref]([IntPtr]::Zero))`
	cmd := exec.Command(psPath, "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", script)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	_ = cmd.Run()
}

// splitPath splits a PATH string into individual entries.
func splitPath(pathStr string) []string {
	if pathStr == "" {
		return nil
	}
	parts := strings.Split(pathStr, ";")
	var result []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

// EnvAddPaths parses the manifest's env_add_path field for paths to add
// relative to the app directory.
func EnvAddPaths(envAddPath any, appDir string) []string {
	if envAddPath == nil {
		return nil
	}

	switch v := envAddPath.(type) {
	case string:
		return []string{filepath.Join(appDir, v)}
	case []any:
		var paths []string
		for _, item := range v {
			if s, ok := item.(string); ok {
				paths = append(paths, filepath.Join(appDir, s))
			}
		}
		return paths
	default:
		return nil
	}
}

// EnvSetVars parses the manifest's env_set field for environment variables.
func EnvSetVars(envSet map[string]string) map[string]string {
	if envSet == nil {
		return nil
	}
	result := make(map[string]string, len(envSet))
	for k, v := range envSet {
		result[k] = v
	}
	return result
}