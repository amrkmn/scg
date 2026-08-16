package install

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"regexp"
	"sort"
	"strings"
	"time"

	"go.noz.one/scg/internal/scoop"
	"go.noz.one/scg/internal/ui"
)

// hashTemplateRegexes mirrors Scoop's find_hash_in_textfile template substitutions,
// used to expand placeholders like $sha256 inside a hash 'find'/'regex' pattern.
var hashTemplateRegexes = map[string]string{
	"$md5":      `([a-fA-F0-9]{32})`,
	"$sha1":     `([a-fA-F0-9]{40})`,
	"$sha256":   `([a-fA-F0-9]{64})`,
	"$sha512":   `([a-fA-F0-9]{128})`,
	"$checksum": `([a-fA-F0-9]{32,128})`,
	"$base64":   `([a-zA-Z0-9+\/=]{24,88})`,
}

// userAgent is sent on checksum-file requests; Scoop sends a similar identifier
// (Get-UserAgent) and some hosts reject or throttle clients without one.
const userAgent = "scg (+https://github.com/amrkmn/scg)"

var (
	base64ShapeRe = regexp.MustCompile(`^(?:[A-Za-z0-9+/]{4})*(?:[A-Za-z0-9+/]{2}==|[A-Za-z0-9+/]{3}=|[A-Za-z0-9+/]{4})$`)
	hexOnlyRe     = regexp.MustCompile(`^[a-fA-F0-9]+$`)
)

// VersionSubstitutions returns the version substitution variables for autoupdate
// templates (keys without the leading '$'), mirroring Scoop's Get-VersionSubstitution
// in lib/autoupdate.ps1.
func VersionSubstitutions(version string) map[string]string {
	firstPart, lastPart := version, version
	if i := strings.Index(version, "-"); i >= 0 {
		firstPart = version[:i]
		lastPart = version[strings.LastIndex(version, "-")+1:]
	}

	parts := strings.Split(firstPart, ".")
	part := func(i int) string {
		if i < len(parts) {
			return parts[i]
		}
		return ""
	}

	sepRe := regexp.MustCompile(`[._-]`)
	sep := func(repl string) string {
		return sepRe.ReplaceAllString(version, repl)
	}

	vars := map[string]string{
		"version":           version,
		"dotVersion":        sep("."),
		"underscoreVersion": sep("_"),
		"dashVersion":       sep("-"),
		"cleanVersion":      sep(""),
		"majorVersion":      part(0),
		"minorVersion":      part(1),
		"patchVersion":      part(2),
		"buildVersion":      part(3),
		"preReleaseVersion": lastPart,
	}

	// $matchHead / $matchTail, mirroring Scoop's '(?<head>\d+\.\d+(?:\.\d+)?)(?<tail>.*)'.
	if m := regexp.MustCompile(`^(\d+\.\d+(?:\.\d+)?)(.*)$`).FindStringSubmatch(version); m != nil {
		vars["matchHead"] = m[1]
		vars["matchTail"] = m[2]
	}

	return vars
}

// SubstituteAutoupdate substitutes variables ($name / ${name}) into an autoupdate
// template, mirroring Scoop's substitute.
func SubstituteAutoupdate(template string, vars map[string]string) string {
	return expandTemplateVars(template, vars)
}

