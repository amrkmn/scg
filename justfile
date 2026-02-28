# Get version from env or use default
VERSION := env_var_or_default("VERSION", "0.1.0")

# Build scg binary (64-bit Windows)
build:
    go build -ldflags "-X main.Version={{VERSION}} -s -w" -o dist/scg.exe ./cmd

# Build for different architectures
build-win64:
    GOOS=windows GOARCH=amd64 go build -ldflags "-X main.Version={{VERSION}} -s -w" -o dist/scg-{{VERSION}}-windows-amd64.exe ./cmd

build-win32:
    GOOS=windows GOARCH=386 go build -ldflags "-X main.Version={{VERSION}} -s -w" -o dist/scg-{{VERSION}}-windows-386.exe ./cmd

build-arm64:
    GOOS=windows GOARCH=arm64 go build -ldflags "-X main.Version={{VERSION}} -s -w" -o dist/scg-{{VERSION}}-windows-arm64.exe ./cmd

# Build all architectures
build-all:
    just build-win64
    just build-win32
    just build-arm64

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
