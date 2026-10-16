package service

import (
	"testing"
	"time"

	"go.noz.one/scg/internal/scoop"
)

func TestResolveDownloadConcurrency(t *testing.T) {
	t.Setenv("SCG_DOWNLOAD_CONCURRENCY", "")
	if got := ResolveDownloadConcurrency(0); got != DefaultDownloadConcurrency {
		t.Fatalf("default = %d, want %d", got, DefaultDownloadConcurrency)
	}
	if got := ResolveDownloadConcurrency(8); got != 8 {
		t.Fatalf("explicit 8 = %d", got)
	}
	if got := ResolveDownloadConcurrency(100); got != MaxDownloadConcurrency {
		t.Fatalf("clamp = %d, want %d", got, MaxDownloadConcurrency)
	}
	if got := ResolveDownloadConcurrency(-3); got != DefaultDownloadConcurrency {
		t.Fatalf("negative = %d, want default", got)
	}

	t.Setenv("SCG_DOWNLOAD_CONCURRENCY", "6")
	if got := ResolveDownloadConcurrency(0); got != 6 {
		t.Fatalf("env 6 = %d", got)
	}
	t.Setenv("SCG_DOWNLOAD_CONCURRENCY", " 3 ")
	if got := ResolveDownloadConcurrency(0); got != 3 {
		t.Fatalf("env spaced 3 = %d", got)
	}
	t.Setenv("SCG_DOWNLOAD_CONCURRENCY", "bogus")
	if got := ResolveDownloadConcurrency(0); got != DefaultDownloadConcurrency {
		t.Fatalf("env bogus = %d, want default", got)
	}
	// Explicit flag wins over env.
	t.Setenv("SCG_DOWNLOAD_CONCURRENCY", "6")
	if got := ResolveDownloadConcurrency(2); got != 2 {
		t.Fatalf("explicit over env = %d, want 2", got)
	}
}

func TestDedupeFetchTasks(t *testing.T) {
	tasks := []fetchTask{
		{App: "git", URL: "https://example.com/a.zip", Scope: scoop.ScopeUser},
		{App: "git", URL: "https://example.com/a.zip", Scope: scoop.ScopeUser},
		{App: "git", URL: "", Scope: scoop.ScopeUser},
		{App: "node", URL: "https://example.com/a.zip", Scope: scoop.ScopeUser},
	}
	got := dedupeFetchTasks(tasks)
	if len(got) != 2 {
		t.Fatalf("deduped len = %d, want 2", len(got))
	}
	if got[0].App != "git" || got[1].App != "node" {
		t.Fatalf("deduped order = %v", got)
	}
}

func TestFetchDownloadsEmpty(t *testing.T) {
	if errs := fetchDownloads(nil, 4); len(errs) != 0 {
		t.Fatalf("empty tasks errs = %d, want 0", len(errs))
	}
}

// TestFetchDownloadsOverlap proves tasks genuinely overlap: with concurrency 2
// both workers must be inside fetch at the same time (no timing involved,
// the fake fetch blocks until released).
func TestFetchDownloadsOverlap(t *testing.T) {
	tasks := []fetchTask{
		{App: "aaa", URL: "https://example.com/a.zip", Scope: scoop.ScopeUser},
		{App: "bbb", URL: "https://example.com/b.zip", Scope: scoop.ScopeUser},
	}
	started := make(chan string, 2)
	release := make(chan struct{})
	fake := func(task fetchTask) error {
		started <- task.App
		<-release
		return nil
	}
	done := make(chan map[string]error, 1)
	go func() { done <- fetchDownloadsWith(tasks, 2, fake) }()

	next := func() string {
		t.Helper()
		select {
		case app := <-started:
			return app
		case <-time.After(10 * time.Second):
			t.Fatal("timed out waiting for overlapping fetch to start")
			return ""
		}
	}
	first, second := next(), next()
	if first == second {
		t.Fatalf("same task started twice: %q", first)
	}
	close(release)
	errs := <-done
	if len(errs) != 2 {
		t.Fatalf("errs = %d, want 2", len(errs))
	}
	for _, err := range errs {
		if err != nil {
			t.Fatalf("unexpected fetch error: %v", err)
		}
	}
}

// TestFetchDownloadsSerial proves concurrency 1 never overlaps: the second
// task must not start while the first is still blocked inside fetch.
func TestFetchDownloadsSerial(t *testing.T) {
	tasks := []fetchTask{
		{App: "aaa", URL: "https://example.com/a.zip", Scope: scoop.ScopeUser},
		{App: "bbb", URL: "https://example.com/b.zip", Scope: scoop.ScopeUser},
	}
	started := make(chan string, 2)
	release := make(chan struct{})
	fake := func(task fetchTask) error {
		started <- task.App
		<-release
		return nil
	}
	done := make(chan map[string]error, 1)
	go func() { done <- fetchDownloadsWith(tasks, 1, fake) }()

	first := <-started
	select {
	case second := <-started:
		t.Fatalf("task %q started while %q still in flight", second, first)
	case <-done:
		t.Fatalf("fetch returned while a task was still blocked")
	default:
	}
	close(release)
	errs := <-done
	if len(errs) != 2 {
		t.Fatalf("errs = %d, want 2", len(errs))
	}
}
