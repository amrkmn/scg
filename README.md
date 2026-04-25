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
- Go (1.24+)
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
```

## Command reference

| Command | Description |
|---|---|
| `scg install <app> [app...]` | Install one or more apps |
| `scg uninstall <app> [app...]` | Uninstall one or more apps |
| `scg update <app> [app...]` | Update installed apps |
| `scg update --all` | Update all outdated apps |
| `scg list [query]` | List installed apps |
| `scg search <query>` | Search apps across buckets |
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

- Global scope: `--global` (`-g`) on install/update/uninstall/cleanup/cache
- Dry-run mode: `--dry-run` on update/cleanup/cache
- Skip hash checks: `--skip` on install/update
- Don’t use cache: `--no-cache` on install/update
- Verbose mode: `-v` (global flag)

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

## Safety notes

Some commands modify real Scoop state:
- `install`, `update`, `uninstall` change installed packages
- `cleanup` and `cache rm` remove files
- global operations may write to `C:\ProgramData\scoop`

Use `--dry-run` where available before destructive operations.

## License

[MIT](LICENSE)
