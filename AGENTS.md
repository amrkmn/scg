# AGENTS.md

Guidance for coding agents working in this repository.

## Project overview

`scg` is a Scoop-compatible Windows package manager CLI written in Go.

- Module: `go.noz.one/scg`
- Runtime target: Windows
- CLI framework: Cobra
- Main entrypoint: `cmd/main.go`
- Core behavior: reimplements Scoop operations in Go, with PowerShell/Windows tools where needed for compatibility.

Use the real Scoop installation as the compatibility reference when behavior is unclear. The active Scoop app usually lives under `~/scoop/apps/scoop/current`. User scope resolves under `%USERPROFILE%\scoop`; global scope resolves under `C:\ProgramData\scoop`.

## Before changing code

- Keep changes surgical and directly tied to the request.
- Match existing style; do not introduce broad refactors while fixing a focused issue.
- Be careful with commands that mutate real Scoop state.
- Prefer unit tests over integration tests that touch the host Scoop installation.
- If behavior differs from Scoop, preserve Scoop compatibility unless the user explicitly asks otherwise.

## Build prerequisites

- Go 1.26+ (`go.mod` uses `go 1.26`; CI uses `stable`).
- Zig 0.16.0 for building the embedded shim.
- Windows for normal runtime behavior and CI parity.

The shim binary is generated from `shim/src/main.zig` and copied to `internal/install/assets/shim.exe`, which is embedded by `internal/install/shim.go`.

`internal/install/assets/shim.exe` is gitignored. If it is missing, stale, or `shim/` changes, run:

```sh
make build-shim
```

## Developer commands

Use `make`.

```sh
make build                 # Build dist/scg.exe; runs build-shim first
make build-shim            # Build shim/src/main.zig into internal/install/assets/shim.exe
make test                  # Run go test ./...
make bench                 # Run benchmarks
make lint                  # Run golangci-lint --timeout=5m
make fmt                   # Run go fmt ./...
make check                 # fmt -> lint -> test
make run ARGS=version      # Run the CLI through go run
make clean                 # Remove dist/
make build-amd64           # Cross-build amd64 Windows exe and shim
make build-386             # Cross-build 386 Windows exe and shim
make build-arm64           # Cross-build arm64 Windows exe and shim
make build-all             # Build all release architectures
```

Single package/test examples:

```sh
go test ./internal/scoop/...
go test -run TestParseBinField ./internal/install/...
```

Version injection uses:

```sh
go build -ldflags "-X main.Version=<version>"
```

## Architecture map

```text
cmd/main.go              -> entrypoint, version variable, alias expansion
cmd/commands/root.go     -> root Cobra command, global flags, app context injection
cmd/commands/*.go        -> top-level CLI commands
cmd/commands/bucket/     -> bucket subcommands
internal/app/            -> app Context, logger, service wiring
internal/cmdctx/         -> Cobra context bridge: Inject / FromCmd / MustFromCmd
internal/service/        -> business logic: install, update, search, status, cleanup, etc.
internal/install/        -> low-level install operations: downloads, hashes, extraction, shims, env, hooks
internal/scoop/          -> Scoop paths, manifests, install metadata, scope types
internal/git/            -> git commands used for bucket management
internal/known/          -> known Scoop bucket list
internal/ui/             -> output formatting, tables, colors, loader
internal/utils/          -> small helpers
shim/src/main.zig        -> Windows shim executable source
```

Context flow:

```text
cmd/main.go
  -> commands.NewRootCommand(version)
  -> commands.ExpandAliasArgs(...)
  -> root.Execute()
  -> root PersistentPreRunE creates app.NewContext(version, verbose)
  -> cmdctx.Inject(...)
  -> command handlers call cmdctx.MustFromCmd(cmd)
  -> ctx.Services.<Service>
```

## Key implementation patterns

### Scope threading

Always thread `scoop.InstallScope` through operations that touch Scoop state.

- `scoop.ScopeUser` -> `%USERPROFILE%\scoop`
- `scoop.ScopeGlobal` -> `C:\ProgramData\scoop`

