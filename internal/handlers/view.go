package handlers

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"

	"github.com/lnoxsian/gophrdrv/internal/filesystem"
	"github.com/lnoxsian/gophrdrv/internal/templates"
)

type ViewData struct {
	Title      string
	FileName   string
	FilePath   string
	ParentPath string
	Content    string
	SizeStr    string
	ModTimeStr string
	IsPrivate  bool
}

const maxViewableTextSize = 5 * 1024 * 1024 // 5MB limit for inline text viewing

// ViewHandler handles viewing text files in a beautiful format in the browser
func (h *HandlerContext) ViewHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.RenderError(w, http.StatusMethodNotAllowed, "Method Not Allowed", "Only GET requests are allowed for file viewing.")
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

	// Stat file
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
		h.RenderError(w, http.StatusBadRequest, "Bad Request", "Directories cannot be viewed in text viewer.")
		return
	}

	fileName := filepath.Base(safePath)

	// Verify if it is a text-viewable file type
	if !templates.IsTextViewable(fileName) {
		h.RenderError(w, http.StatusBadRequest, "Unsupported File Type", "This file type cannot be viewed in the browser. Please download it instead.")
		return
	}

	// Verify that the file does not contain binary data
	isBinary, err := filesystem.IsBinaryFile(safePath)
	if err != nil {
		h.LogError("Failed to check binary status of %s: %v", safePath, err)
		h.RenderError(w, http.StatusInternalServerError, "Internal Server Error", "Failed to scan file content.")
		return
	}
	if isBinary {
		h.RenderError(w, http.StatusBadRequest, "Binary File Detected", "This file contains binary data and cannot be viewed in the browser. Please download it instead.")
		return
	}

	// Check file size before loading to avoid memory exhaustion
	if info.Size() > maxViewableTextSize {
		h.RenderError(w, http.StatusRequestEntityTooLarge, "File Too Large", "This file is too large to view inside the browser. Please download it instead.")
		return
	}

	// Read file contents
	contentBytes, err := os.ReadFile(safePath)
	if err != nil {
		h.LogError("Failed to read file %s: %v", safePath, err)
		h.RenderError(w, http.StatusInternalServerError, "Internal Server Error", "Failed to read file content.")
		return
	}

	// Parent path for navigation
	parentPath := filepath.Dir(relPath)
	parentPath = filepath.ToSlash(parentPath)
	if parentPath == "." || parentPath == "/" {
		parentPath = ""
	}

	data := ViewData{
		Title:      "Viewing " + fileName,
		FileName:   fileName,
		FilePath:   relPath,
		ParentPath: parentPath,
		Content:    string(contentBytes),
		SizeStr:    templates.FormatSize(info.Size()),
		ModTimeStr: templates.FormatTime(info.ModTime()),
		IsPrivate:  h.Cfg.Private,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	err = templates.ExecuteTemplate(w, "view.html", data)
	if err != nil {
		h.LogError("Failed to render view template: %v", err)
	}
}
