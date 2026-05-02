package service

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"go.noz.one/scg/internal/install"
	"go.noz.one/scg/internal/scoop"
)

type UninstallOptions struct {
	Scope  scoop.InstallScope
	Purge  bool
	DryRun bool
}

type UninstallResult struct {
	App      string
	Version  string
	Success  bool
	Error    error
	Duration time.Duration
}

type UninstallService struct {
	ctx       AppContext
	manifests *ManifestService
	apps      *AppsService
}

func NewUninstallService(ctx AppContext) *UninstallService {
	return &UninstallService{
		ctx:       ctx,
		manifests: NewManifestService(ctx),
		apps:      NewAppsService(ctx),
	}
}

func (s *UninstallService) Uninstall(apps []string, opts UninstallOptions) []UninstallResult {
	results := make([]UninstallResult, 0, len(apps))

	for _, app := range apps {
		start := time.Now()
		result := s.UninstallSingle(app, opts)
		result.Duration = time.Since(start)
		results = append(results, result)

		if result.Success {
			s.ctx.GetLogger().Success(fmt.Sprintf("'%s' was uninstalled.", result.App))
			s.ctx.GetLogger().Verbose(fmt.Sprintf("duration: %s", result.Duration.Round(time.Millisecond)))
		} else if result.Error != nil {
			s.ctx.GetLogger().Error(fmt.Sprintf("%s: %v", result.App, result.Error))
		}
	}

	return results
}

