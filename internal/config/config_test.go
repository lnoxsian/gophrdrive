package config

import (
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
