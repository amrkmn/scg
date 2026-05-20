package service

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.noz.one/scg/internal/install"
	"go.noz.one/scg/internal/scoop"
)

// ResetOptions configures reset operations.
type ResetOptions struct {
	Scope          scoop.InstallScope
	GlobalExplicit bool
	All            bool
}

// ResetResult holds the outcome of resetting one app.
type ResetResult struct {
	App      string
	Version  string
	Success  bool
	Skipped  bool
	Error    error
	Duration time.Duration
}

// ResetService handles Scoop-compatible app reset operations.
type ResetService struct {
	ctx  AppContext
	apps *AppsService
}

// NewResetService creates a ResetService.
func NewResetService(ctx AppContext) *ResetService {
	return &ResetService{ctx: ctx, apps: NewAppsService(ctx)}
}

// Reset resets selected apps, or all installed apps when opts.All is true.
func (s *ResetService) Reset(inputs []string, opts ResetOptions) []ResetResult {
	if opts.Scope == "" {
		opts.Scope = scoop.ScopeUser
	}
	if opts.All || (len(inputs) == 1 && inputs[0] == "*") {
		installed, _ := s.apps.ListInstalled("")
		inputs = inputs[:0]
		for _, app := range installed {
			if strings.EqualFold(app.Name, "scoop") {
				continue
			}
			inputs = append(inputs, string(app.Scope)+"/"+app.Name)
		}
	}

	results := make([]ResetResult, 0, len(inputs))
	for _, input := range inputs {
		start := time.Now()
		result := s.ResetSingle(input, opts)
		result.Duration = time.Since(start)
		results = append(results, result)
		if result.Success {
			s.ctx.GetLogger().Done(result.App, fmt.Sprintf("reset %s", result.Version))
		} else if result.Skipped {
			s.ctx.GetLogger().Skip(result.App, "skipped")
		} else if result.Error != nil {
			s.ctx.GetLogger().Error(fmt.Sprintf("%s: %v", result.App, result.Error))
		}
	}
	return results
}

