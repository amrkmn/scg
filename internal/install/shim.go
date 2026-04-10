package install

import (
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"go.noz.one/scg/internal/scoop"
)

//go:embed assets/shim.exe
var shimExe []byte

// ShimDef represents a single shim entry parsed from a manifest's bin field.
type ShimDef struct {
	// Target is the relative path to the executable within the app directory.
	Target string
	// Name is the shim name (the executable name placed in the shims directory).
	Name string
	// Args are default arguments to pass to the target.
	Args string
}

// CreateShims creates shim files for the given list of ShimDef entries.
// For each entry, it writes a .shim file and copies the shim executable.
func CreateShims(shims []ShimDef, appCurrentDir string, scope scoop.InstallScope) error {
	paths := scoop.ResolvePaths(scope)

	for _, def := range shims {
		if err := createSingleShim(def, appCurrentDir, paths.Shims, scope); err != nil {
			return fmt.Errorf("failed to create shim for %q: %w", def.Name, err)
		}
	}
	return nil
}

// resolveTarget resolves a shim target to an absolute path using Scoop's resolution order:
// 1. Relative to app directory ($dir\$target)
// 2. Absolute path (as-is)
// 3. PATH resolution (Get-Command equivalent)
func resolveTarget(target, appCurrentDir string) (string, error) {
	// 1. Try relative to app directory.
	rel := filepath.Join(appCurrentDir, target)
	if fi, err := os.Stat(rel); err == nil && !fi.IsDir() {
		return filepath.Abs(rel)
	}

	// 2. Try as absolute path.
	if filepath.IsAbs(target) {
		if fi, err := os.Stat(target); err == nil && !fi.IsDir() {
			return target, nil
		}
	}

	// 3. PATH resolution (exec.LookPath).
	if found, err := exec.LookPath(target); err == nil {
		return filepath.Abs(found)
	}

	// Fall back to relative path (may not exist, but preserves original behavior).
	return filepath.Abs(rel)
}

// createSingleShim creates a single shim pair (.shim file + .exe copy).
func createSingleShim(def ShimDef, appCurrentDir, shimDir string, scope scoop.InstallScope) error {
	// Resolve the target to an absolute path.
	// Scoop resolution order:
	// 1. "$dir\$target" (relative to app directory)
	// 2. "$target" (absolute path)
	// 3. Get-Command $target (PATH resolution)
	absTarget, err := resolveTarget(def.Target, appCurrentDir)
	if err != nil {
		return fmt.Errorf("failed to resolve target %q: %w", def.Target, err)
	}

	// Ensure the shim directory exists.
	if err := os.MkdirAll(shimDir, 0o755); err != nil {
		return fmt.Errorf("failed to create shim directory: %w", err)
	}

	// Write the .shim file.
	shimFilePath := filepath.Join(shimDir, def.Name+".shim")
	shimContent := fmt.Sprintf("path = %q\n", absTarget)
	if def.Args != "" {
		// Expand Scoop variables in args (scoop compat).
		appDir := filepath.Dir(absTarget) // .../apps/<app>/current
		expanded := def.Args
		expanded = strings.ReplaceAll(expanded, "$dir", appDir)
		// Derive original_dir from appDir: replace /current with /<version> if possible.
		// Since we don't know the version here, approximate with appDir.
		expanded = strings.ReplaceAll(expanded, "$original_dir", appDir)
		shimContent += fmt.Sprintf("args = %s\n", expanded)
	}
	if err := os.WriteFile(shimFilePath, []byte(shimContent), 0o644); err != nil {
		return fmt.Errorf("failed to write .shim file: %w", err)
	}

	// Write the shim executable.
	shimExePath := filepath.Join(shimDir, def.Name+".exe")
	if err := writeShimExe(shimExePath, scope); err != nil {
		return fmt.Errorf("failed to write shim exe: %w", err)
	}

	return nil
}

// RemoveShims removes all shim files for the given app name and bin definitions.
func RemoveShims(shims []ShimDef, scope scoop.InstallScope) error {
	paths := scoop.ResolvePaths(scope)

	for _, def := range shims {
		shimExePath := filepath.Join(paths.Shims, def.Name+".exe")
		shimFilePath := filepath.Join(paths.Shims, def.Name+".shim")

		// Remove .shim file.
		_ = os.Remove(shimFilePath)

		// Remove .exe file.
		_ = os.Remove(shimExePath)

		// Also try removing .cmd wrapper (scoop creates these too).
		cmdPath := filepath.Join(paths.Shims, def.Name+".cmd")
		_ = os.Remove(cmdPath)
	}

	return nil
}

