# Scoop Functions Implementation Plan

Tracking file for mapping Scoop PowerShell functions to Go implementations in scg.

## Legend

| Status | Meaning |
|--------|---------|
| ✅ | Implemented in Go |
| 🔲 | Missing - needs Go implementation |
| ⚠️ | Partially implemented or needs review |
| 🔧 | Uses PowerShell fallback (acceptable) |
| ❌ | Deprecated - do not implement |
| 🛠️ | Bucket maintainer tool - not needed for runtime |

---

## 1. CORE FUNCTIONS (app installation lifecycle)

| Scoop Function | Status | Go Location | Notes |
|----------------|--------|-------------|-------|
| `install_app` | ✅ | `service/install.go` | Full workflow: download → hash → extract → hooks → shims → persist → env → shortcuts |
| `uninstall_app` | ✅ | `service/uninstall.go` | Full workflow: hooks → shims → shortcuts → PATH → env → persist → version dir |
| `update_app` | ✅ | `service/update.go` | Pre-download → hash → uninstall old → install new (skip re-download) |
| `shim_app` | ✅ | `install/shim.go` | Creates shim pairs (.shim + .exe), detects overwrites |
| `unshim_app` | ✅ | `install/shim.go` | `RemoveShims()` |
| `ensure_architecture` | ✅ | `scoop/manifest.go` | Architecture-specific field resolution |
| `get_install_info` | ✅ | `scoop/install.go` | `ReadInstallInfo()` |
| `save_install_info` | ✅ | `install/io.go` | `WriteInstallInfo()` |
| `get_manifest` | ✅ | `scoop/manifest.go` | `ReadManifest()` |
| `get_url` | ✅ | `install/download.go` | HTTP download with progress bar |
| `test_url` | ✅ | `install/download.go` | URL validation in download flow |
| `dl_with_cache` | ✅ | `install/download.go` | Cache naming: `app#version#hash.ext` |
| `cached` | ✅ | `install/download.go` | `FindCachedPath()` |
| `uri_filename` | ✅ | `install/download.go` | `DownloadFileName()` |
| `hash_for_url` | ✅ | `install/hash.go` | SHA256/512/SHA1/MD5 auto-detect |
| `hash_check` | ✅ | `install/hash.go` | `VerifyHash()` |
| `check_hash` | ✅ | `install/hash.go` | Hash computation and verification |
| `extract_7zip` | ✅ | `install/extract.go` | 7z.exe extraction for all archive types |
| `extract_msi` | ✅ | `install/extract.go` | lessmsi → msiexec fallback |
| `extract_inno` | ✅ | `install/extract.go` | innounp extraction |
| `run_hook` | ✅ | `install/hooks.go` | PowerShell pre/post install/uninstall hooks |
| `run_installer` | ✅ | `install/hooks.go` | `RunInstallerHook()` for script/file+args |
| `invoke_installer` | ✅ | `install/hooks.go` | Runs installer executables with args |
| `ensure_scoop` | ✅ | `install/helper.go` | `EnsureScoopInstalled()` |
| `warn_insecure` | ✅ | `service/install.go` | Warning for insecure installs |
| `is_insecure` | ✅ | `scoop/paths.go` | Insecure path detection |
| `secure` | ✅ | `scoop/paths.go` | Secure path resolution |
| `secure_path` | ✅ | `scoop/paths.go` | Path security validation |
| `abort` | ✅ | `service/install.go` | Error handling in install flow |

---

## 2. DOWNLOAD & NETWORK FUNCTIONS

