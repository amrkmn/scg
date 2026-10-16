package service

import (
	"fmt"
	"go.noz.one/scg/internal/install"
	"go.noz.one/scg/internal/scoop"
	"os"
	"path/filepath"
)

func (s *InstallService) buildInstallPlan(m *scoop.Manifest, appName, bucketName, displayName, arch string, opts InstallOptions) (*installPlan, error) {
	paths := scoop.ResolvePaths(opts.Scope)
	appDir := filepath.Join(paths.Apps, appName)
	versionDir := filepath.Join(appDir, m.Version)

	binDefs := install.ParseBinField(m.Bin, arch)
	archBins := install.ParseArchBinField(m.Architecture, arch)
	binDefs = append(binDefs, archBins...)
	preInstall := getArchString(m, "pre_install", arch)
	currentLink := filepath.Join(appDir, "current")
	persistItems := install.ParsePersistItems(m.Persist)
	archPersist := parseArchPersist(m.Architecture, arch)
	persistItems = append(persistItems, archPersist...)
	envAddPath := mergeEnvAddPath(m.EnvAddPath, parseArchEnvAddPath(m.Architecture, arch))
	envPaths := install.EnvAddPaths(envAddPath, currentLink)
	envSet := mergeEnvSet(m.EnvSet, parseArchEnvSet(m.Architecture, arch))
	envVars := install.EnvSetVars(envSet)
	postInstall := getArchString(m, "post_install", arch)
	shortcuts := install.ParseShortcutsField(m.Shortcuts)

	currentDir := versionDir
	logger := s.ctx.GetLogger()

	// Resolve download URLs.
	dlURLs, err := resolveDownloadURLs(m, arch)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve download URLs: %w", err)
	}
	firstURL := dlURLs[0]

	// Build environment variables (after download so $fname is available).
	persistDir := filepath.Join(paths.Root, "persist", appName)
	downloadFile := install.DownloadFileName(appName, firstURL)
	preHookEnv := install.SetupHookEnvVars(
		versionDir,   // dir (version path before current is linked)
		versionDir,   // original_dir (version-specific)
		m.Version,    // version
		arch,         // architecture
		appName,      // app
		persistDir,   // persist_dir
		paths.Root,   // scoopdir
		downloadFile, // fname
		opts.Scope == scoop.ScopeGlobal,
	)
	expandedEnvVars := install.SetupHookEnvVars(
		currentLink,  // dir (current junction)
		versionDir,   // original_dir (version-specific)
		m.Version,    // version
		arch,         // architecture
		appName,      // app
		persistDir,   // persist_dir
		paths.Root,   // scoopdir
		downloadFile, // fname
		opts.Scope == scoop.ScopeGlobal,
	)
	envVars = install.ExpandEnvSetVars(envVars, expandedEnvVars)

	return &installPlan{
		m:               m,
		appName:         appName,
		bucketName:      bucketName,
		displayName:     displayName,
		arch:            arch,
		opts:            opts,
		logger:          logger,
		verbose:         s.ctx.GetVerbose(),
		paths:           paths,
		appDir:          appDir,
		versionDir:      versionDir,
		currentLink:     currentLink,
		currentDir:      currentDir,
		persistDir:      persistDir,
		downloadFile:    downloadFile,
		binDefs:         binDefs,
		preInstall:      preInstall,
		postInstall:     postInstall,
		persistItems:    persistItems,
		envPaths:        envPaths,
		envVars:         envVars,
		preHookEnv:      preHookEnv,
		expandedEnvVars: expandedEnvVars,
		shortcuts:       shortcuts,
		dlURLs:          dlURLs,
		firstURL:        firstURL,
	}, nil
}

