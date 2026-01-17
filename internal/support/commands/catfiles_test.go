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

// TestCatfilesJSONOutput tests JSON output mode
func TestCatfilesJSONOutput(t *testing.T) {
	tmpDir := t.TempDir()
	file := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(file, []byte("test content"), 0644)

	cmd := newCatfilesCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{file, "--json"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	// JSON output should be valid JSON
	if !strings.Contains(output, "{") {
		t.Errorf("JSON output should contain JSON structure, got: %s", output)
	}
}

// TestCatfilesMinimalOutput tests minimal output mode
func TestCatfilesMinimalOutput(t *testing.T) {
	tmpDir := t.TempDir()
	file := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(file, []byte("test content"), 0644)

	cmd := newCatfilesCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{file, "--min"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	// Minimal output should contain the content
	if !strings.Contains(output, "test content") {
		t.Errorf("minimal output should contain content, got: %s", output)
	}
}

// TestCatfilesRecursiveDirectory tests recursive directory scanning
func TestCatfilesRecursiveDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "subdir")
	os.MkdirAll(subDir, 0755)

	os.WriteFile(filepath.Join(tmpDir, "root.txt"), []byte("root content"), 0644)
	os.WriteFile(filepath.Join(subDir, "sub.txt"), []byte("sub content"), 0644)

	cmd := newCatfilesCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{tmpDir, "--no-gitignore"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	// Should find files in both directories
	if !strings.Contains(output, "root content") || !strings.Contains(output, "sub content") {
		t.Errorf("should find files in subdirectories, got: %s", output)
	}
}

// TestCatfilesEmptyArgs tests error when no arguments provided
func TestCatfilesEmptyArgs(t *testing.T) {
	cmd := newCatfilesCmd()
	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(errBuf)
	cmd.SetArgs([]string{})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for no arguments")
	}
}
