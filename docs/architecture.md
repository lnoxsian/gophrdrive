# GOPHRDRV Architecture Design Document

GOPHRDRV is a lightweight single-binary file server. This document describes its internal package structure, request processing pipeline, and component communication patterns.

---

## 1. Request Flow Pipeline

The lifetime of an HTTP request within GOPHRDRV passes through three distinct layers: the **Server Layer**, the **Handler Layer**, and the **Filesystem Layer**.

```text
 Client Request
       │
       ▼ (1) Binds TCP port & coordinates TLS/HTTP
┌──────────────┐
│ Server Layer │ (internal/server)
└──────┬───────┘
       │
       ▼ (2) Route matching & Context resolution
┌──────────────┐
│ Handler Layer│ (internal/handlers)
│              ├──────────────────────┐
│              │ (3) Renders HTML     ▼
│              │               ┌──────────────┐
│              │               │ Template Eng.│ (internal/templates)
│              │               └──────────────┘
└──────┬───────┘
       │
       ▼ (4) Clean path, block traversal, scan contents
┌──────────────┐
│ Filesystem L.│ (internal/filesystem)
└──────┬───────┘
       │
       ▼ (5) Reads/Writes byte streams
┌──────────────┐
│   Host OS    │
└──────────────┘
```

1. **Server Layer**: Receives the raw TCP stream, coordinates timeouts (read/write durations), routes the request using Go's `http.ServeMux`, and initiates graceful shutdown protocols when signals are captured.
2. **Handler Layer**: Parses input query parameters or multipart form bodies. Validates request parameters against constraints, calls templates to render views, and returns errors where appropriate.
3. **Filesystem Layer**: Checks target pathways for safety. Normalizes relative targets to secure, absolute host paths. Performs checks for filename validity, folder sorting, and content type scanning.
4. **Template Engine**: Formats database entries into light/dark high-contrast layout code, embeds static files into the binary, and compiles variables dynamically.

---

## 2. Module Specifications

### 2.1 Server Module (`internal/server`)
Manages TCP listeners and routes requests.
* **Component**: `Server` struct.
* **Main Methods**:
  * `NewServer(cfg *config.Config) *Server`: Initializes the configuration parameters.
  * `Start() error`: Boots the HTTP server. Instantiates router endpoints, binds the TCP address, monitors runtime signals (`SIGINT`, `SIGTERM`), and calls `Shutdown()` to drain active requests gracefully.

### 2.2 Config Module (`internal/config`)
Manages parsing configurations.
* **Component**: `Config` struct.
* **Main Methods**:
  * `ParseSize(sizeStr string) (int64, error)`: Converts human-readable formats (e.g. `100MB`, `1.5GB`) into raw bytes.

### 2.3 Filesystem Module (`internal/filesystem`)
Encapsulates low-level OS operations, ensuring security boundaries.
* **Main Functions**:
  * `ResolveSafePath(root, relPath string) (string, error)`: Standardizes target paths and prevents path traversal (e.g., using `../`).
  * `IsValidFilename(name string) bool`: Rejects filenames containing invalid control characters or reserved symbols.
  * `IsBinaryFile(filePath string) (bool, error)`: Scans the first 512 bytes of a file. Returns `true` if a `NULL` byte (`0x00`) is detected.
  * `ListDirectory(root, relPath string) ([]FileInfo, error)`: Lists directory contents, sorted directories-first and then alphabetically.
  * `SearchFiles(root, relPath, query string) ([]FileInfo, error)`: Recursively scans files matching a search term.

### 2.4 Handlers Module (`internal/handlers`)
Implements the controllers for each HTTP endpoint.
* **Component**: `HandlerContext` struct (stores shared references to configs and loggers).
* **Main Handlers**:
  * `BrowseHandler`: Lists directory trees or serves search queries.
  * `UploadHandler`: Processes file uploads using streaming `io.Copy`.
  * `DownloadHandler`: Streams requested downloads to client connections.
  * `ViewHandler`: Reads text file content up to 5MB, verifies it has no binary signatures, and displays it in the browser code viewer.
  * `EditHandler`: Reads text file content and renders the visual inline text editor.
  * `SaveHandler`: Writes text edits to disk (or creates new files) with standard limit checks.
  * `MkdirHandler`, `RenameHandler`, `DeleteHandler`: Processes directory creation, renaming, and item deletions.

### 2.5 Templates Module (`internal/templates`)
Packages frontend assets inside the single binary.
* **Main Functions**:
  * `IsTextViewable(name string) bool`: Filters out known binary file extensions (e.g. `.png`, `.zip`, `.pdf`).
  * `FormatSize`, `FormatTime`: UI template helper formatters.
  * `ExecuteTemplate`: Compiles and executes the requested HTML view template.
