# Go Single-Binary File Server 
Version: 0.0.4
Status: Design Specification

---

# Overview

A lightweight file server written entirely in Go using only the standard library.

The application must compile into a single executable binary with no runtime dependencies, no external packages, no database, and no separate frontend assets.

Primary goals:

- Single binary deployment
- Cross-platform support
- HTTP-based file management
- Simple web interface
- File upload/download
- Text file viewing
- Directory browsing
- Secure filesystem access
- Low memory usage
- Suitable for self-hosting

---

# Project Goals

## Functional Goals

The server must provide:

1. Browse directories
2. Navigate folders
3. Upload files
4. Download files
5. View text files in browser
6. Create directories
7. Delete files
8. Delete directories
9. Rename files
10. Rename directories
11. Show file metadata
12. Display file sizes
13. Display modification times
14. Sort listings
15. Search files
16. Support large files
17. Stream downloads
18. Stream uploads

---

# Non-Functional Goals

## Deployment

Requirements:

- Single executable
- No external assets
- No database
- No NodeJS
- No npm
- No Docker requirement
- No external packages

Only Go standard library:

```go
net/http
os
io
path/filepath
html/template
embed
mime
time
sort
strings
context
sync
encoding/json
```

---

# Directory Scope

The server operates inside a configured root directory.

Example:

```text
/data
```

Users cannot access:

```text
/etc
/home
/windows
```

outside configured root.

---

# Security Model

## Path Traversal Prevention

Must block:

```text
../../../etc/passwd
```

```text
..\..\windows\system32
```

Implementation:

1. Clean path
2. Resolve absolute path
3. Verify prefix equals root directory

Pseudo:

```go
requested := filepath.Clean(path)
absolute := filepath.Abs(requested)

if !strings.HasPrefix(
    absolute,
    rootDirectory,
) {
    reject()
}
```

---

## Upload Restrictions

Configurable:

```go
MaxUploadSize
```

Example:

```go
1GB
5GB
10GB
```

Default:

```go
100MB
```

---

## Filename Validation

Remove:

```text
NULL bytes
control chars
```

Reject:

```text
/
\
:
*
?
"
<
>
|
```

when platform requires.

---

# Architecture

```text
Browser
   |
HTTP
   |
Router
   |
Handlers
   |
Filesystem Layer
   |
OS
```

---

# Package Layout

```text
cmd/gophrdrv/
    main.go

internal/server/
    server.go

internal/handlers/
    browse.go
    upload.go
    download.go
    view.go

internal/filesystem/
    fs.go

internal/templates/
    templates.go

internal/config/
    config.go
```

For single-binary simplicity these may later be merged.

---

# HTTP Endpoints

---

## GET /

Directory listing

Example:

```text
/
```

Response:

```html
File Browser
```

Displays:

- directories
- files
- sizes
- timestamps

---

## GET /browse

Browse subdirectory

Example:

```text
/browse?path=docs
```

---

## GET /download

Download file

Example:

```text
/download?file=docs/readme.pdf
```

Behavior:

```http
Content-Disposition: attachment
```

Uses:

```go
http.ServeFile()
```

---

## GET /view

View text file

Example:

```text
/view?file=logs/app.log
```

---

## POST /upload

Upload file

Multipart upload.

Example:

```html
<form enctype="multipart/form-data">
```

---

## POST /mkdir

Create folder.

Example:

```text
new-folder
```

---

## POST /rename

Rename file or folder.

---

## POST /delete

Delete file or folder.

---

# UI Requirements

## Layout

Single page web application.

Sections:

```text
Header
Toolbar
Breadcrumbs
File Table
Status Bar
```

---

# Header

Contains:

```text
Application Name
Current Path
```

Example:

```text
File Server
/home/files
```

---

# Toolbar

Actions:

```text
Upload
New Folder
Refresh
Search
```

---

# Breadcrumbs

Example:

```text
Root / Documents / Projects
```

Clickable navigation.

---

