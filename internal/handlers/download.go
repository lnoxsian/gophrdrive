package handlers

import (
	"archive/zip"
	"bufio"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

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
		// Set headers to stream ZIP content to the browser
		dirName := filepath.Base(safePath)
		zipFilename := fmt.Sprintf("%s.zip", dirName)
		w.Header().Set("Content-Disposition", "attachment; filename=\""+zipFilename+"\"")
		w.Header().Set("Content-Type", "application/zip")

		// Create buffered writer to reduce network writing system calls
		bufWriter := bufio.NewWriter(w)
		defer bufWriter.Flush()

		// Create zip writer writing to buffered writer
		zw := zip.NewWriter(bufWriter)
		defer zw.Close()

		// Pre-allocate 128KB copy buffer for reuse
		copyBuf := make([]byte, 128*1024)

		safeParent := filepath.Dir(safePath)

		// Recursively walk directory and add files to zip
		err = filepath.WalkDir(safePath, func(path string, d os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}

			// Get relative path in zip with respect to parent dir
			relInZip, err := filepath.Rel(safeParent, path)
			if err != nil {
				return err
			}

			// Convert to slash for zip standard
			relInZip = filepath.ToSlash(relInZip)

			if d.IsDir() {
				// Directory entry in ZIP ends with a slash and has no content
				if relInZip != "" && relInZip != "." {
					_, err = zw.Create(relInZip + "/")
					return err
				}
				return nil
			}

			// It's a file, add it
			return addFileToZip(zw, path, relInZip, copyBuf)
		})
		if err != nil {
			h.LogError("Directory ZIP download: failed walking folder %s: %v", relPath, err)
			return
		}

		// Log download operation
		h.LogInfo("downloaded directory %s as ZIP", relPath)
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

// DownloadZipHandler handles downloading multiple files/folders as a single ZIP archive.
func (h *HandlerContext) DownloadZipHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.RenderError(w, http.StatusMethodNotAllowed, "Method Not Allowed", "Only POST requests are allowed for ZIP downloads.")
		return
	}

	err := r.ParseForm()
	if err != nil {
		h.RenderError(w, http.StatusBadRequest, "Bad Request", "Failed to parse form data.")
		return
	}

	parentPath := r.FormValue("path")
	files := r.Form["files"]

	if len(files) == 0 {
		h.RenderError(w, http.StatusBadRequest, "Bad Request", "No files selected for download.")
		return
	}

	// Resolve the parent path safely
	safeParent, err := filesystem.ResolveSafePath(h.Cfg.Root, parentPath)
	if err != nil {
		if errors.Is(err, filesystem.ErrUnsafePath) {
			h.RenderError(w, http.StatusForbidden, "Forbidden", "Path traversal attempts are strictly forbidden.")
		} else {
			h.RenderError(w, http.StatusBadRequest, "Bad Request", "Invalid parent path specification.")
		}
		return
	}

	// Validate all requested paths exist and are safe first
	for _, relItemPath := range files {
		safeItemPath, err := filesystem.ResolveSafePath(h.Cfg.Root, relItemPath)
		if err != nil {
			if errors.Is(err, filesystem.ErrUnsafePath) {
				h.RenderError(w, http.StatusForbidden, "Forbidden", "Path traversal attempts are strictly forbidden.")
			} else {
				h.RenderError(w, http.StatusBadRequest, "Bad Request", "Invalid path specification.")
			}
			return
		}
		if _, err := os.Stat(safeItemPath); err != nil {
			if os.IsNotExist(err) {
				h.RenderError(w, http.StatusNotFound, "Not Found", fmt.Sprintf("Selected item not found: %s", filepath.Base(relItemPath)))
			} else {
				h.RenderError(w, http.StatusInternalServerError, "Internal Server Error", "Failed to verify selected items.")
			}
			return
		}
	}

	// Set headers to stream ZIP content to the browser
	timestamp := time.Now().Format("20060102-150405")
	zipFilename := fmt.Sprintf("archive-%s.zip", timestamp)
	w.Header().Set("Content-Disposition", "attachment; filename=\""+zipFilename+"\"")
	w.Header().Set("Content-Type", "application/zip")

	// Create buffered writer to reduce network writing system calls
	bufWriter := bufio.NewWriter(w)
	defer bufWriter.Flush()

	// Create zip writer writing to buffered writer
	zw := zip.NewWriter(bufWriter)
	defer zw.Close()

	// Pre-allocate 128KB copy buffer for reuse
	copyBuf := make([]byte, 128*1024)

	for _, relItemPath := range files {
		safeItemPath, _ := filesystem.ResolveSafePath(h.Cfg.Root, relItemPath)
		info, err := os.Stat(safeItemPath)
		if err != nil {
			continue // Already verified to exist, but skip if something changed
		}

		if info.IsDir() {
			// Recursively walk directory and add files to zip
			err = filepath.WalkDir(safeItemPath, func(path string, d os.DirEntry, walkErr error) error {
				if walkErr != nil {
					return walkErr
				}

				// Get relative path in zip with respect to parent dir
				relInZip, err := filepath.Rel(safeParent, path)
				if err != nil {
					return err
				}

				// Convert to slash for zip standard
				relInZip = filepath.ToSlash(relInZip)

				if d.IsDir() {
					// Directory entry in ZIP ends with a slash and has no content
					if relInZip != "" && relInZip != "." {
						_, err = zw.Create(relInZip + "/")
						return err
					}
					return nil
				}

				// It's a file, add it
				return addFileToZip(zw, path, relInZip, copyBuf)
			})
			if err != nil {
				h.LogError("ZIP download: failed walking folder %s: %v", relItemPath, err)
				return
			}
		} else {
			// It's a single file
			relInZip, err := filepath.Rel(safeParent, safeItemPath)
			if err != nil {
				h.LogError("ZIP download: failed relative path for %s: %v", relItemPath, err)
				return
			}
			relInZip = filepath.ToSlash(relInZip)

			err = addFileToZip(zw, safeItemPath, relInZip, copyBuf)
			if err != nil {
				h.LogError("ZIP download: failed adding file %s to zip: %v", relItemPath, err)
				return
			}
		}
	}

	h.LogInfo("downloaded ZIP archive containing %d items", len(files))
}

func addFileToZip(zw *zip.Writer, safeFilePath, relInZip string, copyBuf []byte) error {
	file, err := os.Open(safeFilePath)
	if err != nil {
		return err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return err
	}

	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}

	header.Name = relInZip
	header.Method = zip.Deflate

	writer, err := zw.CreateHeader(header)
	if err != nil {
		return err
	}

	_, err = io.CopyBuffer(writer, file, copyBuf)
	return err
}
