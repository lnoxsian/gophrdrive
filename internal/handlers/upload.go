package handlers

import (
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/lnoxsian/gophrdrv/internal/filesystem"
)

// UploadHandler handles file upload POST requests
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

	// Retrieve uploaded file from form data
	multipartFile, fileHeader, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Bad Request: file is required", http.StatusBadRequest)
		return
	}
	defer multipartFile.Close()

	// Validate filename
	fileName := filepath.Base(fileHeader.Filename)
	if !filesystem.IsValidFilename(fileName) {
		http.Error(w, "Invalid filename: contains forbidden characters", http.StatusBadRequest)
		return
	}

	// Form target file path
	targetFilePath := filepath.Join(safeParent, fileName)

	// Additional path safety check on final filename path
	_, err = filesystem.ResolveSafePath(h.Cfg.Root, filepath.Join(parentPath, fileName))
	if err != nil {
		http.Error(w, "Forbidden: Invalid file path target", http.StatusForbidden)
		return
	}

	// Check if target path exists and is a directory
	info, err := os.Stat(targetFilePath)
	if err == nil && info.IsDir() {
		http.Error(w, "Conflict: A folder with this name already exists", http.StatusConflict)
		return
	}

	// Create/overwrite destination file
	destFile, err := os.OpenFile(targetFilePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		h.LogError("Failed to create destination file %s: %v", targetFilePath, err)
		http.Error(w, "Internal Server Error: failed to save file", http.StatusInternalServerError)
		return
	}
	defer destFile.Close()

	// Stream file content chunk-by-chunk using io.Copy (prevents loading whole file into memory)
	_, err = io.Copy(destFile, multipartFile)
	if err != nil {
		h.LogError("Error streaming file data: %v", err)
		http.Error(w, "Internal Server Error: file transfer interrupted", http.StatusInternalServerError)
		return
	}

	h.LogInfo("uploaded %s", filepath.Join(parentPath, fileName))
	w.WriteHeader(http.StatusOK)
}