func (s *UninstallService) UninstallSingle(appInput string, opts UninstallOptions) UninstallResult {
	result := UninstallResult{App: appInput}

	paths := scoop.ResolvePaths(opts.Scope)
	appDir := filepath.Join(paths.Apps, appInput)
	currentLink := filepath.Join(appDir, "current")

	versionDir, err := resolveCurrentVersionDir(appInput, opts.Scope)
	if err != nil {
		result.Error = fmt.Errorf("'%s' isn't installed", appInput)
		return result
	}

	version := filepath.Base(versionDir)
	result.Version = version

	manifestPath := filepath.Join(versionDir, "manifest.json")
	m, err := scoop.ReadManifest(manifestPath)
	if err != nil {
		s.ctx.GetLogger().Warn(fmt.Sprintf("Warning: could not read manifest for %s: %v", appInput, err))
		m = &scoop.Manifest{}
	}

	installInfoPath := filepath.Join(versionDir, "install.json")
	arch := detectArch()
	if info, err := scoop.ReadInstallInfo(installInfoPath); err == nil && info.Architecture != "" {
		arch = info.Architecture
	}

	persistDir := filepath.Join(paths.Root, "persist", appInput)
	scoopDir := paths.Root

	envVars := install.SetupHookEnvVars(
		currentLink, versionDir, version, arch, appInput,
		persistDir, scoopDir, "", opts.Scope == scoop.ScopeGlobal,
	)

	log := s.ctx.GetLogger().Log

	log(fmt.Sprintf("Uninstalling '%s' (%s).", appInput, version))

	preUninstall := joinAnyString(m.PreUninstall)
	if preUninstall != "" {
		if opts.DryRun {
			log("[dry-run] Would run pre_uninstall script")
		} else {
			logHookStepStart("Running pre_uninstall script")
			if err := install.RunHook("pre_uninstall", preUninstall, currentLink, envVars); err != nil {
				logHookStepError()
				result.Error = fmt.Errorf("pre_uninstall hook failed: %w", err)
				return result
			}
			logHookStepDone("Finished running pre_uninstall script.")
		}
	}

	runningProcs, err := install.FindRunningProcesses(currentLink)
	if err != nil {
		s.ctx.GetLogger().Warn(fmt.Sprintf("Warning: couldn't check for running processes: %v", err))
	} else if len(runningProcs) > 0 {
		s.ctx.GetLogger().Warn(fmt.Sprintf("Warning: %s is currently running. Close it before uninstalling.", appInput))
		for _, p := range runningProcs {
			s.ctx.GetLogger().Warn(fmt.Sprintf("  - %s (PID %d)", p.Name, p.PID))
		}
		if !opts.DryRun {
			result.Error = fmt.Errorf("%s is currently running", appInput)
			return result
		}
	}

	if m.Uninstaller != nil {
		if opts.DryRun {
			log("[dry-run] Would run uninstaller script")
		} else {
			logHookStepStart("Running uninstaller script")
			if err := install.RunInstallerHook("uninstaller", m.Uninstaller, currentLink, envVars); err != nil {
				logHookStepError()
				result.Error = fmt.Errorf("uninstaller failed: %w", err)
				return result
			}
			logHookStepDone("Finished running uninstaller script.")
		}
	}

	shims := collectUninstallShims(m, arch)
	if len(shims) > 0 {
		for _, def := range shims {
			log(fmt.Sprintf("Removing shim '%s.shim'.", def.Name))
			log(fmt.Sprintf("Removing shim '%s.exe'.", def.Name))
		}
		if opts.DryRun {
			log("[dry-run] Would remove shims")
		} else {
			if err := install.RemoveShims(shims, opts.Scope); err != nil {
				s.ctx.GetLogger().Warn(fmt.Sprintf("Warning: failed to remove some shims: %v", err))
			}
		}
	}

	shortcuts := install.ParseShortcutsField(m.Shortcuts)
	if len(shortcuts) > 0 {
		for _, shortcut := range shortcuts {
			shortcutPath := install.StartMenuShortcutPath(shortcut.Name, opts.Scope)
			if shortcutPath == "" {
				continue
			}
			log(fmt.Sprintf("Removing shortcut %s", displayPath(shortcutPath)))
		}
		if opts.DryRun {
			log("[dry-run] Would remove start menu shortcuts")
		} else {
			if err := install.RemoveStartMenuShortcuts(shortcuts, opts.Scope); err != nil {
				s.ctx.GetLogger().Warn(fmt.Sprintf("Warning: failed to remove shortcuts: %v", err))
			}
		}
	}

	log(fmt.Sprintf("Unlinking %s", displayPath(currentLink)))

	envAddPath := collectUninstallEnvAddPaths(m, arch, currentLink)
	if len(envAddPath) > 0 {
		pathTarget := "your"
		if opts.Scope == scoop.ScopeGlobal {
			pathTarget = "global"
		}
		for _, p := range envAddPath {
			log(fmt.Sprintf("Removing %s from %s path.", displayPath(p), pathTarget))
		}
		if opts.DryRun {
			log("[dry-run] Would remove from PATH")
		} else {
			if err := install.RemoveFromPath(envAddPath, opts.Scope); err != nil {
				s.ctx.GetLogger().Warn(fmt.Sprintf("Warning: failed to remove from PATH: %v", err))
			}
		}
	}

	envSet := collectUninstallEnvSet(m, arch)
	for key := range envSet {
		log(fmt.Sprintf("Removing environment variable '%s'.", key))
		if opts.DryRun {
			log("[dry-run] Would remove environment variable")
		} else {
			if err := install.RemoveEnvVar(key, opts.Scope); err != nil {
				s.ctx.GetLogger().Warn(fmt.Sprintf("Warning: failed to remove env var %s: %v", key, err))
			}
		}
	}

	persistItems := collectUninstallPersistItems(m, arch)
	for _, item := range persistItems {
		targetInApp := filepath.Join(currentLink, item)
		fi, err := os.Lstat(targetInApp)
		if err != nil {
			continue
		}
		if fi.Mode()&os.ModeSymlink != 0 || install.IsJunction(targetInApp) {
			if opts.DryRun {
				log(fmt.Sprintf("[dry-run] Would remove persist junction %s", displayPath(targetInApp)))
			} else {
				install.RemoveReadOnly(targetInApp)
				_ = os.Remove(targetInApp)
			}
		}
	}

	entries, _ := os.ReadDir(appDir)
	for _, entry := range entries {
		entryPath := filepath.Join(appDir, entry.Name())
		if entry.Name() != version && entry.Name() != "current" {
			log(fmt.Sprintf("Removing older version (%s).", entry.Name()))
			if opts.DryRun {
				log(fmt.Sprintf("[dry-run] Would remove older version %s", entry.Name()))
			} else {
				if err := os.RemoveAll(entryPath); err != nil {
					s.ctx.GetLogger().Warn(fmt.Sprintf("Warning: failed to remove %s: %v", entry.Name(), err))
				}
			}
		}
	}

	postUninstall := joinAnyString(m.PostUninstall)
	if postUninstall != "" {
		if opts.DryRun {
			log("[dry-run] Would run post_uninstall script")
		} else {
			logHookStepStart("Running post_uninstall script")
			if err := install.RunHook("post_uninstall", postUninstall, currentLink, envVars); err != nil {
				logHookStepError()
				s.ctx.GetLogger().Warn(fmt.Sprintf("Warning: post_uninstall hook failed: %v", err))
			}
			logHookStepDone("Finished running post_uninstall script.")
		}
	}

	if opts.DryRun {
		log(fmt.Sprintf("[dry-run] Would remove junction %s", displayPath(currentLink)))
		log(fmt.Sprintf("[dry-run] Would remove version dir %s", displayPath(versionDir)))
		entries, _ := os.ReadDir(appDir)
		if len(entries) <= 2 { // version dir + current
			log(fmt.Sprintf("[dry-run] Would remove app dir %s", displayPath(appDir)))
		}
	} else {
		install.RemoveReadOnly(currentLink)
		_ = os.Remove(currentLink)

		if err := os.RemoveAll(versionDir); err != nil {
			s.ctx.GetLogger().Warn(fmt.Sprintf("Couldn't remove '%s'; it may be in use. (%v)", displayPath(versionDir), err))
		}

		remaining, _ := os.ReadDir(appDir)
		if len(remaining) == 0 {
			_ = os.RemoveAll(appDir)
		}
	}

	if opts.Purge {
		log("Removing persisted data.")
		if opts.DryRun {
			log(fmt.Sprintf("[dry-run] Would remove persisted data in %s", displayPath(persistDir)))
		} else {
			if err := install.RemovePersistData(appInput, opts.Scope, true); err != nil {
				s.ctx.GetLogger().Warn(fmt.Sprintf("Couldn't remove '%s'; it may be in use. (%v)", displayPath(persistDir), err))
			}
		}
	}

	result.Success = true
	return result
}

