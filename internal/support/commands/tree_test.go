package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTreeCommand(t *testing.T) {
	tmpDir := t.TempDir()

	// Create directory structure
	os.Mkdir(filepath.Join(tmpDir, "subdir"), 0755)
	os.WriteFile(filepath.Join(tmpDir, "file1.txt"), []byte("content"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "subdir", "nested.txt"), []byte("nested"), 0644)

	tests := []struct {
		name     string
		args     []string
		expected []string
		hasError bool
	}{
		{
			name:     "basic tree",
			args:     []string{tmpDir, "--no-gitignore"},
			expected: []string{"file1.txt", "subdir/"},
		},
		{
			name:     "tree with sizes",
			args:     []string{tmpDir, "--sizes", "--no-gitignore"},
			expected: []string{"file1.txt"},
		},
		{
			name:     "tree with depth limit",
			args:     []string{tmpDir, "--depth", "1", "--no-gitignore"},
			expected: []string{"subdir/"},
		},
		{
			name:     "non-existent path",
			args:     []string{"/nonexistent/path"},
			hasError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := newTreeCmd()
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