# File Table

Columns:

```text
Name
Size
Modified
Actions
```

---

# Actions

Per file:

```text
View
Download
Rename
Delete
```

Per directory:

```text
Open
Rename
Delete
```

---

# File Viewer

Supported text formats:

```text
txt
log
json
yaml
yml
xml
csv
md
go
js
ts
html
css
py
java
c
cpp
rs
toml
ini
conf
```

---

# Viewer Requirements

Render:

```html
<pre>
```

Content escaped through:

```go
html/template
```

Must not render raw HTML.

---

# Large File Handling

Requirements:

- Stream files
- Avoid loading entire file into memory

Download:

```go
http.ServeFile()
```

Upload:

```go
io.Copy()
```

---

# Directory Listing Logic

Retrieve entries:

```go
os.ReadDir()
```

Collect:

```go
Name
Type
Size
ModTime
```

Sort:

1. Directories first
2. Alphabetical

---

# Search Feature

Search current directory recursively.

Query:

```text
?q=report
```

Returns:

```text
report.pdf
annual-report.docx
```

---

# Configuration

## CLI Flags

```bash
gophrdrv \
  --root /data \
  --port 8080
```

---

## Supported Flags

### Root

```bash
--root
```

Filesystem root.

---

### Port

```bash
--port
```

Default:

```text
8080
```

---

### Host

```bash
--host
```

Default:

```text
0.0.0.0
```

---

### Read Timeout

```bash
--read-timeout
```

Default:

```text
30s
```

---

### Write Timeout

```bash
--write-timeout
```

Default:

```text
30s
```

---

### Max Upload

```bash
--max-upload
```

Default:

```text
100MB
```

---

# Logging

Log:

```text
Server startup
Uploads
Downloads
Deletes
Errors
```

Format:

```text
timestamp level message
```

Example:

```text
2026-06-12 INFO uploaded report.pdf
```

---

# Templates

Templates embedded into binary.

Use:

```go
//go:embed templates/*
```

or inline strings.

No external files required.

---

# Error Handling

User-facing errors:

```text
404 Not Found
403 Forbidden
500 Internal Server Error
```

Friendly HTML pages.

---

# Performance Targets

## Small Instance

```text
1 CPU
512MB RAM
```

Target:

```text
100 concurrent users
```

---

## Large Downloads

Support:

```text
10GB+
```

without memory exhaustion.

---

# Future Features

Phase 2

- Authentication
- User accounts
- Permissions
- ZIP download
- ZIP upload
- Drag-and-drop uploads
- Dark mode
- Thumbnail generation
- File previews
- API endpoints

---

# Future Features

Phase 3

- WebDAV
- S3 backend
- Multi-user support
- Audit logs
- Share links
- Expiring links
- WebSocket updates

---

# Testing Plan

## Unit Tests

Coverage:

- path sanitization
- uploads
- downloads
- rename
- delete
- directory listing

Target:

```text
80%+
```

---

# Integration Tests

Verify:

1. Upload file
2. Download file
3. View text file
4. Create folder
5. Rename folder
6. Delete folder
7. Search files

---

# Build Process

Development:

```bash
go run .
```

Production:

```bash
go build \
  -trimpath \
  -ldflags="-s -w" \
  -o gophrdrv
```

Result:

```text
gophrdrv
```

Single executable.

---

# Acceptance Criteria

The project is considered complete when:

- Single executable binary exists
- No external packages are used
- Directory browsing works
- Upload works
- Download works
- Text viewer works
- Search works
- Rename works
- Delete works
- Folder creation works
- Path traversal is prevented
- Large files stream correctly
- Runs on Linux
- Runs on Windows
- Runs on macOS

---

# Final Deliverable

A standalone Go application that:

- Uses only the Go standard library
- Compiles into a single binary
- Serves files over HTTP
- Provides upload/download functionality
- Provides text viewing functionality
- Provides file management operations
- Can be deployed by copying one executable file to a server and running it