| Scoop Function | Status | Go Location | Notes |
|----------------|--------|-------------|-------|
| `url` / `urls` | ✅ | `install/download.go` | Download manager handles single/multiple URLs |
| `aria2` | ✅ | `install/download.go` | `downloadWithAria2()` with config support |
| `progress` | ✅ | `install/download.go` | `progressWriter` with human-readable output |
| `setup_proxy` | ⚠️ | `install/download.go` | Proxy via `--proxy` CLI flag; not read from config.json automatically |
| `get_useragent` | ❌ | - | Discarded — Go default `Go-http-client/1.1` is sufficient |
| `get_cookies` | ❌ | - | Discarded — niche authenticated downloads, not needed |
| `get_github_token` | ❌ | - | Discarded — no GitHub API calls made; manifests have direct URLs, buckets use `git clone` |
| `special_url` | ❌ | - | Discarded — manifests have resolved URLs; no URL transformation needed |
| `is_gh_url` | ❌ | - | Discarded — no GitHub API integration |
| `is_gh_api_url` | ❌ | - | Discarded — no GitHub API integration |
| `get_gh_url` | ❌ | - | Discarded — no GitHub API integration |
| `get_gh_api_url` | ❌ | - | Discarded — no GitHub API integration |
| `get_magic_bytes` | ✅ | `install/extract.go` | `looksLikeTar()` + extension-based detection (`ExtractExtension()`) is sufficient |

---

## 3. ENVIRONMENT & PATH FUNCTIONS

| Scoop Function | Status | Go Location | Notes |
|--------|--------|-------------|-------|
| `env_add_path` | ✅ | `install/env.go` | PATH additions from manifest |
| `env_set` | ✅ | `install/env.go` | Environment variable setting |
| `env_rm_path` | ✅ | `install/env.go` | PATH removal |
| `env_rm` | ✅ | `install/env.go` | Environment variable removal |
| `ensure_path` | ✅ | `install/env.go` | PATH validation and setup |
| `ensure_env` | ✅ | `install/env.go` | Environment validation |
| `add_first_path` | ✅ | `install/env.go` | First PATH entry addition |
| `rm_path` | ✅ | `install/env.go` | PATH entry removal |
| `broadcast_env` | ✅ | `install/winapi_windows.go` | `BroadcastEnvironmentChange()` |
| `get_config` | ✅ | `service/config.go` | Config reading |
| `set_config` | ✅ | `service/config.go` | Config writing |

---

## 4. PERSIST DATA FUNCTIONS

| Scoop Function | Status | Go Location | Notes |
|----------------|--------|-------------|-------|
| `persist_def` | ✅ | `install/persist.go` | `ParsePersistField()` |
| `persist` | ✅ | `install/persist.go` | `SetupPersistData()` - junctions/hardlinks |
| `unpersist` | ✅ | `install/persist.go` | `RemovePersistData()` |
| `ensure_persist` | ✅ | `install/persist.go` | Persist directory creation |
| `get_persist_dir` | ✅ | `install/persist.go` | Persist directory resolution |

---

## 5. SHORTCUT FUNCTIONS

| Scoop Function | Status | Go Location | Notes |
|----------------|--------|-------------|-------|
| `create_startmenu_shortcuts` | ✅ | `install/shortcuts.go` | PowerShell COM-based shortcut creation |
| `rm_startmenu_shortcuts` | ✅ | `install/shortcuts.go` | Shortcut removal |
| `shortcut` | ✅ | `install/shortcuts.go` | `ParseShortcutsField()` |

---

## 6. BUCKET FUNCTIONS

