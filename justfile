# Build scg binary (64-bit Windows)
# Usage: just build 0.1.0
build version="0.1.0":
    go build -ldflags "-X main.Version={{version}} -s -w" -o dist/scg.exe ./cmd

# Build for different architectures
build-win64 version="0.1.0":
    GOOS=windows GOARCH=amd64 go build -ldflags "-X main.Version={{version}} -s -w" -o dist/scg-{{version}}-windows-amd64.exe ./cmd

build-win32 version="0.1.0":
    GOOS=windows GOARCH=386 go build -ldflags "-X main.Version={{version}} -s -w" -o dist/scg-{{version}}-windows-386.exe ./cmd

build-arm64 version="0.1.0":
    GOOS=windows GOARCH=arm64 go build -ldflags "-X main.Version={{version}} -s -w" -o dist/scg-{{version}}-windows-arm64.exe ./cmd

# Build all architectures
build-all version="0.1.0":
    just build-win64 {{version}}
    just build-win32 {{version}}
    just build-arm64 {{version}}

# Build with debug symbols
build-debug:
    go build -o dist/scg.exe ./cmd

# Run scg
run *args:
    go run ./cmd {{args}}

# Install dependencies
install:
    go mod download
    go mod tidy

# Run tests
test:
    go test ./...

# Run benchmarks
bench:
    go test -bench=. -benchmem ./...

# Clean build artifacts
clean:
    rm -rf dist/

# Lint code
lint:
    go vet ./...

# Format code
fmt:
    go fmt ./...

# Full build: clean, install, build
all: clean install build

# Run hyperfine benchmark
benchmark-search query="git":
    hyperfine --warmup 2 "dist/scg.exe search {{query}}" "sfsu search {{query}}"
