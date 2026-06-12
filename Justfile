# Justfile for GOPHRDRV (Go Single-Binary File Server)

default:
    @just --list

# Build the production single-binary executable
build:
	CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o fileserver ./cmd/fileserver

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

# Run the compiled fileserver binary
run root="." port="8080" host="0.0.0.0" max-upload="100MB": build
    ./fileserver --root "{{root}}" --port {{port}} --host {{host}} --max-upload "{{max-upload}}"

# Run the fileserver in help mode to show usage flags
help: build
    ./fileserver --help

# Clean build artifacts and coverage reports
clean:
    rm -f fileserver coverage.out coverage.html

# Update/bump the application version. Bumps the patch version if no argument is specified (e.g. just update-version [1.0.2])
update-version version="":
	#!/usr/bin/env bash
	set -euo pipefail
	TARGET_VERSION="{{version}}"
	if [ -z "$TARGET_VERSION" ]; then
		CURRENT_VERSION=$(grep -o 'var Version = "[^"]*"' internal/version/version.go | cut -d'"' -f2)
		IFS='.' read -r major minor patch <<< "$CURRENT_VERSION"
		TARGET_VERSION="$major.$minor.$((patch + 1))"
	fi
	sed -i "s/var Version = \"[^\"]*\"/var Version = \"$TARGET_VERSION\"/" internal/version/version.go
	sed -i "s/Version: [0-9.]*/Version: $TARGET_VERSION/" Plan.md
	echo "Version updated to $TARGET_VERSION"
