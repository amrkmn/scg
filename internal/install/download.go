package install

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"go.noz.one/scg/internal/scoop"
	"go.noz.one/scg/internal/ui"
)

// DownloadManager handles downloading files to the Scoop cache directory.
type DownloadManager struct {
	cacheDir string
	verbose  bool
}

type aria2Config struct {
	Enabled                bool
	RetryWait              int
	Split                  int
	MaxConnectionPerServer int
	MinSplitSize           string
	Options                []string
	Proxy                  string
}

// NewDownloadManager creates a DownloadManager for the given scope.
func NewDownloadManager(scope scoop.InstallScope, verbose bool) *DownloadManager {
	paths := scoop.ResolvePaths(scope)
	return &DownloadManager{
		cacheDir: paths.Cache,
		verbose:  verbose,
	}
}

// DownloadResult contains the result of a download operation.
type DownloadResult struct {
	CachePath  string // Full path to the cached file
	Downloaded bool   // Whether the file was newly downloaded (vs cache hit)
	UsedAria2  bool   // Whether aria2 was used for the download
	Size       int64  // File size in bytes
}

// CachePath returns the expected cache path for a file, following Scoop's
// cache naming convention: <app>#<version>#<url_hash>
func (dm *DownloadManager) CachePath(app, version, downloadURL string) string {
	hash := cacheHash(downloadURL)
	filename := DownloadFileName(app, downloadURL)

	// Handle URLs that end with a path but no filename.
	if filename == "" || filename == "." || filename == "/" {
		filename = app
	}

	return filepath.Join(dm.cacheDir, fmt.Sprintf("%s#%s#%s%s",
		strings.ToLower(app), version, hash, cacheExtension(filename)))
}

func cacheExtension(filename string) string {
	return filepath.Ext(filename)
}

func legacyCacheExtension(filename string) string {
	lower := strings.ToLower(filename)
	switch {
	case strings.HasSuffix(lower, ".tar.gz"):
		return ".tar.gz"
	case strings.HasSuffix(lower, ".tar.bz2"):
		return ".tar.bz2"
	case strings.HasSuffix(lower, ".tar.xz"):
		return ".tar.xz"
	case strings.HasSuffix(lower, ".tar.zst"):
		return ".tar.zst"
	case strings.HasSuffix(lower, ".tar.lz4"):
		return ".tar.lz4"
	default:
		return filepath.Ext(filename)
	}
}

func (dm *DownloadManager) legacyCachePath(app, version, downloadURL string) string {
	hash := cacheHash(downloadURL)
	filename := DownloadFileName(app, downloadURL)
	if filename == "" || filename == "." || filename == "/" {
		filename = app
	}
	return filepath.Join(dm.cacheDir, fmt.Sprintf("%s#%s#%s%s",
		strings.ToLower(app), version, hash, legacyCacheExtension(filename)))
}

// FindCachedPath returns an existing cache path (current format, then legacy format).
func (dm *DownloadManager) FindCachedPath(app, version, downloadURL string) (string, bool) {
	cachePath := dm.CachePath(app, version, downloadURL)
	if _, err := os.Stat(cachePath); err == nil {
		return cachePath, true
	}

	legacyPath := dm.legacyCachePath(app, version, downloadURL)
	if legacyPath != cachePath {
		if _, err := os.Stat(legacyPath); err == nil {
			return legacyPath, true
		}
	}
	return "", false
}