func resolveCurrentVersionDir(appName string, scope scoop.InstallScope) (string, error) {
	return scoop.ResolveCurrentDir(appName, scope)
}

func collectUninstallShims(m *scoop.Manifest, arch string) []install.ShimDef {
	var shims []install.ShimDef
	if m.Bin != nil {
		shims = append(shims, install.ParseBinField(m.Bin, arch)...)
	}
	if m.Architecture != nil {
		archShims := install.ParseArchBinField(m.Architecture, arch)
		shims = append(shims, archShims...)
	}
	return shims
}

func collectUninstallEnvAddPaths(m *scoop.Manifest, arch, appCurrentDir string) []string {
	var paths []string
	if m.EnvAddPath != nil {
		paths = append(paths, install.EnvAddPaths(m.EnvAddPath, appCurrentDir)...)
	}
	if m.Architecture != nil {
		if archSection, ok := m.Architecture[arch]; ok {
			if section, ok := archSection.(map[string]any); ok {
				if v, ok := section["env_add_path"]; ok {
					paths = append(paths, install.EnvAddPaths(v, appCurrentDir)...)
				}
			}
		}
	}
	return paths
}

func collectUninstallEnvSet(m *scoop.Manifest, arch string) map[string]string {
	envSet := make(map[string]string)
	if m.EnvSet != nil {
		for k, v := range m.EnvSet {
			envSet[k] = v
		}
	}
	if m.Architecture != nil {
		archEnvSet := parseArchEnvSet(m.Architecture, arch)
		for k, v := range archEnvSet {
			envSet[k] = v
		}
	}
	return envSet
}

func collectUninstallPersistItems(m *scoop.Manifest, arch string) []string {
	var items []string
	if m.Persist != nil {
		items = append(items, install.ParsePersistField(m.Persist)...)
	}
	if m.Architecture != nil {
		archItems := parseArchPersist(m.Architecture, arch)
		items = append(items, archItems...)
	}
	return items
}