// writeShimExe copies the shim executable to the target path.
// It first tries to use scoop's own shim.exe (for compatibility),
// then falls back to our embedded Zig-built shim.
func writeShimExe(destPath string, scope scoop.InstallScope) error {
	// Try scoop's shim first (for compatibility with existing installations).
	scoopShimPath := findScoopShim(scope)
	if scoopShimPath != "" {
		return copyFile(scoopShimPath, destPath)
	}

	// Fall back to our embedded shim.
	if len(shimExe) > 0 {
		return os.WriteFile(destPath, shimExe, 0o755)
	}

	return fmt.Errorf("no shim executable available")
}

// findScoopShim tries to locate scoop's kiennq shim.exe in the scoop installation.
func findScoopShim(scope scoop.InstallScope) string {
	// Check both scopes for scoop's shim.
	for _, s := range scoop.BothScopes() {
		candidate := filepath.Join(s.Apps, "scoop", "current", "supporting", "shims", "kiennq", "shim.exe")
		if fi, err := os.Stat(candidate); err == nil && !fi.IsDir() {
			return candidate
		}
	}
	return ""
}

// copyFile copies a file from src to dst.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()

	if _, err := out.ReadFrom(in); err != nil {
		return err
	}

	// Copy file permissions.
	fi, err := in.Stat()
	if err != nil {
		return err
	}
	return os.Chmod(dst, fi.Mode())
}

// shimName derives the shim name from a target path, stripping common executable extensions.
// "wush.exe" → "wush", "terraform.cmd" → "terraform", "script.ps1" → "script".
// Names without known extensions are returned as-is.
func shimName(target string) string {
	base := filepath.Base(target)
	ext := strings.ToLower(filepath.Ext(base))
	switch ext {
	case ".exe", ".cmd", ".bat", ".ps1":
		return base[:len(base)-len(ext)]
	}
	return base
}

// ParseBinField parses the manifest's bin field into a list of ShimDef entries.
// The bin field can be:
//   - string: "bin/app.exe" → {Target: "bin/app.exe", Name: "app"}
//   - []any: ["bin/app.exe", "alias"] → {Target: "bin/app.exe", Name: "alias"}
//   - []any: ["bin/app.exe", "alias", "--flag"] → {Target: "bin/app.exe", Name: "alias", Args: "--flag"}
//   - map: {"bin/app.exe": "alias"} → {Target: "bin/app.exe", Name: "alias"}
func ParseBinField(bin any, arch string) []ShimDef {
	if bin == nil {
		return nil
	}

	var defs []ShimDef

	switch v := bin.(type) {
	case string:
		defs = append(defs, ShimDef{
			Target: v,
			Name:   shimName(v),
		})
	case []any:
		for _, item := range v {
			defs = append(defs, parseBinItem(item)...)
		}
	case map[string]any:
		for target, alias := range v {
			name := shimName(target)
			if aliasStr, ok := alias.(string); ok && aliasStr != "" {
				name = aliasStr
			}
			defs = append(defs, ShimDef{
				Target: target,
				Name:   name,
			})
		}
	}

	return defs
}

// parseBinItem parses a single bin item which can be a string or an array.
func parseBinItem(item any) []ShimDef {
	switch v := item.(type) {
	case string:
		return []ShimDef{{Target: v, Name: shimName(v)}}
	case []any:
		if len(v) == 0 {
			return nil
		}
		target, _ := v[0].(string)
		if target == "" {
			return nil
		}

		name := shimName(target)
		args := ""

		if len(v) >= 2 {
			if n, ok := v[1].(string); ok && n != "" {
				name = n
			}
		}
		if len(v) >= 3 {
			if a, ok := v[2].(string); ok {
				args = a
			}
		}

		return []ShimDef{{Target: target, Name: name, Args: args}}
	default:
		return nil
	}
}

// ParseArchBinField parses architecture-specific bin entries from the architecture field.
func ParseArchBinField(archData map[string]any, arch string) []ShimDef {
	if archData == nil {
		return nil
	}

	archSection, ok := archData[arch]
	if !ok {
		return nil
	}

	section, ok := archSection.(map[string]any)
	if !ok {
		return nil
	}

	binField, ok := section["bin"]
	if !ok {
		return nil
	}

	return ParseBinField(binField, arch)
}
