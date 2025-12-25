package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCatfilesCommand(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test files
	file1 := filepath.Join(tmpDir, "file1.txt")
	file2 := filepath.Join(tmpDir, "file2.txt")
	os.WriteFile(file1, []byte("content of file 1"), 0644)
	os.WriteFile(file2, []byte("content of file 2"), 0644)

	tests := []struct {
		name     string
		args     []string
		expected []string
		hasError bool
	}{
		{
			name:     "single file",
			args:     []string{file1},
			expected: []string{"FILE:", "content of file 1"},
		},
		{
			name:     "multiple files",
			args:     []string{file1, file2},
			expected: []string{"file1.txt", "file2.txt", "content of file 1", "content of file 2"},
		},
		{
			name:     "directory",
			args:     []string{tmpDir, "--no-gitignore"},
			expected: []string{"file1.txt", "file2.txt"},
		},
		{
			name:     "non-existent file",
			args:     []string{"/nonexistent/file.txt"},
			hasError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := newCatfilesCmd()
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
