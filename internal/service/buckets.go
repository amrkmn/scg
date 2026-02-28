package service

import (
	"os"
	"path/filepath"
	"time"

	"go.noz.one/scg/internal/git"
	"go.noz.one/scg/internal/scoop"
)

// BucketInfo holds metadata about an installed Scoop bucket.
type BucketInfo struct {
	Name      string
	Path      string
	Source    string
	Updated   time.Time
	Manifests int
	Scope     scoop.InstallScope
}

// UpdateResult is the result of updating a single bucket.
type UpdateResult struct {
	Name    string
	Status  string // "updated" | "up-to-date" | "failed"
	Commits []string
	Error   error
}

// BucketService provides operations on Scoop buckets.
type BucketService struct {
	ctx AppContext
}

// NewBucketService creates a BucketService.
func NewBucketService(ctx AppContext) *BucketService {
	return &BucketService{ctx: ctx}
}

// List returns all installed buckets in the given scope.
// If scope is empty, both scopes are listed.
func (s *BucketService) List(scope scoop.InstallScope) ([]BucketInfo, error) {
	var paths []scoop.ScoopPaths
	if scope == "" {
		paths = scoop.BothScopes()
	} else {
		paths = []scoop.ScoopPaths{scoop.ResolvePaths(scope)}
	}

	var allBuckets []BucketInfo
	for _, p := range paths {
		buckets, err := s.listBucketsInScope(p)
		if err != nil {
			continue
		}
		allBuckets = append(allBuckets, buckets...)
	}
	return allBuckets, nil
}

func (s *BucketService) listBucketsInScope(paths scoop.ScoopPaths) ([]BucketInfo, error) {
	entries, err := os.ReadDir(paths.Buckets)
	if err != nil {
		return nil, err
	}

	type result struct {
		info BucketInfo
		err  error
	}
	ch := make(chan result, len(entries))

	count := 0
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		count++
		name := e.Name()
		bucketPath := filepath.Join(paths.Buckets, name)
		go func(n, bp string) {
			info, err := s.getBucketInfo(n, bp, paths.Scope)
			ch <- result{info, err}
		}(name, bucketPath)
	}

	var buckets []BucketInfo
	for i := 0; i < count; i++ {
		r := <-ch
		if r.err == nil {
			buckets = append(buckets, r.info)
		}
	}
	return buckets, nil
}

// getBucketInfo fetches remote URL, last commit date, and manifest count for a bucket.
func (s *BucketService) getBucketInfo(name, bucketPath string, scope scoop.InstallScope) (BucketInfo, error) {
	info := BucketInfo{Name: name, Path: bucketPath, Scope: scope}

	if git.IsGitRepo(bucketPath) {
		if url, err := git.GetRemoteURL(bucketPath); err == nil {
			info.Source = url
		}
		if t, err := git.GetLastCommitDate(bucketPath); err == nil {
			info.Updated = t
		}
	} else {
		// Non-git bucket: use directory mtime.
		if fi, err := os.Stat(bucketPath); err == nil {
			info.Updated = fi.ModTime()
		}
	}
	info.Manifests = s.GetBucketManifestCount(bucketPath)
	return info, nil
}

// GetBucketManifestCount counts .json manifest files in a bucket directory.
// Prefers the bucket/ subdirectory (modern layout); falls back to the root.
func (s *BucketService) GetBucketManifestCount(bucketPath string) int {
	subdir := filepath.Join(bucketPath, "bucket")
	if fi, err := os.Stat(subdir); err == nil && fi.IsDir() {
		return countJSONFiles(subdir)
	}
	return countJSONFiles(bucketPath)
}

// countJSONFiles counts files with a .json extension in a directory (non-recursive).
func countJSONFiles(dir string) int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	count := 0
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".json" {
			count++
		}
	}
	return count
}

// Exists reports whether a bucket with the given name exists in the given scope.
func (s *BucketService) Exists(name string, scope scoop.InstallScope) bool {
	paths := scoop.ResolvePaths(scope)
	_, err := os.Stat(filepath.Join(paths.Buckets, name))
	return err == nil
}

// Add clones a bucket from url into the given scope.
func (s *BucketService) Add(name, url string, scope scoop.InstallScope, onProgress func(current, total int)) error {
	paths := scoop.ResolvePaths(scope)
	dest := filepath.Join(paths.Buckets, name)
	return git.Clone(url, dest, git.CloneOptions{OnProgress: onProgress})
}

