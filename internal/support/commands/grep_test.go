package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGrepCommand(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test files
	file1 := filepath.Join(tmpDir, "file1.txt")
	file2 := filepath.Join(tmpDir, "file2.txt")
	os.WriteFile(file1, []byte("hello world\nfoo bar"), 0644)
	os.WriteFile(file2, []byte("another line\nhello again"), 0644)

	tests := []struct {
		name     string
		args     []string
		expected []string
		hasError bool
	}{
		{
			name:     "basic grep",
			args:     []string{"hello", file1},
			expected: []string{"hello world"},
		},
		{
			name:     "grep with line numbers",
			args:     []string{"hello", file1, "-n"},
			expected: []string{"1:"},
		},
		{
			name:     "case insensitive",
			args:     []string{"HELLO", file1, "-i"},
			expected: []string{"hello world"},
		},
		{
			name:     "files only mode",
			args:     []string{"hello", tmpDir, "-l"},
			expected: []string{"file1.txt", "file2.txt"},
		},
		{
			name:     "no matches",
			args:     []string{"nonexistent", file1},
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := newGrepCmd()
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

// TestGrepJSONOutput tests JSON output mode
func TestGrepJSONOutput(t *testing.T) {
	tmpDir := t.TempDir()
	file := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(file, []byte("hello world\nfoo bar"), 0644)

	cmd := newGrepCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"hello", file, "--json"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	// JSON output should have proper structure
	if !strings.Contains(output, "{") {
		t.Errorf("JSON output should contain JSON structure, got: %s", output)
	}
}

// TestGrepMinimalOutput tests minimal output mode
func TestGrepMinimalOutput(t *testing.T) {
	tmpDir := t.TempDir()
	file := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(file, []byte("hello world\nfoo bar"), 0644)

	cmd := newGrepCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"hello", file, "--min"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	// Minimal output should contain the match
	if !strings.Contains(output, "hello") {
		t.Errorf("minimal output should contain 'hello', got: %s", output)
	}
}

// TestGrepDirectory tests grepping in a directory
func TestGrepDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "subdir")
	os.MkdirAll(subDir, 0755)

	os.WriteFile(filepath.Join(tmpDir, "root.txt"), []byte("root hello"), 0644)
	os.WriteFile(filepath.Join(subDir, "sub.txt"), []byte("sub hello"), 0644)

	cmd := newGrepCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"hello", tmpDir})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	// Should find matches in both files
	if !strings.Contains(output, "root") || !strings.Contains(output, "sub") {
		t.Errorf("should find matches in both files, got: %s", output)
	}
}

// TestGrepInvalidPattern tests error handling for invalid regex
func TestGrepInvalidPattern(t *testing.T) {
	tmpDir := t.TempDir()
	file := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(file, []byte("hello world"), 0644)

	cmd := newGrepCmd()
	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(errBuf)
	cmd.SetArgs([]string{"[invalid", file})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for invalid regex pattern")
	}
}