| Scoop Function | Status | Go Location | Notes |
|----------------|--------|-------------|-------|
| `add_bucket` | ✅ | `service/buckets.go` | `BucketService.Add()` |
| `rm_bucket` | ✅ | `service/buckets.go` | `BucketService.Remove()` |
| `list_buckets` | ✅ | `service/buckets.go` | `BucketService.List()` |
| `update_bucket` | ✅ | `service/buckets.go` | `BucketService.UpdateBuckets()` |
| `known_bucket` | ✅ | `known/buckets.go` | Known bucket list |
| `unused_bucket` | ✅ | `service/buckets.go` | `BucketService.GetBucketNames()` |
| `find_manifest` | ✅ | `service/manifests.go` | `FindManifest()` |
| `search_bucket` | ✅ | `service/search.go` | `SearchService.SearchBuckets()` |
| `get_bucket_dir` | ✅ | `scoop/paths.go` | Bucket directory resolution |
| `get_manifest_dir` | ✅ | `scoop/paths.go` | Manifest directory resolution |
| `get_app_dir` | ✅ | `scoop/paths.go` | App directory resolution |
| `get_version_dir` | ✅ | `scoop/paths.go` | Version directory resolution |
| `get_current_dir` | ✅ | `scoop/paths.go` | Current version directory resolution |
| `get_current_version` | ✅ | `scoop/paths.go` | `ResolveCurrentVersion()` |
| `get_user_root` | ✅ | `scoop/paths.go` | `GetUserRoot()` |
| `get_global_root` | ✅ | `scoop/paths.go` | `GetGlobalRoot()` |
| `resolve_paths` | ✅ | `scoop/paths.go` | `ResolvePaths()` |
| `both_scopes` | ✅ | `scoop/paths.go` | `BothScopes()` |
| `scope_exists` | ✅ | `scoop/paths.go` | `ScopeExists()` |

---

## 7. GIT FUNCTIONS

| Scoop Function | Status | Go Location | Notes |
|----------------|--------|-------------|-------|
| `git_clone` | ✅ | `git/git.go` | `Clone()` with progress parsing |
| `git_pull` | ✅ | `git/git.go` | `Pull()` |
| `git_fetch` | ✅ | `git/git.go` | `Fetch()` |
| `git_remote` | ✅ | `git/git.go` | `GetRemoteURL()` |
| `git_commit` | ✅ | `git/git.go` | `GetCommitHash()`, `GetCommitsSince()` |
| `git_branch` | ✅ | `git/git.go` | `GetCurrentBranch()`, `GetRemoteTrackingBranch()` |
| `git_head` | ✅ | `git/git.go` | `ReadHEAD()` |
| `git_is_repo` | ✅ | `git/git.go` | `IsGitRepo()` |
| `has_remote_updates` | ✅ | `git/git.go` | `HasRemoteUpdates()` |
| `get_commit_count` | ✅ | `git/git.go` | `GetCommitCount()` |

---

## 8. FILE & ARCHIVE FUNCTIONS

| Scoop Function | Status | Go Location | Notes |
|----------------|--------|-------------|-------|
| `extract_dir` | ✅ | `install/extract.go` | extract_dir handling in extraction |
| `extract_to` | ✅ | `install/extract.go` | extract_to handling in extraction |
| `move_content` | ✅ | `install/extract.go` | `MoveContents()` |
| `flatten_extract_dir` | ✅ | `install/extract.go` | `FlattenExtractDir()` |
| `movefile` | ✅ | `install/extract.go` | File move operations |
| `copyfile` | ✅ | `service/install.go` | `copyFile()` |
| `removefolder` | ✅ | `service/cleanup.go` | Folder removal in cleanup |
| `get_helper_path` | ✅ | `install/helper.go` | `FindHelper()`, `Find7zip()`, etc. |
| `helper_available` | ✅ | `install/helper.go` | `HelperAvailable()` |
| `is_archive` | ✅ | `install/download.go` | `IsArchive()` |
| `extract_ext` | ✅ | `install/download.go` | `ExtractExtension()` |
| `is_msi` | ✅ | `install/extract.go` | MSI detection |
| `is_inno` | ✅ | `install/extract.go` | Inno Setup detection |
| `is_7zip` | ✅ | `install/extract.go` | 7z archive detection |
| `is_tar_wrapper` | ✅ | `install/extract.go` | Tar wrapper detection |
| `looks_like_tar` | ✅ | `install/extract.go` | Tar magic bytes detection |

---

## 9. SHIM FUNCTIONS

