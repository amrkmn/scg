package service

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"go.noz.one/scg/internal/install"
	"go.noz.one/scg/internal/scoop"
)

// InstallOptions configures an install operation.
type InstallOptions struct {
	Scope       scoop.InstallScope
	Independent bool   // Don't install dependencies
	NoCache     bool   // Don't use download cache
	SkipHash    bool   // Skip hash verification
	Arch        string // Force architecture (64bit, 32bit, arm64)
	Proxy       string // Proxy URL for downloads
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
		if result.Skipped {
			s.ctx.GetLogger().Info(fmt.Sprintf("%s is already installed", result.App))
		} else if result.Success {
			s.ctx.GetLogger().Success(fmt.Sprintf("%s %s installed", result.App, result.Version))
		} else if result.Error != nil {
			s.ctx.GetLogger().Error(fmt.Sprintf("%s: %v", result.App, result.Error))
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

	// Find the manifest.
	installed, bucket := s.manifests.FindManifestPair(appInput)

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

	// Check if already installed (upgrade case).
	if installed != nil {
		// Check if same bucket and version.
		if installed.Bucket == bucketName && installed.Manifest.Version == m.Version {
			result.Skipped = true
			return result
		}
	}

	// Check for running processes (simplified - just warn).
	paths := scoop.ResolvePaths(opts.Scope)
	appDir := filepath.Join(paths.Apps, appInput)
	versionDir := filepath.Join(appDir, m.Version)

	// Resolve dependencies (if not --independent).
	if !opts.Independent {
		deps := scoop.GetDependencies(m.Depends)
		if len(deps) > 0 {
			s.ctx.GetLogger().Info(fmt.Sprintf("Installing dependencies for %s: %v", appInput, deps))
			depResults := s.Install(deps, opts)
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
	persistItems := install.ParsePersistField(m.Persist)
	archPersist := parseArchPersist(m.Architecture, arch)
	persistItems = append(persistItems, archPersist...)
	envPaths := install.EnvAddPaths(m.EnvAddPath, currentLink)
	envVars := install.EnvSetVars(m.EnvSet)
	postInstall := getArchString(m, "post_install", arch)

	currentDir := versionDir
	log := s.ctx.GetLogger().Log

	// Resolve download URL.
	dlURL, err := resolveDownloadURL(m, arch)
	if err != nil {
		result.Error = fmt.Errorf("failed to resolve download URL: %w", err)
		return result
	}

	// Download.
	log("  Downloading " + appInput + "...")
	dm := install.NewDownloadManager(opts.Scope, s.ctx.GetVerbose())
	dlResult, err := dm.Download(appInput, m.Version, dlURL, !opts.NoCache, opts.Proxy)
	if err != nil {
		result.Error = fmt.Errorf("download failed: %w", err)
		return result
	}

	if dlResult.Downloaded {
		s.ctx.GetLogger().Log(fmt.Sprintf("  Downloaded to %s", dlResult.CachePath))
	} else {
		s.ctx.GetLogger().Log(fmt.Sprintf("  Using cached %s", dlResult.CachePath))
	}

	// Build hook environment variables (after download so $fname is available).
	persistDir := filepath.Join(paths.Root, "persist", appInput)
	downloadFile := filepath.Base(dlResult.CachePath)
	hookEnv := install.SetupHookEnvVars(
		currentDir,   // dir (current junction)
		versionDir,   // original_dir (version-specific)
		m.Version,    // version
		arch,         // architecture
		appInput,     // app
		persistDir,   // persist_dir
		paths.Root,   // scoopdir
		downloadFile, // fname
		opts.Scope == scoop.ScopeGlobal,
	)

	// Verify hash (unless --skip).
	if !opts.SkipHash {
		expectedHash, err := resolveHash(m, arch)
		if err == nil && expectedHash != nil {
			log("  Verifying hash...")
			hashFormat, parseErr := install.ParseHash(*expectedHash)
			if parseErr != nil {
				result.Error = fmt.Errorf("failed to parse hash: %w", parseErr)
				return result
			}
			if hashFormat != nil {
				if err := install.VerifyHash(dlResult.CachePath, hashFormat); err != nil {
					result.Error = fmt.Errorf("hash verification failed: %w", err)
					return result
				}
				s.ctx.GetLogger().Log("  Hash verified.")
			}
		}
	}

	// Extract.
	log("  Extracting to " + versionDir + "...")
	extractor := install.NewExtractor(false, s.ctx.GetVerbose())
	extractOpts := install.ExtractionOptions{
		InnoSetup: isManifestInnoSetup(m, arch),
		MSI:       isManifestMSI(dlURL),
	}
	if err := extractor.Extract(dlResult.CachePath, versionDir, extractOpts); err != nil {
		result.Error = fmt.Errorf("extraction failed: %w", err)
		return result
	}

	// Run pre_install hook.
	if preInstall != "" {
		log("  Running pre_install hook...")
		if err := install.RunHook("pre_install", preInstall, currentDir, hookEnv); err != nil {
			result.Error = fmt.Errorf("pre_install hook failed: %w", err)
			return result
		}
	}

	// Create current/ junction.
	log("  Creating current/ junction...")
	if err := install.CreateJunction(currentLink, versionDir); err != nil {
		result.Error = fmt.Errorf("failed to create junction: %w", err)
		return result
	}

	// Create shims.
	if len(binDefs) > 0 {
		log("  Creating " + fmt.Sprint(len(binDefs)) + " shim(s)...")
		if err := install.CreateShims(binDefs, currentLink, opts.Scope); err != nil {
			s.ctx.GetLogger().Warn(fmt.Sprintf("  Warning: shim creation: %v", err))
			// Continue - best effort.
		}
	}

	// Setup persist data.
	if len(persistItems) > 0 {
		log("  Setting up " + fmt.Sprint(len(persistItems)) + " persist item(s)...")
		if err := install.SetupPersistData(appInput, persistItems, currentLink, opts.Scope); err != nil {
			s.ctx.GetLogger().Warn(fmt.Sprintf("  Warning: persist setup: %v", err))
		}
	}

	// Add to PATH.
	if len(envPaths) > 0 {
		log("  Adding " + fmt.Sprint(len(envPaths)) + " path(s) to PATH...")
		if err := install.AddToPath(envPaths, opts.Scope); err != nil {
			s.ctx.GetLogger().Warn(fmt.Sprintf("  Warning: PATH update: %v", err))
		}
	}

	// Set environment variables.
	if len(envVars) > 0 {
		for k, v := range envVars {
			log("  Setting " + k + "=" + v)
			if err := install.SetEnvVar(k, v, opts.Scope); err != nil {
				s.ctx.GetLogger().Warn(fmt.Sprintf("  Warning: env set %s: %v", k, err))
			}
		}
	}

	// Run post_install hook.
	if postInstall != "" {
		log("  Running post_install hook...")
		if err := install.RunHook("post_install", postInstall, currentDir, hookEnv); err != nil {
			result.Error = fmt.Errorf("post_install hook failed: %w", err)
			return result
		}
	}

	// Save install.json.
	log("  Saving metadata...")
	info := &install.InstallInfo{
		Architecture: arch,
		URL:          dlURL,
		Bucket:       bucketName,
	}
	if err := install.WriteInstallInfo(filepath.Join(versionDir, "install.json"), info); err != nil {
		s.ctx.GetLogger().Warn(fmt.Sprintf("  Warning: failed to save install.json: %v", err))
	}

	// Save manifest.json.
	if err := install.WriteManifest(filepath.Join(versionDir, "manifest.json"), m); err != nil {
		s.ctx.GetLogger().Warn(fmt.Sprintf("  Warning: failed to save manifest.json: %v", err))
	}

	result.Success = true
	return result
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

// resolveDownloadURL resolves the download URL from the manifest.
// Priority: architecture-specific URL > top-level URL.
func resolveDownloadURL(m *scoop.Manifest, arch string) (string, error) {
	// Try architecture-specific URL first.
	if m.Architecture != nil {
		if archSection, ok := m.Architecture[arch]; ok {
			if section, ok := archSection.(map[string]any); ok {
				if url, ok := section["url"]; ok {
					if s, ok := extractFirstString(url); ok {
						return s, nil
					}
				}
			}
		}
	}

	// Fall back to top-level URL.
	if m.URL != nil {
		if s, ok := extractFirstString(m.URL); ok {
			return s, nil
		}
	}

	return "", fmt.Errorf("no download URL found in manifest for architecture %s", arch)
}

// extractFirstString extracts the first string from a value that may be
// a string, []string, or []any containing strings.
func extractFirstString(v any) (string, bool) {
	switch val := v.(type) {
	case string:
		return val, val != ""
	case []string:
		if len(val) > 0 {
			return val[0], true
		}
	case []any:
		if len(val) > 0 {
			if s, ok := val[0].(string); ok {
				return s, true
			}
		}
	}
	return "", false
}

// resolveHash resolves the expected hash for the download.
// Priority: architecture-specific hash > top-level hash.
func resolveHash(m *scoop.Manifest, arch string) (*string, error) {
	// Try architecture-specific hash first.
	if m.Architecture != nil {
		if archSection, ok := m.Architecture[arch]; ok {
			if section, ok := archSection.(map[string]any); ok {
				if hash, ok := section["hash"]; ok {
					if s, ok := extractFirstString(hash); ok {
						return &s, nil
					}
				}
			}
		}
	}

	// Fall back to top-level hash.
	if m.Hash != nil {
		if s, ok := extractFirstString(m.Hash); ok {
			return &s, nil
		}
	}

	return nil, nil
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
func parseArchPersist(archData map[string]any, arch string) []string {
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
		return install.ParsePersistField(persist)
	}
	return nil
}
