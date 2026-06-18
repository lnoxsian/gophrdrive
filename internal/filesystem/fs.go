package filesystem

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lnoxsian/gophrdrv/internal/templates"
)

var (
	ErrUnsafePath      = errors.New("unsafe path: path traversal detected")
	ErrInvalidFilename = errors.New("invalid filename: contains forbidden characters")
)

// ResolveSafePath validates and converts a relative path to a safe absolute path.
func ResolveSafePath(root, relPath string) (string, error) {
	// Clean the relative path to remove any dots or duplicate separators
	relPath = filepath.Clean(relPath)
	if relPath == "." || relPath == "/" || relPath == "" {
		return root, nil
	}

	// Join root with relative path
	fullPath := filepath.Join(root, relPath)

	// Get absolute path of joined path
	absTarget, err := filepath.Abs(fullPath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve absolute path: %w", err)
	}

	// Clean both
	absRoot := filepath.Clean(root)
	absTarget = filepath.Clean(absTarget)

	if absTarget == absRoot {
		return absTarget, nil
	}

	// Ensure prefix matching works correctly without partial folder matching (e.g. /data vs /data-extra)
	prefix := absRoot
	if !strings.HasSuffix(prefix, string(filepath.Separator)) {
		prefix += string(filepath.Separator)
	}

	if !strings.HasPrefix(absTarget, prefix) {
		return "", ErrUnsafePath
	}

	return absTarget, nil
}

// IsValidFilename checks if the filename contains only valid characters.
func IsValidFilename(name string) bool {
	if name == "" || name == "." || name == ".." {
		return false
	}
	for _, r := range name {
		if r < 32 || r == 127 { // control characters
			return false
		}
		// Characters not allowed in Windows/Linux filesystems or unsafe for URLs
		if strings.ContainsRune(`/\\:*?"<>|`, r) {
			return false
		}
	}
	return true
}

// IsBinaryFile checks the first 512 bytes of a file. If it contains a NULL byte, it is considered binary.
func IsBinaryFile(filePath string) (bool, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return false, err
	}
	defer f.Close()

	buffer := make([]byte, 512)
	n, err := f.Read(buffer)
	if err != nil && err != io.EOF {
		return false, err
	}

	for i := 0; i < n; i++ {
		if buffer[i] == 0 {
			return true, nil // contains NULL byte -> binary
		}
	}
	return false, nil // text
}

// FileInfo holds metadata about a file or directory
type FileInfo struct {
	Name     string    `json:"name"`
	Path     string    `json:"path"` // relative path from root
	IsDir    bool      `json:"is_dir"`
	Size     int64     `json:"size"`
	ModTime  time.Time `json:"mod_time"`
	IsBinary bool      `json:"is_binary"`
}

// ListDirectory lists files in target path, sorted directories first, then alphabetically.
func ListDirectory(root, relPath string) ([]FileInfo, error) {
	safePath, err := ResolveSafePath(root, relPath)
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(safePath)
	if err != nil {
		return nil, err
	}

	var dirs []FileInfo
	var files []FileInfo

	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue // Skip entries we can't read info for
		}

		name := entry.Name()
		var itemRelPath string
		if relPath == "" || relPath == "." {
			itemRelPath = name
		} else {
			itemRelPath = filepath.Join(relPath, name)
		}

		// Convert to slash-based relative path for URL uniformity
		itemRelPath = filepath.ToSlash(itemRelPath)

		isBinary := false
		if !entry.IsDir() {
			var err error
			isBinary, err = IsBinaryFile(filepath.Join(safePath, name))
			if err != nil {
				isBinary = templates.IsBinaryExtension(filepath.Ext(name))
			}
		}

		fileInfo := FileInfo{
			Name:     name,
			Path:     itemRelPath,
			IsDir:    entry.IsDir(),
			Size:     info.Size(),
			ModTime:  info.ModTime(),
			IsBinary: isBinary,
		}

		if entry.IsDir() {
			dirs = append(dirs, fileInfo)
		} else {
			files = append(files, fileInfo)
		}
	}

	// Sort directories and files alphabetically case-insensitively
	sortFunc := func(list []FileInfo) {
		for i := 0; i < len(list); i++ {
			for j := i + 1; j < len(list); j++ {
				if strings.ToLower(list[i].Name) > strings.ToLower(list[j].Name) {
					list[i], list[j] = list[j], list[i]
				}
			}
		}
	}

	sortFunc(dirs)
	sortFunc(files)

	return append(dirs, files...), nil
}