| Scoop Function | Status | Go Location | Notes |
|----------------|--------|-------------|-------|
| `create_shim` | ✅ | `install/shim.go` | `CreateShims()` |
| `remove_shim` | ✅ | `install/shim.go` | `RemoveShims()` |
| `shim_def` | ✅ | `install/shim.go` | `ParseBinField()` |
| `detect_overwrites` | ✅ | `install/shim.go` | `DetectShimOverwrites()` |
| `shim_path` | ✅ | `install/shim.go` | Shim path resolution |
| `shim_name` | ✅ | `install/shim.go` | `shimName()` |
| `resolve_target` | ✅ | `install/shim.go` | `resolveTarget()` |
| `write_shim_exe` | ✅ | `install/shim.go` | `writeShimExe()` |
| `find_scoop_shim` | ✅ | `install/shim.go` | `findScoopShim()` |

---

## 10. VERSION & STATUS FUNCTIONS

| Scoop Function | Status | Go Location | Notes |
|----------------|--------|-------------|-------|
| `version` | ✅ | `cmd/commands/version.go` | Version command |
| `status` | ✅ | `service/status.go` | `StatusService.CheckStatus()` |
| `list_installed` | ✅ | `service/apps.go` | `AppsService.ListInstalled()` |
| `get_app_prefix` | ✅ | `service/apps.go` | `AppsService.GetAppPrefix()` |
| `get_installed_app` | ✅ | `service/apps.go` | `AppsService.GetInstalledApp()` |
| `hold_app` | ✅ | `service/apps.go` | `AppsService.SetHold()` |
| `info` | ✅ | `service/manifests.go` | `ManifestService.ReadManifestFields()` |
| `cat_manifest` | ✅ | `cmd/commands/cat.go` | Raw manifest JSON output |
| `which` | ✅ | `service/shims.go` | `ShimService.FindExecutable()` |
| `prefix` | ✅ | `service/apps.go` | `AppsService.GetAppPrefix()` |
| `home` | ✅ | `cmd/commands/home.go` | Open homepage |
| `search` | ✅ | `service/search.go` | `SearchService.SearchBuckets()` |
| `cleanup` | ✅ | `service/cleanup.go` | `CleanupService.CleanupApp()` |
| `cache` | ✅ | `service/cache.go` | `CacheService.ListCache()`, `RemoveCache()` |

---

## 10b. CLI COMMANDS (scoop → scg)

| Scoop Command | Status | Go Location | Notes |
|---------------|--------|-------------|-------|
| `scoop bucket` | ✅ | `cmd/commands/bucket/` | Bucket subcommands (add, rm, list, known) |
| `scoop cache` | ✅ | `cmd/commands/cache.go` | Cache list/rm |
| `scoop cat` | ✅ | `cmd/commands/cat.go` | Raw manifest JSON |
| `scoop cleanup` | ✅ | `cmd/commands/cleanup.go` | Cleanup old versions |
| `scoop config` | ✅ | `cmd/commands/config.go` | Get/set config |
| `scoop hold` | ✅ | `cmd/commands/hold.go` | Hold app from updates |
| `scoop unhold` | ✅ | `cmd/commands/unhold.go` | Unhold app |
| `scoop home` | ✅ | `cmd/commands/home.go` | Open homepage |
| `scoop info` | ✅ | `cmd/commands/info.go` | App info |
| `scoop install` | ✅ | `cmd/commands/install.go` | Install app |
| `scoop list` | ✅ | `cmd/commands/list.go` | List installed apps |
| `scoop prefix` | ✅ | `cmd/commands/prefix.go` | Show app prefix |
| `scoop search` | ✅ | `cmd/commands/search.go` | Search buckets |
| `scoop status` | ✅ | `cmd/commands/status.go` | Show outdated apps |
| `scoop uninstall` | ✅ | `cmd/commands/uninstall.go` | Uninstall app |
| `scoop update` | ✅ | `cmd/commands/update.go` | Update app/bucket |
| `scoop which` | ✅ | `cmd/commands/which.go` | Find shim target |
| `scoop help` | ✅ | `cmd/commands/root.go` | Help/usage |
| `scoop version` | ✅ | `cmd/commands/version.go` | Version info |
| `scoop completion` | ✅ | `cmd/commands/completion.go` | Shell completion |
| `scoop alias` | ✅ | `cmd/commands/alias.go` | Command aliases |
| `scoop checkup` | ✅ | `cmd/commands/checkup.go` | Environment diagnostics (dirs, PATH, git, 7zip, innounp, lessmsi, dark, long paths, developer mode, NTFS, Defender exclusions) |
| `scoop depends` | ✅ | `cmd/commands/depends.go` | Dependency tree with `source/name` output, circular detection, `--arch` flag |
| `scoop shim` | ✅ | `cmd/commands/shim.go` | Shim management: add/rm/list/info subcommands |
| `scoop export` | ✅ | `cmd/commands/export.go` | JSON export of buckets, apps, config (`--config` flag) |
| `scoop import` | ✅ | `cmd/commands/import.go` | JSON import of buckets, config, and apps with hold support and source-qualified names; stdin via `-` |
| `scoop download` | 🔲 | — | Low value; `install` already downloads |
| `scoop reset` | 🔲 | — | Low value; rarely used |
| `scoop virustotal` | 🔲 | — | Low value; external service |
| `scoop create` | 🛠️ | — | Bucket maintainer tool |

