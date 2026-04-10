# AGENTS

## Scope and platform
- This is a single Go module (`go.noz.one/scg`) targeting Windows Scoop workflows; many code paths shell out to Windows tools (`cmd`, `reg`, `attrib`, `msiexec`, PowerShell).
- CLI entrypoint is `cmd/main.go`; command wiring is in `cmd/commands/root.go`; core behavior lives in `internal/service/*` plus `internal/install/*`.

## High-value commands
- Prefer `just` tasks over ad-hoc commands: `just test`, `just build`, `just lint`, `just fmt`, `just check`.
- `just check` runs `fmt -> lint -> test` (same intended pre-commit flow).
- CI on GitHub Actions runs on `windows-latest` and effectively does: `just test`, `just build`, then `just run version`.
- Focused tests: `go test ./internal/...` or `go test ./internal/install -run TestParseBinField`.

## Safety and side effects
- `scg install` and install service code modify real Scoop state (`~/scoop` or `C:\ProgramData\scoop`), create junctions, write shims, and update persistent env/`PATH` via registry + PowerShell; avoid running these in a normal dev shell unless you intend host changes.
- `cleanup` can remove installed versions and cache files from Scoop directories; treat as destructive.

## Build and release specifics
- Version is injected at build time via `-ldflags "-X main.Version=..."`; use `VERSION` env with just recipes.
- Multi-arch Windows artifacts are produced via `just build-amd64`, `just build-386`, `just build-arm64` (or `just build-all`).
- Embedded shim binary is `internal/install/assets/shim.exe` (`//go:embed` in `internal/install/shim.go`); if shim behavior changes, rebuild under `shim/` and refresh this asset.

## Repo conventions that are easy to miss
- Scoop paths are scope-aware via `internal/scoop/paths.go` (`ScopeUser` vs `ScopeGlobal`); thread scope through new features.
- Manifest fields use permissive `any` typing in `internal/scoop/manifest.go`; keep parsing tolerant of Scoop's mixed JSON shapes.
- Search and bucket scans are intentionally concurrent in `internal/service/search.go`; preserve order-sensitive output behavior in command layer, not worker layer.
