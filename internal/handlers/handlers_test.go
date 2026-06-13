package handlers

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lnoxsian/gophrdrv/internal/config"
)

func setupTestContext(t *testing.T) (*HandlerContext, string) {
	tmpDir, err := os.MkdirTemp("", "handlers-test-root")
	if err != nil {
		t.Fatalf("failed to create temp root: %v", err)
	}

	cfg := &config.Config{
		Root:         tmpDir,
		Port:         8080,
		Host:         "0.0.0.0",
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		MaxUpload:    10 * 1024 * 1024, // 10MB
	}

	ctx := NewHandlerContext(cfg)
	return ctx, tmpDir
}

func TestBrowseHandler(t *testing.T) {
	ctx, tmpDir := setupTestContext(t)
	defer os.RemoveAll(tmpDir)

	// Write a test file
	testFile := filepath.Join(tmpDir, "readme.md")
	if err := os.WriteFile(testFile, []byte("README content"), 0644); err != nil {
		t.Fatal(err)
	}

	// 1. Success case: directory listing
	req := httptest.NewRequest(http.MethodGet, "/?path=", nil)
	w := httptest.NewRecorder()
	ctx.BrowseHandler(w, req)
	if w.Result().StatusCode != http.StatusOK {
		t.Errorf("expected status OK, got %v", w.Result().StatusCode)
	}
	if !strings.Contains(w.Body.String(), "readme.md") {
		t.Errorf("expected response to contain 'readme.md'")
	}

	// 2. Success case: file preview redirect (text file)
	req = httptest.NewRequest(http.MethodGet, "/?path=readme.md", nil)
	w = httptest.NewRecorder()
	ctx.BrowseHandler(w, req)
	if w.Result().StatusCode != http.StatusSeeOther {
		t.Errorf("expected status SeeOther, got %v", w.Result().StatusCode)
	}
	if loc := w.Result().Header.Get("Location"); loc != "/view?file=readme.md" {
		t.Errorf("expected redirect to '/view?file=readme.md', got %q", loc)
	}

	// 3. Success case: file download redirect (binary file)
	binaryFile := filepath.Join(tmpDir, "image.png")
	if err := os.WriteFile(binaryFile, []byte("PNG"), 0644); err != nil {
		t.Fatal(err)
	}
	req = httptest.NewRequest(http.MethodGet, "/?path=image.png", nil)
	w = httptest.NewRecorder()
	ctx.BrowseHandler(w, req)
	if w.Result().StatusCode != http.StatusSeeOther {
		t.Errorf("expected status SeeOther, got %v", w.Result().StatusCode)
	}
	if loc := w.Result().Header.Get("Location"); loc != "/download?file=image.png" {
		t.Errorf("expected redirect to '/download?file=image.png', got %q", loc)
	}

	// 4. Error case: path traversal
	req = httptest.NewRequest(http.MethodGet, "/?path=../", nil)
	w = httptest.NewRecorder()
	ctx.BrowseHandler(w, req)
	if w.Result().StatusCode != http.StatusForbidden {
		t.Errorf("expected status Forbidden (403), got %v", w.Result().StatusCode)
	}

	// 5. Error case: not found
	req = httptest.NewRequest(http.MethodGet, "/?path=nonexistent-folder", nil)
	w = httptest.NewRecorder()
	ctx.BrowseHandler(w, req)
	if w.Result().StatusCode != http.StatusNotFound {
		t.Errorf("expected status NotFound (404), got %v", w.Result().StatusCode)
	}

	// 6. Error case: wrong method
	req = httptest.NewRequest(http.MethodPost, "/?path=", nil)
	w = httptest.NewRecorder()
	ctx.BrowseHandler(w, req)
	if w.Result().StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("expected status MethodNotAllowed (405), got %v", w.Result().StatusCode)
	}
}

