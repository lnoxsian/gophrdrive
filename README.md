# GOPHRDRV: Go Single-Binary File Server

GOPHRDRV is a lightweight, secure, high-performance file server written entirely in Go using only the standard library. The application compiles into a single executable binary with no runtime dependencies, no external packages, no database, and no separate frontend assets.

---

## Table of Contents
1. [Overview & Design Aesthetics](#overview--design-aesthetics)
2. [Quick Start](#quick-start)
   - [System Requirements](#system-requirements)
   - [Standard Build Commands](#standard-build-commands)
   - [Running the Server](#running-the-server)
3. [Module Reference & Architecture](#module-reference--architecture)
   - [Architecture Diagram](#architecture-diagram)
   - [Module Directory Structure](#module-directory-structure)
   - [In-Depth Module Documentation](#in-depth-module-documentation)
4. [Developer Task Guide (Justfile)](#developer-task-guide-justfile)
5. [Security Controls](#security-controls)
6. [Performance & Binary Size Optimization](#performance--binary-size-optimization)

---

## Overview & Design Aesthetics

GOPHRDRV is designed for fast, zero-dependency self-hosting. It adheres strictly to the following principles:
* **Single Binary**: Embedded static template files (`index.html`, `view.html`, `error.html`) are compiled directly into the binary using Go's native `embed` package.
* **Pure High-Contrast Monochrome**: A pure terminal-inspired design. Features two modes (`MODE: LIGHT` and `MODE: DARK`) with absolute pure black and pure white colors. Theme preferences are persisted in `localStorage`.
* **Zero Icons & SVGs**: All graphical elements and vector icons are removed. Folders are represented simply by a leading slash prefix (e.g. `/docs` vs `docs.txt`) to maintain a clean layout.
* **Zero Border Radius**: Every button, input element, panel, border, modal, and uploader is strictly styled with `border-radius: 0;` for a blocky, clean design.
* **Offline Independence**: Zero external CSS, JS, or web fonts are loaded from CDNs, meaning GOPHRDRV runs perfectly inside air-gapped systems or offline networks.

---

## Quick Start

### System Requirements
* Go `1.20` or higher
* [Just](https://github.com/casey/just) task runner (optional, but recommended for build recipes)

### Standard Build Commands
If you do not have `just` installed, you can build GOPHRDRV using standard Go tools:

```bash
# Build the default development binary
go build -o gophrdrv ./cmd/gophrdrv

# Build the optimized production binary (static, stripped, path-trimmed)
CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o gophrdrv ./cmd/gophrdrv
```

### Running the Server
Launch the server by pointing it to a root directory:

```bash
# Run server serving the current directory on default port 8080
./gophrdrv --root="."

# Run server with custom port, host, and max upload body size limits
./gophrdrv --host="127.0.0.1" --port=9090 --root="/path/to/data" --max-upload="500MB"
```

#### CLI Command Flags:
* `--root`: Path to the directory root to serve (default: `.`).
* `--port`: Port to listen on (default: `8080`).
* `--host`: Host bind address (default: `0.0.0.0`).
* `--max-upload`: Maximum allowable file upload size (default: `100MB`). Supports formats like `10B`, `500KB`, `100MB`, `1GB`.
* `--read-timeout`: Maximum duration for reading the entire request (default: `30s`).
* `--write-timeout`: Maximum duration for writing the response (default: `30s`).
* `--private`: Enable private mode with password protection (default: `false`).
* `-r`: Generate a random 6-digit password for private mode (default: `false`).
* `--qr`: Show QR code for the server URL in the terminal (default: `false`).

---

## Module Reference & Architecture

### Architecture Diagram
```text
  Browser (HTTP/HTML Client)
            │
            ▼
┌─────────────────────────┐
│     HTTP Server         │ (internal/server)
│   (ServeMux Router)     │
└───────────┬─────────────┘
            │
            ▼
┌─────────────────────────┐
│    Handler Context      │ (internal/handlers)
│                         ├────────────────────────┐
│  ┌───────────────────┐  │                        │
│  │   BrowseHandler   │  │                        ▼
│  ├───────────────────┤  │             ┌────────────────────┐
│  │   UploadHandler   │  │             │ Embedded Templates │ (internal/templates)
│  ├───────────────────┤  │             │    (html/template) │
│  │  DownloadHandler  │  │             └────────────────────┘
│  ├───────────────────┤  │
│  │    ViewHandler    │  │
│  └─────────┬─────────┘  │
└────────────┼────────────┘
             │
             ▼
┌─────────────────────────┐
│    Filesystem Layer     │ (internal/filesystem)
│                         ├──────────────┐
│  ┌───────────────────┐  │              │
│  │  ResolveSafePath  │  │              ▼
│  ├───────────────────┤  │      ┌───────────────┐
│  │   IsBinaryFile    │  │      │  Host OS File │
│  └───────────────────┘  │      │    System     │
└─────────────────────────┘      └───────────────┘
```

### Module Directory Structure
```text
cmd/
 └── gophrdrv/
      └── main.go           # Application entrypoint
internal/
 ├── config/
 │    └── config.go         # Parses config flags and size limits
 ├── filesystem/
 │    ├── fs.go             # Resolves safe paths, validates names, scans files
 │    └── fs_test.go        # Unit tests for filesystem logic
 ├── handlers/
 │    ├── context.go        # Handler state, logs, and HTML error rendering
 │    ├── browse.go         # Folder listing, Mkdir, Rename, Delete handlers
 │    ├── edit.go           # Text file editing and saving handlers
 │    ├── upload.go         # Dynamic chunked upload streaming
 │    ├── download.go       # Safe, buffered download handler
 │    ├── view.go           # Text viewer handler with binary detection
 │    └── handlers_test.go  # Context and HTTP handler unit tests
 ├── server/
 │    └── server.go         # Router configuration & graceful shutdown controls
 └── templates/
      ├── templates.go      # Embeds assets & implements template functions
      ├── index.html        # Main dashboard / file manager template
      ├── view.html         # Code/Text viewer template (with sticky line numbers)
      ├── edit.html         # Monospaced browser-based text editor template
      └── error.html        # Standard error card page template
```

### In-Depth Module Documentation

#### 1. `cmd/gophrdrv`
* **File**: `cmd/gophrdrv/main.go`
* **Purpose**: Orchestrates the bootstrap of the application. It parses configuration flags using `internal/config`, instantiates a server context using `internal/server`, and initializes execution.

#### 2. `internal/config`
* **File**: `internal/config/config.go`
* **Purpose**: Handles CLI arguments and parameters.
* **Key Components**:
  * `Config` Struct: Contains parameters for host, port, root directory, timeouts, and maximum file upload size.
  * Size Parsing (`ParseSize`): Custom parser converting size strings (e.g. `2.5MB`, `1GB`) into raw bytes for body limits.

#### 3. `internal/filesystem`
* **File**: `internal/filesystem/fs.go`
* **Purpose**: Implements all OS-level filesystem interactions, prioritizing security.
* **Key Components**:
  * `ResolveSafePath(root, relPath string) (string, error)`: Cleans and resolves paths to prevent directory traversal exploits (e.g. `../../etc/passwd`). Validates that the resolved absolute target matches the served root prefix.
  * `IsValidFilename(name string) bool`: Asserts filename safety by blocking NULL bytes, control characters, and reserved filesystem characters (`/\\:*?"<>|`).
  * `IsBinaryFile(filePath string) (bool, error)`: Scans the first 512 bytes of a file. If it detects a NULL byte (`0x00`), it marks the file as binary. Used to prevent binary files from being rendered in the text viewer.
  * `ListDirectory` & `SearchFiles`: Recursively crawls directories, formats metadata, sorts results (directories first, then alphabetically), and returns directory trees or matching results.

#### 4. `internal/handlers`
* **Directory**: `internal/handlers/`
* **Purpose**: Processes incoming HTTP requests, coordinates business logic, and outputs HTML views or JSON responses.
* **Key Components**:
  * `HandlerContext`: Holds configuration parameters, logs errors, and renders custom errors using the embedded `error.html` template.
  * `BrowseHandler`: Renders the file browser dashboard or searches files recursively.
  * `UploadHandler`: Streams multi-part file chunks directly to disk using `io.Copy`. Uses `http.MaxBytesReader` to enforce upload limits.
  * `DownloadHandler`: Serves raw downloads using Go's native `http.ServeFile`, optimizing memory via chunked streaming.
  * `ViewHandler`: Reads files (capped at `5MB`) and renders them in the code viewer using `view.html`. Intercepts non-text formats and binary content.
  * `EditHandler`: Reads text file content and renders the monospaced browser-based text editor layout.
  * `SaveHandler`: Receives edited content (or creates new empty files) and commits changes safely to disk.

#### 5. `internal/templates`
* **Directory**: `internal/templates/`
* **Purpose**: Manages and compiles embedded HTML/CSS/JS files inside the binary.
* **Key Components**:
  * `templates.go`: Employs Go's `//go:embed` directive to bundle the UI templates. It parses the templates with a custom `FuncMap` containing utility functions:
    * `isTextViewable(name string) bool`: Blacklists known binary file extensions (e.g. `.jpg`, `.zip`, `.pdf`) from displaying a "View" button in the UI. Unrecognized extensions or extensionless files (like `LICENSE`, `Makefile`) default to `true` and undergo a secondary content-based binary check at the API level.
    * `formatSize`: Formats raw bytes to human-readable strings.
    * `formatTime`: Standardizes timestamp displays.

#### 6. `internal/server`
* **File**: `internal/server/server.go`
* **Purpose**: Manages TCP binding, router mapping, and graceful shutdown behavior.
* **Key Components**:
  * `Start()`: Binds the HTTP handlers to the router, starts the TCP listener, listens for OS terminate signals (`SIGINT`, `SIGTERM`), and gracefully stops connections using a `10s` timeout window.

---

## Developer Task Guide (Justfile)

If you have `just` installed, you can automate standard development tasks:

```bash
# List all available recipes
just

# Build the optimized single-binary executable
just build

# Run unit tests with verbose output and coverage
just test

# Run unit tests and generate an HTML coverage report
just test-coverage

# Format and analyze the codebase using go fmt and go vet
just lint

# Run the compiled gophrdrv locally (builds automatically)
just run

# Clean build binaries and coverage reports
just clean
```

---

## Security Controls

GOPHRDRV implements the following defensive controls to ensure secure operation:

1. **Path Traversal Protection**: All paths are cleaned and validated via absolute prefix verification before the OS reads or writes to them. If a path escapes the configured root folder, the request is rejected with `403 Forbidden`.
2. **Buffer Overflow & Memory Protections**: Large files are processed strictly as streams (using buffered chunks via `io.Copy`). File uploads are constrained in size via `http.MaxBytesReader`.
3. **Execution Limits**: Files larger than `5MB` are blocked from rendering in the inline text viewer to avoid browser freeze and memory spikes.
4. **Binary Viewing Prevention**: Double-barrier verification blocks non-text files:
   * Extension check blocks known binary file extensions (e.g. `.zip`, `.pdf`, `.png`) instantly.
   * Content check scans the first 512 bytes for a NULL byte (`0x00`). If a NULL byte is found, the server blocks rendering to prevent binary corruption from breaking UI layouts.

---

## Performance & Binary Size Optimization

To keep GOPHRDRV light and self-contained, we employ the following optimization pipeline:

1. **CGO Disabled (`CGO_ENABLED=0`)**: Disabling CGO forces Go to build a purely static binary. This removes dynamic link dependencies (such as `glibc`), making the executable portable across Linux distributions.
2. **Stripping Debugging Tables (`-ldflags="-s -w"`)**:
   * `-s`: Strips symbol table information, saving around `1.5MB` of metadata.
   * `-w`: Strips DWARF debugging tables, saving another `1.0MB`.
3. **Trimmed Paths (`-trimpath`)**: Trims developer file paths from the compilation output, preserving privacy and reducing binary file footprint.
4. **Result**: Compiles into a single, fully-contained executable of **`8.6 MB`** that handles the entire HTTP router, server engine, HTML template compilation, and filesystem manager.
