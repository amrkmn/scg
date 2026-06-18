# Variables
# Auto-detect version from git. Override with VERSION=vX.Y.Z on command line or env.
LOCAL_SHA := $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
LOCAL_DATE := $(shell date +%Y.%m.%d 2>/dev/null || echo unknown)
DEFAULT_VERSION := v0.0.0-dev.$(LOCAL_DATE).$(LOCAL_SHA)
VERSION ?= $(DEFAULT_VERSION)

.PHONY: build build-amd64 build-386 build-arm64 build-all
.PHONY: build-shim build-shim-amd64 build-shim-386 build-shim-arm64
.PHONY: install test bench clean lint fmt check run
.PHONY: ci-test ci-lint checksums release draft-release delete-tag tags

# Build targets
build: build-shim
	go build -ldflags "-X main.Version=$(VERSION) -s -w" -o dist/scg.exe ./cmd

build-amd64: build-shim-amd64
	GOOS=windows GOARCH=amd64 go build -ldflags "-X main.Version=$(VERSION) -s -w" -o dist/scg-amd64.exe ./cmd

build-386: build-shim-386
	GOOS=windows GOARCH=386 go build -ldflags "-X main.Version=$(VERSION) -s -w" -o dist/scg-386.exe ./cmd

build-arm64: build-shim-arm64
	GOOS=windows GOARCH=arm64 go build -ldflags "-X main.Version=$(VERSION) -s -w" -o dist/scg-arm64.exe ./cmd

build-all: build-amd64 build-386 build-arm64

build-shim:
	cd shim && zig build
	@mkdir -p internal/install/assets
	cp shim/zig-out/bin/shim.exe internal/install/assets/shim.exe

build-shim-amd64:
	cd shim && zig build -Dtarget=x86_64-windows --prefix "zig-out/x64"
	@mkdir -p internal/install/assets
	cp shim/zig-out/x64/bin/shim.exe internal/install/assets/shim.exe

build-shim-386:
	cd shim && zig build -Dtarget=x86-windows --prefix "zig-out/x86"
	@mkdir -p internal/install/assets
	cp shim/zig-out/x86/bin/shim.exe internal/install/assets/shim.exe

build-shim-arm64:
	cd shim && zig build -Dtarget=aarch64-windows --prefix "zig-out/arm64"
	@mkdir -p internal/install/assets
	cp shim/zig-out/arm64/bin/shim.exe internal/install/assets/shim.exe

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
	go run ./cmd $(ARGS)

# CI targets
ci-test: build-shim build test
	go run ./cmd version

ci-lint: build-shim
	golangci-lint run --timeout=5m

checksums: build-all
	cd dist && sha256sum *.exe > sha256sums.txt
	@echo Checksums written to dist/sha256sums.txt

# CI: release helpers
tags:
	git tag -l --sort=-v:refname

release:
	@echo Creating release $(VERSION)...
	go test ./...
	git tag -a "$(VERSION)" -m "chore: release $(VERSION)"
	git push origin "$(VERSION)"
	@echo Release $(VERSION) tag pushed! CI will build and attach artifacts.

draft-release: build-all checksums
	gh release create "$(VERSION)" --draft --title "Release $(VERSION)" --notes "$(BODY)" dist/scg-*.exe dist/sha256sums.txt

delete-tag:
	git tag -d "$(VERSION)"
	git push origin --delete "$(VERSION)"
	@echo Deleted tag $(VERSION)
