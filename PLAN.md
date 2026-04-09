# scg Install Command — Implementation Plan

**Goal**: Implement `scg install` as part of making scg a **drop-in replacement** for Scoop.

**Architecture**: Hybrid — Go orchestration + PowerShell hooks (for `pre_install`/`post_install`).

---

## Confirmed Decisions

| Decision | Choice |
|----------|--------|
| Goal | Drop-in scoop replacement |
| Architecture | Hybrid: Go orchestration + PowerShell hooks |
| Shim language | Zig |
| Shim deployment | Pre-compiled, embedded in scg via `//go:embed` |
| Shim format | Same `.shim` INI format as scoop for compatibility |
| Shim discovery | Copy `shim.exe` per app, detect via filename |
| Error handling | Best-effort between apps, stop on hook failure |
| Rollback | None (like scoop) |
| Junctions | Windows junctions (not symlinks) |
| PowerShell | Try `pwsh.exe` first, fallback to `powershell.exe` |
| Tool discovery | Scoop apps dir first, then system PATH |
| Cache format | `<app>#<version>#<url_hash>` (same as scoop) |
| Dir structure | 100% scoop-compatible |
| Config format | Scoop-compatible JSON config |
| Dependencies | Install recursively (unless `--independent`) |
| Multiple apps | Sequential, best-effort |

---

## Directory Structure (New/Modified Files)

```
scg/
├── shim/
│   ├── build.zig                    # Zig build config
│   └── src/
│       └── main.zig                  # Shim source code
├── cmd/commands/
│   ├── root.go                      # MODIFY: register install command
│   └── install.go                    # NEW: CLI command
├── internal/
│   ├── app/
│   │   └── context.go               # MODIFY: add InstallService
│   ├── install/
│   │   ├── assets/shim.exe           # Embedded shim binary
│   │   ├── download.go              # HTTP/aria2 download manager
│   │   ├── extract.go               # Archive extraction (7z, msi, zip)
│   │   ├── hash.go                   # SHA256/SHA512 verification
│   │   ├── shim.go                   # Shim creation (embeds zig shim.exe)
│   │   ├── junction.go              # Windows junction management
│   │   ├── persist.go               # Persist data junctions
│   │   ├── env.go                    # PATH & environment variable management
│   │   ├── hooks.go                  # PowerShell hook execution
│   │   ├── io.go                     # install.json / manifest.json writing
│   │   └── helper.go                # External tool discovery (7z, aria2)
│   ├── scoop/
│   │   ├── install.go               # InstallInfo struct + read/write
│   │   ├── manifest.go              # Manifest struct + GetDependencies
│   │   └── paths.go                 # ScoopPaths, InstallScope
│   └── service/
│       └── install.go               # Install orchestration service
```

---

## Phase 1: Zig Shim

### 1.1 Create `shim/src/main.zig`

- [x] Read `{own-filename}.shim` file (INI format)
- [x] Parse `path = "..."` and `args = "..."`
- [x] Resolve target executable path
- [ ] Detect GUI vs Console subsystem (for proper window handling)
- [x] Forward all command-line arguments
- [x] Spawn target process with `CreateProcess`
- [x] Propagate exit code
- [x] Set working directory to target's parent directory
- [x] Handle missing `.shim` file with clear error message
- [x] Ctrl+C handling (SetConsoleCtrlHandler)
- [x] Job Object orphan prevention (KILL_ON_JOB_CLOSE)
- [x] Environment variable expansion (%VAR%, %~dp0)
- [x] UAC elevation fallback (ShellExecuteExW)
- [x] Custom env vars from .shim file

### 1.2 Create `shim/build.zig`

- [x] Configure ReleaseSmall build target (`x86_64-windows-gnu`)
- [x] Output `shim.exe` (~5-10KB)

### 1.3 Integration

- [x] Compile `shim.exe` and commit to repo
- [x] Add `//go:embed` in `internal/install/shim.go`
- [x] Copy `shim.exe` for each app during shim creation

**Shim format** (same as scoop):
```ini
path = "C:\Users\Name\scoop\apps\git\current\cmd\git.exe"
args = 
```

---

## Phase 2: Core Utilities

