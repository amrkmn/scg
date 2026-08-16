package service

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"go.noz.one/scg/internal/install"
	"go.noz.one/scg/internal/scoop"
	"go.noz.one/scg/internal/ui"
)

// InstallOptions configures an install operation.
type InstallOptions struct {
	Scope            scoop.InstallScope
	Independent      bool   // Don't install dependencies
	NoCache          bool   // Don't use download cache
	SkipHash         bool   // Skip hash verification
	Arch             string // Force architecture (64bit, 32bit, arm64)
	Proxy            string // Proxy URL for downloads
	AsDependencyFlow bool   // Internal: compact/dependency-style logs
	DryRun           bool   // Simulate without side effects
	SkipDownload     bool   // Use already-cached file (for update flow)
	FromBucket       string // Original bucket (e.g. from install.json); suppresses multi-bucket warning
}

// InstallResult holds the outcome of installing a single app.
type InstallResult struct {
	App      string
	Version  string
	Success  bool
	Skipped  bool // Already installed
	Error    error
	Duration time.Duration
}

// InstallService handles app installation.
type InstallService struct {
	ctx       AppContext
	manifests *ManifestService
	apps      *AppsService
}

// NewInstallService creates an InstallService.
func NewInstallService(ctx AppContext) *InstallService {
	return &InstallService{
		ctx:       ctx,
		manifests: NewManifestService(ctx),
		apps:      NewAppsService(ctx),
	}
}

// Install installs one or more apps with the given options.
// It processes apps sequentially (like scoop) and returns results for each.
func (s *InstallService) Install(apps []string, opts InstallOptions) []InstallResult {
	results := make([]InstallResult, 0, len(apps))

	for _, app := range apps {
		start := time.Now()
		result := s.InstallSingle(app, opts)
		result.Duration = time.Since(start)
		results = append(results, result)

		// Display result.
		if opts.AsDependencyFlow {
			if result.Skipped {
				s.ctx.GetLogger().Skip(result.App, "already installed")
			} else if result.Success {
				s.ctx.GetLogger().Done(result.App, "installed")
			} else if result.Error != nil {
				s.ctx.GetLogger().Error(fmt.Sprintf("%s: %v", result.App, result.Error))
			}
		} else {
			if result.Skipped {
				s.ctx.GetLogger().Skip(result.App, fmt.Sprintf("%s already installed", result.Version))
			} else if result.Success {
				s.ctx.GetLogger().Done(result.App, fmt.Sprintf("installed %s", result.Version))
				s.ctx.GetLogger().Verbose(fmt.Sprintf("duration: %s", result.Duration.Round(time.Millisecond)))
			} else if result.Error != nil {
				s.ctx.GetLogger().Error(fmt.Sprintf("%s: %v", result.App, result.Error))
			}
		}
	}

	return results
}

// InstallSingle handles the installation of a single app.
func (s *InstallService) InstallSingle(appInput string, opts InstallOptions) InstallResult {
	result := InstallResult{App: appInput}

	// Determine architecture.
	arch := opts.Arch
	if arch == "" {
		arch = detectArch()
	}

	// Resolve scope.
	if opts.Scope == "" {
		opts.Scope = scoop.ScopeUser
	}

	_, appName := parseBucketAndApp(appInput)

	// If we know the original bucket, use it to find the manifest.
	lookupName := appInput
	if opts.FromBucket != "" {
		lookupName = fmt.Sprintf("%s/%s", opts.FromBucket, appName)
	}

	// Find the manifest.
	installed, bucket := s.manifests.FindManifestPair(lookupName)

	// Determine the manifest to install from.
	var m *scoop.Manifest
	var bucketName string
	if bucket != nil {
		m = bucket.Manifest
		bucketName = bucket.Bucket
	} else if installed != nil {
		// Already installed and no bucket manifest found.
		result.Skipped = true
		result.Version = installed.Manifest.Version
		return result
	} else {
		result.Error = fmt.Errorf("app %q not found in any bucket", appInput)
		return result
	}

	result.Version = m.Version

	// Warn when multiple buckets contain the same app and no bucket was explicitly requested.
	requestedBucket, _ := parseBucketAndApp(appInput)
	if requestedBucket == "" && opts.FromBucket == "" {
		allMatches := s.manifests.FindAllManifests(appInput)
		matchCount := 0
		for _, fm := range allMatches {
			if fm.Source == "bucket" {
				matchCount++
			}
		}
		if matchCount > 1 {
			s.ctx.GetLogger().Warn(fmt.Sprintf("Multiple buckets contain manifest '%s', the current selection is '%s/%s'.", appName, bucketName, appName))
		}
	}

	// Check if already installed (upgrade case).
	if installed != nil {
		// Check if same bucket and version.
		if installed.Bucket == bucketName && installed.Manifest.Version == m.Version {
			result.Skipped = true
			return result
		}
	}

	return s.installCore(m, appName, bucketName, appInput, arch, opts)
}

