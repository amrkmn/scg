package service

import (
	"fmt"
	"path/filepath"
	"time"

	"go.noz.one/scg/internal/install"
	"go.noz.one/scg/internal/scoop"
	"go.noz.one/scg/internal/ui"
)

// UpdateOptions configures an update operation.
type UpdateOptions struct {
	Scope       scoop.InstallScope
	Independent bool
	NoCache     bool
	SkipHash    bool
	Arch        string
	Proxy       string
	Force       bool
	All         bool
	DryRun      bool
	Quiet       bool
}

// AppUpdateResult holds the outcome of updating a single app.
type AppUpdateResult struct {
	App        string
	OldVersion string
	NewVersion string
	Success    bool
	Skipped    bool
	DryRun     bool
	Error      error
	Duration   time.Duration
}

// UpdateService orchestrates app updates (download new → uninstall old → install new).
type UpdateService struct {
	ctx         AppContext
	apps        *AppsService
	status      *StatusService
	manifests   *ManifestService
	installer   *InstallService
	uninstaller *UninstallService
}

// NewUpdateService creates an UpdateService.
func NewUpdateService(ctx AppContext) *UpdateService {
	return &UpdateService{
		ctx:         ctx,
		apps:        NewAppsService(ctx),
		status:      NewStatusService(ctx),
		manifests:   NewManifestService(ctx),
		installer:   NewInstallService(ctx),
		uninstaller: NewUninstallService(ctx),
	}
}

// Update updates one or more apps with the given options.
func (s *UpdateService) Update(apps []string, opts UpdateOptions) []AppUpdateResult {
	var targets []InstalledApp
	var failedLookups []AppUpdateResult

	if opts.All {
		installed, err := s.apps.ListInstalled("")
		if err != nil {
			s.ctx.GetLogger().Error(fmt.Sprintf("Failed to list installed apps: %v", err))
			return nil
		}
		for _, app := range installed {
			if opts.Scope == scoop.ScopeGlobal && app.Scope != scoop.ScopeGlobal {
				continue
			}
			targets = append(targets, app)
		}
	} else {
		for _, name := range apps {
			found := false
			var scopes []scoop.ScoopPaths
			if opts.Scope == scoop.ScopeGlobal {
				// Respect explicit global scope.
				scopes = append(scopes, scoop.ResolvePaths(scoop.ScopeGlobal))
			} else {
				// Default behavior: prefer user scope, then fall back to global.
				scopes = append(scopes, scoop.ResolvePaths(scoop.ScopeUser))
				scopes = append(scopes, scoop.ResolvePaths(scoop.ScopeGlobal))
			}
			for _, paths := range scopes {
				app, err := readAppInfo(name, paths)
				if err == nil {
					targets = append(targets, app)
					found = true
					break
				}
			}
			if !found {
				err := fmt.Errorf("'%s' isn't installed", name)
				s.ctx.GetLogger().Error(err.Error())
				failedLookups = append(failedLookups, AppUpdateResult{App: name, Error: err})
			}
		}
	}

	if len(targets) == 0 {
		if len(failedLookups) > 0 {
			return failedLookups
		}
		return nil
	}

	bucketsSvc := NewBucketService(s.ctx)
	buckets, err := bucketsSvc.List("")
	if err != nil {
		s.ctx.GetLogger().Error(fmt.Sprintf("Failed to list buckets: %v", err))
		return nil
	}

	bucketInfos := make([]BucketInfo, 0, len(buckets))
	bucketInfos = append(bucketInfos, buckets...)

	// Check status to find apps needing attention.
	statusResults := s.status.CheckStatus(targets, bucketInfos, nil)
	needsAttention := make(map[string]AppStatusResult)
	for _, sr := range statusResults {
		if sr.Outdated || sr.Failed || len(sr.MissingDeps) > 0 || opts.Force {
			needsAttention[appKey(sr.Name, sr.Scope)] = sr
		}
	}

	// When --all, only process apps needing attention.
	if opts.All {
		var filtered []InstalledApp
		for _, app := range targets {
			if _, ok := needsAttention[appKey(app.Name, app.Scope)]; ok {
				filtered = append(filtered, app)
			}
		}
		targets = filtered
	}

	// Print update plan before mutating installed apps.
	if !opts.Quiet {
		s.ctx.GetLogger().Header("Checking outdated apps")
		if opts.All {
			s.ctx.GetLogger().Log(fmt.Sprintf("You have %d outdated app(s) installed.", len(targets)))
		}

		if len(targets) == 1 {
			s.ctx.GetLogger().Header("Upgrading 1 outdated app")
		} else if len(targets) > 1 {
			s.ctx.GetLogger().Header(fmt.Sprintf("Upgrading %d outdated apps", len(targets)))
		}
		for _, app := range targets {
			if sr, ok := needsAttention[appKey(app.Name, app.Scope)]; ok {
				s.ctx.GetLogger().Log(ui.VersionChange(app.Name, sr.Installed, sr.Latest))
			}
		}
	}

	results := make([]AppUpdateResult, 0, len(targets)+len(failedLookups))
	results = append(results, failedLookups...)
	for _, app := range targets {
		start := time.Now()
		result := s.updateSingle(app, needsAttention[appKey(app.Name, app.Scope)], opts)
		result.Duration = time.Since(start)
		results = append(results, result)

		if result.Error != nil {
			s.ctx.GetLogger().Error(fmt.Sprintf("'%s' failed to update: %v", result.App, result.Error))
		}
	}

	return results
}