// Remove deletes a bucket directory from the given scope.
func (s *BucketService) Remove(name string, scope scoop.InstallScope) error {
	paths := scoop.ResolvePaths(scope)
	bucketPath := filepath.Join(paths.Buckets, name)
	return os.RemoveAll(bucketPath)
}

// UpdateBuckets updates the named buckets concurrently (one goroutine per bucket).
// onStart is called when a bucket begins updating, onComplete when it finishes.
func (s *BucketService) UpdateBuckets(
	names []string,
	scope scoop.InstallScope,
	showChangelog bool,
	onStart func(name string),
	onComplete func(result UpdateResult),
) []UpdateResult {
	ch := make(chan UpdateResult, len(names))

	for _, name := range names {
		go func(n string) {
			if onStart != nil {
				onStart(n)
			}
			result := s.updateOne(n, scope, showChangelog)
			if onComplete != nil {
				onComplete(result)
			}
			ch <- result
		}(name)
	}

	results := make([]UpdateResult, 0, len(names))
	for range names {
		results = append(results, <-ch)
	}
	return results
}

// updateOne performs a fetch+merge on a single bucket and returns the result.
// It returns "up-to-date" immediately if the remote has no new commits,
// skipping the merge entirely.
func (s *BucketService) updateOne(name string, scope scoop.InstallScope, showChangelog bool) UpdateResult {
	paths := scoop.ResolvePaths(scope)
	bucketPath := filepath.Join(paths.Buckets, name)

	if !git.IsGitRepo(bucketPath) {
		return UpdateResult{Name: name, Status: "failed", Error: os.ErrNotExist}
	}

	var hashBefore string
	if showChangelog {
		var err error
		hashBefore, err = git.GetCommitHash("HEAD", bucketPath)
		if err != nil {
			return UpdateResult{Name: name, Status: "failed", Error: err}
		}
	}

	status, _, err := git.FetchAndMerge(bucketPath)
	if err != nil {
		return UpdateResult{Name: name, Status: "failed", Error: err}
	}

	result := UpdateResult{Name: name, Status: status}
	if showChangelog && status == "updated" && hashBefore != "" {
		commits, _ := git.GetCommitsSince(hashBefore, bucketPath)
		result.Commits = commits
	}
	return result
}

// CheckScoopStatus reports whether the Scoop installation itself has remote updates.
func (s *BucketService) CheckScoopStatus(local bool) (bool, error) {
	// Scoop is installed as an app named "scoop".
	for _, p := range scoop.BothScopes() {
		scoopPath := filepath.Join(p.Apps, "scoop", "current")
		resolved, err := filepath.EvalSymlinks(scoopPath)
		if err != nil {
			continue
		}
		if !git.IsGitRepo(resolved) {
			continue
		}
		if !local {
			_ = git.Fetch(resolved)
		}
		has, err := git.HasRemoteUpdates(resolved)
		if err != nil {
			return false, err
		}
		return has, nil
	}
	return false, nil
}

// CheckBucketsStatus reports whether any bucket has remote updates available.
// If buckets is nil, it lists all buckets automatically.
func (s *BucketService) CheckBucketsStatus(local bool, buckets []BucketInfo) (bool, error) {
	if buckets == nil {
		var err error
		buckets, err = s.List("")
		if err != nil {
			return false, err
		}
	}

	type result struct {
		has bool
		err error
	}
	ch := make(chan result, len(buckets))

	for _, b := range buckets {
		go func(bp string) {
			if !git.IsGitRepo(bp) {
				ch <- result{false, nil}
				return
			}
			if !local {
				_ = git.Fetch(bp)
			}
			has, err := git.HasRemoteUpdates(bp)
			ch <- result{has, err}
		}(b.Path)
	}

	for range buckets {
		r := <-ch
		if r.has {
			return true, nil
		}
	}
	return false, nil
}

// GetBucketNames returns the names of all installed buckets in the given scope.
func (s *BucketService) GetBucketNames(scope scoop.InstallScope) []string {
	buckets, err := s.List(scope)
	if err != nil {
		return nil
	}
	names := make([]string, len(buckets))
	for i, b := range buckets {
		names[i] = b.Name
	}
	return names
}

// FindBucketDir returns the manifest directory for a bucket (prefers bucket/ subdir).
func FindBucketDir(bucketPath string) string {
	subdir := filepath.Join(bucketPath, "bucket")
	if fi, err := os.Stat(subdir); err == nil && fi.IsDir() {
		return subdir
	}
	return bucketPath
}