// GenerateManifestForVersion builds a manifest for a pinned version of an app,
// mirroring Scoop's generate_user_manifest + Invoke-AutoUpdate:
//   - If the requested version equals the manifest's current version, the manifest is
//     returned unchanged.
//   - Otherwise the manifest's autoupdate templates are used to resolve download URLs
//     (and hashes) for the requested version. Apps without autoupdate capability cannot
//     be pinned, matching Scoop.
func GenerateManifestForVersion(m *scoop.Manifest, appName, version string, scope scoop.InstallScope, proxy string, verbose bool) (*scoop.Manifest, error) {
	if m.Version == version {
		return m, nil
	}
	if strings.TrimSpace(version) == "" {
		return nil, fmt.Errorf("invalid version %q", version)
	}
	if m.Autoupdate == nil {
		return nil, fmt.Errorf("'%s' does not have autoupdate capability; couldn't find manifest for '%s@%s'", appName, appName, version)
	}
	autoupdate, ok := m.Autoupdate.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("invalid autoupdate field in manifest for '%s'", appName)
	}

	generated, err := cloneManifest(m)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare manifest for %s@%s: %w", appName, version, err)
	}
	generated.Version = version
	vars := VersionSubstitutions(version)

	// Top-level url.
	if m.URL != nil {
		if tmpl, ok := autoupdate["url"]; ok {
			sub, ok := substituteURLTemplate(tmpl, vars)
			if !ok {
				return nil, fmt.Errorf("could not generate manifest for %s@%s: unsupported autoupdate url template", appName, version)
			}
			generated.URL = sub
		}
	}

	// Per-architecture urls. Mirrors Scoop, which only rewrites arch urls when there is
	// no top-level url in the manifest.
	if m.URL == nil {
		for arch, rawSection := range generated.Architecture {
			section, ok := rawSection.(map[string]any)
			if !ok {
				continue
			}
			if _, hasURL := section["url"]; !hasURL {
				continue
			}
			if tmpl := autoupdateTemplate(autoupdate, arch, "url"); tmpl != nil {
				if sub, ok := substituteURLTemplate(tmpl, vars); ok {
					section["url"] = sub
				}
			}
		}
	}

	// extract_dir / extract_to often embed the version; rewrite them like Scoop does for
	// any autoupdate-templated property (top-level, then per-arch).
	for _, prop := range []string{"extract_dir", "extract_to"} {
		if tmpl, ok := autoupdate[prop].(string); ok && tmpl != "" {
			switch prop {
			case "extract_dir":
				generated.ExtractDir = SubstituteAutoupdate(tmpl, vars)
			case "extract_to":
				generated.ExtractTo = SubstituteAutoupdate(tmpl, vars)
			}
		}
	}
	for _, prop := range []string{"extract_dir", "extract_to"} {
		for arch, rawSection := range generated.Architecture {
			section, ok := rawSection.(map[string]any)
			if !ok {
				continue
			}
			if _, hasProp := section[prop]; !hasProp {
				continue
			}
			if tmpl := autoupdateTemplate(autoupdate, arch, prop); tmpl != nil {
				if s, ok := tmpl.(string); ok {
					section[prop] = SubstituteAutoupdate(s, vars)
				}
			}
		}
	}

	// Hashes: top-level when the manifest has a top-level hash, otherwise per-arch.
	if m.Hash != nil {
		urls, err := resolvedTemplateURLs(autoupdate, "", "url", vars)
		if err != nil {
			return nil, fmt.Errorf("could not generate manifest for %s@%s: %w", appName, version, err)
		}
		cfg := autoupdateHashConfig(autoupdate, "")
		hashes, err := resolveAutoupdateHashes(appName, version, urls, cfg, scope, proxy, verbose)
		if err != nil {
			return nil, fmt.Errorf("could not generate manifest for %s@%s: %w", appName, version, err)
		}
		generated.Hash = singleOrList(hashes)
	} else {
		for arch, rawSection := range generated.Architecture {
			section, ok := rawSection.(map[string]any)
			if !ok {
				continue
			}
			if _, hasHash := section["hash"]; !hasHash {
				continue
			}
			urls, err := resolvedTemplateURLs(autoupdate, arch, "url", vars)
			if err != nil {
				return nil, fmt.Errorf("could not generate manifest for %s@%s: %w", appName, version, err)
			}
			cfg := autoupdateHashConfig(autoupdate, arch)
			hashes, err := resolveAutoupdateHashes(appName, version, urls, cfg, scope, proxy, verbose)
			if err != nil {
				return nil, fmt.Errorf("could not generate manifest for %s@%s: %w", appName, version, err)
			}
			section["hash"] = singleOrList(hashes)
		}
	}

	return generated, nil
}

// resolvedTemplateURLs substitutes a URL template for the given arch (or the top-level
// template when arch is "") and flattens it to a string slice.
func resolvedTemplateURLs(autoupdate map[string]any, arch, prop string, vars map[string]string) ([]string, error) {
	tmpl := autoupdate[prop]
	if arch != "" {
		tmpl = autoupdateTemplate(autoupdate, arch, prop)
	}
	if tmpl == nil {
		return nil, fmt.Errorf("no autoupdate %s template for %s", prop, orArch(arch))
	}
	sub, ok := substituteURLTemplate(tmpl, vars)
	if !ok {
		return nil, fmt.Errorf("unsupported autoupdate %s template for %s", prop, orArch(arch))
	}
	urls := templateURLs(sub)
	if len(urls) == 0 {
		return nil, fmt.Errorf("autoupdate %s template for %s produced no URLs", prop, orArch(arch))
	}
	return urls, nil
}

