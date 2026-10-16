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
		if err := createSingleShim(def, appCurrentDir, paths.Shims); err != nil {
			return fmt.Errorf("failed to create shim for %q: %w", def.Name, err)
		}
	}
	return nil
}

// DetectShimOverwrites returns warning messages for shims that will overwrite an existing shim.
func DetectShimOverwrites(shims []ShimDef, appCurrentDir string, scope scoop.InstallScope) []string {
	paths := scoop.ResolvePaths(scope)
	warnings := make([]string, 0)
	seen := make(map[string]string)

	for _, def := range shims {
		absTarget, err := resolveTarget(def.Target, appCurrentDir)
		if err != nil {
			continue
		}

		key := strings.ToLower(def.Name)
		if prior, ok := seen[key]; ok && !samePath(prior, absTarget) {
			warnings = append(warnings, fmt.Sprintf("Overwriting duplicate shim in manifest ('%s.exe' -> '%s')", def.Name, filepath.Base(def.Target)))
		}
		seen[key] = absTarget

		existing := filepath.Join(paths.Shims, def.Name+".shim")
		existingTarget, ok := readShimPath(existing)
		if !ok || samePath(existingTarget, absTarget) {
			continue
		}

		oldApp := appFromTargetPath(existingTarget)
		msg := fmt.Sprintf("Overwriting shim ('%s.exe' -> '%s')", def.Name, filepath.Base(def.Target))
		if oldApp != "" {
			msg += fmt.Sprintf(" installed from %s", oldApp)
		}
		warnings = append(warnings, msg)
	}

	return warnings
}

func readShimPath(shimFilePath string) (string, bool) {
	data, err := os.ReadFile(shimFilePath)
	if err != nil {
		return "", false
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(strings.ToLower(line), "path = ") {
			continue
		}
		value := strings.TrimSpace(line[len("path = "):])
		value = strings.Trim(value, "\"")
		if value == "" {
			return "", false
		}
		return value, true
	}
	return "", false
}

func appFromTargetPath(target string) string {
	parts := strings.Split(filepath.Clean(target), string(filepath.Separator))
	for i := 0; i+2 < len(parts); i++ {
		if strings.EqualFold(parts[i], "apps") {
			return parts[i+1]
		}
	}
	return ""
}

