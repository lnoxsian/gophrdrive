package handlers

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"

	"github.com/lnoxsian/gophrdrv/internal/filesystem"
	"github.com/lnoxsian/gophrdrv/internal/templates"
)

// EditHandler renders the text editor interface for text files
func (h *HandlerContext) EditHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.RenderError(w, http.StatusMethodNotAllowed, "Method Not Allowed", "Only GET requests are allowed for this endpoint.")
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
		h.RenderError(w, http.StatusBadRequest, "Bad Request", "Directories cannot be opened in the text editor.")
		return
	}

	fileName := filepath.Base(safePath)

	// Verify if it is a text-viewable file type (blacklist check)
	if !templates.IsTextViewable(fileName) {
		h.RenderError(w, http.StatusBadRequest, "Unsupported File Type", "This file type is not supported for editing in the browser.")
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
		h.RenderError(w, http.StatusBadRequest, "Binary File Detected", "This file contains binary data and cannot be edited in the browser.")
		return
	}

	// Check file size before loading to avoid memory exhaustion
	if info.Size() > maxViewableTextSize {
		h.RenderError(w, http.StatusRequestEntityTooLarge, "File Too Large", "This file is too large to edit inside the browser. Max limit is 5MB.")
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
		Title:      "Editing " + fileName,
		FileName:   fileName,
		FilePath:   relPath,
		ParentPath: parentPath,
		Content:    string(contentBytes),
		SizeStr:    templates.FormatSize(info.Size()),
		ModTimeStr: templates.FormatTime(info.ModTime()),
		IsPrivate:  h.Cfg.Private,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	err = templates.ExecuteTemplate(w, "edit.html", data)
	if err != nil {
		h.LogError("Failed to render edit template: %v", err)
	}
}

// SaveHandler handles saving file contents or creating new text files
func (h *HandlerContext) SaveHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	err := r.ParseForm()
	if err != nil {
		http.Error(w, "Bad Request: failed to parse form", http.StatusBadRequest)
		return
	}

	relPath := r.FormValue("file")
	content := r.FormValue("content")

	if relPath == "" {
		http.Error(w, "File path parameter is required", http.StatusBadRequest)
		return
	}

	fileName := filepath.Base(relPath)
	if !filesystem.IsValidFilename(fileName) {
		http.Error(w, "Invalid file name: contains forbidden characters", http.StatusBadRequest)
		return
	}

	// Verify if it is a text file type by extension
	if !templates.IsTextViewable(fileName) {
		http.Error(w, "Unsupported File Type: this format cannot be edited/saved as text", http.StatusBadRequest)
		return
	}

	// Check size of the content to save (5MB limit)
	if len(content) > maxViewableTextSize {
		http.Error(w, "File Too Large: max limit is 5MB", http.StatusRequestEntityTooLarge)
		return
	}

	// Resolve target path safely
	safePath, err := filesystem.ResolveSafePath(h.Cfg.Root, relPath)
	if err != nil {
		if errors.Is(err, filesystem.ErrUnsafePath) {
			http.Error(w, "Forbidden: Path traversal detected", http.StatusForbidden)
		} else {
			http.Error(w, "Bad Request: invalid path specification", http.StatusBadRequest)
		}
		return
	}

	// Check if file exists, if it is a directory
	info, err := os.Stat(safePath)
	if err == nil {
		if info.IsDir() {
			http.Error(w, "Bad Request: cannot write content to a directory", http.StatusBadRequest)
			return
		}

		// Verify that the existing file is not binary (to avoid overwriting key binaries)
		isBinary, err := filesystem.IsBinaryFile(safePath)
		if err == nil && isBinary {
			http.Error(w, "Bad Request: cannot overwrite a binary file with text content", http.StatusBadRequest)
			return
		}
	} else if !os.IsNotExist(err) {
		h.LogError("Failed to stat target save path %s: %v", safePath, err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Write file contents to disk
	err = os.WriteFile(safePath, []byte(content), 0644)
	if err != nil {
		h.LogError("Failed to write/save file %s: %v", safePath, err)
		http.Error(w, "Internal Server Error: failed to write to disk", http.StatusInternalServerError)
		return
	}

	h.LogInfo("saved/wrote file %s", relPath)
	w.WriteHeader(http.StatusOK)
}
