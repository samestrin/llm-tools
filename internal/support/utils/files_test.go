package utils

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsValidFilePath(t *testing.T) {
	// Create temp file
	tmpFile, err := os.CreateTemp("", "test-*.txt")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	// Create temp directory
	tmpDir := t.TempDir()

	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{"existing file", tmpFile.Name(), true},
		{"existing directory", tmpDir, false},
		{"non-existent path", "/nonexistent/path/file.txt", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsValidFilePath(tt.path)
			if result != tt.expected {
				t.Errorf("IsValidFilePath(%q) = %v, want %v", tt.path, result, tt.expected)
			}
		})
	}
}

func TestIsValidDirPath(t *testing.T) {
	// Create temp file
	tmpFile, err := os.CreateTemp("", "test-*.txt")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	// Create temp directory
	tmpDir := t.TempDir()

	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{"existing directory", tmpDir, true},
		{"existing file", tmpFile.Name(), false},
		{"non-existent path", "/nonexistent/path/dir", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsValidDirPath(tt.path)
			if result != tt.expected {
				t.Errorf("IsValidDirPath(%q) = %v, want %v", tt.path, result, tt.expected)
			}
		})
	}
}

func TestExpandPath(t *testing.T) {
	home, _ := os.UserHomeDir()

	tests := []struct {
		name        string
		input       string
		shouldStart string
	}{
		{"home expansion", "~/test", filepath.Join(home, "test")},
		{"absolute path", "/absolute/path", "/absolute/path"},
		{"relative path", "relative/path", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ExpandPath(tt.input)
			if err != nil {
				t.Errorf("ExpandPath(%q) returned error: %v", tt.input, err)
				return
			}
			if tt.shouldStart != "" && result != tt.shouldStart {
				t.Errorf("ExpandPath(%q) = %q, want %q", tt.input, result, tt.shouldStart)
			}
		})
	}
}
