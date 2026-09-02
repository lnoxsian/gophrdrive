package handlers

import (
	"errors"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/lnoxsian/gophrdrv/internal/filesystem"
)

// UploadHandler handles file and folder upload POST requests
func (h *HandlerContext) UploadHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	// Restrict the request body size using http.MaxBytesReader to prevent denial of service / memory exhaustion
	r.Body = http.MaxBytesReader(w, r.Body, h.Cfg.MaxUpload)

	// Parse multipart form (use a reasonable memory buffer like 32MB, rest goes to temp files automatically)
	err := r.ParseMultipartForm(32 << 20)
	if err != nil {
		h.LogError("Upload parsing error: %v", err)
		http.Error(w, "Request body too large or invalid multipart form", http.StatusRequestEntityTooLarge)
		return
	}

	// Get destination folder
	parentPath := r.FormValue("path")

	// Resolve parent folder path safely
	safeParent, err := filesystem.ResolveSafePath(h.Cfg.Root, parentPath)
	if err != nil {
		if errors.Is(err, filesystem.ErrUnsafePath) {
			http.Error(w, "Forbidden: Path traversal detected", http.StatusForbidden)
		} else {
			http.Error(w, "Bad Request: invalid path", http.StatusBadRequest)
		}
		return
	}

	// Retrieve uploaded files from form data
	files := r.MultipartForm.File["files"]
	if len(files) == 0 {
		// Fallback to "file" field
		files = r.MultipartForm.File["file"]
	}

	if len(files) == 0 {
		http.Error(w, "Bad Request: at least one file is required", http.StatusBadRequest)
		return
	}

	// Pre-allocate 128KB copy buffer to reduce system calls during file writes
	uploadBuf := make([]byte, 128*1024)

	for i, fileHeader := range files {
		multipartFile, err := fileHeader.Open()
		if err != nil {
			h.LogError("Failed to open file in multipart form: %v", err)
			http.Error(w, "Internal Server Error: failed to read uploaded file", http.StatusInternalServerError)
			return
		}

		// Normalize raw relative path from Content-Disposition header, form fields, or filename
		rawRelPath := fileHeader.Filename
		if cd := fileHeader.Header.Get("Content-Disposition"); cd != "" {
			if _, params, err := mime.ParseMediaType(cd); err == nil {
				if f, ok := params["filename"]; ok && f != "" {
					rawRelPath = f
				}
			}
		}
		rawRelPath = filepath.ToSlash(rawRelPath)
		if len(r.MultipartForm.Value["paths"]) == len(files) {
			if p := r.MultipartForm.Value["paths"][i]; p != "" {
				rawRelPath = filepath.ToSlash(p)
			}
		} else if p := r.FormValue("relativePath"); p != "" && len(files) == 1 {
			rawRelPath = filepath.ToSlash(p)
		}

		cleanRelPath := filepath.Clean(rawRelPath)
		cleanRelPath = strings.TrimPrefix(cleanRelPath, "/")

		if cleanRelPath == "" || cleanRelPath == "." || cleanRelPath == ".." || strings.HasPrefix(cleanRelPath, "../") {
			multipartFile.Close()
			http.Error(w, "Invalid filename or path: unsafe path", http.StatusBadRequest)
			return
		}

		// Validate all path segments in the relative path
		if !filesystem.IsValidRelPath(cleanRelPath) {
			multipartFile.Close()
			http.Error(w, "Invalid filename or path: contains forbidden characters", http.StatusBadRequest)
			return
		}

		// Form target file path inside safeParent
		targetFilePath, err := filesystem.ResolveSafePath(safeParent, cleanRelPath)
		if err != nil {
			multipartFile.Close()
			http.Error(w, "Forbidden: Invalid file path target", http.StatusForbidden)
			return
		}

		// Additional path safety check against root
		_, err = filesystem.ResolveSafePath(h.Cfg.Root, filepath.Join(parentPath, cleanRelPath))
		if err != nil {
			multipartFile.Close()
			http.Error(w, "Forbidden: Invalid file path target", http.StatusForbidden)
			return
		}

		// Check if target path exists and is a directory
		info, err := os.Stat(targetFilePath)
		if err == nil && info.IsDir() {
			multipartFile.Close()
			http.Error(w, "Conflict: A folder with this name already exists", http.StatusConflict)
			return
		}

		// Ensure parent directory exists for nested folder uploads
		targetDir := filepath.Dir(targetFilePath)
		if err := os.MkdirAll(targetDir, 0755); err != nil {
			multipartFile.Close()
			h.LogError("Failed to create parent directory %s: %v", targetDir, err)
			http.Error(w, "Internal Server Error: failed to create directories", http.StatusInternalServerError)
			return
		}

		// Create/overwrite destination file
		destFile, err := os.OpenFile(targetFilePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
		if err != nil {
			multipartFile.Close()
			h.LogError("Failed to create destination file %s: %v", targetFilePath, err)
			http.Error(w, "Internal Server Error: failed to save file", http.StatusInternalServerError)
			return
		}

		// Stream file content using copy buffer to optimize network/disk writing performance
		_, err = io.CopyBuffer(destFile, multipartFile, uploadBuf)
		destFile.Close()
		multipartFile.Close()
		if err != nil {
			h.LogError("Error streaming file data to %s: %v", targetFilePath, err)
			http.Error(w, "Internal Server Error: file transfer interrupted", http.StatusInternalServerError)
			return
		}

		h.LogInfo("uploaded %s", filepath.Join(parentPath, cleanRelPath))
	}

	w.WriteHeader(http.StatusOK)
}