func (s *InstallService) stageInstallPlan(plan *installPlan, quiet bool) (*installStaged, error) {
	m := plan.m
	appName := plan.appName
	arch := plan.arch
	opts := plan.opts
	versionDir := plan.versionDir
	dlURLs := plan.dlURLs
	log := &stepLogger{inner: plan.logger, quiet: quiet}

	// Download (or reuse cached files).
	dm := install.NewDownloadManager(opts.Scope, plan.verbose)
	dlResults := make([]*install.DownloadResult, 0, len(dlURLs))

	for _, dlURL := range dlURLs {
		if opts.SkipDownload {
			if existingPath, ok := dm.FindCachedPath(appName, m.Version, dlURL); ok {
				dlResults = append(dlResults, &install.DownloadResult{
					CachePath:  existingPath,
					Downloaded: false,
					UsedAria2:  false,
				})
				log.Done("cache", filepath.Base(existingPath))
			} else if opts.DryRun {
				// In dry-run the pre-download phase only simulated the download;
				// create a fake result so the rest of the flow can simulate too.
				cachePath := dm.CachePath(appName, m.Version, dlURL)
				dlResults = append(dlResults, &install.DownloadResult{
					CachePath:  cachePath,
					Downloaded: false,
					UsedAria2:  false,
				})
				log.Dry("cache", filepath.Base(cachePath))
			} else {
				return nil, fmt.Errorf("cached file not found for %s (%s)", appName, m.Version)
			}
		} else {
			// Determine if a download will actually happen (vs cache hit).
			willDownload := opts.NoCache
			cachePath := dm.CachePath(appName, m.Version, dlURL)
			if !willDownload {
				if existingPath, ok := dm.FindCachedPath(appName, m.Version, dlURL); ok {
					cachePath = existingPath
				} else {
					willDownload = true
				}
			}
			if !willDownload {
				log.Done("cache", filepath.Base(cachePath))
			}

			dlResult, err := dm.Download(appName, m.Version, dlURL, !opts.NoCache, opts.Proxy)
			if err != nil {
				return nil, fmt.Errorf("download failed: %w", err)
			}

			if dlResult.Downloaded {
				if !dlResult.UsedAria2 {
					log.Done("download", filepath.Base(dlResult.CachePath))
				}
			}
			dlResults = append(dlResults, dlResult)
		}
	}

	// Verify hash (unless --skip).
	if !opts.SkipHash {
		expectedHashes := resolveHashes(m, arch)
		for i, dlResult := range dlResults {
			expectedHash := hashAt(expectedHashes, i)
			if expectedHash == "" {
				continue
			}
			hashFormat, parseErr := install.ParseHash(expectedHash)
			if parseErr != nil {
				return nil, fmt.Errorf("failed to parse hash: %w", parseErr)
			}
			if hashFormat != nil {
				log.stepStart("Checking hash")
				if err := install.VerifyHash(dlResult.CachePath, hashFormat); err != nil {
					log.stepError()
					return nil, fmt.Errorf("hash verification failed: %w", err)
				}
				log.stepOK()
			}
		}
	}

	// Extract archives; plain executables are staged directly unless explicitly marked as Inno Setup.
	for i, dlResult := range dlResults {
		dlURL := dlURLs[i]
		if install.IsArchive(dlResult.CachePath) || isManifestInnoSetup(m, arch) {
			extractor := install.NewExtractor(false, plan.verbose)
			extractOpts := install.ExtractionOptions{
				InnoSetup: isManifestInnoSetup(m, arch),
				MSI:       isManifestMSI(dlURL),
			}
			if opts.DryRun {
				log.Dry("extract", fmt.Sprintf("%s to %s", filepath.Base(dlResult.CachePath), displayPath(versionDir)))
			} else {
				log.stepStart("Extracting " + filepath.Base(dlResult.CachePath))
				if err := extractor.Extract(dlResult.CachePath, versionDir, extractOpts); err != nil {
					log.stepError()
					return nil, fmt.Errorf("extraction failed: %w", err)
				}
				log.stepDone()
				log.Done("extract", displayPath(versionDir))
			}
		} else {
			if opts.DryRun {
				log.Dry("stage", fmt.Sprintf("%s in %s", filepath.Base(install.DownloadFileName(appName, dlURL)), displayPath(versionDir)))
			} else {
				if err := os.MkdirAll(versionDir, 0o755); err != nil {
					return nil, fmt.Errorf("failed to create version dir: %w", err)
				}
				dst := filepath.Join(versionDir, install.DownloadFileName(appName, dlURL))
				if err := copyFile(dlResult.CachePath, dst); err != nil {
					return nil, fmt.Errorf("failed to stage installer payload: %w", err)
				}
			}
		}
	}

	// Handle extract_to: move extracted contents into a subdirectory.
	if et := getArchString(m, "extract_to", arch); et != "" {
		targetDir := filepath.Join(versionDir, et)
		if opts.DryRun {
			log.Dry("extract_to", displayPath(targetDir))
		} else {
			if err := install.MoveContents(versionDir, targetDir); err != nil {
				return nil, fmt.Errorf("extract_to failed: %w", err)
			}
		}
	}

	// Flatten extract_dir if specified.
	if ed := getArchString(m, "extract_dir", arch); ed != "" {
		if opts.DryRun {
			log.Dry("extract_dir", ed)
		} else {
			if err := install.FlattenExtractDir(versionDir, ed); err != nil {
				return nil, fmt.Errorf("extract_dir failed: %w", err)
			}
		}
	}

	return &installStaged{dlResults: dlResults}, nil
}

