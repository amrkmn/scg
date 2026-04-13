# AGENTS

## Scope and platform
- Single Go module (`go.noz.one/scg`) targeting Windows Scoop workflows; many code paths shell out to Windows tools (`cmd`, `reg`, `attrib`, `msiexec`, PowerShell).
- CLI entrypoint: `cmd/main.go`; command wiring: `cmd/commands/root.go`; core behavior: `internal/service/*` and `internal/install/*`.

## Developer commands
Use `make` (not `just`):
- `make build` - Build scg binary to `dist/scg.exe`
- `make test` - Run tests
- `make lint` - Run golangci-lint
- `make fmt` - Format code
- `make check` - fmt -> lint -> test (pre-commit flow)
- `make build-amd64`, `make build-386`, `make build-arm64` - Cross-arch builds
- `make build-shim` - Rebuild shim.exe from `shim/src/main.zig` and refresh `internal/install/assets/shim.exe`

CI runs: `make test`, `make build`, then `make run ARGS=version`.

Version is injected at build time via `-ldflags "-X main.Version=..."`.

## Safety and side effects
- `scg install` modifies real Scoop state (`~/scoop` or `C:\ProgramData\scoop`), creates junctions, writes shims, and updates env/PATH via registry + PowerShell.
- `cleanup` removes installed versions and cache files from Scoop directories; treat as destructive.

## Architecture notes
- Scoop paths are scope-aware (`internal/scoop/paths.go`: `ScopeUser` vs `ScopeGlobal`); thread scope through new features.
- Manifest fields use permissive `any` typing (`internal/scoop/manifest.go`); keep parsing tolerant of Scoop's mixed JSON shapes.
- Search and bucket scans are intentionally concurrent (`internal/service/search.go`); preserve order-sensitive output behavior in command layer, not worker layer.
- Embedded shim binary is `internal/install/assets/shim.exe` (`//go:embed` in `internal/install/shim.go`); if shim behavior changes, rebuild under `shim/` and refresh this asset.
