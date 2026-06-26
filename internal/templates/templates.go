package templates

import (
	"embed"
	"fmt"
	"html/template"
	"io"
	"path/filepath"
	"strings"
	"time"
)

//go:embed index.html view.html error.html edit.html lock.html
var TemplatesFS embed.FS

var parsedTemplates *template.Template

type Breadcrumb struct {
	Name   string
	Path   string
	Active bool
}

func init() {
	var err error
	funcMap := template.FuncMap{
		"getFileIconType": GetFileIconType,
		"isTextViewable":  IsTextViewable,
		"formatSize":      FormatSize,
		"formatTime":      FormatTime,
	}

	parsedTemplates, err = template.New("").Funcs(funcMap).ParseFS(TemplatesFS, "*.html")
	if err != nil {
		panic(fmt.Sprintf("failed to parse templates: %v", err))
	}
}

// IsBinaryExtension returns true if the extension belongs to a known binary file type.
func IsBinaryExtension(ext string) bool {
	switch ext {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp", ".ico", ".bmp", ".svg",
		".pdf",
		".zip", ".tar", ".gz", ".rar", ".7z", ".bz2", ".xz",
		".exe", ".dll", ".so", ".dylib", ".bin", ".out", ".app",
		".mp4", ".mp3", ".wav", ".avi", ".mov", ".flv", ".webm", ".mkv",
		".dmg", ".iso", ".img",
		".woff", ".woff2", ".ttf", ".eot",
		".db", ".sqlite", ".dat":
		return true
	default:
		return false
	}
}

// GetFileIconType maps filenames/extensions to categories
func GetFileIconType(name string) string {
	ext := strings.ToLower(filepath.Ext(name))
	if IsBinaryExtension(ext) {
		return "binary"
	}
	return "text"
}

// IsTextViewable determines if a file can be opened in the text viewer
func IsTextViewable(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	return !IsBinaryExtension(ext)
}

// FormatSize converts bytes to a human-readable string
func FormatSize(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB", float64(size)/float64(div), "KMGTPE"[exp])
}

// FormatTime formats a time.Time object
func FormatTime(t time.Time) string {
	return t.Format("2006-01-02 15:04:05")
}

// ExecuteTemplate executes a parsed template
func ExecuteTemplate(w io.Writer, name string, data interface{}) error {
	return parsedTemplates.ExecuteTemplate(w, name, data)
}
