# Build scg binary
build:
    go build -ldflags "-X main.Version=0.1.0 -s -w" -o dist/scg.exe ./cmd

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
