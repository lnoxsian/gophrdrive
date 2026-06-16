package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseSize(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
		hasError bool
	}{
		{"100MB", 100 * 1024 * 1024, false},
		{"1GB", 1024 * 1024 * 1024, false},
		{"500KB", 500 * 1024, false},
		{"10B", 10, false},
		{"5", 5, false},
		{"2.5MB", 2621440, false}, // 2.5 * 1024 * 1024 = 2,621,440
		{"", 0, true},
		{"100XB", 0, true},
		{"abc", 0, true},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got, err := ParseSize(tc.input)
			if tc.hasError {
				if err == nil {
					t.Errorf("expected error for input %q, got none", tc.input)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error for input %q: %v", tc.input, err)
				}
				if got != tc.expected {
					t.Errorf("for input %q: expected %d, got %d", tc.input, tc.expected, got)
				}
			}
		})
	}
}

func TestLoadEnv(t *testing.T) {
	// Create a temporary file
	tmpFile := filepath.Join(t.TempDir(), ".env")
	content := `
# This is a comment
GOPHRDRV_TEST_PORT=9090
GOPHRDRV_TEST_HOST="127.0.0.1" # inline comment
GOPHRDRV_TEST_MAX_UPLOAD='500MB'
`
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write temp env file: %v", err)
	}

	// Set an environment variable beforehand to test that it is not overwritten
	os.Setenv("GOPHRDRV_TEST_PORT", "9999")
	defer os.Unsetenv("GOPHRDRV_TEST_PORT")
	defer os.Unsetenv("GOPHRDRV_TEST_HOST")
	defer os.Unsetenv("GOPHRDRV_TEST_MAX_UPLOAD")

	if err := LoadEnv(tmpFile); err != nil {
		t.Fatalf("LoadEnv failed: %v", err)
	}

	if os.Getenv("GOPHRDRV_TEST_PORT") != "9999" {
		t.Errorf("expected GOPHRDRV_TEST_PORT to remain 9999, got %q", os.Getenv("GOPHRDRV_TEST_PORT"))
	}
	if os.Getenv("GOPHRDRV_TEST_HOST") != "127.0.0.1" {
		t.Errorf("expected GOPHRDRV_TEST_HOST to be 127.0.0.1, got %q", os.Getenv("GOPHRDRV_TEST_HOST"))
	}
	if os.Getenv("GOPHRDRV_TEST_MAX_UPLOAD") != "500MB" {
		t.Errorf("expected GOPHRDRV_TEST_MAX_UPLOAD to be 500MB, got %q", os.Getenv("GOPHRDRV_TEST_MAX_UPLOAD"))
	}
}