func orArch(arch string) string {
	if arch == "" {
		return "top level"
	}
	return arch
}

// autoupdateTemplate returns the autoupdate template for a property, preferring the
// architecture-specific override and falling back to the top-level template. When the
// top-level template is a per-architecture map (e.g. autoupdate.url), the entry for the
// requested arch is used instead.
func autoupdateTemplate(autoupdate map[string]any, arch, prop string) any {
	if archCfg, ok := autoupdate["architecture"].(map[string]any); ok {
		if asection, ok := archCfg[arch].(map[string]any); ok {
			if v, ok := asection[prop]; ok {
				return v
			}
		}
	}
	if v, ok := autoupdate[prop]; ok {
		if m, isMap := v.(map[string]any); isMap {
			if archVal, ok := m[arch]; ok {
				return archVal
			}
			return nil
		}
		return v
	}
	return nil
}

// autoupdateHashConfig returns the hash extraction config for the given arch, preferring
// autoupdate.architecture[arch].hash and falling back to autoupdate.hash.
func autoupdateHashConfig(autoupdate map[string]any, arch string) any {
	if arch != "" {
		if archCfg, ok := autoupdate["architecture"].(map[string]any); ok {
			if asection, ok := archCfg[arch].(map[string]any); ok {
				if v, ok := asection["hash"]; ok {
					return v
				}
			}
		}
	}
	return autoupdate["hash"]
}

// substituteURLTemplate substitutes a URL template (string or array of strings).
// Returns ok=false for unsupported shapes (e.g. a per-arch map used at the top level).
func substituteURLTemplate(tmpl any, vars map[string]string) (any, bool) {
	switch v := tmpl.(type) {
	case string:
		return SubstituteAutoupdate(v, vars), true
	case []any:
		out := make([]any, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				out = append(out, SubstituteAutoupdate(s, vars))
			}
		}
		return out, true
	case []string:
		out := make([]any, 0, len(v))
		for _, s := range v {
			out = append(out, SubstituteAutoupdate(s, vars))
		}
		return out, true
	default:
		return nil, false
	}
}

