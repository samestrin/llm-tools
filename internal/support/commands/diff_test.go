package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiffCommand(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test files
	file1 := filepath.Join(tmpDir, "file1.txt")
	file2 := filepath.Join(tmpDir, "file2.txt")
	file3 := filepath.Join(tmpDir, "file3.txt")

	os.WriteFile(file1, []byte("line1\nline2\nline3\n"), 0644)
	os.WriteFile(file2, []byte("line1\nmodified\nline3\n"), 0644)
	os.WriteFile(file3, []byte("line1\nline2\nline3\n"), 0644)

	tests := []struct {
		name     string
		args     []string
		expected []string
		hasError bool
	}{
		{
			name:     "files with differences",
			args:     []string{file1, file2},
			expected: []string{"-", "+"}, // Has additions and deletions
		},
		{
			name:     "identical files",
			args:     []string{file1, file3},
			expected: []string{"IDENTICAL"},
		},
		{
			name:     "unified format",
			args:     []string{file1, file2, "-u"},
			expected: []string{"---", "+++"}, // Unified format headers
		},
		{
			name:     "non-existent file",
			args:     []string{file1, "/nonexistent"},
			hasError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := newDiffCmd()
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
