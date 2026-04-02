# feat: current version (defaults to 0.1.0)
VERSION := env_var_or_default("VERSION", "0.1.0")

# build: compile scg binary for Windows x64
build:
    go build -ldflags "-X main.Version={{VERSION}} -s -w" -o dist/scg.exe ./cmd

# build: compile scg binary for Windows x64 (explicit)
build-win64:
    GOOS=windows GOARCH=amd64 go build -ldflags "-X main.Version={{VERSION}} -s -w" -o dist/scg-{{VERSION}}-windows-amd64.exe ./cmd

# build: compile scg binary for Windows x86
build-win32:
    GOOS=windows GOARCH=386 go build -ldflags "-X main.Version={{VERSION}} -s -w" -o dist/scg-{{VERSION}}-windows-386.exe ./cmd

# build: compile scg binary for Windows ARM64
build-arm64:
    GOOS=windows GOARCH=arm64 go build -ldflags "-X main.Version={{VERSION}} -s -w" -o dist/scg-{{VERSION}}-windows-arm64.exe ./cmd

# build: compile scg binaries for all architectures
build-all:
    just build-win64
    just build-win32
    just build-arm64

# build: compile scg binary with debug symbols
build-debug:
    go build -o dist/scg.exe ./cmd

# run: execute scg with arguments
run *args:
    go run ./cmd {{args}}

# build: install dependencies
install:
    go mod download
    go mod tidy

# test: run all tests
test:
    go test ./...

# test: run benchmarks
bench:
    go test -bench=. -benchmem ./...

# build: remove build artifacts
clean:
    rm -rf dist/

# style: lint code
lint:
    go vet ./...

# style: format code
fmt:
    go fmt ./...

# build: clean install and build
all: clean install build

# perf: benchmark search command
benchmark-search query="git":
    hyperfine --warmup 2 "dist/scg.exe search {{query}}" "sfsu search {{query}}"

# ===========================================
# ci: release helpers
# ===========================================

# ci: list all release tags
tags:
    git tag -l --sort=-v:refname

# docs: show current version
current-version:
    @echo "Current version: {{VERSION}}"

# ci(release): create and push a release tag
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
    git tag -a "$VERSION" -m "chore: release $VERSION"
    
    # Push tag
    echo "Pushing tag $VERSION..."
    git push origin "$VERSION"
    
    echo "✓ Release $VERSION created and pushed!"
    echo "Check CI at: https://github.com/amrkmn/scg/actions"

# ci(release): create a draft release on GitHub
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

# ci(release): delete a local and remote tag
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

# docs: show release workflow help
help-release:
    @echo "Release Workflow:"
    @echo ""
    @echo "  just release v1.0.0           Create and push release tag"
    @echo "  just draft-release v1.0.0     Create draft release (requires gh CLI)"
    @echo "  just delete-tag v1.0.0        Delete a tag"
    @echo "  just tags                     List all tags"
    @echo ""
    @echo "Manual steps:"
    @echo "  1. just release v1.0.0        Creates and pushes tag"
    @echo "  2. Wait for CI to complete    https://github.com/amrkmn/scg/actions"
    @echo "  3. Download binary            https://github.com/amrkmn/scg/releases"
