package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestListdirCommand(t *testing.T) {
	// Create temp directory structure for testing
	tmpDir := t.TempDir()

	// Create some files and directories
	os.WriteFile(filepath.Join(tmpDir, "file1.txt"), []byte("content"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "file2.go"), []byte("package main"), 0644)
	os.Mkdir(filepath.Join(tmpDir, "subdir"), 0755)
	os.WriteFile(filepath.Join(tmpDir, "subdir", "nested.txt"), []byte("nested"), 0644)

	tests := []struct {
		name     string
		args     []string
		expected []string
		hasError bool
	}{
		{
			name:     "list directory",
			args:     []string{tmpDir},
			expected: []string{"file1.txt", "file2.go", "subdir"},
		},
		{
			name:     "list with sizes",
			args:     []string{tmpDir, "--sizes"},
			expected: []string{"file1.txt", "7 B"},
		},
		{
			name:     "list with dates",
			args:     []string{tmpDir, "--dates"},
			expected: []string{"file1.txt"},
		},
		{
			name:     "non-existent path",
			args:     []string{"/nonexistent/path"},
			hasError: true,
		},
		{
			name:     "file instead of directory",
			args:     []string{filepath.Join(tmpDir, "file1.txt")},
			hasError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := newListdirCmd()
			buf := new(bytes.Buffer)
			errBuf := new(bytes.Buffer)
			cmd.SetOut(buf)
			cmd.SetErr(errBuf)
			cmd.SetArgs(tt.args)

			err := cmd.Execute()

			if tt.hasError {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			output := buf.String()
			for _, exp := range tt.expected {
				if !strings.Contains(output, exp) {
					t.Errorf("output %q should contain %q", output, exp)
				}
			}
		})
	}
}

func TestListdirEmptyDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	cmd := newListdirCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{tmpDir, "--no-gitignore"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "EMPTY_DIRECTORY") {
		t.Errorf("output %q should contain EMPTY_DIRECTORY", output)
	}
}