### 2.1 `internal/install/download.go`

- [x] `DownloadManager` struct with cache directory
- [x] Detect aria2 in `~/scoop/apps/aria2/current/` or system PATH
- [x] Use aria2 for multi-connection downloads if available
- [x] Fall back to Go `net/http` client
- [x] Cache downloads in `~/scoop/cache/<app>#<version>#<hash>`
- [x] Support `--no-cache` flag
- [x] Support `--proxy` flag
- [ ] Progress reporting (verbose logging only, no TUI progress)

### 2.2 `internal/install/extract.go`

- [x] `ExtractionManager` struct
- [x] Detect 7zip in `~/scoop/apps/7zip/current/` or system PATH
- [x] Detect lessmsi, innounp, dark similarly
- [x] ZIP extraction using Go `archive/zip` (native) or 7zip
- [x] 7z extraction via external `7z.exe`
- [x] MSI extraction via `lessmsi` or `msiexec`
- [x] Inno Setup extraction via `innounp`
- [x] Select extractor based on file extension and manifest hints
- [x] Support `use_external_7zip` config option (hardcoded to false, not read from config)

### 2.3 `internal/install/hash.go`

- [x] Compute SHA256 hash of file
- [x] Compute SHA512 hash of file
- [x] Compare against manifest hash
- [x] Support hash formats: `sha256:<hex>`, `sha512:<hex>`, or plain hex (assume SHA256)
- [ ] Support hash from URL (download hash file)
- [x] Skip verification when `--skip` flag is set

### 2.4 `internal/install/helper.go`

- [x] `FindHelper(name string) (string, error)` — find external tools
- [x] Search order: `~/scoop/apps/<name>/current/` → system PATH
- [x] Helpers: `7zip`, `aria2`, `lessmsi`, `innounp`, `dark`, `git`

### 2.5 `internal/install/junction.go`

- [x] `CreateJunction(link, target string) error`
- [x] `RemoveJunction(link string) error`
- [ ] `IsJunction(path string) bool`
- [x] Use Windows API (`DeviceIoControl` or `cmd /c mklink /j`)
- [x] Set read-only attribute on junction (like scoop)

---

## Phase 3: Shim & Data Management

### 3.1 `internal/install/shim.go`

- [x] Embed `shim.exe` via `//go:embed`
- [x] `CreateShim(name, targetPath, shimDir string, args string) error`
- [x] Parse manifest `bin` field (string, array, object formats)
- [x] Handle `shim_def` to extract target, name, args
- [x] Write `.shim` INI file
- [x] Copy `shim.exe` as `<name>.exe`
- [ ] Handle GUI applications (create `.shim` with proper subsystem)
- [x] `RemoveShim(name string, scope InstallScope) error`

### 3.2 `internal/install/persist.go`

- [x] `SetupPersistData(appName string, items []any, appDir, persistDir string) error`
- [x] Create junction: `apps/<app>/current/<item>` → `persist/<app>/<item>`
- [x] Create persist directory if it doesn't exist
- [x] Handle string and array persist definitions
- [x] `RemovePersistData(appName string, scope InstallScope) error`

### 3.3 `internal/install/env.go`

- [x] `AddToPath(paths []string, scope InstallScope) error`
- [x] `RemoveFromPath(paths []string, scope InstallScope) error`
- [x] `SetEnvVar(key, value string, scope InstallScope) error`
- [x] `RemoveEnvVar(key string, scope InstallScope) error`
- [x] Use Windows Registry for persistent changes (`HKCU\Environment` or `HKLM\...`)
- [x] Broadcast `WM_SETTINGCHANGE` after changes
- [x] Parse manifest `env_add_path` and `env_set` fields
- [ ] Handle `NO_JUNCTION` config for PATH isolation

---

## Phase 4: PowerShell Hooks

### 4.1 `internal/install/hooks.go`

