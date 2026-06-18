# scg

A fast, native [Scoop](https://scoop.sh) CLI for Windows, written in Go.

`scg` is Scoop-compatible and focuses on fast operations, clean output, and an ergonomic command set for daily package management.

## Installation

### Install via Scoop (recommended)

```powershell
scoop bucket add amrkmn https://github.com/amrkmn/baldi
scoop install amrkmn/scg
```

### Build from source

Requirements:
- Go (1.26+)
- Zig (required to build `shim.exe`)
- Scoop (required to use `scg install/update/...` commands)

```powershell
git clone https://github.com/amrkmn/scg
cd scg
make build
```

Binary output:
- `dist/scg.exe`

Set a custom version string at build time:

```powershell
make build VERSION=0.1.0
```

## Quick start

```powershell
scg search git
scg install git
scg list
scg status
scg update --all
scg update *      # alias for --all
```

## Command reference

| Command | Description |
|---|---|
| `scg install <app> [app...] [url] [file] [app@version]` | Install apps from buckets, URLs, local manifests, or pinned versions |
| `scg uninstall <app> [app...]` | Uninstall one or more apps |
| `scg update <app> [app...]` | Update installed apps (requires explicit targets or `--all`) |
| `scg update --all` | Update all outdated apps |
| `scg update *` | Alias for `--all` |
| `scg list [query]` | List installed apps |
| `scg search [query]` | Search apps across buckets (no query shows all) |
| `scg info <app>` | Show app information |
| `scg status` | Show update status for installed apps |
| `scg prefix <app>` | Show an app install path |
| `scg which <command>` | Resolve command path managed by Scoop |
| `scg cleanup [app]` | Remove old installed versions |
| `scg cleanup --all --cache` | Cleanup all apps + cached installers |
| `scg cache show [app...]` | Show cached download files |
| `scg cache rm <app\|*>` | Remove cached files |
| `scg config [name] [value]` | Get/set/delete config values |
| `scg cat <app>` | Print app manifest |
| `scg create <url>` | Create a manifest skeleton from a URL |
| `scg alias <subcommand>` | Manage custom alias scripts |
| `scg checkup` | Check for Scoop environment problems |
| `scg depends <app>` | List dependencies for an app |
| `scg download <app> [app...]` | Download apps to cache without installing |
| `scg export` | Export installed apps/buckets to JSON |
| `scg import <file\|->` | Install apps from an exported JSON file |
| `scg home <app>` | Open an app's homepage |
| `scg hold <app> [app...]` | Hold apps to prevent updates |
| `scg reset <app> [app...]` | Reset apps to resolve conflicts |
| `scg shim <subcommand>` | Manage Scoop shims |
| `scg unhold <app> [app...]` | Unhold apps to allow updates |
| `scg virustotal [* \| app...]` | Check app hashes/URLs on VirusTotal |
| `scg bucket <subcommand>` | Manage Scoop buckets |
| `scg completion` | Generate PowerShell completion script |
| `scg version` | Print `scg` version |

### Bucket commands

| Command | Description |
|---|---|
| `scg bucket list` | List installed buckets |
| `scg bucket add <name> [url]` | Add a bucket |
| `scg bucket remove <name>` | Remove a bucket |
| `scg bucket update [name...]` | Update buckets |
| `scg bucket known` | List known buckets |
| `scg bucket unused` | List buckets with no installed apps |

## Useful flags

- Global scope: `--global` (`-g`) on install/update/uninstall/cleanup/cache/hold/unhold/reset/prefix
- Dry-run mode: `--dry-run` on update/cleanup/cache
- Skip hash checks: `--skip-hash-check` (`-s`) on install/update
- Don't use cache: `--no-cache` (`-k`) on install/update
- Force reinstall: `--force` (`-f`) on update
- Purge persisted data: `--purge` (`-p`) on uninstall
- Quiet mode: `--quiet` (`-q`) (global flag)
- Verbose mode: `--verbose` (`-v`) (global flag)
- Disable colors: `--no-color` (global flag)

## PowerShell completion

Load completion in current session:

```powershell
scg completion | Out-String | Invoke-Expression
```

Enable it for every new PowerShell session:

```powershell
Add-Content $PROFILE "`nInvoke-Expression (scg completion | Out-String)"
```

## Development

Use `make` targets:

```powershell
make fmt            # Format code
make lint           # Run golangci-lint
make test           # Run tests
make check          # fmt -> lint -> test
make build          # Build dist/scg.exe
make run ARGS=version
```

Cross-arch builds:

```powershell
make build-amd64
make build-386
make build-arm64
```

Rebuild and embed shim binary:

```powershell
make build-shim
```

## Intentional differences from Scoop

- **`scg update`** with no args does not self-update Scoop. Use `--all`, `*`, or explicit app names.
- **Bucket refresh** runs before every `update --all` (not only when stale). This ensures fresh manifest data.
- **`bucket update`** and **`bucket unused`** are `scg`-specific bucket subcommands.
- **`--proxy`** and **`--dry-run`** flags are `scg` extensions.
- **No Scoop self-update or SQLite cache parity** — `scg` is a standalone CLI, not a Scoop wrapper.
- **Hook helper support** covers common archive helpers, message helpers, `Invoke-ExternalCommand`, `Find-BucketDirectory`, and standard Scoop runtime variables. Full hook compatibility is not guaranteed; see `.plans/scoop-compatibility-update-and-hooks-plan.md` for detail.

## Hook compatibility

`scg` supports a subset of Scoop's PowerShell hook runtime, allowing manifests with `pre_install`, `post_install`, `installer`, and `uninstaller` hooks to function.

### Hook variables

The following variables are available in hook scripts both as PowerShell variables (`$variable`) and environment variables (`$env:variable`):

| Variable | Description |
|---|---|
| `$dir` | App's `current` directory (the version being installed) |
| `$original_dir` | The `current` directory before update (same as `$dir` for installs) |
| `$persist_dir` | Path to persisted data directory |
| `$version` | App version being installed |
| `$architecture` | Target architecture (e.g. `64bit`) |
| `$arch` | Legacy alias for `$architecture` |
| `$app` | App name |
| `$fname` | Download filename of the first URL |
| `$scoopdir` | Scoop root directory |
| `$cachedir` | Scoop cache directory |
| `$bucketsdir` | Scoop buckets directory |
| `$global` | `true` for global installs, empty string for user installs |

### Supported helper functions

| Function | Description |
|---|---|
| `Expand-7zipArchive` | Extract .7z/.tar/.gz archives using 7-Zip |
| `Expand-MsiArchive` | Extract .msi archives using lessmsi or msiexec |
| `Expand-InnoArchive` | Extract Inno Setup installers using innounp |
| `Expand-DarkArchive` | Extract WiX .msi archives using dark |
| `Expand-ZipArchive` | Extract .zip archives using PowerShell |
| `Invoke-ExternalCommand` | Run external commands (supports `-RunAs`, `-Quiet`, `-LogPath`, `-ContinueExitCodes`, `-Activity`) |
| `Get-HelperPath` | Resolve path to helper tools (7zip, lessmsi, innounp, dark, aria2, git) |
| `Get-AppFilePath` | Find an app file path across scopes |
| `Find-BucketDirectory` | Resolve a bucket's directory (supports `-Root` and `-Name`) |
| `bucketdir` | Alias for `Find-BucketDirectory` |
| `appdir` / `versiondir` / `currentdir` | Resolve app, version, and current directories |
| `persistdir` | Resolve a persist directory |
| `ensure` | Create directory if it doesn't exist |
| `movedir` | Move directories using robocopy |
| `is_admin` | Check if running as administrator |
| `error` / `abort` / `warn` / `info` / `success` | Scoop-compatible message helpers |

### Known helper limitations

- **`currentdir`** does not implement Scoop's `NO_JUNCTION` support (NTFS junctions are not used by `scg`).
- **`Expand-*Archive`** helpers require the corresponding helper tools (7-Zip, lessmsi, innounp, WiX toolset) to be installed. Error messages guide users to install them via `scg install`.
- **`Invoke-ExternalCommand`** with `-RunAs` triggers UAC elevation and may prompt for administrator credentials.
- **Third-party bucket modules** (e.g. Dorado `Invoke-ExternalCommand2`) that import helper scripts from their own `bucket/` directory have limited support beyond `Find-BucketDirectory`.



Some commands modify real Scoop state:
- `install`, `update`, `uninstall` change installed packages
- `cleanup` and `cache rm` remove files
- global operations may write to `C:\ProgramData\scoop`

Use `--dry-run` where available before destructive operations.

## License

[MIT](LICENSE)
