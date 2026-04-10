package install

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
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

	prelude := buildHookPrelude(envVars)

	// Construct the full script.
	var fullScript strings.Builder
	if prelude != "" {
		fullScript.WriteString(prelude + "\n")
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

	// Use inherited console streams so child processes (e.g. python installers)
	// detect an interactive terminal and flush output progressively.
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s hook failed: %w", hookType, err)
	}

	return nil
}

// RunInstallerHook executes an installer or uninstaller from the manifest.
// Supported forms: script string/array, or object with script/file (+args).
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
		fileName, ok := v["file"].(string)
		if !ok || strings.TrimSpace(fileName) == "" {
			return fmt.Errorf("unsupported %s definition: expected script or file", hookType)
		}
		args, err := installerArgs(v["args"], envVars)
		if err != nil {
			return fmt.Errorf("invalid %s args: %w", hookType, err)
		}
		return runInstallerFile(hookType, appDir, fileName, args, envVars)
	default:
		return fmt.Errorf("unsupported %s definition type %T", hookType, installer)
	}

	return RunHook(hookType, script, appDir, envVars)
}

func runInstallerFile(hookType, appDir, fileName string, args []string, envVars map[string]string) error {
	installerPath := filepath.Join(appDir, filepath.FromSlash(fileName))
	if _, err := os.Stat(installerPath); err != nil {
		return fmt.Errorf("%s file not found: %s", hookType, installerPath)
	}

	var cmd *exec.Cmd
	if strings.EqualFold(filepath.Ext(installerPath), ".ps1") {
		psPath := FindPowerShell()
		if psPath == "" {
			return fmt.Errorf("no PowerShell found (tried pwsh.exe and powershell.exe)")
		}
		psArgs := append([]string{"-NoProfile", "-ExecutionPolicy", "Bypass", "-File", installerPath}, args...)
		cmd = exec.Command(psPath, psArgs...)
	} else {
		cmd = exec.Command(installerPath, args...)
	}

	cmd.Dir = appDir
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if len(envVars) > 0 {
		cmd.Env = os.Environ()
		for k, v := range envVars {
			cmd.Env = append(cmd.Env, k+"="+v)
		}
	}

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s hook failed: %w", hookType, err)
	}
	return nil
}

func installerArgs(raw any, vars map[string]string) ([]string, error) {
	switch v := raw.(type) {
	case nil:
		return nil, nil
	case string:
		if strings.TrimSpace(v) == "" {
			return nil, nil
		}
		return splitArgs(expandTemplateVars(v, vars)), nil
	case []string:
		out := make([]string, 0, len(v))
		for _, s := range v {
			out = append(out, expandTemplateVars(s, vars))
		}
		return out, nil
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			s, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("argument %v is not a string", item)
			}
			out = append(out, expandTemplateVars(s, vars))
		}
		return out, nil
	default:
		return nil, fmt.Errorf("unsupported args type %T", raw)
	}
}

func splitArgs(input string) []string {
	var args []string
	var current strings.Builder
	inQuotes := false

	for _, r := range input {
		switch {
		case r == '"':
			inQuotes = !inQuotes
		case (r == ' ' || r == '\t') && !inQuotes:
			if current.Len() > 0 {
				args = append(args, current.String())
				current.Reset()
			}
		default:
			current.WriteRune(r)
		}
	}
	if current.Len() > 0 {
		args = append(args, current.String())
	}
	return args
}

// escapePS escapes a string for use in a single-quoted PowerShell string.
func escapePS(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}

func buildHookPrelude(envVars map[string]string) string {
	if len(envVars) == 0 {
		return ""
	}

	keys := make([]string, 0, len(envVars))
	for k := range envVars {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	lines := make([]string, 0, len(keys)*2)
	for _, k := range keys {
		v := escapePS(envVars[k])
		lines = append(lines, fmt.Sprintf("$env:%s = '%s'", k, v))
		// Scoop hooks rely on regular PowerShell variables like $dir, not only $env:dir.
		lines = append(lines, fmt.Sprintf("Set-Variable -Name '%s' -Value '%s'", escapePS(k), v))
	}
	return strings.Join(lines, "\n")
}

// SetupHookEnvVars creates the standard environment variables for install hooks.
// This matches Scoop's variable set for pre_install/post_install scripts.
func SetupHookEnvVars(dir, originalDir, version, architecture, appName, persistDir, scoopDir, downloadFile string, isGlobal bool) map[string]string {
	env := map[string]string{
		// Core variables
		"dir":          dir,
		"original_dir": originalDir,
		"version":      version,
		"architecture": architecture,
		"arch":         architecture, // legacy alias
		"app":          appName,

		// Paths
		"persist_dir": persistDir,
		"scoopdir":    scoopDir,
		"cachedir":    joinPath(scoopDir, "cache"),
		"bucketsdir":  joinPath(scoopDir, "buckets"),

		// Download
		"fname": downloadFile,
	}
	if isGlobal {
		env["global"] = "true"
	}
	return env
}

func joinPath(base, sub string) string {
	if base == "" {
		return ""
	}
	return base + string('\\') + sub
}
