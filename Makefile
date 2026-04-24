# Variables
VERSION = 0.1.0

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

test: build-shim
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

# CI: release helpers
tags:
	git tag -l --sort=-v:refname

release:
	@echo Creating release $(VERSION)...
	go test ./...
	git tag -a "$(VERSION)" -m "chore: release $(VERSION)"
	git push origin "$(VERSION)"
	@echo Release $(VERSION) created!

draft-release:
	gh release create "$(VERSION)" --draft --title "Release $(VERSION)" --notes "$(BODY)"

delete-tag:
	git tag -d "$(VERSION)"
	git push origin --delete "$(VERSION)"
	@echo Deleted tag $(VERSION)
