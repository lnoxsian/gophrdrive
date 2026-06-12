package handlers

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"

	"github.com/lnoxsian/gophrdrv/internal/filesystem"
)

// DownloadHandler handles download requests for files, streaming content to clients.
func (h *HandlerContext) DownloadHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.RenderError(w, http.StatusMethodNotAllowed, "Method Not Allowed", "Only GET requests are allowed for file downloads.")
		return
	}

	relPath := r.URL.Query().Get("file")
	if relPath == "" {
		h.RenderError(w, http.StatusBadRequest, "Bad Request", "File parameter is required.")
		return
	}

	// Resolve target safely
	safePath, err := filesystem.ResolveSafePath(h.Cfg.Root, relPath)
	if err != nil {
		if errors.Is(err, filesystem.ErrUnsafePath) {
			h.RenderError(w, http.StatusForbidden, "Forbidden", "Path traversal attempts are strictly forbidden.")
		} else {
			h.RenderError(w, http.StatusBadRequest, "Bad Request", "Invalid file path specification.")
		}
		return
	}

	// Stat file to verify it exists and is a file
	info, err := os.Stat(safePath)
	if err != nil {
		if os.IsNotExist(err) {
			h.RenderError(w, http.StatusNotFound, "Not Found", "The requested file does not exist.")
		} else {
			h.RenderError(w, http.StatusInternalServerError, "Internal Server Error", "Failed to retrieve file details.")
		}
		return
	}

	if info.IsDir() {
		h.RenderError(w, http.StatusBadRequest, "Bad Request", "Directories cannot be downloaded directly.")
		return
	}

	// Set header to trigger download dialog in browser
	fileName := filepath.Base(safePath)
	w.Header().Set("Content-Disposition", "attachment; filename=\""+fileName+"\"")
	w.Header().Set("Content-Type", "application/octet-stream")

	// Log download operation
	h.LogInfo("downloaded %s", relPath)

	// Serve the file (uses chunked transfer or streams content automatically without fully loading to memory)
	http.ServeFile(w, r, safePath)
}
