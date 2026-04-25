# AGENTS

## Scope
Single Go module (`go.noz.one/scg`) — a Scoop-compatible Windows package manager CLI. Windows-only runtime; most commands shell out to Windows tools (`cmd`, `reg`, `attrib`, `msiexec`, PowerShell).

## Build prerequisites
- **Go 1.26+** (go.mod specifies `go 1.26`; CI uses `stable`)
- **Zig 0.16.0** — required to build `shim.exe`, which is `//go:embed`'d into the binary. Zig is **not optional**: `build`, `test`, and `check` all depend on `build-shim` first.
- The file `internal/install/assets/shim.exe` is gitignored and must be rebuilt locally via `make build-shim` before anything else compiles.
- Zig shim links Win32 libs (`shell32`, `shlwapi`); built with `optimize = .ReleaseSmall` and `strip = true`.

## Developer commands
Use `make` (not `just`):

- `make build` — Build to `dist/scg.exe` (depends on `build-shim`)
- `make build-shim` — Compile `shim/src/main.zig` → `internal/install/assets/shim.exe`
- `make test` — `go test ./...` (depends on `build-shim` — Zig must be available)
- `make bench` — `go test -bench=. -benchmem ./...`
- `make lint` — `golangci-lint run --timeout=5m` (no custom config file)
- `make fmt` — `go fmt ./...`
- `make check` — `fmt → lint → test` (full pre-commit flow)
- `make run ARGS=version` — Quick smoke test
- `make build-amd64 / build-386 / build-arm64` — Cross-arch builds (each also builds arch-specific shim)
- `make clean` — Remove `dist/`
- `make release` — Tag and push a version (uses `$(VERSION)`)
- `make draft-release` — Create GitHub draft release via `gh`

Run a single test or package: `go test ./internal/scoop/...` or `go test -run TestName ./internal/install/...`

Version injected at build time: `-ldflags "-X main.Version=..."`.

## Architecture
```
cmd/main.go              → entrypoint, sets Version
cmd/commands/root.go     → cobra wiring, PersistentPreRunE injects app.Context
cmd/commands/*.go        → one file per CLI command
cmd/commands/bucket/    → bucket subcommands
internal/app/context.go  → DI container: holds all service instances + logger
internal/cmdctx/         → cobra↔context bridge: Inject / FromCmd / MustFromCmd
internal/service/        → business logic (search, install, update, status, etc.)
internal/install/        → low-level ops: shims, junctions, downloads, extraction, env, hooks
internal/scoop/          → Scoop domain types: paths, manifests, install info
internal/known/          → hard-coded known buckets list
internal/ui/             → output formatting, tables, colors, loader spinner
internal/git/            → git operations for bucket management
internal/utils/          → string helpers
shim/src/main.zig        → the shim binary source (compiled separately)
```

Context flow: root `PersistentPreRunE` → `app.NewContext(version, verbose)` → `cmdctx.Inject` → individual commands use `cmdctx.MustFromCmd(cmd)` to access services.

## Key patterns
- **Scope threading**: `scoop.InstallScope` (`ScopeUser` / `ScopeGlobal`) must be passed through; resolves to `%USERPROFILE%\scoop` vs `C:\ProgramData\scoop`. All path resolution goes through `scoop.ResolvePaths(scope)`.
- **Manifest `any` typing**: `scoop.Manifest` uses `any` for fields where Scoop allows multiple JSON shapes (string, array, map). Keep parsing tolerant; see `ParseBinField`, `GetDependencies` for patterns.
- **Concurrent search**: `service/search.go` uses `sync.WaitGroup` for parallel bucket scans. Output ordering is handled in the command layer, not the worker layer.
- **Embedded shim**: `internal/install/shim.go` uses `//go:embed assets/shim.exe`; falls back to Scoop's own `kiennq/shim.exe` if embedded binary is empty. Rebuild shim with `make build-shim` whenever `shim/src/main.zig` changes.
- **JSON iteration**: Uses `json-iterator/go` (`jsonFast` var in `scoop/manifest.go`) for performance-critical manifest parsing.

## Safety
- `install`, `update`, `uninstall` modify real Scoop state (shims, junctions, env/PATH via registry).
- `cleanup` and `cache rm` delete files from Scoop directories.
- Global operations (`--global`/`-g`) write to `C:\ProgramData\scoop` — requires elevation.
- Use `--dry-run` flags where available before destructive changes.

## CI
Three jobs on `windows-latest`: **test** (build + test + smoke run), **lint** (build-shim + golangci-lint), **codeql** (build-shim + CodeQL analysis). All require Zig 0.16.0 setup step.

Release pipeline (tag `v*`): cross-arch build (amd64/386/arm64) → SHA256 checksums → GitHub release. Nightly build on cron + manual dispatch.