// InstallFromURL downloads a manifest from a URL and installs the app.
func (s *InstallService) InstallFromURL(manifestURL string, opts InstallOptions) InstallResult {
	appName := appNameFromURLOrPath(manifestURL)
	result := InstallResult{App: appName}

	resp, err := http.Get(manifestURL)
	if err != nil {
		result.Error = fmt.Errorf("failed to download manifest: %w", err)
		return result
	}
	defer func() { _ = resp.Body.Close() }()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		result.Error = fmt.Errorf("failed to read manifest: %w", err)
		return result
	}

	m, err := scoop.ParseManifestBytes(data)
	if err != nil {
		result.Error = fmt.Errorf("invalid manifest JSON: %w", err)
		return result
	}

	arch := opts.Arch
	if arch == "" {
		arch = detectArch()
	}
	if opts.Scope == "" {
		opts.Scope = scoop.ScopeUser
	}

	return s.installCore(m, appName, "[url]", appName, arch, opts)
}

// InstallFromFile reads a manifest from a local file and installs the app.
func (s *InstallService) InstallFromFile(filePath string, opts InstallOptions) InstallResult {
	appName := appNameFromURLOrPath(filePath)
	result := InstallResult{App: appName}

	m, err := scoop.ReadManifest(filePath)
	if err != nil {
		result.Error = fmt.Errorf("failed to read manifest: %w", err)
		return result
	}

	arch := opts.Arch
	if arch == "" {
		arch = detectArch()
	}
	if opts.Scope == "" {
		opts.Scope = scoop.ScopeUser
	}

	return s.installCore(m, appName, "[file]", appName, arch, opts)
}

// InstallWithVersion installs a specific version of an app from a bucket manifest.
func (s *InstallService) InstallWithVersion(appInput, version string, opts InstallOptions) InstallResult {
	result := InstallResult{App: appInput}
	_, appName := parseBucketAndApp(appInput)
	lookupName := appInput
	if opts.FromBucket != "" {
		lookupName = fmt.Sprintf("%s/%s", opts.FromBucket, appName)
	}

	_, bucket := s.manifests.FindManifestPair(lookupName)
	if bucket == nil {
		result.Error = fmt.Errorf("app %q not found in any bucket", appInput)
		return result
	}

	m := bucket.Manifest
	if m.Version != version {
		// Generate a manifest for the pinned version using the autoupdate template
		// (Scoop-compatible); apps without autoupdate cannot be pinned.
		generated, err := install.GenerateManifestForVersion(m, appName, version, opts.Scope, opts.Proxy, s.ctx.GetVerbose())
		if err != nil {
			result.Error = fmt.Errorf("version %q not found in manifest for %q: %w", version, appName, err)
			return result
		}
		m = generated
	}

	arch := opts.Arch
	if arch == "" {
		arch = detectArch()
	}
	if opts.Scope == "" {
		opts.Scope = scoop.ScopeUser
	}

	return s.installCore(m, appName, bucket.Bucket, appName, arch, opts)
}