- [x] `FindPowerShell() string` — find `pwsh.exe` first, then `powershell.exe`
- [x] `RunHook(hookType string, script []string, appDir string, envVars map[string]string) error`
- [x] Set environment variables: `$dir`, `$version`, `$architecture`, `$global`
- [x] Execute PowerShell with `-NoProfile -ExecutionPolicy Bypass`
- [x] Capture stdout/stderr
- [x] On failure: stop installation (no rollback)
- [x] Support hook types: `pre_install`, `post_install`, `installer`, `pre_uninstall`, `post_uninstall`, `uninstaller`
- [ ] Create `.cmd` wrapper (code exists in `WriteCmdShim`, not called from install pipeline)
- [ ] Create `.ps1` wrapper

---

## Phase 5: Manifest Enhancement

### 5.1 `internal/scoop/manifest.go`

- [ ] `GetArchitecture(manifest *Manifest, arch string) map[string]any` (logic inline in service)
- [x] `ResolveURL(manifest *Manifest, arch string) (string, error)` (architecture-specific)
- [x] `ResolveHash(manifest *Manifest, arch string) (string, error)` (architecture-specific)
- [ ] `ExtractBinaries(bin any, arch string) ([]ShimDef, error)` (in search.go, not scoop pkg)
- [ ] `ExtractShortcuts(shortcuts []any) ([]ShortcutDef, error)`
- [x] `ExtractPersist(persist any) []string`
- [x] `ExtractEnvAddPath(envAddPath any) []string`
- [x] `GetDependencies(depends any) []string`
- [x] Support `architecture` field with `64bit`/`32bit`/`arm64` sub-objects

### 5.2 `internal/scoop/install.go`

- [x] Extend `InstallInfo` with `Architecture` field
- [x] Add `WriteInstallInfo(path string, info *InstallInfo) error`

---

## Phase 6: Install Service & CLI

### 6.1 `internal/service/install.go`

```go
type InstallOptions struct {
    Scope        scoop.InstallScope
    Independent  bool
    NoCache      bool
    SkipHash     bool
    Arch         string
    Proxy        string
}

type InstallResult struct {
    App       string
    Version   string
    Success   bool
    Skipped   bool
    Error     error
}

type InstallService struct { ... }

func (s *InstallService) Install(apps []string, opts InstallOptions) []InstallResult
func (s *InstallService) InstallSingle(app string, opts InstallOptions) InstallResult
func (s *InstallService) ResolveDependencies(manifest *scoop.Manifest) ([]string, error)
func (s *InstallService) IsInstalled(app string, scope scoop.InstallScope) (bool, string)
func (s *InstallService) SaveInstallInfo(appDir string, info InstallInfo) error
func (s *InstallService) SaveManifest(appDir string, manifest *scoop.Manifest) error
```

- [x] All structs and methods implemented

### 6.2 `cmd/commands/install.go`

```
scg install <app> [apps...] [flags]

Flags:
  -g, --global          Install globally
  -i, --independent     Don't install dependencies
  -k, --no-cache        Don't use download cache
  -s, --skip            Skip hash validation
      --arch string     Force architecture (64bit/32bit/arm64)
      --proxy string    Download via proxy
```

- [x] CLI command implemented with all flags

### 6.3 Wire Up

- [x] Add `InstallService` to `internal/app/context.go`
- [x] Register `NewInstallCommand()` in `cmd/commands/root.go`

---

## Phase 7: Testing

### 7.1 Manual Testing

- [ ] Test with simple app (e.g., `awk`)
- [ ] Test with app that has dependencies (e.g., `nodejs`)
- [ ] Test with app that has `pre_install`/`post_install` hooks
- [ ] Test with app that has architecture-specific URLs
- [ ] Test `--global` flag
- [ ] Test `--skip` flag
- [ ] Test `--no-cache` flag
- [ ] Test `--independent` flag
- [ ] Test already-installed detection
- [ ] Test shim creation and execution
- [ ] Test persist data
- [ ] Test environment variable changes
- [ ] Test junction creation/removal

### 7.2 Compatibility Testing

- [ ] Install app with scg, verify visible in `scoop list`
- [ ] Install app with scoop, verify visible in `scg list`
- [ ] Verify shims work interchangeably
- [ ] Verify `install.json` format is compatible

---

## Known Gaps

