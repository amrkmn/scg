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
	Scope               scoop.InstallScope
	Independent         bool   // Don't install dependencies
	NoCache             bool   // Don't use download cache
	SkipHash            bool   // Skip hash verification
	Arch                string // Force architecture (64bit, 32bit, arm64)
	Proxy               string // Proxy URL for downloads
	AsDependencyFlow    bool   // Internal: compact/dependency-style logs
	DryRun              bool   // Simulate without side effects
	SkipDownload        bool   // Use already-cached file (for update flow)
	FromBucket          string // Original bucket (e.g. from install.json); suppresses multi-bucket warning
	DownloadConcurrency int    // Max parallel downloads; <=0 means auto (SCG_DOWNLOAD_CONCURRENCY or default)
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
// Downloads for all apps are pre-fetched in parallel (Homebrew-style),
// then each app is installed sequentially to keep filesystem/PATH/env
// mutations safe on Windows. Returns results for each.
func (s *InstallService) Install(apps []string, opts InstallOptions) []InstallResult {
	results := make([]InstallResult, 0, len(apps))

	if phasedInstallEligible(apps, opts) {
		return s.installBatched(dedupeInputs(apps), opts)
	}

	for _, app := range apps {
		start := time.Now()
		result := s.InstallSingle(app, opts)
		result.Duration = time.Since(start)
		results = append(results, result)

		s.displayInstallResult(result, opts)
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

	if err := s.installDeps(m, opts); err != nil {
		result.Error = err
		return result
	}

	plan, err := s.buildInstallPlan(m, appName, bucketName, displayName, arch, opts)
	if err != nil {
		result.Error = err
		return result
	}

	staged, err := s.stageInstallPlan(plan, false)
	if err != nil {
		result.Error = err
		return result
	}

	return s.linkInstallPlan(plan, staged)
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