// installCore performs the full install flow given a resolved manifest.
func (s *InstallService) installCore(m *scoop.Manifest, appName, bucketName, displayName, arch string, opts InstallOptions) InstallResult {
	result := InstallResult{App: displayName, Version: m.Version}

	if opts.AsDependencyFlow {
		s.ctx.GetLogger().Header(fmt.Sprintf("Installing dependency %s", appName))
	} else {
		s.ctx.GetLogger().Header(fmt.Sprintf("Installing %s %s [%s] from %s", displayName, m.Version, arch, bucketName))
	}

	// Check for running processes (simplified - just warn).
	paths := scoop.ResolvePaths(opts.Scope)
	appDir := filepath.Join(paths.Apps, appName)
	versionDir := filepath.Join(appDir, m.Version)

	// Resolve dependencies (if not --independent).
	if !opts.Independent {
		deps := scoop.GetDependencies(m.Depends)
		if len(deps) > 0 {
			depOpts := opts
			depOpts.AsDependencyFlow = true
			depResults := s.Install(deps, depOpts)
			for _, dr := range depResults {
				if !dr.Success && !dr.Skipped {
					result.Error = fmt.Errorf("dependency %s failed to install: %w", dr.App, dr.Error)
					return result
				}
			}
		}
	}

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
		result.Error = fmt.Errorf("failed to resolve download URLs: %w", err)
		return result
	}
	firstURL := dlURLs[0]

	// Download (or reuse cached files).
	dm := install.NewDownloadManager(opts.Scope, s.ctx.GetVerbose())
	dlResults := make([]*install.DownloadResult, 0, len(dlURLs))

	for _, dlURL := range dlURLs {
		if opts.SkipDownload {
			if existingPath, ok := dm.FindCachedPath(appName, m.Version, dlURL); ok {
				dlResults = append(dlResults, &install.DownloadResult{
					CachePath:  existingPath,
					Downloaded: false,
					UsedAria2:  false,
				})
				logger.Done("cache", filepath.Base(existingPath))
			} else if opts.DryRun {
				// In dry-run the pre-download phase only simulated the download;
				// create a fake result so the rest of the flow can simulate too.
				cachePath := dm.CachePath(appName, m.Version, dlURL)
				dlResults = append(dlResults, &install.DownloadResult{
					CachePath:  cachePath,
					Downloaded: false,
					UsedAria2:  false,
				})
				logger.Dry("cache", filepath.Base(cachePath))
			} else {
				result.Error = fmt.Errorf("cached file not found for %s (%s)", appName, m.Version)
				return result
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
				logger.Done("cache", filepath.Base(cachePath))
			}

			dlResult, err := dm.Download(appName, m.Version, dlURL, !opts.NoCache, opts.Proxy)
			if err != nil {
				result.Error = fmt.Errorf("download failed: %w", err)
				return result
			}

			if dlResult.Downloaded {
				if !dlResult.UsedAria2 {
					logger.Done("download", filepath.Base(dlResult.CachePath))
				}
			}
			dlResults = append(dlResults, dlResult)
		}
	}

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

	// Extract archives; plain executables are staged directly unless explicitly marked as Inno Setup.
	for i, dlResult := range dlResults {
		dlURL := dlURLs[i]
		if install.IsArchive(dlResult.CachePath) || isManifestInnoSetup(m, arch) {
			extractor := install.NewExtractor(false, s.ctx.GetVerbose())
			extractOpts := install.ExtractionOptions{
				InnoSetup: isManifestInnoSetup(m, arch),
				MSI:       isManifestMSI(dlURL),
			}
			if opts.DryRun {
				logger.Dry("extract", fmt.Sprintf("%s to %s", filepath.Base(dlResult.CachePath), displayPath(versionDir)))
			} else {
				logStepStart("Extracting " + filepath.Base(dlResult.CachePath))
				if err := extractor.Extract(dlResult.CachePath, versionDir, extractOpts); err != nil {
					logStepError()
					result.Error = fmt.Errorf("extraction failed: %w", err)
					return result
				}
				logStepDone()
				logger.Done("extract", displayPath(versionDir))
			}
		} else {
			if opts.DryRun {
				logger.Dry("stage", fmt.Sprintf("%s in %s", filepath.Base(install.DownloadFileName(appName, dlURL)), displayPath(versionDir)))
			} else {
				if err := os.MkdirAll(versionDir, 0o755); err != nil {
					result.Error = fmt.Errorf("failed to create version dir: %w", err)
					return result
				}
				dst := filepath.Join(versionDir, install.DownloadFileName(appName, dlURL))
				if err := copyFile(dlResult.CachePath, dst); err != nil {
					result.Error = fmt.Errorf("failed to stage installer payload: %w", err)
					return result
				}
			}
		}
	}

	// Handle extract_to: move extracted contents into a subdirectory.
	if et := getArchString(m, "extract_to", arch); et != "" {
		targetDir := filepath.Join(versionDir, et)
		if opts.DryRun {
			logger.Dry("extract_to", displayPath(targetDir))
		} else {
			if err := install.MoveContents(versionDir, targetDir); err != nil {
				result.Error = fmt.Errorf("extract_to failed: %w", err)
				return result
			}
		}
	}

	// Flatten extract_dir if specified.
	if ed := getArchString(m, "extract_dir", arch); ed != "" {
		if opts.DryRun {
			logger.Dry("extract_dir", ed)
		} else {
			if err := install.FlattenExtractDir(versionDir, ed); err != nil {
				result.Error = fmt.Errorf("extract_dir failed: %w", err)
				return result
			}
		}
	}

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

