# Variables
VERSION ?= v0.1.0

.PHONY: build build-amd64 build-386 build-arm64 build-all
.PHONY: build-shim
.PHONY: install test bench clean lint fmt check run
.PHONY: ci-test ci-lint checksums release draft-release delete-tag tags

# Build targets
build:
	@powershell -NoProfile -File scripts/build.ps1

build-shim:
	cd shim && zig build
	@mkdir -p internal/install/assets
	cp shim/zig-out/bin/shim.exe internal/install/assets/shim.exe

build-amd64:
	@VERSION=$(VERSION) powershell -NoProfile -File scripts/build.ps1 -Arch amd64

build-386:
	@VERSION=$(VERSION) powershell -NoProfile -File scripts/build.ps1 -Arch 386

build-arm64:
	@VERSION=$(VERSION) powershell -NoProfile -File scripts/build.ps1 -Arch arm64

build-all:
	@VERSION=$(VERSION) powershell -NoProfile -File scripts/build.ps1 -All

install:
	go mod download
	go mod tidy

test:
	go test ./...

bench:
	go test -bench=. -benchmem ./...

clean:
	rm -rf dist/

lint:
	golangci-lint run --timeout=5m

fmt:
	go fmt ./...

check: fmt lint test
	@echo All checks passed!

run:
	go run -ldflags "-X main.Version=$(VERSION)" ./cmd $(ARGS)

# CI targets
ci-test: build-shim build test
	go run ./cmd version

ci-lint: build-shim
	golangci-lint run --timeout=5m

checksums: build-all
	cd dist && sha256sum *.exe > sha256sums.txt
	@echo Checksums written to dist/sha256sums.txt

# CI: release helpers
release:
	@powershell -NoProfile -File scripts/release.ps1 patch

release-minor:
	@powershell -NoProfile -File scripts/release.ps1 minor

release-major:
	@powershell -NoProfile -File scripts/release.ps1 major

release-dry:
	@powershell -NoProfile -File scripts/release.ps1 patch -DryRun

delete-tag:
	@git tag -d "$(TAG)" 2>/dev/null; git push origin --delete "$(TAG)" 2>/dev/null; echo "Deleted tag $(TAG)"
