package service

import (
	"os"
	"strconv"
	"strings"
	"sync"

	"go.noz.one/scg/internal/install"
	"go.noz.one/scg/internal/scoop"
)

// Download concurrency tuning for the Homebrew-style prefetch phase.
// Flag --download-concurrency wins, then SCG_DOWNLOAD_CONCURRENCY, then default.
const (
	DefaultDownloadConcurrency = 4
	MaxDownloadConcurrency     = 16
)

// ResolveDownloadConcurrency normalizes an explicit flag value using the
// SCG_DOWNLOAD_CONCURRENCY env var and clamps the result to [1, Max].
// An explicit value <= 0 means "auto".
func ResolveDownloadConcurrency(explicit int) int {
	if explicit > 0 {
		return clampDownloadConcurrency(explicit)
	}
	if raw, ok := os.LookupEnv("SCG_DOWNLOAD_CONCURRENCY"); ok {
		if n, err := strconv.Atoi(strings.TrimSpace(raw)); err == nil && n > 0 {
			return clampDownloadConcurrency(n)
		}
	}
	return DefaultDownloadConcurrency
}

func clampDownloadConcurrency(n int) int {
	if n < 1 {
		return 1
	}
	if n > MaxDownloadConcurrency {
		return MaxDownloadConcurrency
	}
	return n
}

// fetchTask is one URL to pre-warm into the Scoop cache.
type fetchTask struct {
	App     string
	Version string
	URL     string
	Scope   scoop.InstallScope
	Proxy   string
	NoCache bool
	Verbose bool
}

func fetchTaskKey(app, url string) string {
	return app + "\x00" + url
}

// dedupeFetchTasks drops exact app+URL duplicates while preserving order.
func dedupeFetchTasks(tasks []fetchTask) []fetchTask {
	seen := make(map[string]struct{}, len(tasks))
	out := make([]fetchTask, 0, len(tasks))
	for _, t := range tasks {
		if t.URL == "" {
			continue
		}
		k := fetchTaskKey(t.App, t.URL)
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		out = append(out, t)
	}
	return out
}

// fetchDownloads fetches tasks concurrently with bounded parallelism.
// Workers perform no logging; callers report results sequentially to keep
// output stable. Returns per-task errors keyed by fetchTaskKey.
func fetchDownloads(tasks []fetchTask, concurrency int) map[string]error {
	return fetchDownloadsWith(tasks, concurrency, fetchOne)
}

func fetchDownloadsWith(tasks []fetchTask, concurrency int, fetch func(fetchTask) error) map[string]error {
	tasks = dedupeFetchTasks(tasks)
	errs := make(map[string]error, len(tasks))
	if len(tasks) == 0 {
		return errs
	}
	if concurrency < 1 {
		concurrency = DefaultDownloadConcurrency
	}
	if concurrency == 1 {
		for _, t := range tasks {
			errs[fetchTaskKey(t.App, t.URL)] = fetch(t)
		}
		return errs
	}
	if concurrency > len(tasks) {
		concurrency = len(tasks)
	}

	var mu sync.Mutex
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	for _, t := range tasks {
		wg.Add(1)
		go func(task fetchTask) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			err := fetch(task)
			mu.Lock()
			errs[fetchTaskKey(task.App, task.URL)] = err
			mu.Unlock()
		}(t)
	}
	wg.Wait()
	return errs
}

func fetchOne(t fetchTask) error {
	dm := install.NewDownloadManager(t.Scope, t.Verbose)
	_, err := dm.Download(t.App, t.Version, t.URL, !t.NoCache, t.Proxy)
	return err
}

// prefetchUpdateTargets pre-warms update downloads before any uninstall.
// Returns the set of app keys whose prefetch hard-failed (missing manifest or
// unresolvable URL); per-URL download errors are left for updateSingle to
// report so single-network-flake behavior is unchanged.
func (s *UpdateService) prefetchUpdateTargets(targets []InstalledApp, needsAttention map[string]AppStatusResult, opts UpdateOptions) map[string]bool {
	var tasks []fetchTask
	failed := map[string]bool{}
	arch := opts.Arch
	if arch == "" {
		arch = detectArch()
	}
	for _, app := range targets {
		_, appName := parseBucketAndApp(app.Name)
		lookup := app.Name
		if app.Bucket != "" {
			lookup = app.Bucket + "/" + appName
		}
		_, bucket := s.manifests.FindManifestPair(lookup)
		if bucket == nil {
			failed[appKey(app.Name, app.Scope)] = true
			continue
		}
		urls, err := resolveDownloadURLs(bucket.Manifest, arch)
		if err != nil {
			failed[appKey(app.Name, app.Scope)] = true
			continue
		}
		for _, u := range urls {
			tasks = append(tasks, fetchTask{
				App: appName, Version: bucket.Manifest.Version, URL: u,
				Scope: app.Scope, Proxy: opts.Proxy, NoCache: opts.NoCache,
				Verbose: s.ctx.GetVerbose(),
			})
		}
	}
	if len(tasks) == 0 {
		return failed
	}
	if !opts.Quiet {
		s.ctx.GetLogger().Header("Fetching downloads")
	}
	fetchDownloads(tasks, ResolveDownloadConcurrency(opts.DownloadConcurrency))
	return failed
}
