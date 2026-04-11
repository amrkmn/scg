package install

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"go.noz.one/scg/internal/scoop"
)

func AddToPath(paths []string, scope scoop.InstallScope) error {
	_, err := addToPathWithResult(paths, scope, true)
	return err
}

func AddToPathWithResult(paths []string, scope scoop.InstallScope) ([]string, error) {
	return addToPathWithResult(paths, scope, true)
}

func AddToPathWithResultNoBroadcast(paths []string, scope scoop.InstallScope) ([]string, error) {
	return addToPathWithResult(paths, scope, false)
}

func addToPathWithResult(paths []string, scope scoop.InstallScope, broadcast bool) ([]string, error) {
	if len(paths) == 0 {
		return nil, nil
	}

	additions, currentPath := getPathAdditions(paths, scope)
	if len(additions) == 0 {
		return nil, nil
	}

	newPath := currentPath
	if newPath != "" && !strings.HasSuffix(newPath, ";") {
		newPath += ";"
	}
	newPath += strings.Join(additions, ";")

	if err := setRegistryPath(newPath, scope); err != nil {
		return nil, fmt.Errorf("failed to set PATH: %w", err)
	}

	if broadcast {
		BroadcastEnvironmentChange()
	}
	return additions, nil
}

func GetPathAdditions(paths []string, scope scoop.InstallScope) []string {
	additions, _ := getPathAdditions(paths, scope)
	return additions
}

func getPathAdditions(paths []string, scope scoop.InstallScope) ([]string, string) {
	currentPath, err := getRegistryPath(scope)
	if err != nil {
		currentPath = ""
	}
	return pathAdditions(paths, splitPath(currentPath)), currentPath
}

func pathAdditions(paths []string, currentEntries []string) []string {
	seen := make(map[string]struct{}, len(currentEntries))
	for _, existing := range currentEntries {
		seen[strings.ToLower(filepath.Clean(existing))] = struct{}{}
	}

	additions := make([]string, 0, len(paths))
	for _, p := range paths {
		normalized := strings.ToLower(filepath.Clean(p))
		if _, exists := seen[normalized]; exists {
			continue
		}
		additions = append(additions, p)
		seen[normalized] = struct{}{}
	}
	return additions
}

func RemoveFromPath(paths []string, scope scoop.InstallScope) error {
	if len(paths) == 0 {
		return nil
	}

	currentPath, err := getRegistryPath(scope)
	if err != nil {
		return nil
	}

	removeSet := make(map[string]bool)
	for _, p := range paths {
		removeSet[strings.ToLower(filepath.Clean(p))] = true
	}

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

	BroadcastEnvironmentChange()
	return nil
}

func SetEnvVar(keyStr, value string, scope scoop.InstallScope) error {
	if err := SetEnvVarNoBroadcast(keyStr, value, scope); err != nil {
		return err
	}
	BroadcastEnvironmentChange()
	return nil
}

func SetEnvVarNoBroadcast(keyStr, value string, scope scoop.InstallScope) error {
	scopeStr := scopeToStr(scope)
	return nativeSetRegistryEnvVar(keyStr, value, scopeStr)
}

func SetEnvVarsNoBroadcast(vars map[string]string, scope scoop.InstallScope) error {
	scopeStr := scopeToStr(scope)
	keys := make([]string, 0, len(vars))
	for k := range vars {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		if err := nativeSetRegistryEnvVar(k, vars[k], scopeStr); err != nil {
			return fmt.Errorf("failed to set env var %s: %w", k, err)
		}
	}
	return nil
}

func RemoveEnvVar(keyStr string, scope scoop.InstallScope) error {
	scopeStr := scopeToStr(scope)
	if err := nativeSetRegistryEnvVar(keyStr, "", scopeStr); err != nil {
		return fmt.Errorf("failed to remove env var %s: %w", keyStr, err)
	}
	BroadcastEnvironmentChange()
	return nil
}

func getRegistryPath(scope scoop.InstallScope) (string, error) {
	scopeStr := scopeToStr(scope)
	return nativeGetRegistryPath(scopeStr)
}

func setRegistryPath(pathValue string, scope scoop.InstallScope) error {
	scopeStr := scopeToStr(scope)
	return nativeSetRegistryPath(pathValue, scopeStr)
}

func BroadcastEnvironmentChange() {
	nativeBroadcastEnvironmentChange()
}

func scopeToStr(scope scoop.InstallScope) string {
	if scope == scoop.ScopeGlobal {
		return scoopScopeGlobal
	}
	return scoopScopeUser
}

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

func ExpandEnvSetVars(envSet map[string]string, vars map[string]string) map[string]string {
	if envSet == nil {
		return nil
	}
	out := make(map[string]string, len(envSet))
	for k, v := range envSet {
		out[k] = expandTemplateVars(v, vars)
	}
	return out
}

func expandTemplateVars(value string, vars map[string]string) string {
	if value == "" || len(vars) == 0 {
		return value
	}
	expanded := value
	keys := make([]string, 0, len(vars))
	for k := range vars {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if len(keys[i]) == len(keys[j]) {
			return keys[i] < keys[j]
		}
		return len(keys[i]) > len(keys[j])
	})
	for _, k := range keys {
		v := vars[k]
		expanded = strings.ReplaceAll(expanded, "$"+k, v)
		expanded = strings.ReplaceAll(expanded, "${"+k+"}", v)
	}
	return expanded
}