1. **Top-level URL fallback** — `resolveDownloadURL()` only checks architecture-specific URLs, not top-level `m.URL`
2. **Hash from URL** — not implemented
3. **`IsJunction()`** — not implemented
4. **`.cmd`/`.ps1` wrappers** — `WriteCmdShim()` exists but not called from install pipeline
5. **`use_external_7zip`** — hardcoded to false, not read from config
6. **ExtractShortcuts** — Start Menu shortcuts not handled
7. **GUI subsystem detection** — shim doesn't detect GUI vs console apps
8. **`NO_JUNCTION` config** — PATH isolation not implemented

---

## Scoop Install Workflow Reference

```
scoop install <app>

1. Parse arguments & resolve app names
2. Check if already installed → warn and skip
3. Resolve dependencies (unless --independent)
   → Install each dependency first (recursive)
4. Resolve manifest from buckets
5. Determine architecture (64bit/32bit/arm64)
6. Check for running processes → warn user
7. Download to cache
   → Try aria2 if available, fallback to WebClient
   → Cache: ~/scoop/cache/<app>#<version>#<hash>
8. Verify hash (SHA256/SHA512)
   → Skip if --skip flag
9. Extract archive to ~/scoop/apps/<app>/<version>/
   → ZIP: archive/zip or 7z
   → 7z: 7zip
   → MSI: lessmsi or msiexec
   → EXE (Inno): innounp
10. Run pre_install hook (PowerShell)
    → Set env: $dir, $version, $architecture, $global
    → On failure: STOP (no rollback)
11. Run custom installer (if manifest has `installer`)
12. Create current/ junction → ~/scoop/apps/<app>/<version>/
    → Mark as read-only
13. Create shims
    → Copy shim.exe as <name>.exe
    → Write <name>.shim with path + args
    → Place in ~/scoop/shims/
14. Create Start Menu shortcuts (if manifest has `shortcuts`)
15. Setup persist data
    → Create junction: current/<item> → persist/<app>/<item>
16. Add to PATH (env_add_path)
    → Modify user/global PATH in Windows Registry
    → Broadcast WM_SETTINGCHANGE
17. Set environment variables (env_set)
    → Modify user/global env vars in Windows Registry
    → Broadcast WM_SETTINGCHANGE
18. Run post_install hook (PowerShell)
    → On failure: STOP (no rollback)
19. Save install.json to version directory
    → { "architecture": "64bit", "url": "...", "bucket": "main" }
20. Save manifest.json to version directory
21. Display success message
```

## Scoop Shim File Format Reference

```ini
path = "C:\Users\Name\scoop\apps\git\current\cmd\git.exe"
args = 
```

- `path`: Absolute path to target executable
- `args`: Additional arguments (optional, can be empty)
- File: `<name>.shim` alongside `<name>.exe` in shims directory
- `.cmd` file also created for cmd.exe compatibility
- `.ps1` file also created with `pwsh`/`powershell` detection

## Scoop Directory Structure Reference

```
~/scoop/
├── apps/
│   └── <app>/
│       ├── current/              ← Junction to active version
│       └── <version>/
│           ├── (app files)
│           ├── install.json      ← Installation metadata
│           └── manifest.json     ← Copy of original manifest
├── buckets/
│   └── <bucket>/                 ← Git repositories
│       └── bucket/
│           └── <app>.json        ← Manifest files
├── cache/
│   └── <app>#<version>#<hash>    ← Downloaded archives
├── persist/
│   └── <app>/                    ← Persistent data
├── shims/
│   ├── <name>.exe                ← Shim executables
│   ├── <name>.shim               ← Shim config files
│   └── <name>.cmd                ← CMD wrappers
└── apps/scoop/current/
    └── supporting/
        └── shims/
            ├── kiennq/shim.exe   ← Default shim binary
            ├── scoopcs/shim.exe
            └── 71/shim.exe
```

## Scoop install.json Reference

```json
{
    "architecture": "64bit",
    "url": "https://github.com/.../bucket/git.json",
    "bucket": "main"
}
```

- `architecture`: `64bit`, `32bit`, or `arm64`
- `url`: Source manifest URL or path (may be null for bucket installs)
- `bucket`: Source bucket name
- `hold`: Boolean (optional, for `scoop hold`)
