package service

import (
	"os"
	"path/filepath"
	"strings"
	"sync"

	"go.noz.one/scg/internal/scoop"
)

// AppStatusResult holds the update status for a single installed application.
type AppStatusResult struct {
	Name        string
	Scope       scoop.InstallScope
	Installed   string
	Latest      string
	Outdated    bool
	Failed      bool
	Held        bool
	MissingDeps []string
}

// StatusService checks the update status of installed apps.
type StatusService struct {
	ctx           AppContext
	installedSet  map[string]bool
	installedOnce sync.Once
}

// NewStatusService creates a StatusService.
func NewStatusService(ctx AppContext) *StatusService {
	return &StatusService{ctx: ctx}
}

// CheckStatus checks all given apps against available buckets concurrently.
// onProgress is called after each app is processed.
func (s *StatusService) CheckStatus(apps []InstalledApp, buckets []BucketInfo, onProgress func()) []AppStatusResult {
	if len(apps) == 0 {
		return nil
	}

	ch := make(chan AppStatusResult, len(apps))
	var wg sync.WaitGroup

	for _, app := range apps {
		wg.Add(1)
		go func(a InstalledApp) {
			defer wg.Done()
			result := s.checkApp(a, buckets)
			if onProgress != nil {
				onProgress()
			}
			ch <- result
		}(app)
	}

	go func() {
		wg.Wait()
		close(ch)
	}()

	var results []AppStatusResult
	for r := range ch {
		results = append(results, r)
	}
	return results
}

func (s *StatusService) checkApp(app InstalledApp, buckets []BucketInfo) AppStatusResult {
	result := AppStatusResult{
		Name:      app.Name,
		Scope:     app.Scope,
		Installed: app.Version,
		Held:      app.Held,
	}

	if app.CurrentPath == "" || app.Version == "" {
		result.Failed = true
		return result
	}

	latest, deprecated := s.findLatestVersion(app, buckets)
	result.Latest = latest
	_ = deprecated

	if latest != "" {
		cmp := compareVersionArrays(
			parseVersionString(app.Version),
			parseVersionString(latest),
		)
		result.Outdated = cmp < 0
	}

	// Check missing dependencies.
	result.MissingDeps = s.findMissingDeps(app, buckets)

	return result
}

// findLatestVersion searches all buckets for the highest available version of the app.
func (s *StatusService) findLatestVersion(app InstalledApp, buckets []BucketInfo) (string, bool) {
	var best string
	deprecated := false

	checkManifest := func(b BucketInfo) {
		manifestDir := b.ManifestDir
		manifestPath := filepath.Join(manifestDir, app.Name+".json")
		m, err := scoop.ReadManifest(manifestPath)
		if err != nil {
			return
		}
		if m.Version == "" {
			return
		}
		if best == "" {
			best = m.Version
			if m.Deprecated != nil {
				deprecated = true
			}
		} else if compareVersionArrays(parseVersionString(m.Version), parseVersionString(best)) > 0 {
			best = m.Version
			deprecated = m.Deprecated != nil
		}
	}

	// Check installed bucket first.
	if app.Bucket != "" {
		for _, b := range buckets {
			if strings.EqualFold(b.Name, app.Bucket) {
				checkManifest(b)
				break
			}
		}
	}

	// Check remaining buckets.
	for _, b := range buckets {
		if strings.EqualFold(b.Name, app.Bucket) {
			continue
		}
		checkManifest(b)
	}

	return best, deprecated
}

func (s *StatusService) getInstalledSet() map[string]bool {
	s.installedOnce.Do(func() {
		appsSvc := &AppsService{ctx: s.ctx}
		installed, err := appsSvc.ListInstalled("")
		if err == nil {
			s.installedSet = make(map[string]bool, len(installed))
			for _, a := range installed {
				s.installedSet[strings.ToLower(a.Name)] = true
			}
		} else {
			s.installedSet = make(map[string]bool)
		}
	})
	return s.installedSet
}

// findMissingDeps checks whether the app's dependencies are installed.
func (s *StatusService) findMissingDeps(app InstalledApp, buckets []BucketInfo) []string {
	// Load manifest to get depends field.
	var depends []string
	for _, b := range buckets {
		if app.Bucket != "" && !strings.EqualFold(b.Name, app.Bucket) {
			continue
		}
		manifestDir := b.ManifestDir
		m, err := scoop.ReadManifest(filepath.Join(manifestDir, app.Name+".json"))
		if err != nil {
			continue
		}
		depends = toStringSlice(m.Depends)
		break
	}

	if len(depends) == 0 {
		return nil
	}

	installedSet := s.getInstalledSet()

	var missing []string
	for _, dep := range depends {
		// deps can be "bucket/app" format — strip bucket prefix.
		parts := strings.SplitN(dep, "/", 2)
		depName := parts[len(parts)-1]
		if !installedSet[strings.ToLower(depName)] {
			missing = append(missing, depName)
		}
	}
	return missing
}

// toStringSlice converts an any (string or []any of strings) to []string.
func toStringSlice(v any) []string {
	if v == nil {
		return nil
	}
	switch val := v.(type) {
	case string:
		if val == "" {
			return nil
		}
		return []string{val}
	case []any:
		out := make([]string, 0, len(val))
		for _, item := range val {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	case []string:
		return val
	}
	return nil
}

// parseVersionString converts a version string into a comparable 4-part integer array.
// e.g. "1.2.3-beta" -> [1, 2, 3, 0]
func parseVersionString(v string) [4]int {
	var result [4]int
	idx := 0
	start := 0
	inPart := false
	for i := 0; i < len(v); i++ {
		c := v[i]
		if c == '.' || c == '-' || c == '_' || c == '+' {
			if inPart {
				result[idx] = leadingInt(v[start:i])
				idx++
				if idx >= 4 {
					return result
				}
				inPart = false
			}
		} else {
			if !inPart {
				start = i
				inPart = true
			}
		}
	}
	if inPart && idx < 4 {
		result[idx] = leadingInt(v[start:])
	}
	return result
}

func leadingInt(s string) int {
	n := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= '0' && c <= '9' {
			n = n*10 + int(c-'0')
		} else {
			break
		}
	}
	return n
}

// compareVersionArrays returns -1 if a < b, 0 if equal, 1 if a > b.
func compareVersionArrays(a, b [4]int) int {
	for i := 0; i < 4; i++ {
		if a[i] < b[i] {
			return -1
		}
		if a[i] > b[i] {
			return 1
		}
	}
	return 0
}

// GetInstalledAppsForScope returns installed apps filtered to a specific scope.
func GetInstalledAppsForScope(apps []InstalledApp, scope scoop.InstallScope) []InstalledApp {
	var out []InstalledApp
	for _, a := range apps {
		if a.Scope == scope {
			out = append(out, a)
		}
	}
	return out
}

// ExistsInBuckets checks if an app manifest exists in any of the given buckets.
func ExistsInBuckets(appName string, buckets []BucketInfo) bool {
	for _, b := range buckets {
		manifestDir := b.ManifestDir
		if _, err := os.Stat(filepath.Join(manifestDir, appName+".json")); err == nil {
			return true
		}
	}
	return false
}
