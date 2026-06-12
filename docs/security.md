# GOPHRDRV Security Architecture

This document outlines the security controls, validation routines, and defensive designs implemented in GOPHRDRV to protect the host filesystem.

---

## 1. Path Traversal Prevention

Path traversal is a common security issue in file browsers, where malicious inputs (e.g., `../../etc/passwd`) attempt to access files outside the designated root directory. 

GOPHRDRV prevents path traversal through a strict normalization and prefix-checking pipeline in the `internal/filesystem` module:

```text
       Raw Relative Path (from query/form input)
                         │
                         ▼
             [ filepath.Clean(path) ]
                         │
                         ▼
             [ filepath.Join(root, path) ]
                         │
                         ▼
             [ filepath.Abs(fullPath) ]
                         │
                         ▼
   [ Ensure absTarget matches absRoot directory prefix ]
             /                               \
         Matches?                          Doesn't Match?
           /                                   \
          ▼                                     ▼
   [ Return Safe Path ]                   [ Return ErrUnsafePath ]
                                          (Triggers 403 Forbidden)
```

### Prefix Edge-Case Mitigation
To prevent subfolder prefix collisions (e.g. allowing `/data-extra` to match a root of `/data`), a path separator suffix is appended to the root boundary during prefix validation:

```go
prefix := absRoot
if !strings.HasSuffix(prefix, string(filepath.Separator)) {
    prefix += string(filepath.Separator)
}
if !strings.HasPrefix(absTarget, prefix) {
    return "", ErrUnsafePath
}
```

---

## 2. Filename Validation

Filenames received via file uploads, creation requests, or rename inputs are validated to prevent shell injection, URL breakage, or file creation exploits.

Validation logic in `IsValidFilename` blocks:
1. **Empty Names & Dot Directories**: Empty inputs, `.`, and `..` are immediately rejected.
2. **Control Characters**: Characters with ASCII values less than 32 (non-printable control codes) and ASCII 127 (`DEL`) are blocked.
3. **Reserved Platform Characters**: Characters that are forbidden on Windows or unsafe in web URLs (`/`, `\`, `:`, `*`, `?`, `"`, `<`, `>`, `|`) are rejected.

---

## 3. Resource & Memory Exhaustion Controls

To ensure high performance and flat memory utilization on low-resource environments (e.g., `512MB RAM`), the server enforces strict resource boundaries.

### 3.1 Stream Processing
* **Buffered Downloads**: Served using standard Go `http.ServeFile` directly, streaming file bytes on-the-fly to client TCP sockets without reading full contents into RAM.
* **Buffered Uploads**: The file upload endpoint processes multipart data chunks and pipes them directly to disk using `io.Copy`, ensuring memory consumption remains flat regardless of upload file size.

### 3.2 Size Limiters
* **Body Limitations**: Every upload request wraps the raw connection reader using `http.MaxBytesReader` configured to the `MaxUploadSize` setting. Attempting to upload a file exceeding this limit immediately triggers a `413 Payload Too Large` error and terminates the request.
* **Inline View Restrictions**: A hard limit of `5MB` (`maxViewableTextSize`) is enforced on text file previews. Attempting to view files larger than `5MB` in the text viewer is rejected with a `413` response, directing users to download the file instead.

---

## 4. Binary File Filtering & Content Analysis

GOPHRDRV employs a double-barrier validation system to verify that binary content cannot be loaded in the inline text viewer, which prevents browser layout breakage:

1. **Extension Blacklist**:
   * Any file with a known binary format extension (e.g. `.jpg`, `.pdf`, `.zip`, `.exe`, `.mp4`) is immediately blocked from viewing at the UI level.
2. **Content-Based Detection**:
   * For files with unrecognized extensions or extensionless configurations (like `LICENSE`, `Makefile`, `Justfile`), the server scans the first 512 bytes for a `NULL` byte (`0x00`).
   * A `NULL` byte is a definitive indicator of a compiled binary format. If detected, the server aborts rendering and returns `400 Bad Request` with `Binary File Detected`.