// SearchFiles recursively searches for files matching a query in target relative path.
func SearchFiles(root, relPath, query string) ([]FileInfo, error) {
	safeStartPath, err := ResolveSafePath(root, relPath)
	if err != nil {
		return nil, err
	}

	var results []FileInfo

	err = filepath.WalkDir(safeStartPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // ignore permission or read errors on individual files/dirs
		}

		// Calculate relative path from root
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)

		// Don't include the start path itself
		if rel == filepath.ToSlash(relPath) {
			return nil
		}

		name := d.Name()
		if MatchesFZF(name, query) || MatchesFZF(rel, query) {
			info, err := d.Info()
			if err != nil {
				return nil
			}

			isBinary := false
			if !d.IsDir() {
				var err error
				isBinary, err = IsBinaryFile(path)
				if err != nil {
					isBinary = templates.IsBinaryExtension(filepath.Ext(name))
				}
			}

			results = append(results, FileInfo{
				Name:     name,
				Path:     rel,
				IsDir:    d.IsDir(),
				Size:     info.Size(),
				ModTime:  info.ModTime(),
				IsBinary: isBinary,
			})
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	// Sort results: directories first, then files alphabetically
	var dirs []FileInfo
	var files []FileInfo
	for _, res := range results {
		if res.IsDir {
			dirs = append(dirs, res)
		} else {
			files = append(files, res)
		}
	}

	sortFunc := func(list []FileInfo) {
		for i := 0; i < len(list); i++ {
			for j := i + 1; j < len(list); j++ {
				if strings.ToLower(list[i].Name) > strings.ToLower(list[j].Name) {
					list[i], list[j] = list[j], list[i]
				}
			}
		}
	}
	sortFunc(dirs)
	sortFunc(files)

	return append(dirs, files...), nil
}

// MatchesFZF checks if the target string matches the fzf-like query.
func MatchesFZF(target, query string) bool {
	query = strings.TrimSpace(query)
	if query == "" {
		return true
	}

	targetLower := strings.ToLower(target)
	terms := strings.Fields(strings.ToLower(query))

	for _, term := range terms {
		if term == "" {
			continue
		}

		matched := false
		if strings.HasPrefix(term, "!") {
			// Inverse match
			subterm := term[1:]
			if subterm == "" {
				// solitary ! matches nothing or is ignored.
				continue
			}
			if strings.HasPrefix(subterm, "^") {
				matched = !strings.HasPrefix(targetLower, subterm[1:])
			} else if strings.HasSuffix(subterm, "$") {
				matched = !strings.HasSuffix(targetLower, subterm[:len(subterm)-1])
			} else {
				matched = !strings.Contains(targetLower, subterm)
			}
		} else {
			// Positive match
			if strings.HasPrefix(term, "'") {
				// Exact match
				matched = strings.Contains(targetLower, term[1:])
			} else if strings.HasPrefix(term, "^") {
				// Prefix match
				matched = strings.HasPrefix(targetLower, term[1:])
			} else if strings.HasSuffix(term, "$") {
				// Suffix match
				matched = strings.HasSuffix(targetLower, term[:len(term)-1])
			} else {
				// Fuzzy match
				matched = fuzzyMatchLower(targetLower, term)
			}
		}

		// Since all terms are ANDed, if any term fails to match, the entire query fails.
		if !matched {
			return false
		}
	}

	return true
}

func fuzzyMatchLower(target, term string) bool {
	tIdx := 0
	targetRunes := []rune(target)
	for _, r := range term {
		found := false
		for tIdx < len(targetRunes) {
			tr := targetRunes[tIdx]
			tIdx++
			if tr == r {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}
