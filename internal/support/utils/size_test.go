package utils

import (
	"testing"
)

func TestFormatSize(t *testing.T) {
	tests := []struct {
		name     string
		bytes    int64
		expected string
	}{
		{"bytes", 500, "500 B"},
		{"kilobytes", 1024, "1.0 KB"},
		{"megabytes", 1024 * 1024, "1.0 MB"},
		{"gigabytes", 1024 * 1024 * 1024, "1.0 GB"},
		{"1.5 KB", 1536, "1.5 KB"},
		{"zero", 0, "0 B"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatSize(tt.bytes)
			if result != tt.expected {
				t.Errorf("FormatSize(%d) = %q, want %q", tt.bytes, result, tt.expected)
			}
		})
	}
}

func TestParseSize(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int64
		hasError bool
	}{
		{"bytes", "500 B", 500, false},
		{"kilobytes", "1 KB", 1024, false},
		{"megabytes", "1 MB", 1024 * 1024, false},
		{"gigabytes", "1 GB", 1024 * 1024 * 1024, false},
		{"no space", "1KB", 1024, false},
		{"just number", "1024", 1024, false},
		{"lowercase", "1 kb", 1024, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseSize(tt.input)
			if tt.hasError {
				if err == nil {
					t.Errorf("ParseSize(%q) should return error", tt.input)
				}
				return
			}
			if err != nil {
				t.Errorf("ParseSize(%q) returned error: %v", tt.input, err)
				return
			}
			if result != tt.expected {
				t.Errorf("ParseSize(%q) = %d, want %d", tt.input, result, tt.expected)
			}
		})
	}
}
