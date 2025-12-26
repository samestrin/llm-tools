package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSummarizeDirCommand(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test structure
	os.Mkdir(filepath.Join(tmpDir, "src"), 0755)
	os.WriteFile(filepath.Join(tmpDir, "README.md"), []byte("# Project\n\nDescription here."), 0644)
	os.WriteFile(filepath.Join(tmpDir, "src", "main.go"), []byte("package main\n\nfunc main() {\n\t// code\n}"), 0644)

	tests := []struct {
		name     string
		args     []string
		expected []string
		hasError bool
	}{
		{
			name:     "summarize directory",
			args:     []string{"--path", tmpDir, "--no-gitignore"},
			expected: []string{"README.md", "src"},
		},
		{
			name:     "with format outline",
			args:     []string{"--path", tmpDir, "--format", "outline", "--no-gitignore"},
			expected: []string{"README.md"},
		},
		{
			name:     "non-existent path",
			args:     []string{"--path", "/nonexistent/path"},
			hasError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := newSummarizeDirCmd()
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
