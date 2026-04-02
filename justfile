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

# ===========================================
# Release helpers
# ===========================================

# List all tags
tags:
    git tag -l --sort=-v:refname

# Show current version
current-version:
    @echo "Current version: {{VERSION}}"

# Create and push a release tag (usage: just release v1.0.0)
release version:
    #!/usr/bin/env bash
    set -e
    
    VERSION="{{version}}"
    
    # Ensure version starts with 'v'
    if [[ ! "$VERSION" =~ ^v ]]; then
        VERSION="v$VERSION"
    fi
    
    echo "Creating release $VERSION..."
    
    # Check for uncommitted changes
    if ! git diff --quiet; then
        echo "Error: You have uncommitted changes. Commit or stash them first."
        exit 1
    fi
    
    # Run tests
    just test
    
    # Create tag
    git tag -a "$VERSION" -m "Release $VERSION"
    
    # Push tag
    echo "Pushing tag $VERSION..."
    git push origin "$VERSION"
    
    echo "✓ Release $VERSION created and pushed!"
    echo "Check CI at: https://github.com/amrkmn/scg/actions"

# Create a draft release (usage: just draft-release v1.0.0 "Release notes here")
draft-release version body="Release notes":
    #!/usr/bin/env bash
    set -e
    
    VERSION="{{version}}"
    if [[ ! "$VERSION" =~ ^v ]]; then
        VERSION="v$VERSION"
    fi
    
    # Check if gh is installed
    if ! command -v gh &> /dev/null; then
        echo "Error: GitHub CLI (gh) is required for this command."
        echo "Install from: https://cli.github.com"
        exit 1
    fi
    
    gh release create "$VERSION" --draft --title "Release $VERSION" --notes "{{body}}"

# Delete a local and remote tag (usage: just delete-tag v1.0.0)
delete-tag version:
    #!/usr/bin/env bash
    set -e
    
    VERSION="{{version}}"
    if [[ ! "$VERSION" =~ ^v ]]; then
        VERSION="v$VERSION"
    fi
    
    git tag -d "$VERSION"
    git push origin --delete "$VERSION"
    echo "✓ Deleted tag $VERSION"

# Show release instructions
help-release:
    @echo "Release Workflow:"
    @echo ""
    @echo "  just release v1.0.0           Create and push release tag"
    @echo "  just draft-release v1.0.0     Create draft release (requires gh CLI)"
    @echo "  just delete-tag v1.0.0        Delete a tag"
    @echo "  just tags                      List all tags"
    @echo ""
    @echo "Manual steps:"
    @echo "  1. just release v1.0.0        Creates and pushes tag"
    @echo "  2. Wait for CI to complete     https://github.com/amrkmn/scg/actions"
    @echo "  3. Download binary             https://github.com/amrkmn/scg/releases"
