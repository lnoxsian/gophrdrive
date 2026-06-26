package handlers

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/lnoxsian/gophrdrv/internal/filesystem"
	"github.com/lnoxsian/gophrdrv/internal/templates"
	"github.com/lnoxsian/gophrdrv/internal/version"
)

type BrowseData struct {
	Title            string
	CurrentPath      string
	SearchQuery      string
	Breadcrumbs      []templates.Breadcrumb
	Entries          []filesystem.FileInfo
	RootPath         string
	MaxUploadSizeStr string
	EntriesCount     int
	AppVersion       string
	IsPrivate        bool
}

// BrowseHandler handles directory listings and search queries
func (h *HandlerContext) BrowseHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.RenderError(w, http.StatusMethodNotAllowed, "Method Not Allowed", "Only GET requests are allowed for this endpoint.")
		return
	}

	relPath := r.URL.Query().Get("path")
	searchQuery := r.URL.Query().Get("q")

	// Clean path and resolve
	safePath, err := filesystem.ResolveSafePath(h.Cfg.Root, relPath)
	if err != nil {
		if errors.Is(err, filesystem.ErrUnsafePath) {
			h.RenderError(w, http.StatusForbidden, "Forbidden", "Path traversal attempts are strictly forbidden.")
		} else {
			h.RenderError(w, http.StatusBadRequest, "Bad Request", "Invalid path specification.")
		}
		return
	}

	// Double-check if the directory exists and is actually a directory
	info, err := os.Stat(safePath)
	if err != nil {
		if os.IsNotExist(err) {
			h.RenderError(w, http.StatusNotFound, "Not Found", "The requested path does not exist.")
		} else {
			h.RenderError(w, http.StatusInternalServerError, "Internal Server Error", "Failed to retrieve directory details.")
		}
		return
	}

	if !info.IsDir() {
		// If it's a file, redirect to the view or download depending on type
		fileName := filepath.Base(safePath)
		if templates.IsTextViewable(fileName) {
			http.Redirect(w, r, "view?file="+relPath, http.StatusSeeOther)
		} else {
			http.Redirect(w, r, "download?file="+relPath, http.StatusSeeOther)
		}
		return
	}

	var entries []filesystem.FileInfo
	if searchQuery != "" {
		entries, err = filesystem.SearchFiles(h.Cfg.Root, relPath, searchQuery)
		if err != nil {
			h.RenderError(w, http.StatusInternalServerError, "Internal Server Error", "Search operation failed.")
			return
		}
	} else {
		entries, err = filesystem.ListDirectory(h.Cfg.Root, relPath)
		if err != nil {
			h.RenderError(w, http.StatusInternalServerError, "Internal Server Error", "Failed to list directory contents.")
			return
		}
	}

	// Normalize relPath for breadcrumbs and forms
	relPath = filepath.ToSlash(filepath.Clean(relPath))
	if relPath == "." || relPath == "/" {
		relPath = ""
	}

	// Format max upload size
	maxUploadStr := templates.FormatSize(h.Cfg.MaxUpload)

	data := BrowseData{
		Title:            "File Browser",
		CurrentPath:      relPath,
		SearchQuery:      searchQuery,
		Breadcrumbs:      buildBreadcrumbs(relPath),
		Entries:          entries,
		RootPath:         h.Cfg.Root,
		MaxUploadSizeStr: maxUploadStr,
		EntriesCount:     len(entries),
		AppVersion:       version.Version,
		IsPrivate:        h.Cfg.Private,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	err = templates.ExecuteTemplate(w, "index.html", data)
	if err != nil {
		h.LogError("Failed to render index template: %v", err)
	}
}

// MkdirHandler handles the creation of new directories
func (h *HandlerContext) MkdirHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	err := r.ParseForm()
	if err != nil {
		http.Error(w, "Bad Request: failed to parse form", http.StatusBadRequest)
		return
	}

	parentPath := r.FormValue("path")
	dirName := r.FormValue("name")

	if dirName == "" {
		http.Error(w, "Folder name is required", http.StatusBadRequest)
		return
	}

	if !filesystem.IsValidFilename(dirName) {
		http.Error(w, "Invalid folder name: contains forbidden characters", http.StatusBadRequest)
		return
	}

	// Resolve parent path safely
	safeParent, err := filesystem.ResolveSafePath(h.Cfg.Root, parentPath)
	if err != nil {
		if errors.Is(err, filesystem.ErrUnsafePath) {
			http.Error(w, "Forbidden: Path traversal detected", http.StatusForbidden)
		} else {
			http.Error(w, "Bad Request: invalid path", http.StatusBadRequest)
		}
		return
	}

	targetDir := filepath.Join(safeParent, dirName)

	// Ensure the joined path remains safe
	_, err = filesystem.ResolveSafePath(h.Cfg.Root, filepath.Join(parentPath, dirName))
	if err != nil {
		http.Error(w, "Forbidden: Invalid directory path", http.StatusForbidden)
		return
	}

	// Create directory
	err = os.Mkdir(targetDir, 0755)
	if err != nil {
		if os.IsExist(err) {
			http.Error(w, "Folder already exists", http.StatusConflict)
		} else {
			h.LogError("Failed to create folder %s: %v", targetDir, err)
			http.Error(w, "Internal Server Error: failed to create folder", http.StatusInternalServerError)
		}
		return
	}

	h.LogInfo("created directory %s", filepath.Join(parentPath, dirName))
	w.WriteHeader(http.StatusOK)
}

