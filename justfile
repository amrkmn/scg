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

# ci(release): create and push a release tag (usage: just release v1.0.0)
release version:
    @echo "Creating release {{version}}..."
    just test
    git tag -a "{{version}}" -m "chore: release {{version}}"
    git push origin "{{version}}"
    @echo "✓ Release {{version}} created!"
    @echo "Check CI at: https://github.com/amrkmn/scg/actions"

# ci(release): create a draft release on GitHub
draft-release version body="Release notes":
    gh release create "{{version}}" --draft --title "Release {{version}}" --notes "{{body}}"

# ci(release): delete a local and remote tag
delete-tag version:
    git tag -d "{{version}}"
    git push origin --delete "{{version}}"
    @echo "✓ Deleted tag {{version}}"

# docs: show release workflow help
help-release:
    @echo "Release Workflow:"
    @echo ""
    @echo "  just release v1.0.0           Create and push release tag"
    @echo "  just draft-release v1.0.0     Create draft release (requires gh CLI)"
    @echo "  just delete-tag v1.0.0        Delete a tag"
    @echo "  just tags                     List all tags"
    @echo ""
    @echo "Note: Version must include 'v' prefix (e.g., v1.0.0)"
