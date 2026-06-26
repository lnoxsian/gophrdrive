# Justfile for GOPHRDRV (Go Single-Binary File Server)

# List all available recipes
default:
    @just --list

# Build the production single-binary executable
build:
	CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o ./bin/gophrdrv ./cmd/gophrdrv

# Run all unit tests
test:
    go test -v -cover ./...

# Run unit tests and generate HTML coverage report
test-coverage:
    go test -coverprofile=coverage.out ./...
    go tool cover -html=coverage.out -o coverage.html
    @echo "Coverage HTML report generated at coverage.html"

# Run go fmt and go vet on the codebase
lint:
    go fmt ./...
    go vet ./...

# Run the compiled gophrdrv binary
run root="." port="8080" host="0.0.0.0" max-upload="100MB": build
    ./bin/gophrdrv --root "{{root}}" --port {{port}} --host {{host}} --max-upload "{{max-upload}}"

# Run gophrdrv in help mode to show usage flags
help-bin: build
    ./bin/gophrdrv --help

# Clean build artifacts and coverage reports
clean:
    rm -f ./bin/gophrdrv coverage.out coverage.html

# Install the compiled binary to /usr/bin/gophrdrv (requires sudo)
install: build
	sudo ./scripts/install.sh

# Update/bump the application version. Displays the current version and prompts for a new version (defaults to next patch version).
update-version version="":
	#!/usr/bin/env bash
	set -euo pipefail
	TARGET_VERSION="{{version}}"
	if [ -z "$TARGET_VERSION" ]; then
		CURRENT_VERSION=$(grep -o 'var Version = "[^"]*"' internal/version/version.go | cut -d'"' -f2)
		IFS='.' read -r major minor patch <<< "$CURRENT_VERSION"
		DEFAULT_VERSION="$major.$minor.$((patch + 1))"
		echo "Current version: $CURRENT_VERSION"
		read -r -p "Enter new version [$DEFAULT_VERSION]: " input
		TARGET_VERSION="${input:-$DEFAULT_VERSION}"
	fi
	sed -i "s/var Version = \"[^\"]*\"/var Version = \"$TARGET_VERSION\"/" internal/version/version.go
	sed -i "s/Version: [0-9.]*/Version: $TARGET_VERSION/" Plan.md
	echo "$TARGET_VERSION" > VERSION
	echo "Version updated to $TARGET_VERSION"
