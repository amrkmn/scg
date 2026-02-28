package service

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/amrkmn/scg/internal/scoop"
)

// FoundManifest represents a located manifest file and its parsed content.
type FoundManifest struct {
	Source   string // "installed" or "bucket"
	Scope    scoop.InstallScope
	Bucket   string
	App      string
	FilePath string
	Manifest *scoop.Manifest
}

// InfoFields holds the display-ready information about an app.
type InfoFields struct {
	Name             string
	Version          string
	InstalledVersion string
	LatestVersion    string
	Description      string
	Homepage         string
	License          string
	Source           string
	Deprecated       bool
	ReplacedBy       string
	UpdateAvailable  bool
	InstallDate      time.Time
}

// ManifestService locates and reads Scoop manifest files.
type ManifestService struct {
	ctx AppContext
}

// NewManifestService creates a ManifestService.
func NewManifestService(ctx AppContext) *ManifestService {
	return &ManifestService{ctx: ctx}
}

// FindAllManifests finds all manifests matching the given input ("bucket/app" or "app").
func (s *ManifestService) FindAllManifests(input string) []FoundManifest {
	bucketName, appName := parseBucketAndApp(input)

	var results []FoundManifest

	// Check installed locations.
	for _, paths := range scoop.BothScopes() {
		currentPath := filepath.Join(paths.Apps, appName, "current")
		resolved, err := filepath.EvalSymlinks(currentPath)
		if err != nil {
			continue
		}
		mPath := filepath.Join(resolved, "manifest.json")
		m, err := scoop.ReadManifest(mPath)
		if err != nil {
			continue
		}

		// Determine bucket from install.json.
		var bucket string
		if info, err := scoop.ReadInstallInfo(filepath.Join(resolved, "install.json")); err == nil {
			bucket = info.Bucket
		}

		results = append(results, FoundManifest{
			Source:   "installed",
			Scope:    paths.Scope,
			Bucket:   bucket,
			App:      appName,
			FilePath: mPath,
			Manifest: m,
		})
	}

	// Search buckets.
	bucketSvc := &BucketService{ctx: s.ctx}
	allBuckets, _ := bucketSvc.List("")

	for _, b := range allBuckets {
		if bucketName != "" && !strings.EqualFold(b.Name, bucketName) {
			continue
		}
		manifestDir := FindBucketDir(b.Path)
		mPath := filepath.Join(manifestDir, appName+".json")
		m, err := scoop.ReadManifest(mPath)
		if err != nil {
			continue
		}
		results = append(results, FoundManifest{
			Source:   "bucket",
			Scope:    b.Scope,
			Bucket:   b.Name,
			App:      appName,
			FilePath: mPath,
			Manifest: m,
		})
	}

	return results
}

// FindManifestPair returns the installed and bucket manifests for an app (either may be nil).
// When input is "bucket/app", the bucket result is constrained to that specific bucket.
func (s *ManifestService) FindManifestPair(input string) (installed, bucket *FoundManifest) {
	requestedBucket, _ := parseBucketAndApp(input)
	all := s.FindAllManifests(input)
	for i := range all {
		fm := &all[i]
		if fm.Source == "installed" && installed == nil {
			installed = fm
		} else if fm.Source == "bucket" && bucket == nil {
			// If a specific bucket was requested, only accept that one.
			if requestedBucket == "" || strings.EqualFold(fm.Bucket, requestedBucket) {
				bucket = fm
			}
		}
	}
	return
}

// ReadManifestFields extracts InfoFields from a single FoundManifest.
func (s *ManifestService) ReadManifestFields(appName string, fm *FoundManifest) InfoFields {
	if fm == nil {
		return InfoFields{Name: appName}
	}
	fields := InfoFields{
		Name:        appName,
		Version:     fm.Manifest.Version,
		Description: fm.Manifest.Description,
		Homepage:    fm.Manifest.Homepage,
		License:     formatLicense(fm.Manifest.License),
		Source:      fm.Bucket,
	}
	if fm.Manifest.Deprecated != nil {
		fields.Deprecated = true
		if s, ok := fm.Manifest.Deprecated.(string); ok {
			fields.ReplacedBy = s
		}
	}
	return fields
}

// ReadManifestPair merges installed + bucket manifest data into InfoFields.
// When a specific bucket is requested (bucket != nil), it is used as the
// primary source for display fields; installed only contributes version/date.
func (s *ManifestService) ReadManifestPair(input string, installed, bucket *FoundManifest) InfoFields {
	_, appName := parseBucketAndApp(input)
	if installed == nil && bucket == nil {
		return InfoFields{Name: appName}
	}

	// Prefer bucket as primary when it exists (caller already filtered it to
	// the requested bucket); fall back to installed if no bucket result.
	var primary *FoundManifest
	if bucket != nil {
		primary = bucket
	} else {
		primary = installed
	}

	fields := s.ReadManifestFields(appName, primary)

	if installed != nil {
		fields.InstalledVersion = installed.Manifest.Version
	}
	if bucket != nil {
		fields.LatestVersion = bucket.Manifest.Version
	}

	if fields.InstalledVersion != "" && fields.LatestVersion != "" {
		fields.UpdateAvailable = compareVersionArrays(
			parseVersionString(fields.InstalledVersion),
			parseVersionString(fields.LatestVersion),
		) < 0
	}

	// Install date from installed app's current dir mtime.
	if installed != nil {
		if fi, err := os.Stat(filepath.Dir(installed.FilePath)); err == nil {
			fields.InstallDate = fi.ModTime()
		}
	}

	return fields
}

// parseBucketAndApp splits "bucket/app" into (bucket, app); plain "app" returns ("", app).
func parseBucketAndApp(input string) (bucket, app string) {
	parts := strings.SplitN(input, "/", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return "", input
}

// formatLicense converts a license field (string or map) into a display string.
func formatLicense(license any) string {
	if license == nil {
		return ""
	}
	switch v := license.(type) {
	case string:
		return v
	case map[string]any:
		id, _ := v["identifier"].(string)
		url, _ := v["url"].(string)
		if url != "" {
			return fmt.Sprintf("%s (%s)", id, url)
		}
		return id
	}
	return fmt.Sprintf("%v", license)
}