---

## 11. AUDITED MISSING FUNCTIONS — POST-REVIEW

Functions previously marked as missing, audited against actual Scoop source and scg codebase.

### Discarded (no action needed)

| Function | Reason |
|----------|--------|
| `get_useragent` | Go default UA is sufficient; few servers require custom UA |
| `get_cookies` | Niche authenticated downloads; not needed for public package repos |
| `get_github_token` | No GitHub API calls made; manifests have direct URLs, buckets use `git clone` |
| `special_url` family (5 funcs) | Manifests contain resolved URLs; no GitHub release URL transformation needed |
| `ensure_pe_subsystem` | PS fallback acceptable; only affects handful of GUI packages |
| `generate_issue_url` | Developer convenience, not runtime feature |
| `get_nightly_version` | Build/CI concern, not runtime concern |
| `normalize_git_url` | Git handles URL normalization natively |

### Already implemented (plan was not tracking)

| Function | Go Location | Notes |
|----------|-------------|-------|
| `setup_proxy` | `install/download.go` | `--proxy` CLI flag wired to HTTP transport |
| `ensure_file_locks` | `install/helper.go:141` | `FindRunningProcesses()` via PowerShell |
| `cleanup_path` | `install/env.go:64` | `pathAdditions()` deduplicates with case-insensitive comparison |
| `detect_arch_emulation` | `service/install.go:571` | `detectArch()` maps `runtime.GOARCH` to Scoop arch strings |
| `get_magic_bytes` | `install/extract.go:182` | `looksLikeTar()` + `ExtractExtension()` for format detection |
| `check_outdated` | `service/status.go:38` | `CheckStatus()` compares installed vs manifest versions |
| `checkup` | `cmd/commands/checkup.go` | Environment diagnostics: dirs, PATH, git, 7zip, innounp, lessmsi, dark, long paths, developer mode, NTFS, Defender exclusions |

### Genuinely missing (1 function)

| Function | Priority | Implementation Notes |
|----------|----------|---------------------|
| `ensure_persist_acl` | Low | Set ACL inheritance on persist directories; works fine for single-user setups |

### Trivial gap (1 function)

| Function | Priority | Implementation Notes |
|----------|----------|---------------------|
| `get_proxy_config` | Low | Read `cfg["proxy"]` from config.json alongside aria2 config in `loadScoopConfig()` |

---

## 13. FUNCTIONS USING POWERSHELL FALLBACK (🔧)

These functions legitimately use PowerShell and should continue to do so:

| Function | Purpose | Reason for PS Fallback |
|----------|---------|----------------------|
| `create_startmenu_shortcuts` | Create Start Menu shortcuts | Windows COM API via PowerShell is simplest |
| `run_hook` | Execute PowerShell hooks | Hooks are PowerShell scripts by design |
| `run_installer` | Run installer scripts | Installer scripts are PowerShell |
| `ensure_pe_subsystem` | Modify PE subsystem | PS fallback acceptable; niche use case |
| `ensure_file_locks` | Detect running processes | Uses PowerShell `Get-Process` for file lock detection |

---

## 14. DEPRECATED FUNCTIONS (❌)

Do not implement these deprecated functions:

| Function | Reason |
|----------|--------|
| `shim` (old) | Replaced by `create_shim` |
| `ensure` (old) | Replaced by `ensure_scoop` |
| `deprecated_*` | Various deprecated utilities |
| `legacy_*` | Legacy functions with modern replacements |
| `old_*` | Old versions of current functions |

---

## 15. BUCKET MAINTAINER TOOLS (🛠️)

These are autoupdate/bucket maintenance functions, not needed for runtime:

| Function | Purpose |
|----------|---------|
| `autoupdate_*` | Autoupdate manifest generation |
| `generate_*` | Manifest generation utilities |
| `checkver_*` | Version checking for manifests |
| `json_*` | JSON manipulation for manifests |
| `manifest_*` | Manifest creation/editing |
| `bucket_*` (maintainer) | Bucket management for maintainers |

---

## 16. IMPLEMENTATION PRIORITY ORDER

### Remaining work (low urgency)
1. `ensure_persist_acl` — ACL inheritance on persist directories (low priority, single-user setups unaffected)
2. `get_proxy_config` — Read `cfg["proxy"]` from config.json (trivial, ~1 line)

### Previously planned — now discarded
- `setup_proxy` — Already implemented via `--proxy` CLI flag
- `get_useragent` / `get_cookies` / `get_github_token` — Not needed for scg's use case
- `special_url` family (5 funcs) — Not needed; manifests have resolved URLs
- `ensure_pe_subsystem` — PS fallback acceptable
- `ensure_file_locks` / `cleanup_path` / `detect_arch_emulation` / `check_outdated` / `get_magic_bytes` — Already implemented
- `generate_issue_url` / `get_nightly_version` / `normalize_git_url` — Not runtime concerns

---

## 17. SUMMARY STATISTICS (post-audit)

| Category | Count |
|----------|-------|
| ✅ Implemented in Go | ~68 functions |
| 🔲 Genuinely missing (low urgency) | 1 function (`ensure_persist_acl`) |
| ⚠️ Trivial gap | 1 function (`get_proxy_config` — ~1 line) |
| 🔧 PowerShell fallback (acceptable) | 5 functions |
| ❌ Discarded as unnecessary | 8 functions |
| 🛠️ Bucket maintainer tools (not needed) | 13+ functions |

**Total Scoop lib/ functions**: ~110
**Already implemented**: ~62%
**Needs implementation**: ~2% (low urgency)
**Acceptable as PS fallback**: ~5%
**Discarded as unnecessary**: ~7%
**Not needed (maintainer tools)**: ~12%

### CLI Commands (scoop → scg)

| Category | Count |
|----------|-------|
| ✅ Implemented | 25 commands (bucket, cache, cat, cleanup, config, hold, unhold, home, info, install, list, prefix, search, status, uninstall, update, which, help, version, completion, alias, checkup, depends, shim, export, import) |
| 🔲 Not implemented | 4 (download, reset, virustotal, create) |
| 🛠️ Maintainer tool | 1 (create) |

---

*Last updated: 2026-05-16*
*Based on ScoopInstaller/Scoop lib/ directory analysis (19 .ps1 files) and libexec commands (28 scoop-*.ps1)*
*Compared against scg codebase (40+ Go source files)*
*Post-audit: verified all "missing" functions against actual Scoop source; implemented 5 new commands (checkup, depends, shim, export, import)*