// ResetSingle resets one installed app.
func (s *ResetService) ResetSingle(input string, opts ResetOptions) ResetResult {
	appName, requestedVersion, requestedScope := parseResetInput(input)
	result := ResetResult{App: appName, Version: requestedVersion}
	if strings.EqualFold(appName, "scoop") {
		result.Skipped = true
		return result
	}

	scope := opts.Scope
	if requestedScope != "" {
		scope = requestedScope
	} else if !opts.GlobalExplicit {
		if appInstalledInScope(appName, scoop.ScopeGlobal) {
			scope = scoop.ScopeGlobal
		}
	}

	paths := scoop.ResolvePaths(scope)
	appDir := filepath.Join(paths.Apps, appName)
	if _, err := os.Stat(appDir); err != nil {
		result.Error = fmt.Errorf("isn't installed")
		return result
	}

	version := requestedVersion
	if version == "" {
		version = scoop.ResolveCurrentVersion(appName, scope)
	}
	if version == "" {
		result.Error = fmt.Errorf("current version cannot be determined")
		return result
	}
	result.Version = version

	versionDir := filepath.Join(appDir, version)
	manifestPath := filepath.Join(versionDir, "manifest.json")
	m, err := scoop.ReadManifest(manifestPath)
	if err != nil {
		result.Error = fmt.Errorf("'%s (%s)' isn't installed", appName, version)
		return result
	}

	info, _ := scoop.ReadInstallInfo(filepath.Join(versionDir, "install.json"))
	arch := info.Architecture
	if arch == "" {
		arch = detectArch()
	}

	currentLink := filepath.Join(appDir, "current")
	s.ctx.GetLogger().Header(fmt.Sprintf("Resetting %s %s", appName, version))
	s.ctx.GetLogger().Header("Linking current")
	if err := install.CreateJunction(currentLink, versionDir); err != nil {
		result.Error = fmt.Errorf("failed to link current version: %w", err)
		return result
	}

	binDefs := install.ParseBinField(m.Bin, arch)
	binDefs = append(binDefs, install.ParseArchBinField(m.Architecture, arch)...)
	if len(binDefs) > 0 {
		s.ctx.GetLogger().Header("Creating shims")
		if err := install.CreateShims(binDefs, currentLink, scope); err != nil {
			s.ctx.GetLogger().Warn(fmt.Sprintf("shim creation: %v", err))
		} else {
			s.ctx.GetLogger().Done("shims", fmt.Sprintf("%d created", len(binDefs)))
		}
	}

	shortcuts := install.ParseShortcutsField(m.Shortcuts)
	if len(shortcuts) > 0 {
		s.ctx.GetLogger().Header("Creating shortcuts")
		if err := install.CreateStartMenuShortcuts(shortcuts, currentLink, scope); err != nil {
			s.ctx.GetLogger().Warn(fmt.Sprintf("shortcuts: %v", err))
		} else {
			s.ctx.GetLogger().Done("shortcuts", fmt.Sprintf("%d created", len(shortcuts)))
		}
	}

	envPaths := collectUninstallEnvAddPaths(m, arch, currentLink)
	if len(envPaths) > 0 {
		s.ctx.GetLogger().Header("Updating PATH")
		_ = install.RemoveFromPath(envPaths, scope)
		if _, err := install.AddToPathWithResultNoBroadcast(envPaths, scope); err != nil {
			s.ctx.GetLogger().Warn(fmt.Sprintf("PATH update: %v", err))
		} else {
			s.ctx.GetLogger().Done("path", fmt.Sprintf("%d entries", len(envPaths)))
		}
	}

	envVars := collectUninstallEnvSet(m, arch)
	for key := range envVars {
		_ = install.RemoveEnvVar(key, scope)
	}
	if len(envVars) > 0 {
		s.ctx.GetLogger().Header("Setting environment")
		persistDir := filepath.Join(paths.Root, "persist", appName)
		expandedEnv := install.SetupHookEnvVars(currentLink, versionDir, version, arch, appName, persistDir, paths.Root, "", scope == scoop.ScopeGlobal)
		envVars = install.ExpandEnvSetVars(envVars, expandedEnv)
		if err := install.SetEnvVarsNoBroadcast(envVars, scope); err != nil {
			s.ctx.GetLogger().Warn(fmt.Sprintf("env set batch: %v", err))
		} else {
			s.ctx.GetLogger().Done("env", fmt.Sprintf("%d variable(s)", len(envVars)))
		}
	}

	persistItems := collectResetPersistItems(m, arch)
	for _, item := range persistItems {
		unlinkPersistSource(filepath.Join(versionDir, item.Source))
	}
	if len(persistItems) > 0 {
		s.ctx.GetLogger().Header("Persisting data")
		if err := install.SetupPersistData(appName, persistItems, currentLink, scope); err != nil {
			s.ctx.GetLogger().Warn(fmt.Sprintf("persist setup: %v", err))
		} else {
			s.ctx.GetLogger().Done("persist", fmt.Sprintf("%d item(s)", len(persistItems)))
		}
	}

	install.BroadcastEnvironmentChange()
	result.Success = true
	return result
}

func parseResetInput(input string) (appName, version string, scope scoop.InstallScope) {
	input = strings.ReplaceAll(input, `\`, "/")
	if parts := strings.SplitN(input, "/", 2); len(parts) == 2 {
		switch strings.ToLower(parts[0]) {
		case "global":
			scope = scoop.ScopeGlobal
			input = parts[1]
		case "user":
			scope = scoop.ScopeUser
			input = parts[1]
		default:
			input = parts[1]
		}
	}
	if parts := strings.SplitN(input, "@", 2); len(parts) == 2 {
		appName, version = parts[0], parts[1]
	} else {
		appName = input
	}
	return strings.TrimSuffix(appName, ".json"), version, scope
}

func appInstalledInScope(appName string, scope scoop.InstallScope) bool {
	paths := scoop.ResolvePaths(scope)
	_, err := os.Stat(filepath.Join(paths.Apps, appName))
	return err == nil
}

func collectResetPersistItems(m *scoop.Manifest, arch string) []install.PersistItem {
	items := install.ParsePersistItems(m.Persist)
	if m.Architecture != nil {
		items = append(items, parseArchPersist(m.Architecture, arch)...)
	}
	return items
}

func unlinkPersistSource(path string) {
	fi, err := os.Lstat(path)
	if err != nil {
		return
	}
	if fi.Mode()&os.ModeSymlink != 0 || install.IsJunction(path) {
		install.RemoveReadOnly(path)
		_ = os.Remove(path)
	}
}
