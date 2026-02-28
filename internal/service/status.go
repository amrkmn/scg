package service

import (
	"os"
	"path/filepath"
	"strconv"
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
	ctx AppContext
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
		manifestDir := FindBucketDir(b.Path)
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

// findMissingDeps checks whether the app's dependencies are installed.
func (s *StatusService) findMissingDeps(app InstalledApp, buckets []BucketInfo) []string {
	// Load manifest to get depends field.
	var depends []string
	for _, b := range buckets {
		if app.Bucket != "" && !strings.EqualFold(b.Name, app.Bucket) {
			continue
		}
		manifestDir := FindBucketDir(b.Path)
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

	// Get installed app names.
	appsSvc := &AppsService{ctx: s.ctx}
	installed, err := appsSvc.ListInstalled("")
	if err != nil {
		return nil
	}
	installedSet := make(map[string]bool, len(installed))
	for _, a := range installed {
		installedSet[strings.ToLower(a.Name)] = true
	}

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

// parseVersionString converts a version string into a comparable 4-part integer slice.
// e.g. "1.2.3-beta" -> [1, 2, 3, 0]
func parseVersionString(v string) []int {
	// Split on ., -, _, +
	parts := splitVersion(v)
	result := make([]int, 0, 4)
	for _, p := range parts {
		// Take leading digit sequence.
		n := leadingInt(p)
		result = append(result, n)
		if len(result) >= 4 {
			break
		}
	}
	// Pad to 4 parts.
	for len(result) < 4 {
		result = append(result, 0)
	}
	return result
}

func splitVersion(v string) []string {
	var parts []string
	current := strings.Builder{}
	for _, c := range v {
		if c == '.' || c == '-' || c == '_' || c == '+' {
			if current.Len() > 0 {
				parts = append(parts, current.String())
				current.Reset()
			}
		} else {
			current.WriteRune(c)
		}
	}
	if current.Len() > 0 {
		parts = append(parts, current.String())
	}
	return parts
}

func leadingInt(s string) int {
	i := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	if i == 0 {
		return 0
	}
	n, err := strconv.Atoi(s[:i])
	if err != nil {
		return 0
	}
	return n
}

// compareVersionArrays returns -1 if a < b, 0 if equal, 1 if a > b.
func compareVersionArrays(a, b []int) int {
	length := len(a)
	if len(b) > length {
		length = len(b)
	}
	for i := 0; i < length; i++ {
		var av, bv int
		if i < len(a) {
			av = a[i]
		}
		if i < len(b) {
			bv = b[i]
		}
		if av < bv {
			return -1
		}
		if av > bv {
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
		manifestDir := FindBucketDir(b.Path)
		if _, err := os.Stat(filepath.Join(manifestDir, appName+".json")); err == nil {
			return true
		}
	}
	return false
}