// templateURLs flattens a substituted URL template into a string slice.
func templateURLs(sub any) []string {
	switch v := sub.(type) {
	case string:
		if v != "" {
			return []string{v}
		}
	case []any:
		var out []string
		for _, item := range v {
			if s, ok := item.(string); ok && s != "" {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

// singleOrList returns a bare string for a single-element list and a []any otherwise,
// matching the flexible string-or-array shapes Scoop manifests use for url/hash fields.
func singleOrList(items []string) any {
	if len(items) == 1 {
		return items[0]
	}
	out := make([]any, 0, len(items))
	for _, s := range items {
		out = append(out, s)
	}
	return out
}

// cloneManifest deep-copies a manifest via JSON round-trip so generated manifests can be
// modified without mutating shared state.
func cloneManifest(m *scoop.Manifest) (*scoop.Manifest, error) {
	data, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}
	return scoop.ParseManifestBytes(data)
}

// resolveAutoupdateHashes resolves hashes for each URL using the hash extraction config.
// Configs can be a single map or a list of maps (per URL, reusing the last one), matching
// Scoop's HashHelper. When extraction is not possible, the file is downloaded and its
// SHA256 is computed, again matching Scoop's fallback.
func resolveAutoupdateHashes(appName, version string, urls []string, cfg any, scope scoop.InstallScope, proxy string, verbose bool) ([]string, error) {
	configs := hashConfigs(cfg)
	hashes := make([]string, 0, len(urls))
	for i, u := range urls {
		config := configs[min(i, len(configs)-1)]
		hash, err := hashForURL(appName, version, u, config, scope, proxy, verbose)
		if err != nil {
			return nil, err
		}
		hashes = append(hashes, hash)
	}
	return hashes, nil
}

func hashConfigs(cfg any) []map[string]any {
	switch v := cfg.(type) {
	case nil:
		return []map[string]any{nil}
	case map[string]any:
		return []map[string]any{v}
	case []any:
		out := make([]map[string]any, 0, len(v))
		for _, item := range v {
			if m, ok := item.(map[string]any); ok {
				out = append(out, m)
			} else {
				out = append(out, nil)
			}
		}
		if len(out) == 0 {
			return []map[string]any{nil}
		}
		return out
	}
	return []map[string]any{nil}
}

// hashForURL resolves a single download URL's hash, mirroring Scoop's get_hash_for_app:
// extract mode reads a checksum file and finds the hash with a regex; any other mode (or
// failure) falls back to downloading the file and computing SHA256.
func hashForURL(appName, version, rawURL string, cfg map[string]any, scope scoop.InstallScope, proxy string, verbose bool) (string, error) {
	stripped := stripFragment(rawURL)
	basename := urlBasename(stripped)

	vars := VersionSubstitutions(version)
	vars["url"] = stripped
	vars["baseurl"] = strings.TrimSuffix(path.Dir(stripped), "/")
	vars["basename"] = basename
	vars["urlNoExt"] = stripExt(stripped)
	vars["basenameNoExt"] = stripExt(basename)

	// Resolve the checksum file URL and extraction mode.
	var hashfileURL string
	mode := ""
	if cfg != nil {
		if v, ok := cfg["mode"].(string); ok {
			mode = v
		}
		if v, ok := cfg["url"].(string); ok {
			hashfileURL = SubstituteAutoupdate(v, vars)
		}
		if mode == "" && hashfileURL != "" {
			mode = "extract"
		}
	}

	hash := ""
	var extractErr error
	if mode == "extract" {
		regex := ""
		if cfg != nil {
			if v, ok := cfg["find"].(string); ok {
				regex = v
			} else if v, ok := cfg["regex"].(string); ok {
				regex = v
			}
		}
		hash, extractErr = findHashInTextfile(hashfileURL, regex, vars, proxy)
	}

	// Fall back to computing the hash from the downloaded file (Scoop does the same when
	// extraction is unavailable or fails).
	if hash == "" {
		if verbose {
			msg := fmt.Sprintf("hash extraction failed for %s; computing from download", basename)
			if extractErr != nil {
				msg += fmt.Sprintf(" (%v)", extractErr)
			}
			_, _ = fmt.Fprintln(os.Stderr, ui.WarnLine(msg))
		}
		cachePath, err := autoupdateDownload(appName, version, rawURL, scope, proxy, verbose)
		if err != nil {
			return "", err
		}
		hash, err = ComputeSHA256(cachePath)
		if err != nil {
			return "", err
		}
	}
	if hash == "" {
		return "", fmt.Errorf("could not determine hash for %s", basename)
	}
	return hash, nil
}

// findHashInTextfile downloads a checksum file and extracts a hash with the given regex,
// mirroring Scoop's find_hash_in_textfile (including gzip support and the $md5/$sha256/...
// template substitutions).
func findHashInTextfile(hashfileURL, findRegex string, vars map[string]string, proxy string) (string, error) {
	if hashfileURL == "" {
		return "", nil
	}
	data, err := autoupdateFetchURL(hashfileURL, proxy)
	if err != nil {
		return "", err
	}

	// Scoop transparently decompresses gzip-compressed checksum files.
	if len(data) >= 2 && data[0] == 0x1F && data[1] == 0x8B {
		if zr, err := gzip.NewReader(bytes.NewReader(data)); err == nil {
			if decompressed, err := io.ReadAll(zr); err == nil {
				data = decompressed
			}
			_ = zr.Close()
		}
	}
	text := string(data)

	regex := findRegex
	if regex == "" {
		regex = `^\s*([a-fA-F0-9]+)\s*$`
	}
	// Hash template placeholders expand to raw regex fragments; version/filename
	// variables are regex-escaped (mirrors Scoop's substitute with escape=true).
	for k, v := range hashTemplateRegexes {
		regex = strings.ReplaceAll(regex, k, v)
	}
	regex = substituteEscaped(regex, vars)

	re, err := regexp.Compile(regex)
	if err != nil {
		return "", fmt.Errorf("invalid hash find regex %q: %w", findRegex, err)
	}
	hash := ""
	if m := re.FindStringSubmatch(text); len(m) >= 2 {
		hash = strings.TrimSpace(m[1])
	}

	// Fallback: locate the hash by filename in the checksum file.
	if hash == "" {
		basename := regexp.QuoteMeta(vars["basename"])
		fnRe := `([a-fA-F0-9]{32,128})[\x20\t]+.*` + basename + `(?:\s|$)|` + basename + `[\x20\t]+.*?([a-fA-F0-9]{32,128})`
		if re, err := regexp.Compile(fnRe); err == nil {
			if m := re.FindStringSubmatch(text); len(m) >= 2 {
				hash = strings.TrimSpace(m[1])
			}
		}
	}

	return formatHash(decodeBase64Hash(hash)), nil
}

// substituteEscaped substitutes variables into a regex pattern, escaping each value
// (mirrors Scoop's substitute with escape=true).
func substituteEscaped(pattern string, vars map[string]string) string {
	if pattern == "" || len(vars) == 0 {
		return pattern
	}
	keys := make([]string, 0, len(vars))
	for k := range vars {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if len(keys[i]) == len(keys[j]) {
			return keys[i] < keys[j]
		}
		return len(keys[i]) > len(keys[j])
	})
	for _, k := range keys {
		v := regexp.QuoteMeta(vars[k])
		pattern = strings.ReplaceAll(pattern, "$"+k, v)
		pattern = strings.ReplaceAll(pattern, "${"+k+"}", v)
	}
	return pattern
}

// decodeBase64Hash converts a base64-encoded hash to lowercase hex, mirroring Scoop's
// base64 handling in find_hash_in_textfile. Pure hex hashes of standard lengths are
// returned unchanged.
func decodeBase64Hash(hash string) string {
	if hash == "" {
		return ""
	}
	if !base64ShapeRe.MatchString(hash) {
		return hash
	}
	if hexOnlyRe.MatchString(hash) &&
		(len(hash) == 32 || len(hash) == 40 || len(hash) == 64 || len(hash) == 128) {
		return hash
	}
	decoded, err := base64.StdEncoding.DecodeString(hash)
	if err != nil {
		return hash
	}
	return hex.EncodeToString(decoded)
}

// formatHash normalizes an extracted hash, mirroring Scoop's format_hash: strips a
// 'sha256:' prefix and infers the algorithm from the hex length. Unknown lengths yield
// an empty string (the caller falls back to computing the hash).
func formatHash(hash string) string {
	hash = strings.ToLower(strings.TrimSpace(hash))
	hash = strings.TrimPrefix(hash, "sha256:")
	switch len(hash) {
	case 32:
		return "md5:" + hash
	case 40:
		return "sha1:" + hash
	case 64:
		return hash
	case 128:
		return "sha512:" + hash
	}
	return ""
}

func stripFragment(u string) string {
	if i := strings.Index(u, "#"); i >= 0 {
		return u[:i]
	}
	return u
}

func stripExt(s string) string {
	if i := strings.LastIndex(s, "."); i > 0 {
		return s[:i]
	}
	return s
}

// urlBasename returns the decoded filename of a URL, ignoring query strings and
// fragments (mirrors Scoop's url_remote_filename + UrlDecode).
func urlBasename(u string) string {
	u = stripFragment(u)
	if i := strings.IndexAny(u, "?"); i >= 0 {
		u = u[:i]
	}
	if i := strings.LastIndex(u, "/"); i >= 0 {
		u = u[i+1:]
	}
	if i := strings.LastIndex(u, `\`); i >= 0 {
		u = u[i+1:]
	}
	if decoded, err := url.PathUnescape(u); err == nil {
		u = decoded
	}
	return u
}

// fetchURL downloads a URL (used for checksum files).
func fetchURL(target, proxy string) ([]byte, error) {
	client := &http.Client{Timeout: 5 * time.Minute}
	if proxy != "" {
		proxyURL, err := url.Parse(proxy)
		if err != nil {
			return nil, fmt.Errorf("invalid proxy URL: %w", err)
		}
		client.Transport = &http.Transport{Proxy: http.ProxyURL(proxyURL)}
	}
	req, err := http.NewRequest(http.MethodGet, target, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}
	return io.ReadAll(resp.Body)
}

// autoupdateFetchURL and autoupdateDownload are indirection points so tests can serve
// checksum content and downloaded files without network access.
var (
	autoupdateFetchURL = fetchURL

	autoupdateDownload = func(appName, version, rawURL string, scope scoop.InstallScope, proxy string, verbose bool) (string, error) {
		dm := NewDownloadManager(scope, verbose)
		res, err := dm.Download(appName, version, rawURL, true, proxy)
		if err != nil {
			return "", err
		}
		return res.CachePath, nil
	}
)
