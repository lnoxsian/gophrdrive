# GOPHRDRV Development & Tooling Guide

This guide describes development workflows, testing suites, code styling, and production compilation optimizations for GOPHRDRV.

---

## 1. Developer Workflows (`Justfile`)

GOPHRDRV uses [Just](https://github.com/casey/just) to organize build tasks. You can run developer tasks easily:

```bash
# List all available recipes
just

# Build the optimized production binary
just build

# Run all unit tests
just test

# Generate an HTML test coverage report
just test-coverage

# Run code formatters and check static analysis issues
just lint

# Compile and launch gophrdrv locally
just run

# Clean binary and coverage files
just clean
```

### Running with Custom Parameters
You can pass overrides directly to `just` commands:
```bash
# Serves '/home/user/documents' on port 9000
just run root="/home/user/documents" port="9000"
```

---

## 2. Manual Commands (Standard Go Tooling)

If the `just` task runner is not installed, use standard Go CLI commands:

### Formatting & Linting
Ensure codebase style compliance before committing code:
```bash
# Format source code files
go fmt ./...

# Run code analysis checks
go vet ./...
```

### Running Tests
All tests are implemented using standard testing constructs. Run the test suite:
```bash
# Run unit tests
go test -v -cover ./...

# Run tests and generate a coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html
```

---

## 3. Production Binary Size Optimizations

To compile the application into a highly-optimized single executable:

```bash
CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o ./bin/gophrdrv ./cmd/gophrdrv
```

### Compilation Flag Details:
* `CGO_ENABLED=0`: Compiles a completely static binary. This removes dynamic links to standard host libraries (like `glibc`), making the binary independent and portable across Linux systems.
* `-trimpath`: Trims developer file paths from error stack traces, keeping source directories private and reducing binary size.
* `-ldflags="-s -w"`: Passes optimizing linker flags to the compiler:
  * `-s`: Strips the global symbol table, saving ~1.5MB.
  * `-w`: Strips DWARF debugging information, saving ~1.0MB.