// RenameHandler handles renaming files and directories
func (h *HandlerContext) RenameHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	err := r.ParseForm()
	if err != nil {
		http.Error(w, "Bad Request: failed to parse form", http.StatusBadRequest)
		return
	}

	itemPath := r.FormValue("path")
	newName := r.FormValue("newName")

	if newName == "" {
		http.Error(w, "New name is required", http.StatusBadRequest)
		return
	}

	if !filesystem.IsValidFilename(newName) {
		http.Error(w, "Invalid name: contains forbidden characters", http.StatusBadRequest)
		return
	}

	// Resolve the target item safely
	safeItemPath, err := filesystem.ResolveSafePath(h.Cfg.Root, itemPath)
	if err != nil {
		if errors.Is(err, filesystem.ErrUnsafePath) {
			http.Error(w, "Forbidden: Path traversal detected", http.StatusForbidden)
		} else {
			http.Error(w, "Bad Request: invalid path", http.StatusBadRequest)
		}
		return
	}

	// Parent directory check
	parentDir := filepath.Dir(safeItemPath)
	newFullPath := filepath.Join(parentDir, newName)

	// Verify the destination path starts with root directory (no traversal)
	destRelPath := filepath.Join(filepath.Dir(itemPath), newName)
	_, err = filesystem.ResolveSafePath(h.Cfg.Root, destRelPath)
	if err != nil {
		http.Error(w, "Forbidden: Invalid destination path", http.StatusForbidden)
		return
	}

	// Perform rename/move
	err = os.Rename(safeItemPath, newFullPath)
	if err != nil {
		if os.IsExist(err) {
			http.Error(w, "An item with the new name already exists", http.StatusConflict)
		} else {
			h.LogError("Failed to rename %s to %s: %v", safeItemPath, newFullPath, err)
			http.Error(w, "Internal Server Error: rename operation failed", http.StatusInternalServerError)
		}
		return
	}

	h.LogInfo("renamed %s to %s", itemPath, destRelPath)
	w.WriteHeader(http.StatusOK)
}

// DeleteHandler handles deletion of files and directories (recursively for directories)
func (h *HandlerContext) DeleteHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	err := r.ParseForm()
	if err != nil {
		http.Error(w, "Bad Request: failed to parse form", http.StatusBadRequest)
		return
	}

	itemPath := r.FormValue("path")
	if itemPath == "" || itemPath == "." || itemPath == "/" {
		http.Error(w, "Cannot delete root or empty path", http.StatusForbidden)
		return
	}

	// Resolve safe path
	safeItemPath, err := filesystem.ResolveSafePath(h.Cfg.Root, itemPath)
	if err != nil {
		if errors.Is(err, filesystem.ErrUnsafePath) {
			http.Error(w, "Forbidden: Path traversal detected", http.StatusForbidden)
		} else {
			http.Error(w, "Bad Request: invalid path", http.StatusBadRequest)
		}
		return
	}

	// Double check that we aren't trying to delete root
	if safeItemPath == filepath.Clean(h.Cfg.Root) {
		http.Error(w, "Cannot delete root directory", http.StatusForbidden)
		return
	}

	// Delete file or directory recursively
	err = os.RemoveAll(safeItemPath)
	if err != nil {
		h.LogError("Failed to delete %s: %v", safeItemPath, err)
		http.Error(w, "Internal Server Error: failed to delete item", http.StatusInternalServerError)
		return
	}

	h.LogInfo("deleted %s", itemPath)
	w.WriteHeader(http.StatusOK)
}

func buildBreadcrumbs(relPath string) []templates.Breadcrumb {
	if relPath == "" || relPath == "." {
		return nil
	}
	parts := strings.Split(filepath.ToSlash(relPath), "/")
	var breadcrumbs []templates.Breadcrumb
	accumulated := ""
	for i, part := range parts {
		if part == "" {
			continue
		}
		if accumulated == "" {
			accumulated = part
		} else {
			accumulated = accumulated + "/" + part
		}
		breadcrumbs = append(breadcrumbs, templates.Breadcrumb{
			Name:   part,
			Path:   accumulated,
			Active: i == len(parts)-1,
		})
	}
	return breadcrumbs
}