func (s *UpdateService) updateSingle(app InstalledApp, status AppStatusResult, opts UpdateOptions) AppUpdateResult {
	result := AppUpdateResult{App: app.Name, DryRun: opts.DryRun}

	if status.Name == "" && !opts.Force {
		result.Skipped = true
		if !opts.Quiet {
			s.ctx.GetLogger().Skip(app.Name, "up-to-date")
		}
		return result
	}

	if status.Failed {
		result.Skipped = true
		if !opts.Quiet {
			s.ctx.GetLogger().Warn(fmt.Sprintf("%s has a failed status; skipping update", app.Name))
		}
		return result
	}

	result.OldVersion = status.Installed
	result.NewVersion = status.Latest

	if !opts.Quiet {
		s.ctx.GetLogger().Header(fmt.Sprintf("Upgrading %s", app.Name))
		s.ctx.GetLogger().Log(fmt.Sprintf("%s -> %s", result.OldVersion, result.NewVersion))
	}

	// Resolve manifest and architecture to pre-download.
	_, appName := parseBucketAndApp(app.Name)

	// Use the original bucket from install.json to look up the manifest,
	// so the download URL/hash matches what InstallSingle will use.
	manifestLookup := app.Name
	if app.Bucket != "" {
		manifestLookup = fmt.Sprintf("%s/%s", app.Bucket, appName)
	}
	installed, bucket := s.manifests.FindManifestPair(manifestLookup)
	var m *scoop.Manifest
	var bucketName string
	if bucket != nil {
		m = bucket.Manifest
		bucketName = bucket.Bucket
	} else if installed != nil {
		m = installed.Manifest
		bucketName = installed.Bucket
	} else {
		result.Error = fmt.Errorf("manifest not found for %s", app.Name)
		return result
	}

	// Prefer the original bucket from install.json when available.
	if app.Bucket != "" {
		bucketName = app.Bucket
	}

	arch := opts.Arch
	if arch == "" {
		arch = detectArch()
	}

	dlURLs, err := resolveDownloadURLs(m, arch)
	if err != nil {
		result.Error = fmt.Errorf("failed to resolve download URLs: %w", err)
		return result
	}

	// --- Pre-download phase (Scoop downloads before uninstalling) ---
	if !opts.Quiet {
		s.ctx.GetLogger().Header(fmt.Sprintf("Fetching downloads for %s", app.Name))
	}

	dm := install.NewDownloadManager(app.Scope, s.ctx.GetVerbose())
	dlResults := make([]*install.DownloadResult, 0, len(dlURLs))

	if opts.DryRun {
		for _, dlURL := range dlURLs {
			cachePath := dm.CachePath(appName, m.Version, dlURL)
			s.ctx.GetLogger().Dry("download", filepath.Base(cachePath))
			s.ctx.GetLogger().Dry("hash", filepath.Base(cachePath))
		}
	} else {
		for _, dlURL := range dlURLs {
			dlResult, err := dm.Download(appName, m.Version, dlURL, !opts.NoCache, opts.Proxy)
			if err != nil {
				result.Error = fmt.Errorf("download failed: %w", err)
				return result
			}
			if dlResult.Downloaded {
				if !dlResult.UsedAria2 {
					s.ctx.GetLogger().Done("download", filepath.Base(dlResult.CachePath))
				}
			} else {
				s.ctx.GetLogger().Done("cache", filepath.Base(dlResult.CachePath))
			}
			dlResults = append(dlResults, dlResult)
		}

		// Verify hash before uninstalling.
		if !opts.SkipHash {
			expectedHashes := resolveHashes(m, arch)
			for i, dlResult := range dlResults {
				expectedHash := hashAt(expectedHashes, i)
				if expectedHash == "" {
					continue
				}
				hashFormat, parseErr := install.ParseHash(expectedHash)
				if parseErr != nil {
					result.Error = fmt.Errorf("failed to parse hash: %w", parseErr)
					return result
				}
				if hashFormat != nil {
					logStepStart("Checking hash")
					if err := install.VerifyHash(dlResult.CachePath, hashFormat); err != nil {
						logStepError()
						result.Error = fmt.Errorf("hash verification failed: %w", err)
						return result
					}
					logStepOK()
				}
			}
		}
	}

	// --- Uninstall old version ---
	uninstallResult := s.uninstaller.UninstallSingle(app.Name, UninstallOptions{
		Scope:  app.Scope,
		DryRun: opts.DryRun,
	})
	if !uninstallResult.Success {
		result.Error = fmt.Errorf("uninstall failed: %w", uninstallResult.Error)
		return result
	}

	// --- Install new version (skip re-download) ---
	installOpts := InstallOptions{
		Scope:        app.Scope,
		Independent:  opts.Independent,
		NoCache:      opts.NoCache,
		SkipHash:     true, // Already verified above
		Arch:         opts.Arch,
		Proxy:        opts.Proxy,
		DryRun:       opts.DryRun,
		SkipDownload: true,
		FromBucket:   bucketName,
	}
	installResult := s.installer.InstallSingle(app.Name, installOpts)
	if installResult.Error != nil && !installResult.Skipped {
		result.Error = fmt.Errorf("install failed: %w", installResult.Error)
		return result
	}
	result.NewVersion = installResult.Version

	if opts.DryRun {
		s.ctx.GetLogger().Dry(app.Name, fmt.Sprintf("would install %s", result.NewVersion))
	} else {
		s.ctx.GetLogger().Done(app.Name, fmt.Sprintf("upgraded to %s", result.NewVersion))
	}
	result.Success = true
	return result
}

func appKey(name string, scope scoop.InstallScope) string {
	return name + "/" + string(scope)
}
