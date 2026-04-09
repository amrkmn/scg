package install

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
	"syscall"
)

// RunHook executes a PowerShell pre_install or post_install script.
// It tries pwsh.exe first, falling back to powershell.exe.
// On failure, the installation is stopped (no rollback).
func RunHook(hookType, script, appDir string, envVars map[string]string) error {
	if script == "" {
		return nil // No hook to run.
	}

	psPath := FindPowerShell()
	if psPath == "" {
		return fmt.Errorf("no PowerShell found (tried pwsh.exe and powershell.exe)")
	}

	// Build environment variables.
	envLines := make([]string, 0, len(envVars))
	for k, v := range envVars {
		envLines = append(envLines, fmt.Sprintf("$env:%s = '%s'", k, escapePS(v)))
	}
	envBlock := strings.Join(envLines, "\n")

	// Construct the full script.
	var fullScript strings.Builder
	if envBlock != "" {
		fullScript.WriteString(envBlock + "\n")
	}
	fullScript.WriteString(script)

	// Execute PowerShell.
	cmd := exec.Command(psPath,
		"-NoProfile",
		"-ExecutionPolicy", "Bypass",
		"-Command", fullScript.String(),
	)
	cmd.Dir = appDir
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s hook failed: %w\nstdout: %s\nstderr: %s",
			hookType, err, stdout.String(), stderr.String())
	}

	return nil
}

// RunInstallerHook executes an installer or uninstaller script from the manifest.
// The installer field can be a string or a map with "script" key.
func RunInstallerHook(hookType string, installer any, appDir string, envVars map[string]string) error {
	var script string

	switch v := installer.(type) {
	case string:
		script = v
	case []any:
		parts := make([]string, 0, len(v))
		for _, line := range v {
			if s, ok := line.(string); ok {
				parts = append(parts, s)
			}
		}
		script = strings.Join(parts, "\n")
	case map[string]any:
		if s, ok := v["script"]; ok {
			return RunInstallerHook(hookType, s, appDir, envVars)
		}
	default:
		return nil
	}

	return RunHook(hookType, script, appDir, envVars)
}

// escapePS escapes a string for use in a single-quoted PowerShell string.
func escapePS(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}

// SetupHookEnvVars creates the standard environment variables for install hooks.
func SetupHookEnvVars(dir, version, architecture string, isGlobal bool) map[string]string {
	env := map[string]string{
		"dir":     dir,
		"version": version,
		"arch":    architecture,
	}
	if isGlobal {
		env["global"] = "true"
	}
	return env
}
