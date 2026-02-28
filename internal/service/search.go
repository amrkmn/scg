package service

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"go.noz.one/scg/internal/scoop"
)

// SearchResult holds a single search result from a bucket scan.
type SearchResult struct {
	Name        string
	Version     string
	Description string
	Binaries    []string
	Bucket      string
	Scope       scoop.InstallScope
	IsInstalled bool
}

// SearchOptions configures a bucket search operation.
type SearchOptions struct {
	Bucket        string
	CaseSensitive bool
	GlobalOnly    bool
	InstalledOnly bool
	InstalledApps map[string]*InstalledApp // keyed by name
}

// SearchService provides search across Scoop bucket manifests.
type SearchService struct {
	ctx AppContext
}

// NewSearchService creates a SearchService.
func NewSearchService(ctx AppContext) *SearchService {
	return &SearchService{ctx: ctx}
}

// SearchBuckets scans all relevant buckets concurrently and returns matching results.
func (s *SearchService) SearchBuckets(query string, opts SearchOptions) []SearchResult {
	buckets := s.findAllBuckets(opts)
	if len(buckets) == 0 {
		return nil
	}

	lowerQuery := strings.ToLower(query)

	// Fan out: one goroutine per bucket
	type bucketResult struct {
		bucket  BucketInfo
		results []SearchResult
	}
	ch := make(chan bucketResult, len(buckets))

	for _, b := range buckets {
		go func(bucket BucketInfo) {
			ch <- bucketResult{bucket, s.scanBucket(bucket, query, lowerQuery, opts)}
		}(b)
	}

	var all []SearchResult
	for range buckets {
		r := <-ch
		all = append(all, r.results...)
	}
	return all
}

// findAllBuckets returns all bucket directories to search.
func (s *SearchService) findAllBuckets(opts SearchOptions) []BucketInfo {
	svc := &BucketService{ctx: s.ctx}
	var scope scoop.InstallScope
	if opts.GlobalOnly {
		scope = scoop.ScopeGlobal
	}
	buckets, err := svc.List(scope)
	if err != nil {
		return nil
	}

	if opts.Bucket != "" {
		var filtered []BucketInfo
		for _, b := range buckets {
			if strings.EqualFold(b.Name, opts.Bucket) {
				filtered = append(filtered, b)
			}
		}
		return filtered
	}
	return buckets
}

// scanBucket scans a single bucket for manifests matching query,
// parsing JSON files in parallel using a worker pool.
func (s *SearchService) scanBucket(bucket BucketInfo, query, lowerQuery string, opts SearchOptions) []SearchResult {
	manifestDir := FindBucketDir(bucket.Path)
	entries, err := os.ReadDir(manifestDir)
	if err != nil {
		return nil
	}

	// Collect candidate filenames that pass the cheap name-only filters first.
	type candidate struct {
		name string
		path string
	}
	var candidates []candidate
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		appName := strings.TrimSuffix(e.Name(), ".json")
		if !matchQuery(appName, query, lowerQuery, opts.CaseSensitive) {
			continue
		}
		if opts.InstalledOnly && opts.InstalledApps != nil {
			if _, ok := opts.InstalledApps[strings.ToLower(appName)]; !ok {
				continue
			}
		}
		candidates = append(candidates, candidate{
			name: appName,
			path: filepath.Join(manifestDir, e.Name()),
		})
	}

	if len(candidates) == 0 {
		return nil
	}

	// For small candidate sets, sequential processing is faster than goroutine overhead
	if len(candidates) <= runtime.GOMAXPROCS(0) {
		var results []SearchResult
		for _, c := range candidates {
			m, err := scoop.ReadManifest(c.path)
			if err != nil {
				continue
			}
			results = append(results, SearchResult{
				Name:        c.name,
				Version:     m.Version,
				Description: m.Description,
				Binaries:    ExtractBinaries(m.Bin),
				Bucket:      bucket.Name,
				Scope:       bucket.Scope,
			})
		}
		return results
	}

	// Parse manifests in parallel with a worker pool sized to GOMAXPROCS.
	workers := runtime.GOMAXPROCS(0)
	if workers > len(candidates) {
		workers = len(candidates)
	}

	work := make(chan candidate, len(candidates))
	for _, c := range candidates {
		work <- c
	}
	close(work)

	resultCh := make(chan SearchResult, len(candidates))
	var wg sync.WaitGroup

	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for c := range work {
				m, err := scoop.ReadManifest(c.path)
				if err != nil {
					continue
				}
				r := SearchResult{
					Name:        c.name,
					Version:     m.Version,
					Description: m.Description,
					Binaries:    ExtractBinaries(m.Bin),
					Bucket:      bucket.Name,
					Scope:       bucket.Scope,
				}
				if opts.InstalledApps != nil {
					if app, ok := opts.InstalledApps[strings.ToLower(c.name)]; ok {
						if app.Bucket == "" || strings.EqualFold(app.Bucket, bucket.Name) {
							r.IsInstalled = true
						}
					}
				}
				resultCh <- r
			}
		}()
	}

	wg.Wait()
	close(resultCh)

	var results []SearchResult
	for r := range resultCh {
		results = append(results, r)
	}
	return results
}

// ExtractBinaries converts a manifest bin field (any) to a flat list of binary names.
func ExtractBinaries(bin any) []string {
	if bin == nil {
		return nil
	}
	switch v := bin.(type) {
	case string:
		return []string{filepath.Base(v)}
	case []any:
		var out []string
		for _, item := range v {
			switch iv := item.(type) {
			case string:
				out = append(out, filepath.Base(iv))
			case []any:
				// [target, alias, args?] — use alias if present
				if len(iv) >= 2 {
					if alias, ok := iv[1].(string); ok && alias != "" {
						out = append(out, alias)
						continue
					}
				}
				if len(iv) >= 1 {
					if target, ok := iv[0].(string); ok {
						out = append(out, filepath.Base(target))
					}
				}
			}
		}
		return out
	case map[string]any:
		var out []string
		for alias := range v {
			out = append(out, alias)
		}
		return out
	}
	return nil
}

func matchQuery(name, query, lowerQuery string, caseSensitive bool) bool {
	if caseSensitive {
		return strings.Contains(name, query)
	}
	return strings.Contains(strings.ToLower(name), lowerQuery)
}