func samePath(a, b string) bool {
	return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
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
func createSingleShim(def ShimDef, appCurrentDir, shimDir string) error {
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

	// Like Scoop, only real executables get the .exe+.shim pair.
	// Scripts get .cmd/.ps1 wrappers plus an extensionless sh wrapper.
	switch strings.ToLower(filepath.Ext(absTarget)) {
	case ".exe", ".com":
		return writeExeShim(def, absTarget, appCurrentDir, shimDir)
	default:
		return writeScriptShim(def, absTarget, appCurrentDir, shimDir)
	}
}

// writeExeShim writes the .shim file and copies the shim executable.
func writeExeShim(def ShimDef, absTarget, appCurrentDir, shimDir string) error {
	// Write the .shim file.
	shimFilePath := filepath.Join(shimDir, def.Name+".shim")
	shimContent := fmt.Sprintf("path = \"%s\"\n", absTarget)
	if def.Args != "" {
		// Expand Scoop variables using the APP directory (not the resolved target).
		expanded := expandShimArgs(def.Args, appCurrentDir)
		// Normalize to forward slashes like Scoop.
		shimContent += fmt.Sprintf("args = %s\n", filepath.ToSlash(expanded))
	}
	if err := os.WriteFile(shimFilePath, []byte(shimContent), 0o644); err != nil {
		return fmt.Errorf("failed to write .shim file: %w", err)
	}

	// Write the shim executable.
	shimExePath := filepath.Join(shimDir, def.Name+".exe")
	if err := writeShimExe(shimExePath); err != nil {
		return fmt.Errorf("failed to write shim exe: %w", err)
	}

	return nil
}

// expandShimArgs expands Scoop directory variables in shim args.
func expandShimArgs(args, appCurrentDir string) string {
	expanded := args
	expanded = strings.ReplaceAll(expanded, "$dir", appCurrentDir)
	expanded = strings.ReplaceAll(expanded, "$original_dir", appCurrentDir)
	return expanded
}

// writeScriptShim writes Scoop-compatible wrappers for non-executable targets
// (.ps1, .cmd/.bat, .jar, .py, ...): a .cmd wrapper plus an extensionless sh
// wrapper, and for .ps1 additionally a .ps1 proxy. No .exe/.shim pair, since
// the shim binary can only launch real executables.
func writeScriptShim(def ShimDef, absTarget, appCurrentDir, shimDir string) error {
	arg := expandShimArgs(def.Args, appCurrentDir)
	for name, content := range scriptShimFiles(def.Name, absTarget, arg, shimDir) {
		if err := os.WriteFile(filepath.Join(shimDir, name), []byte(content), 0o644); err != nil {
			return fmt.Errorf("failed to write shim wrapper %q: %w", name, err)
		}
	}
	return nil
}

// scriptShimFiles returns the wrapper files for a non-executable shim target,
// mirroring Scoop's shim() in lib/core.ps1. resolved must be absolute.
// .cmd/.ps1 files use CRLF; extensionless sh wrappers use LF with no trailing
// newline.
func scriptShimFiles(name, resolved, arg, shimDir string) map[string]string {
	files := make(map[string]string)
	ext := strings.ToLower(filepath.Ext(resolved))
	crlf := func(lines ...string) string { return strings.Join(lines, "\r\n") }
	lf := func(lines ...string) string { return strings.Join(lines, "\n") }

	switch ext {
	case ".ps1":
		// Reference the target relative to the shim dir when possible so
		// shims survive directory moves, like Scoop's Join-Path approach.
		pathRef := `"` + resolved + `"`
		if rel, err := filepath.Rel(shimDir, resolved); err == nil {
			pathRef = `Join-Path $PSScriptRoot "` + rel + `"`
		}
		files[name+".ps1"] = crlf(
			`# `+resolved,
			`$path = `+pathRef,
			`if ($MyInvocation.ExpectingInput) { $input | & $path `+arg+` @args } else { & $path `+arg+` @args }`,
			`exit $LASTEXITCODE`,
		)
		files[name+".cmd"] = crlf(
			`@rem `+resolved,
			`@echo off`,
			`where /q pwsh.exe`,
			`if %errorlevel% equ 0 (`,
			`    pwsh -noprofile -ex unrestricted -file "`+resolved+`" `+arg+` %*`,
			`) else (`,
			`    powershell -noprofile -ex unrestricted -file "`+resolved+`" `+arg+` %*`,
			`)`,
		)
		files[name] = lf(
			`#!/bin/sh`,
			`# `+resolved,
			`if command -v pwsh.exe > /dev/null 2>&1; then`,
			`    pwsh.exe -noprofile -ex unrestricted -file "`+resolved+`" `+arg+` "$@"`,
			`else`,
			`    powershell.exe -noprofile -ex unrestricted -file "`+resolved+`" `+arg+` "$@"`,
			`fi`,
		)
	case ".bat", ".cmd":
		files[name+".cmd"] = crlf(
			`@rem `+resolved,
			`@"`+resolved+`" `+arg+` %*`,
		)
		files[name] = lf(
			`#!/bin/sh`,
			`# `+resolved,
			`MSYS2_ARG_CONV_EXCL=/C cmd.exe /C "`+resolved+`" `+arg+` "$@"`,
		)
	case ".jar":
		parent := filepath.Dir(resolved)
		files[name+".cmd"] = crlf(
			`@rem `+resolved,
			`@pushd `+parent,
			`@java -jar "`+resolved+`" `+arg+` %*`,
			`@popd`,
		)
		files[name] = lf(
			`#!/bin/sh`,
			`# `+resolved,
			`if [ $WSL_INTEROP ]`,
			`then`,
			`  cd $(wslpath -u '`+parent+`')`,
			`else`,
			`  cd $(cygpath -u '`+parent+`')`,
			`fi`,
			`java.exe -jar "`+resolved+`" `+arg+` "$@"`,
		)
	case ".py":
		files[name+".cmd"] = crlf(
			`@rem `+resolved,
			`@python "`+resolved+`" `+arg+` %*`,
		)
		files[name] = lf(
			`#!/bin/sh`,
			`# `+resolved,
			`python.exe "`+resolved+`" `+arg+` "$@"`,
		)
	default:
		quoted := ""
		if arg != "" {
			quoted = `"` + arg + `"`
		}
		files[name+".cmd"] = crlf(
			`@rem `+resolved,
			`@echo off`,
			`bash -c "command -v wslpath >/dev/null"`,
			`if %errorlevel% equ 0 (`,
			`  bash "$(wslpath -u '`+resolved+`')" `+quoted+` %*`,
			`) else (`,
			`  set args=`+quoted+` %*`,
			`  setlocal enabledelayedexpansion`,
			`  if not "!args!"=="" set args=!args:"=""!`,
			`  bash -c "$(cygpath -u '`+resolved+`') !args!"`,
			`)`,
		)
		files[name] = lf(
			`#!/bin/sh`,
			`# `+resolved,
			`if [ $WSL_INTEROP ]`,
			`then`,
			`  "$(wslpath -u '`+resolved+`')" `+arg+` "$@"`,
			`else`,
			`  "$(cygpath -u '`+resolved+`')" `+arg+` "$@"`,
			`fi`,
		)
	}
	return files
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

		// Script targets (.ps1, .py, ...) get a .ps1 proxy and an
		// extensionless sh wrapper instead of an .exe (see writeScriptShim).
		_ = os.Remove(filepath.Join(paths.Shims, def.Name+".ps1"))
		_ = os.Remove(filepath.Join(paths.Shims, def.Name))
	}

	return nil
}

// writeShimExe copies the shim executable to the target path.
// It prefers our embedded Zig-built shim, then falls back to scoop's shim.
func writeShimExe(destPath string) error {
	// Prefer our embedded shim.
	if len(shimExe) > 0 {
		return os.WriteFile(destPath, shimExe, 0o755)
	}

	// Fall back to scoop's shim.
	scoopShimPath := findScoopShim()
	if scoopShimPath != "" {
		return copyFile(scoopShimPath, destPath)
	}

	return fmt.Errorf("no shim executable available")
}

// findScoopShim tries to locate scoop's kiennq shim.exe in the scoop installation.
func findScoopShim() string {
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