Use `scoop.ResolvePaths(scope)` for Scoop paths. Do not hand-build root/app/shim/cache paths in new code unless there is a clear reason.

### Manifest parsing

Scoop manifests allow multiple JSON shapes for the same field. `scoop.Manifest` intentionally uses `any` for fields like `url`, `hash`, `bin`, `depends`, `persist`, hooks, and architecture-specific data.

Keep parsing tolerant. Follow existing helpers such as:

- `scoop.GetDependencies`
- `install.ParseBinField`
- `install.ParsePersistItems`
- architecture helpers in `internal/service/install.go`

### Service layer

Commands should stay thin:

1. parse flags and args,
2. resolve scope/options,
3. call a service,
4. print summaries/errors.

Business logic belongs in `internal/service/` or `internal/install/`, not directly in Cobra command files.

### Shim handling

- Go writes `.shim` metadata files and shim `.exe` files in the Scoop shims directory.
- The shim executable is built from Zig and embedded with `//go:embed assets/shim.exe`.
- `internal/install/shim.go` falls back to Scoop's `kiennq/shim.exe` if the embedded binary is unavailable.
- Rebuild the shim after changing `shim/src/main.zig` or `shim/build.zig`.

### Output

Use `internal/ui` and the app logger instead of ad-hoc formatting where possible. Keep output stable because command output is user-facing.

## Safety rules

These commands can modify real local Scoop state:

- `install`
- `update`
- `uninstall`
- `reset`
- `cleanup`
- `cache rm`
- env/PATH, shim, shortcut, junction, and hook operations

Global operations (`--global` / `-g`) write under `C:\ProgramData\scoop` and may require elevation.

Use `--dry-run` where available before destructive changes. Avoid running mutating commands during tests or repo exploration unless the user explicitly asks.

## Testing guidance

- Tests use the standard library `testing` package.
- Prefer table-driven tests.
- Keep tests unit-level and isolated from the developer's real Scoop installation.
- Use temp directories and environment overrides where possible.
- Existing tests live mostly under `internal/`; `cmd/commands/root_alias_test.go` covers alias expansion.
- `testify` is present indirectly in `go.mod` but is not used; do not add new dependency on it without a clear reason.

Before finishing Go changes, run at least:

```sh
go test ./...
```

For full pre-commit validation, run:

```sh
make check
```

If `go test ./...` fails because the embedded shim asset is missing, run `make build-shim` and retry.

## CI and release

CI runs on `windows-latest` for pushes/PRs to `main` and `master`, ignoring docs-only changes. Jobs:

- `test`: setup Go/Zig, `make build`, `make test`, smoke `make run ARGS=version`
- `lint`: setup Go/Zig, `make build-shim`, `golangci-lint`
- `codeql`: setup Zig, `make build-shim`, CodeQL Go analysis

Release workflow:

- triggered by `v*` tags or manual dispatch,
- builds `amd64`, `386`, and `arm64` Windows binaries,
- runs tests,
- generates artifact attestation (`actions/attest`),
- a GitHub release with SHA256 checksums is created only for tag pushes; manual dispatch uploads build artifacts but does not create a release.

Nightly workflow:

- scheduled daily and manual,
- builds all release architectures,
- publishes/updates the `nightly` prerelease tag.

## Changelog and commits

- Commits use Conventional Commits (`feat:`, `fix(scope):`, `ci(...)`, ...).
- Repo-local skill `.agents/skills/changelog-skill` converts commits into Keep a Changelog entries for `CHANGELOG.md`; load it for changelog work.

## Common pitfalls

- Do not assume manifest fields are strings; many can be arrays or maps.
- Do not bypass `scoop.ResolvePaths` for scoped paths.
- Do not run real install/update/uninstall flows as tests.
- Do not forget to rebuild `internal/install/assets/shim.exe` after shim changes.
- Do not move command business logic into Cobra handlers.
- Do not reformat unrelated files.