func TestMkdirHandler(t *testing.T) {
	ctx, tmpDir := setupTestContext(t)
	defer os.RemoveAll(tmpDir)

	// 1. Success case
	form := url.Values{}
	form.Add("path", "")
	form.Add("name", "test-folder")
	req := httptest.NewRequest(http.MethodPost, "/mkdir", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	ctx.MkdirHandler(w, req)
	if w.Result().StatusCode != http.StatusOK {
		t.Errorf("expected status OK, got %v", w.Result().StatusCode)
	}

	// Verify folder exists
	expectedPath := filepath.Join(tmpDir, "test-folder")
	if info, err := os.Stat(expectedPath); err != nil || !info.IsDir() {
		t.Errorf("expected folder to exist and be dir")
	}

	// 2. Error case: folder already exists
	req = httptest.NewRequest(http.MethodPost, "/mkdir", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w = httptest.NewRecorder()
	ctx.MkdirHandler(w, req)
	if w.Result().StatusCode != http.StatusConflict {
		t.Errorf("expected status Conflict (409), got %v", w.Result().StatusCode)
	}

	// 3. Error case: invalid name
	form = url.Values{}
	form.Add("path", "")
	form.Add("name", "folder/name")
	req = httptest.NewRequest(http.MethodPost, "/mkdir", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w = httptest.NewRecorder()
	ctx.MkdirHandler(w, req)
	if w.Result().StatusCode != http.StatusBadRequest {
		t.Errorf("expected status BadRequest, got %v", w.Result().StatusCode)
	}

	// 4. Error case: path traversal parent
	form = url.Values{}
	form.Add("path", "../")
	form.Add("name", "safe-name")
	req = httptest.NewRequest(http.MethodPost, "/mkdir", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w = httptest.NewRecorder()
	ctx.MkdirHandler(w, req)
	if w.Result().StatusCode != http.StatusForbidden {
		t.Errorf("expected status Forbidden (403), got %v", w.Result().StatusCode)
	}
}

func TestUploadHandler(t *testing.T) {
	ctx, tmpDir := setupTestContext(t)
	defer os.RemoveAll(tmpDir)

	// 1. Success case
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	_ = writer.WriteField("path", "")
	part, _ := writer.CreateFormFile("file", "upload.txt")
	_, _ = part.Write([]byte("uploaded data"))
	_ = writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	ctx.UploadHandler(w, req)
	if w.Result().StatusCode != http.StatusOK {
		t.Errorf("expected status OK, got %v", w.Result().StatusCode)
	}

	// 2. Error case: unsafe path
	body = &bytes.Buffer{}
	writer = multipart.NewWriter(body)
	_ = writer.WriteField("path", "../")
	part, _ = writer.CreateFormFile("file", "hack.txt")
	_, _ = part.Write([]byte("hack"))
	_ = writer.Close()

	req = httptest.NewRequest(http.MethodPost, "/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w = httptest.NewRecorder()
	ctx.UploadHandler(w, req)
	if w.Result().StatusCode != http.StatusForbidden {
		t.Errorf("expected status Forbidden, got %v", w.Result().StatusCode)
	}

	// 3. Success case: multiple files
	body = &bytes.Buffer{}
	writer = multipart.NewWriter(body)
	_ = writer.WriteField("path", "")
	
	part1, _ := writer.CreateFormFile("files", "upload1.txt")
	_, _ = part1.Write([]byte("first file content"))
	
	part2, _ := writer.CreateFormFile("files", "upload2.txt")
	_, _ = part2.Write([]byte("second file content"))
	
	_ = writer.Close()

	req = httptest.NewRequest(http.MethodPost, "/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w = httptest.NewRecorder()
	ctx.UploadHandler(w, req)
	if w.Result().StatusCode != http.StatusOK {
		t.Errorf("expected status OK for multi-upload, got %v", w.Result().StatusCode)
	}

	// Verify both files exist
	if _, err := os.Stat(filepath.Join(tmpDir, "upload1.txt")); err != nil {
		t.Errorf("expected upload1.txt to exist: %v", err)
	}
	if _, err := os.Stat(filepath.Join(tmpDir, "upload2.txt")); err != nil {
		t.Errorf("expected upload2.txt to exist: %v", err)
	}
}

func TestDownloadHandler(t *testing.T) {
	ctx, tmpDir := setupTestContext(t)
	defer os.RemoveAll(tmpDir)

	// Write test file
	testFile := filepath.Join(tmpDir, "download.txt")
	if err := os.WriteFile(testFile, []byte("secret content"), 0644); err != nil {
		t.Fatal(err)
	}

	// 1. Success case
	req := httptest.NewRequest(http.MethodGet, "/download?file=download.txt", nil)
	w := httptest.NewRecorder()
	ctx.DownloadHandler(w, req)
	if w.Result().StatusCode != http.StatusOK {
		t.Errorf("expected status OK, got %v", w.Result().StatusCode)
	}

	// 2. Error case: download directory
	req = httptest.NewRequest(http.MethodGet, "/download?file=", nil)
	w = httptest.NewRecorder()
	ctx.DownloadHandler(w, req)
	if w.Result().StatusCode != http.StatusBadRequest {
		t.Errorf("expected status BadRequest, got %v", w.Result().StatusCode)
	}

	// 3. Error case: traversal
	req = httptest.NewRequest(http.MethodGet, "/download?file=../download.txt", nil)
	w = httptest.NewRecorder()
	ctx.DownloadHandler(w, req)
	if w.Result().StatusCode != http.StatusForbidden {
		t.Errorf("expected status Forbidden, got %v", w.Result().StatusCode)
	}

	// 4. Error case: not found
	req = httptest.NewRequest(http.MethodGet, "/download?file=nonexistent.txt", nil)
	w = httptest.NewRecorder()
	ctx.DownloadHandler(w, req)
	if w.Result().StatusCode != http.StatusNotFound {
		t.Errorf("expected status NotFound, got %v", w.Result().StatusCode)
	}

	// 5. Success case: download directory as ZIP
	subDir := filepath.Join(tmpDir, "subfolder")
	if err := os.Mkdir(subDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(subDir, "inner.txt"), []byte("inner content"), 0644); err != nil {
		t.Fatal(err)
	}

	req = httptest.NewRequest(http.MethodGet, "/download?file=subfolder", nil)
	w = httptest.NewRecorder()
	ctx.DownloadHandler(w, req)
	if w.Result().StatusCode != http.StatusOK {
		t.Errorf("expected status OK for directory download, got %v", w.Result().StatusCode)
	}
	if contentType := w.Result().Header.Get("Content-Type"); contentType != "application/zip" {
		t.Errorf("expected Content-Type application/zip, got %v", contentType)
	}
	if contentDisposition := w.Result().Header.Get("Content-Disposition"); contentDisposition != "attachment; filename=\"subfolder.zip\"" {
		t.Errorf("expected Content-Disposition attachment; filename=\"subfolder.zip\", got %v", contentDisposition)
	}
}

func TestDownloadZipHandler(t *testing.T) {
	ctx, tmpDir := setupTestContext(t)
	defer os.RemoveAll(tmpDir)

	// Create test file and directory
	file1 := filepath.Join(tmpDir, "file1.txt")
	if err := os.WriteFile(file1, []byte("file1 content"), 0644); err != nil {
		t.Fatal(err)
	}

	subDir := filepath.Join(tmpDir, "folder")
	if err := os.Mkdir(subDir, 0755); err != nil {
		t.Fatal(err)
	}

	file2 := filepath.Join(subDir, "file2.txt")
	if err := os.WriteFile(file2, []byte("file2 content"), 0644); err != nil {
		t.Fatal(err)
	}

	// 1. Success case: Zipping files and folders
	form := url.Values{}
	form.Add("path", "")
	form.Add("files", "file1.txt")
	form.Add("files", "folder")

	req := httptest.NewRequest(http.MethodPost, "/download-zip", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	ctx.DownloadZipHandler(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status OK, got %v", resp.StatusCode)
	}
	if resp.Header.Get("Content-Type") != "application/zip" {
		t.Errorf("expected Content-Type application/zip, got %q", resp.Header.Get("Content-Type"))
	}
	if !strings.Contains(resp.Header.Get("Content-Disposition"), "attachment; filename=\"archive-") {
		t.Errorf("expected Content-Disposition attachment, got %q", resp.Header.Get("Content-Disposition"))
	}

	// 2. Error case: No files
	formEmpty := url.Values{}
	req = httptest.NewRequest(http.MethodPost, "/download-zip", strings.NewReader(formEmpty.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w = httptest.NewRecorder()
	ctx.DownloadZipHandler(w, req)
	if w.Result().StatusCode != http.StatusBadRequest {
		t.Errorf("expected status BadRequest for no files, got %v", w.Result().StatusCode)
	}

	// 3. Error case: Traversal
	formTraversal := url.Values{}
	formTraversal.Add("path", "")
	formTraversal.Add("files", "../somefile.txt")
	req = httptest.NewRequest(http.MethodPost, "/download-zip", strings.NewReader(formTraversal.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w = httptest.NewRecorder()
	ctx.DownloadZipHandler(w, req)
	if w.Result().StatusCode != http.StatusForbidden {
		t.Errorf("expected status Forbidden for traversal, got %v", w.Result().StatusCode)
	}

	// 4. Error case: Not found
	formNotFound := url.Values{}
	formNotFound.Add("path", "")
	formNotFound.Add("files", "nonexistent.txt")
	req = httptest.NewRequest(http.MethodPost, "/download-zip", strings.NewReader(formNotFound.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w = httptest.NewRecorder()
	ctx.DownloadZipHandler(w, req)
	if w.Result().StatusCode != http.StatusNotFound {
		t.Errorf("expected status NotFound, got %v", w.Result().StatusCode)
	}
}

func TestViewHandler(t *testing.T) {
	ctx, tmpDir := setupTestContext(t)
	defer os.RemoveAll(tmpDir)

	// Write test file
	testFile := filepath.Join(tmpDir, "view.txt")
	if err := os.WriteFile(testFile, []byte("text data"), 0644); err != nil {
		t.Fatal(err)
	}

	// 1. Success case
	req := httptest.NewRequest(http.MethodGet, "/view?file=view.txt", nil)
	w := httptest.NewRecorder()
	ctx.ViewHandler(w, req)
	if w.Result().StatusCode != http.StatusOK {
		t.Errorf("expected status OK, got %v", w.Result().StatusCode)
	}

	// 2. Error case: unsupported type
	binaryFile := filepath.Join(tmpDir, "view.png")
	if err := os.WriteFile(binaryFile, []byte("PNG"), 0644); err != nil {
		t.Fatal(err)
	}
	req = httptest.NewRequest(http.MethodGet, "/view?file=view.png", nil)
	w = httptest.NewRecorder()
	ctx.ViewHandler(w, req)
	if w.Result().StatusCode != http.StatusBadRequest {
		t.Errorf("expected status BadRequest, got %v", w.Result().StatusCode)
	}

	// 3. Error case: file too large
	largeFile := filepath.Join(tmpDir, "large.txt")
	largeData := make([]byte, maxViewableTextSize+1)
	for i := range largeData {
		largeData[i] = 'a'
	}
	if err := os.WriteFile(largeFile, largeData, 0644); err != nil {
		t.Fatal(err)
	}
	req = httptest.NewRequest(http.MethodGet, "/view?file=large.txt", nil)
	w = httptest.NewRecorder()
	ctx.ViewHandler(w, req)
	if w.Result().StatusCode != http.StatusRequestEntityTooLarge {
		t.Errorf("expected status RequestEntityTooLarge, got %v", w.Result().StatusCode)
	}

	// 4. Error case: file has text extension but contains binary data
	fakeTextFile := filepath.Join(tmpDir, "fake.txt")
	if err := os.WriteFile(fakeTextFile, []byte("some text\x00more text"), 0644); err != nil {
		t.Fatal(err)
	}
	req = httptest.NewRequest(http.MethodGet, "/view?file=fake.txt", nil)
	w = httptest.NewRecorder()
	ctx.ViewHandler(w, req)
	if w.Result().StatusCode != http.StatusBadRequest {
		t.Errorf("expected status BadRequest for binary content, got %v", w.Result().StatusCode)
	}
}

func TestRenameHandler(t *testing.T) {
	ctx, tmpDir := setupTestContext(t)
	defer os.RemoveAll(tmpDir)

	testFile := filepath.Join(tmpDir, "old.txt")
	if err := os.WriteFile(testFile, []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}

	// 1. Success case
	form := url.Values{}
	form.Add("path", "old.txt")
	form.Add("newName", "new.txt")
	req := httptest.NewRequest(http.MethodPost, "/rename", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	ctx.RenameHandler(w, req)
	if w.Result().StatusCode != http.StatusOK {
		t.Errorf("expected status OK, got %v", w.Result().StatusCode)
	}

	// 2. Error case: empty newName
	form = url.Values{}
	form.Add("path", "new.txt")
	form.Add("newName", "")
	req = httptest.NewRequest(http.MethodPost, "/rename", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w = httptest.NewRecorder()
	ctx.RenameHandler(w, req)
	if w.Result().StatusCode != http.StatusBadRequest {
		t.Errorf("expected status BadRequest, got %v", w.Result().StatusCode)
	}
}

func TestDeleteHandler(t *testing.T) {
	ctx, tmpDir := setupTestContext(t)
	defer os.RemoveAll(tmpDir)

	testFile := filepath.Join(tmpDir, "delete.txt")
	if err := os.WriteFile(testFile, []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}

	// 1. Success case
	form := url.Values{}
	form.Add("path", "delete.txt")
	req := httptest.NewRequest(http.MethodPost, "/delete", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	ctx.DeleteHandler(w, req)
	if w.Result().StatusCode != http.StatusOK {
		t.Errorf("expected status OK, got %v", w.Result().StatusCode)
	}

	// 2. Error case: empty or root deletion
	form = url.Values{}
	form.Add("path", "")
	req = httptest.NewRequest(http.MethodPost, "/delete", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w = httptest.NewRecorder()
	ctx.DeleteHandler(w, req)
	if w.Result().StatusCode != http.StatusForbidden {
		t.Errorf("expected status Forbidden, got %v", w.Result().StatusCode)
	}
}

func TestEditHandler(t *testing.T) {
	ctx, tmpDir := setupTestContext(t)
	defer os.RemoveAll(tmpDir)

	// Write test file
	testFile := filepath.Join(tmpDir, "edit.txt")
	if err := os.WriteFile(testFile, []byte("some text"), 0644); err != nil {
		t.Fatal(err)
	}

	// 1. Success case
	req := httptest.NewRequest(http.MethodGet, "/edit?file=edit.txt", nil)
	w := httptest.NewRecorder()
	ctx.EditHandler(w, req)
	if w.Result().StatusCode != http.StatusOK {
		t.Errorf("expected status OK, got %v", w.Result().StatusCode)
	}

	// 2. Error case: unsupported type
	unsupportedFile := filepath.Join(tmpDir, "edit.png")
	if err := os.WriteFile(unsupportedFile, []byte("fake png"), 0644); err != nil {
		t.Fatal(err)
	}
	req = httptest.NewRequest(http.MethodGet, "/edit?file=edit.png", nil)
	w = httptest.NewRecorder()
	ctx.EditHandler(w, req)
	if w.Result().StatusCode != http.StatusBadRequest {
		t.Errorf("expected status BadRequest, got %v", w.Result().StatusCode)
	}
}

func TestSaveHandler(t *testing.T) {
	ctx, tmpDir := setupTestContext(t)
	defer os.RemoveAll(tmpDir)

	// 1. Success case: create/save file
	form := url.Values{}
	form.Add("file", "save.txt")
	form.Add("content", "new content")
	req := httptest.NewRequest(http.MethodPost, "/save", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	ctx.SaveHandler(w, req)
	if w.Result().StatusCode != http.StatusOK {
		t.Errorf("expected status OK, got %v", w.Result().StatusCode)
	}

	// Verify file was written
	written, err := os.ReadFile(filepath.Join(tmpDir, "save.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(written) != "new content" {
		t.Errorf("expected 'new content', got '%s'", string(written))
	}

	// 2. Error case: unsupported type
	form = url.Values{}
	form.Add("file", "save.png")
	form.Add("content", "fake")
	req = httptest.NewRequest(http.MethodPost, "/save", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w = httptest.NewRecorder()
	ctx.SaveHandler(w, req)
	if w.Result().StatusCode != http.StatusBadRequest {
		t.Errorf("expected status BadRequest, got %v", w.Result().StatusCode)
	}
}