// appNameFromURLOrPath extracts the app name from a URL or file path.
// Strips .json extension and path components.
func appNameFromURLOrPath(s string) string {
	// Remove trailing @version
	if i := strings.Index(s, "@"); i >= 0 {
		s = s[:i]
	}
	s = strings.TrimSuffix(s, ".json")
	// Handle http:// or https:// URLs: get the last path segment
	if i := strings.LastIndex(s, "/"); i >= 0 {
		s = s[i+1:]
	}
	// Handle Windows backslash paths
	if i := strings.LastIndex(s, "\\"); i >= 0 {
		s = s[i+1:]
	}
	// URL fragments
	if i := strings.Index(s, "#"); i >= 0 {
		s = s[:i]
	}
	if i := strings.Index(s, "?"); i >= 0 {
		s = s[:i]
	}
	return s
}

// detectArch returns the appropriate architecture for the current system.
func detectArch() string {
	arch := runtime.GOARCH
	switch arch {
	case "amd64":
		return "64bit"
	case "386":
		return "32bit"
	case "arm64":
		return "arm64"
	default:
		return "64bit" // Default to 64bit.
	}
}

// resolveDownloadURLs resolves all download URLs from the manifest.
// Priority: architecture-specific URL > top-level URL.
func resolveDownloadURLs(m *scoop.Manifest, arch string) ([]string, error) {
	// Try architecture-specific URL first.
	if m.Architecture != nil {
		if archSection, ok := m.Architecture[arch]; ok {
			if section, ok := archSection.(map[string]any); ok {
				if url, ok := section["url"]; ok {
					if urls := extractStrings(url); len(urls) > 0 {
						return urls, nil
					}
				}
			}
		}
	}

	// Fall back to top-level URL.
	if m.URL != nil {
		if urls := extractStrings(m.URL); len(urls) > 0 {
			return urls, nil
		}
	}

	return nil, fmt.Errorf("no download URL found in manifest for architecture %s", arch)
}

