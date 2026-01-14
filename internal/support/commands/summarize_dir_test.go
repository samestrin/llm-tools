package commands

import (
	"bytes"
	"encoding/json"
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

func TestSummarizeDirHeadersFormat(t *testing.T) {
	tmpDir := t.TempDir()

	// Create markdown files with various header levels
	mdContent1 := `# Main Title
Some intro text.

## Section One
Content here.

### Subsection
More content.

## Section Two
Final content.
`
	mdContent2 := `# Another Doc
## Overview
### Details
`
	// Create a non-markdown file (should be skipped)
	goContent := `package main

func main() {
	// This has # but should not be parsed
}
`

	os.WriteFile(filepath.Join(tmpDir, "doc1.md"), []byte(mdContent1), 0644)
	os.WriteFile(filepath.Join(tmpDir, "doc2.md"), []byte(mdContent2), 0644)
	os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(goContent), 0644)

	t.Run("basic headers extraction", func(t *testing.T) {
		cmd := newSummarizeDirCmd()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetArgs([]string{"--path", tmpDir, "--format", "headers", "--no-gitignore"})

		err := cmd.Execute()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		output := buf.String()

		// Should contain headers from markdown files
		expectedStrings := []string{
			"doc1.md",
			"# Main Title",
			"## Section One",
			"### Subsection",
			"## Section Two",
			"doc2.md",
			"# Another Doc",
			"FORMAT: headers",
		}
		for _, exp := range expectedStrings {
			if !strings.Contains(output, exp) {
				t.Errorf("output should contain %q\nGot: %s", exp, output)
			}
		}

		// Should NOT contain go file or its content
		if strings.Contains(output, "main.go") {
			t.Errorf("output should not contain main.go (non-markdown file)")
		}
	})

	t.Run("headers JSON output", func(t *testing.T) {
		cmd := newSummarizeDirCmd()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetArgs([]string{"--path", tmpDir, "--format", "headers", "--json", "--no-gitignore"})

		err := cmd.Execute()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var result SummarizeDirHeadersResult
		if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
			t.Fatalf("failed to parse JSON: %v\nOutput: %s", err, buf.String())
		}

		if result.Format != "headers" {
			t.Errorf("expected format 'headers', got %q", result.Format)
		}
		if result.FileCount != 2 {
			t.Errorf("expected 2 files, got %d", result.FileCount)
		}

		// Check that headers are properly structured
		foundMainTitle := false
		for _, f := range result.Files {
			for _, h := range f.Headers {
				if h.Text == "Main Title" && h.Level == 1 {
					foundMainTitle = true
				}
			}
		}
		if !foundMainTitle {
			t.Error("expected to find 'Main Title' header at level 1")
		}
	})

	t.Run("headers minimal output", func(t *testing.T) {
		cmd := newSummarizeDirCmd()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetArgs([]string{"--path", tmpDir, "--format", "headers", "--json", "--min", "--no-gitignore"})

		err := cmd.Execute()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var result SummarizeDirHeadersResult
		if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
			t.Fatalf("failed to parse JSON: %v", err)
		}

		// Minimal format should use short keys
		if result.Fmt != "headers" {
			t.Errorf("expected fmt 'headers', got %q", result.Fmt)
		}
		if result.FC == nil || *result.FC != 2 {
			t.Errorf("expected fc=2")
		}

		// Check short key fields in files
		if len(result.F) != 2 {
			t.Errorf("expected 2 files in F, got %d", len(result.F))
		}
	})

	t.Run("empty directory", func(t *testing.T) {
		emptyDir := t.TempDir()
		cmd := newSummarizeDirCmd()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetArgs([]string{"--path", emptyDir, "--format", "headers", "--no-gitignore"})

		err := cmd.Execute()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		output := buf.String()
		if !strings.Contains(output, "0 files with headers") {
			t.Errorf("expected '0 files with headers' in output\nGot: %s", output)
		}
	})

	t.Run("markdown file without headers", func(t *testing.T) {
		noHeaderDir := t.TempDir()
		os.WriteFile(filepath.Join(noHeaderDir, "plain.md"), []byte("Just plain text\nNo headers here."), 0644)

		cmd := newSummarizeDirCmd()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetArgs([]string{"--path", noHeaderDir, "--format", "headers", "--no-gitignore"})

		err := cmd.Execute()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		output := buf.String()
		// File with no headers should not appear in output
		if strings.Contains(output, "plain.md") {
			t.Errorf("file without headers should not appear in output")
		}
		if !strings.Contains(output, "0 files with headers") {
			t.Errorf("expected '0 files with headers' in output\nGot: %s", output)
		}
	})
}

func TestSummarizeDirHeadersHierarchy(t *testing.T) {
	tmpDir := t.TempDir()

	// Test that header indentation is correct
	mdContent := `# H1
## H2
### H3
#### H4
##### H5
###### H6
`
	os.WriteFile(filepath.Join(tmpDir, "levels.md"), []byte(mdContent), 0644)

	cmd := newSummarizeDirCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--path", tmpDir, "--format", "headers", "--no-gitignore"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()

	// Verify all header levels are captured
	levels := []string{"# H1", "## H2", "### H3", "#### H4", "##### H5", "###### H6"}
	for _, level := range levels {
		if !strings.Contains(output, level) {
			t.Errorf("output should contain %q\nGot: %s", level, output)
		}
	}
}
