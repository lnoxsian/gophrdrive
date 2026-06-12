package filesystem

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveSafePath(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "fs-test-root")
	if err != nil {
		t.Fatalf("failed to create temp root: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a subfolder inside temp root
	subFolder := filepath.Join(tmpDir, "sub")
	if err := os.Mkdir(subFolder, 0755); err != nil {
		t.Fatalf("failed to create subfolder: %v", err)
	}

	tests := []struct {
		name     string
		relPath  string
		expected string
		wantErr  bool
	}{
		{"Root path", "", tmpDir, false},
		{"Root path dot", ".", tmpDir, false},
		{"Valid subfolder", "sub", subFolder, false},
		{"Valid subfolder trailing slash", "sub/", subFolder, false},
		{"Path traversal back to root", "sub/..", tmpDir, false},
		{"Traversal attempt outside root", "../", "", true},
		{"Complex traversal attempt", "sub/../../", "", true},
		{"Exploit prefix attempt", "../fs-test-root-extra", "", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ResolveSafePath(tmpDir, tc.relPath)
			if tc.wantErr {
				if err == nil {
					t.Errorf("expected error for path %q, got nil", tc.relPath)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error for path %q: %v", tc.relPath, err)
				}
				evalGot, _ := filepath.EvalSymlinks(got)
				evalExpected, _ := filepath.EvalSymlinks(tc.expected)
				if filepath.Clean(evalGot) != filepath.Clean(evalExpected) {
					t.Errorf("expected path %q, got %q", evalExpected, evalGot)
				}
			}
		})
	}
}

func TestIsValidFilename(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"file.txt", true},
		{"archive.tar.gz", true},
		{"folder-123_abc", true},
		{"file/name", false},
		{"file\\name", false},
		{"file:name", false},
		{"file*name", false},
		{"file?name", false},
		{"file\"name", false},
		{"file<name", false},
		{"file>name", false},
		{"file|name", false},
		{"", false},
		{".", false},
		{"..", false},
		{"file\x00name", false},
		{"file\u001Fname", false},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got := IsValidFilename(tc.input)
			if got != tc.expected {
				t.Errorf("for input %q: expected %t, got %t", tc.input, tc.expected, got)
			}
		})
	}
}

func TestListDirectoryAndSearch(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "fs-list-test")
	if err != nil {
		t.Fatalf("failed to create temp root: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test files and folders
	// folders: docs, pictures
	// files: docs/report.pdf, docs/notes.txt, readme.md
	if err := os.Mkdir(filepath.Join(tmpDir, "docs"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(tmpDir, "pictures"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "readme.md"), []byte("Hello"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "docs", "report.pdf"), []byte("PDF"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "docs", "notes.txt"), []byte("Notes"), 0644); err != nil {
		t.Fatal(err)
	}

	// Test ListDirectory on root
	entries, err := ListDirectory(tmpDir, "")
	if err != nil {
		t.Fatalf("failed to list directory: %v", err)
	}

	// Expected order: directories first alphabetically, then files alphabetically
	// Expected: docs (dir), pictures (dir), readme.md (file)
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}
	if entries[0].Name != "docs" || !entries[0].IsDir {
		t.Errorf("expected first entry to be dir 'docs', got %s (dir=%t)", entries[0].Name, entries[0].IsDir)
	}
	if entries[1].Name != "pictures" || !entries[1].IsDir {
		t.Errorf("expected second entry to be dir 'pictures', got %s", entries[1].Name)
	}
	if entries[2].Name != "readme.md" || entries[2].IsDir {
		t.Errorf("expected third entry to be file 'readme.md', got %s", entries[2].Name)
	}

	// Test SearchFiles recursively
	results, err := SearchFiles(tmpDir, "", "report")
	if err != nil {
		t.Fatalf("failed to search: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 search result, got %d", len(results))
	}
	if results[0].Name != "report.pdf" || results[0].Path != "docs/report.pdf" {
		t.Errorf("unexpected search result: %+v", results[0])
	}
}