func extractStrings(v any) []string {
	switch val := v.(type) {
	case string:
		if val != "" {
			return []string{val}
		}
	case []string:
		return val
	case []any:
		out := make([]string, 0, len(val))
		for _, item := range val {
			if s, ok := item.(string); ok && s != "" {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

func resolveHashes(m *scoop.Manifest, arch string) []string {
	// Try architecture-specific hash first.
	if m.Architecture != nil {
		if archSection, ok := m.Architecture[arch]; ok {
			if section, ok := archSection.(map[string]any); ok {
				if hash, ok := section["hash"]; ok {
					if hashes := extractStrings(hash); len(hashes) > 0 {
						return hashes
					}
				}
			}
		}
	}

	// Fall back to top-level hash.
	if m.Hash != nil {
		return extractStrings(m.Hash)
	}

	return nil
}

func hashAt(hashes []string, i int) string {
	if len(hashes) == 0 {
		return ""
	}
	if i < len(hashes) {
		return hashes[i]
	}
	if len(hashes) == 1 {
		return hashes[0]
	}
	return ""
}

// isManifestInnoSetup checks if the manifest specifies an Inno Setup installer.
func isManifestInnoSetup(m *scoop.Manifest, arch string) bool {
	if m.Architecture != nil {
		if archSection, ok := m.Architecture[arch]; ok {
			if section, ok := archSection.(map[string]any); ok {
				if innosetup, ok := section["innosetup"]; ok {
					if b, ok := innosetup.(bool); ok {
						return b
					}
				}
			}
		}
	}
	return false
}

// isManifestMSI checks if the URL suggests an MSI installer.
func isManifestMSI(url string) bool {
	return strings.HasSuffix(strings.ToLower(url), ".msi")
}

// getArchString gets a string field from the manifest, checking architecture
// specific overrides first, then falling back to the top-level field.
func getArchString(m *scoop.Manifest, field, arch string) string {
	// Check architecture-specific override first.
	if m.Architecture != nil {
		if archSection, ok := m.Architecture[arch]; ok {
			if section, ok := archSection.(map[string]any); ok {
				if val, ok := section[field]; ok {
					switch v := val.(type) {
					case string:
						return v
					case []any:
						parts := make([]string, 0, len(v))
						for _, item := range v {
							if s, ok := item.(string); ok {
								parts = append(parts, s)
							}
						}
						return strings.Join(parts, "\n")
					}
				}
			}
		}
	}

	// Fall back to top-level field.
	switch field {
	case "pre_install":
		if m.PreInstall != nil {
			return joinAnyString(m.PreInstall)
		}
	case "post_install":
		if m.PostInstall != nil {
			return joinAnyString(m.PostInstall)
		}
	case "extract_dir":
		return m.ExtractDir
	case "extract_to":
		return m.ExtractTo
	}
	return ""
}

// joinAnyString converts an any that's a string or []any to a single string.
func joinAnyString(v any) string {
	switch val := v.(type) {
	case string:
		return val
	case []any:
		parts := make([]string, 0, len(val))
		for _, item := range val {
			if s, ok := item.(string); ok {
				parts = append(parts, s)
			}
		}
		return strings.Join(parts, "\n")
	}
	return ""
}

// parseArchPersist extracts persist items from architecture-specific sections.
func parseArchPersist(archData map[string]any, arch string) []install.PersistItem {
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
	if persist, ok := section["persist"]; ok {
		return install.ParsePersistItems(persist)
	}
	return nil
}

func parseArchEnvAddPath(archData map[string]any, arch string) any {
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
	return section["env_add_path"]
}

func parseArchEnvSet(archData map[string]any, arch string) map[string]string {
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
	raw, ok := section["env_set"]
	if !ok || raw == nil {
		return nil
	}
	out := map[string]string{}
	switch v := raw.(type) {
	case map[string]string:
		for k, s := range v {
			out[k] = s
		}
	case map[string]any:
		for k, item := range v {
			if s, ok := item.(string); ok {
				out[k] = s
			}
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func mergeEnvAddPath(base, arch any) any {
	toSlice := func(v any) []any {
		switch x := v.(type) {
		case nil:
			return nil
		case string:
			return []any{x}
		case []string:
			out := make([]any, 0, len(x))
			for _, s := range x {
				out = append(out, s)
			}
			return out
		case []any:
			out := make([]any, 0, len(x))
			for _, item := range x {
				if s, ok := item.(string); ok {
					out = append(out, s)
				}
			}
			return out
		default:
			return nil
		}
	}
	merged := append(toSlice(base), toSlice(arch)...)
	if len(merged) == 0 {
		return nil
	}
	return merged
}

func mergeEnvSet(base, arch map[string]string) map[string]string {
	if len(base) == 0 && len(arch) == 0 {
		return nil
	}
	out := map[string]string{}
	for k, v := range base {
		out[k] = v
	}
	for k, v := range arch {
		out[k] = v
	}
	return out
}

func manifestNotes(notes any) []string {
	switch v := notes.(type) {
	case string:
		if v == "" {
			return nil
		}
		return []string{"Notes", "-----", v}
	case []any:
		if len(v) == 0 {
			return nil
		}
		lines := []string{"Notes", "-----"}
		for _, item := range v {
			if s, ok := item.(string); ok && s != "" {
				lines = append(lines, s)
			}
		}
		if len(lines) == 2 {
			return nil
		}
		return lines
	default:
		return nil
	}
}

func manifestSuggestions(suggest map[string]any) []string {
	if len(suggest) == 0 {
		return nil
	}

	keys := make([]string, 0, len(suggest))
	for k := range suggest {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	suggestions := make([]string, 0, len(keys))
	for _, k := range keys {
		items := parseSuggestItems(suggest[k])
		if len(items) == 0 {
			continue
		}
		suggestions = append(suggestions, strings.Join(items, "' or '"))
	}
	return suggestions
}

func parseSuggestItems(v any) []string {
	switch val := v.(type) {
	case string:
		if val == "" {
			return nil
		}
		return []string{val}
	case []any:
		out := make([]string, 0, len(val))
		for _, item := range val {
			if s, ok := item.(string); ok && s != "" {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}

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

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return nil
}

func logStepStart(title string) {
	_, _ = fmt.Fprintln(os.Stdout, ui.Heading(title))
}

func logHookStepStart(title string) {
	_, _ = fmt.Fprintln(os.Stdout, ui.Heading(title))
}

func logStepDone() {
	_, _ = fmt.Fprintln(os.Stdout, ui.Done("done", ""))
}

func logHookStepDone(message string) {
	_, _ = fmt.Fprintf(os.Stdout, "%s\n", ui.Done(strings.TrimSuffix(message, "."), ""))
}

func logStepOK() {
	_, _ = fmt.Fprintln(os.Stdout, ui.Done("verified", ""))
}

func logStepError() {
	_, _ = fmt.Fprintln(os.Stdout, ui.FailLine("failed"))
}

func logHookStepError() {
	_, _ = fmt.Fprintln(os.Stdout, ui.FailLine("hook failed"))
}

func displayPath(path string) string {
	clean := filepath.Clean(path)
	profile := os.Getenv("USERPROFILE")
	if profile == "" {
		return clean
	}

	profileClean := filepath.Clean(profile)
	prefix := strings.ToLower(profileClean + string(filepath.Separator))
	if strings.HasPrefix(strings.ToLower(clean), prefix) {
		return "~" + clean[len(profileClean):]
	}
	return clean
}