// DownloadFileName resolves the effective filename for a download URL.
// Scoop-style URLs can provide a rename suffix with "#/name.ext".
func DownloadFileName(app, downloadURL string) string {
	if u, err := url.Parse(downloadURL); err == nil {
		if fragment := strings.TrimSpace(u.Fragment); strings.HasPrefix(fragment, "/") {
			name := filepath.Base(strings.TrimPrefix(fragment, "/"))
			if name != "" && name != "." && name != "/" && name != `\` {
				return name
			}
		}
		if u.Path == "" || u.Path == "/" {
			return app
		}

		name := filepath.Base(u.Path)
		if name != "" && name != "." && name != "/" && name != `\` {
			return name
		}
	}

	// Fallback for malformed URLs.
	name := filepath.Base(downloadURL)
	if idx := strings.IndexAny(name, "?#"); idx >= 0 {
		name = name[:idx]
	}
	if name == "" || name == "." || name == "/" || name == `\` {
		return app
	}
	return name
}

// cacheHash computes a short hash from a URL for the cache filename.
func cacheHash(downloadURL string) string {
	h := sha256.Sum256([]byte(downloadURL))
	return hex.EncodeToString(h[:])[:7]
}

// Download downloads a file to the cache directory.
// If the file already exists in cache and useCache is true, it returns the cached path.
// If aria2 is available, it uses aria2 for multi-connection downloads.
func (dm *DownloadManager) Download(app, version, downloadURL string, useCache bool, proxy string) (*DownloadResult, error) {
	cachePath := dm.CachePath(app, version, downloadURL)

	// Check cache.
	if useCache {
		if existingPath, ok := dm.FindCachedPath(app, version, downloadURL); ok {
			fi, err := os.Stat(existingPath)
			if err != nil {
				return nil, fmt.Errorf("failed to stat cached file: %w", err)
			}
			return &DownloadResult{
				CachePath:  existingPath,
				Downloaded: false,
				UsedAria2:  false,
				Size:       fi.Size(),
			}, nil
		}
	}

	// Ensure cache directory exists.
	if err := os.MkdirAll(dm.cacheDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	cfg := loadAria2Config()
	if proxy == "" {
		proxy = cfg.Proxy
	}

	// Try aria2 first if enabled by config.
	if cfg.Enabled {
		if aria2Path, err := FindAria2(); err == nil {
			result, err := dm.downloadWithAria2(aria2Path, cachePath, downloadURL, proxy, cfg)
			if err == nil {
				return result, nil
			}
			if dm.verbose {
				_, _ = fmt.Fprintln(os.Stderr, ui.WarnLine("aria2 failed; falling back to HTTP: "+err.Error()))
			}
		}
	}

	if dm.verbose {
		_, _ = fmt.Println(ui.Heading("Downloading"))
		_, _ = fmt.Println(ui.Detail(downloadURL))
	}
	if err := dm.downloadHTTP(cachePath, app, downloadURL, proxy); err != nil {
		return nil, fmt.Errorf("failed to download %s: %w", downloadURL, err)
	}

	fi, err := os.Stat(cachePath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat downloaded file: %w", err)
	}

	return &DownloadResult{
		CachePath:  cachePath,
		Downloaded: true,
		UsedAria2:  false,
		Size:       fi.Size(),
	}, nil
}

// loadAria2Config reads Scoop config and returns aria2 settings using Scoop defaults.
func loadAria2Config() aria2Config {
	cfg := loadScoopConfig()
	return aria2Config{
		Enabled:                boolFromConfig(cfg["aria2-enabled"], true),
		RetryWait:              intFromConfig(cfg["aria2-retry-wait"], 2),
		Split:                  intFromConfig(cfg["aria2-split"], 5),
		MaxConnectionPerServer: intFromConfig(cfg["aria2-max-connection-per-server"], 5),
		MinSplitSize:           stringFromConfig(cfg["aria2-min-split-size"], "5M"),
		Options:                stringSliceFromConfig(cfg["aria2-options"]),
		Proxy:                  stringFromConfig(cfg["proxy"], ""),
	}
}

func loadScoopConfig() map[string]any {
	profile := os.Getenv("USERPROFILE")
	if profile == "" {
		profile = os.Getenv("HOME")
	}
	if profile == "" {
		return map[string]any{}
	}

	configPath := filepath.Join(profile, ".config", "scoop", "config.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return map[string]any{}
	}

	var cfg map[string]any
	if err := json.Unmarshal(data, &cfg); err != nil {
		return map[string]any{}
	}
	return cfg
}

func boolFromConfig(v any, defaultValue bool) bool {
	switch val := v.(type) {
	case bool:
		return val
	case string:
		switch strings.ToLower(strings.TrimSpace(val)) {
		case "true":
			return true
		case "false":
			return false
		default:
			return defaultValue
		}
	case float64:
		return val != 0
	default:
		return defaultValue
	}
}

func intFromConfig(v any, defaultValue int) int {
	switch val := v.(type) {
	case int:
		return val
	case float64:
		return int(val)
	case string:
		parsed := strings.TrimSpace(val)
		if parsed == "" {
			return defaultValue
		}
		var n int
		if _, err := fmt.Sscanf(parsed, "%d", &n); err == nil {
			return n
		}
		return defaultValue
	default:
		return defaultValue
	}
}

func stringFromConfig(v any, defaultValue string) string {
	if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
		return s
	}
	return defaultValue
}

func stringSliceFromConfig(v any) []string {
	switch val := v.(type) {
	case []any:
		out := make([]string, 0, len(val))
		for _, item := range val {
			if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
				out = append(out, s)
			}
		}
		return out
	case []string:
		out := make([]string, 0, len(val))
		for _, s := range val {
			if strings.TrimSpace(s) != "" {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}

func formatAria2Summary(raw string) []string {
	lines := strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n")
	start := -1
	for i, line := range lines {
		if strings.Contains(strings.ToLower(line), "download results:") {
			start = i
			break
		}
	}
	if start == -1 {
		return nil
	}
	out := make([]string, 0, len(lines)-start)
	for _, line := range lines[start:] {
		line = strings.TrimRight(line, "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		out = append(out, line)
	}
	return out
}

func printAria2Summary(raw string) {
	for _, line := range formatAria2Summary(raw) {
		_, _ = fmt.Println(ui.Detail(line))
	}
}

func isAria2ProgressLine(line string) bool {
	line = strings.TrimSpace(line)
	return strings.HasPrefix(line, "[#")
}

type aria2OutputWriter struct {
	mu      sync.Mutex
	raw     bytes.Buffer
	pending string
	last    string
	width   int
	shown   bool
}

func (w *aria2OutputWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	_, _ = w.raw.Write(p)

	if !ui.IsTTY() {
		return len(p), nil
	}

	w.pending += string(p)

	for {
		iCR := strings.IndexByte(w.pending, '\r')
		iLF := strings.IndexByte(w.pending, '\n')
		idx := -1
		if iCR >= 0 && iLF >= 0 {
			if iCR < iLF {
				idx = iCR
			} else {
				idx = iLF
			}
		} else if iCR >= 0 {
			idx = iCR
		} else if iLF >= 0 {
			idx = iLF
		}
		if idx == -1 {
			break
		}
		line := w.pending[:idx]
		w.pending = w.pending[idx+1:]
		if isAria2ProgressLine(line) {
			progress := strings.TrimSpace(line)
			if progress == w.last {
				continue
			}
			w.last = progress
			rendered := "Download: " + progress
			padding := ""
			if w.width > len(rendered) {
				padding = strings.Repeat(" ", w.width-len(rendered))
			}
			_, _ = fmt.Fprintf(os.Stderr, "\r%s%s", rendered, padding)
			w.width = len(rendered)
			w.shown = true
		}
	}

	return len(p), nil
}

func (w *aria2OutputWriter) finishProgressLine() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.shown && ui.IsTTY() {
		_, _ = fmt.Fprintln(os.Stderr)
		w.shown = false
	}
}

func (w *aria2OutputWriter) RawOutput() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.raw.String()
}

// downloadWithAria2 downloads a file using aria2c.
func (dm *DownloadManager) downloadWithAria2(aria2Path, destPath, downloadURL, proxy string, cfg aria2Config) (*DownloadResult, error) {
	args := []string{
		"--continue=true",
		fmt.Sprintf("--retry-wait=%d", cfg.RetryWait),
		fmt.Sprintf("--max-connection-per-server=%d", cfg.MaxConnectionPerServer),
		fmt.Sprintf("--split=%d", cfg.Split),
		fmt.Sprintf("--min-split-size=%s", cfg.MinSplitSize),
		"--summary-interval=1",
		"--download-result=full",
		"--console-log-level=warn",
		"--enable-color=false",
		"--file-allocation=none",
		fmt.Sprintf("--dir=%s", filepath.Dir(destPath)),
		fmt.Sprintf("--out=%s", filepath.Base(destPath)),
		"--auto-file-renaming=false",
		"--allow-overwrite=true",
	}
	args = append(args, cfg.Options...)
	if proxy != "" {
		args = append(args, fmt.Sprintf("--all-proxy=%s", proxy))
	}
	args = append(args, downloadURL)

	cmd := exec.Command(aria2Path, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out := &aria2OutputWriter{}
	cmd.Stdout = out
	cmd.Stderr = out
	err := cmd.Run()
	if err != nil {
		return nil, fmt.Errorf("aria2 failed: %w\n%s", err, strings.TrimSpace(out.RawOutput()))
	}
	out.finishProgressLine()
	printAria2Summary(out.RawOutput())

	fi, err := os.Stat(destPath)
	if err != nil {
		return nil, fmt.Errorf("aria2 completed but file not found: %w", err)
	}

	return &DownloadResult{
		CachePath:  destPath,
		Downloaded: true,
		UsedAria2:  true,
		Size:       fi.Size(),
	}, nil
}

// downloadHTTP downloads a file using Go's net/http package with a progress bar.
func (dm *DownloadManager) downloadHTTP(destPath, app, downloadURL, proxy string) error {
	client := &http.Client{
		Timeout: 30 * time.Minute,
	}
	if proxy != "" {
		proxyURL, err := url.Parse(proxy)
		if err != nil {
			return fmt.Errorf("invalid proxy URL: %w", err)
		}
		client.Transport = &http.Transport{
			Proxy: http.ProxyURL(proxyURL),
		}
	}

	resp, err := client.Get(downloadURL)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	f, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	pw := newProgressWriter(app, resp.ContentLength)
	_, err = io.Copy(io.MultiWriter(f, pw), resp.Body)
	pw.finish()
	return err
}

// progressWriter tracks download progress and displays a progress bar.
type progressWriter struct {
	app     string
	total   int64
	written int64
	start   time.Time
	prints  int64 // number of print calls for spinner animation
	width   int
}

func newProgressWriter(app string, total int64) *progressWriter {
	return &progressWriter{
		app:   app,
		total: total,
		start: time.Now(),
	}
}

func (pw *progressWriter) Write(p []byte) (int, error) {
	n := len(p)
	pw.written += int64(n)
	pw.print()
	return n, nil
}

func (pw *progressWriter) print() {
	pw.prints++
	app := strings.TrimSpace(pw.app)
	if app == "" {
		app = "download"
	}

	if !ui.IsTTY() {
		return
	}

	if pw.total <= 0 {
		frame := ui.SpinnerFrames[pw.prints%int64(len(ui.SpinnerFrames))]
		msg := fmt.Sprintf("  %s downloading %s (%s)", ui.Cyan(frame), app, humanSize(pw.written))
		padding := ""
		if pw.width > len(msg) {
			padding = strings.Repeat(" ", pw.width-len(msg))
		}
		_, _ = fmt.Fprintf(os.Stderr, "\r%s%s", msg, padding)
		pw.width = len(msg)
		return
	}
	pct := float64(pw.written) / float64(pw.total) * 100
	if pct > 100 {
		pct = 100
	}
	barWidth := 50
	filled := int(pct / 100 * float64(barWidth))
	if filled > barWidth {
		filled = barWidth
	}
	bar := strings.Repeat("=", filled)
	if filled < barWidth {
		bar += ">"
		bar += strings.Repeat(" ", barWidth-filled-1)
	}
	msg := fmt.Sprintf("  downloading %s (%s) [%s] %.0f%%", app, humanSize(pw.total), bar, pct)
	padding := ""
	if pw.width > len(msg) {
		padding = strings.Repeat(" ", pw.width-len(msg))
	}
	_, _ = fmt.Fprintf(os.Stderr, "\r%s%s", msg, padding)
	pw.width = len(msg)
}

func (pw *progressWriter) finish() {
	if ui.IsTTY() {
		pw.written = pw.total
		pw.print()
		_, _ = fmt.Fprintln(os.Stderr)
	}
}

func humanSize(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(b)/float64(div), "KMGTPE"[exp])
}