func (s *InstallService) linkInstallPlan(plan *installPlan, staged *installStaged) InstallResult {
	m := plan.m
	appName := plan.appName
	opts := plan.opts
	versionDir := plan.versionDir
	bucketName := plan.bucketName
	displayName := plan.displayName
	arch := plan.arch
	currentLink := plan.currentLink
	currentDir := plan.currentDir
	binDefs := plan.binDefs
	preInstall := plan.preInstall
	postInstall := plan.postInstall
	persistItems := plan.persistItems
	envPaths := plan.envPaths
	envVars := plan.envVars
	preHookEnv := plan.preHookEnv
	expandedEnvVars := plan.expandedEnvVars
	shortcuts := plan.shortcuts
	_ = staged
	logger := plan.logger
	result := InstallResult{App: plan.displayName, Version: plan.m.Version}

	// Run pre_install hook.
	if preInstall != "" {
		if opts.DryRun {
			logger.Dry("pre_install", "script")
		} else {
			logHookStepStart("Running pre_install")
			if err := install.RunHook("pre_install", preInstall, currentDir, preHookEnv); err != nil {
				logHookStepError()
				result.Error = fmt.Errorf("pre_install hook failed: %w", err)
				return result
			}
			logHookStepDone("Finished running pre_install.")
		}
	}

	// Run installer hook.
	if m.Installer != nil {
		if opts.DryRun {
			logger.Dry("installer", "script")
		} else {
			logHookStepStart("Running installer")
			if err := install.RunInstallerHook("installer", m.Installer, currentDir, preHookEnv); err != nil {
				logHookStepError()
				result.Error = fmt.Errorf("installer failed: %w", err)
				return result
			}
			logHookStepDone("Finished running installer.")
		}
	}

	// Create current/ junction.
	logger.Header("Linking current")
	logger.Detail(fmt.Sprintf("%s -> %s", displayPath(currentLink), displayPath(versionDir)))
	if opts.DryRun {
		logger.Dry("link", displayPath(currentLink))
	} else {
		if err := install.CreateJunction(currentLink, versionDir); err != nil {
			result.Error = fmt.Errorf("failed to create junction: %w", err)
			return result
		}
	}

	// Create shims.
	if len(binDefs) > 0 {
		logger.Header("Creating shims")
		for _, w := range install.DetectShimOverwrites(binDefs, currentLink, opts.Scope) {
			logger.Warn(w)
		}
		if opts.DryRun {
			for _, bin := range binDefs {
				logger.Dry("shim", bin.Name)
			}
		} else {
			if err := install.CreateShims(binDefs, currentLink, opts.Scope); err != nil {
				s.ctx.GetLogger().Warn(fmt.Sprintf("shim creation: %v", err))
				// Continue - best effort.
			} else {
				for _, bin := range binDefs {
					logger.Done("shim", bin.Name)
				}
			}
		}
	}

	// Add to PATH.
	pathChanged := false
	if len(envPaths) > 0 {
		logger.Header("Updating PATH")
		if opts.DryRun {
			pathTarget := "your"
			if opts.Scope == scoop.ScopeGlobal {
				pathTarget = "global"
			}
			for _, p := range envPaths {
				logger.Dry("path", fmt.Sprintf("add %s to %s path", displayPath(p), pathTarget))
			}
		} else {
			pathAdditions, err := install.AddToPathWithResultNoBroadcast(envPaths, opts.Scope)
			if err != nil {
				s.ctx.GetLogger().Warn(fmt.Sprintf("PATH update: %v", err))
			} else {
				pathChanged = len(pathAdditions) > 0
				pathTarget := "your"
				if opts.Scope == scoop.ScopeGlobal {
					pathTarget = "global"
				}
				for _, p := range pathAdditions {
					logger.Done("path", fmt.Sprintf("added %s to %s path", displayPath(p), pathTarget))
				}
			}
		}
	}

	// Setup persist data.
	if len(persistItems) > 0 {
		logger.Header("Persisting data")
		for _, item := range persistItems {
			logger.Detail(item.Source)
		}
		if opts.DryRun {
			logger.Dry("persist", "data")
		} else {
			if err := install.SetupPersistData(appName, persistItems, currentLink, opts.Scope); err != nil {
				s.ctx.GetLogger().Warn(fmt.Sprintf("persist setup: %v", err))
			}
		}
	}

	// Create start menu shortcuts.
	if len(shortcuts) > 0 {
		logger.Header("Creating shortcuts")
		for _, shortcut := range shortcuts {
			logger.Detail(fmt.Sprintf("%s (%s)", shortcut.Name, filepath.Base(filepath.FromSlash(shortcut.Target))))
		}
		if opts.DryRun {
			logger.Dry("shortcuts", "start menu")
		} else {
			if err := install.CreateStartMenuShortcuts(shortcuts, currentLink, opts.Scope); err != nil {
				s.ctx.GetLogger().Warn(fmt.Sprintf("shortcuts: %v", err))
			}
		}
	}

	// Set environment variables.
	envChanged := false
	if len(envVars) > 0 {
		logger.Header("Setting environment")
		for k, v := range envVars {
			logger.Detail(k + "=" + v)
		}

		if opts.DryRun {
			logger.Dry("env", "variables")
		} else {
			if err := install.SetEnvVarsNoBroadcast(envVars, opts.Scope); err != nil {
				s.ctx.GetLogger().Warn(fmt.Sprintf("env set batch: %v", err))
				for k, v := range envVars {
					if err := install.SetEnvVarNoBroadcast(k, v, opts.Scope); err != nil {
						s.ctx.GetLogger().Warn(fmt.Sprintf("env set %s: %v", k, err))
						continue
					}
					envChanged = true
				}
			} else {
				envChanged = true
			}
		}
	}

	if !opts.DryRun && (pathChanged || envChanged) {
		install.BroadcastEnvironmentChange()
	}

	// Run post_install hook.
	if postInstall != "" {
		if opts.DryRun {
			logger.Dry("post_install", "script")
		} else {
			logHookStepStart("Running post_install")
			if err := install.RunHook("post_install", postInstall, currentLink, expandedEnvVars); err != nil {
				logHookStepError()
				result.Error = fmt.Errorf("post_install hook failed: %w", err)
				return result
			}
			logHookStepDone("Finished running post_install.")
		}
	}

	// Save install.json.
	logger.Header("Saving metadata")
	if opts.DryRun {
		logger.Dry("metadata", "install.json and manifest.json")
	} else {
		info := &install.InstallInfo{
			Architecture: arch,
			Bucket:       bucketName,
		}
		if err := install.WriteInstallInfo(filepath.Join(versionDir, "install.json"), info); err != nil {
			s.ctx.GetLogger().Warn(fmt.Sprintf("failed to save install.json: %v", err))
		}

		// Save manifest.json.
		if err := install.WriteManifest(filepath.Join(versionDir, "manifest.json"), m); err != nil {
			s.ctx.GetLogger().Warn(fmt.Sprintf("failed to save manifest.json: %v", err))
		}
	}

	// Show manifest notes.
	for _, note := range manifestNotes(m.Notes) {
		logger.Log(note)
	}

	// Show feature suggestions.
	for _, suggestion := range manifestSuggestions(m.Suggest) {
		logger.Log(fmt.Sprintf("'%s' suggests installing '%s'.", displayName, suggestion))
	}

	result.Success = true
	return result
}
