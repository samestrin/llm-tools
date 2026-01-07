package commands

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// resetTreeFlags resets all tree command flags to defaults
func resetTreeFlags() {
	treeDepth = 999
	treeSizes = false
	treeNoGitignore = false
	treeNoDefaultExcludes = false
	treePath = "."
	treeJSON = false
	treeMinimal = false
	treeMaxEntries = 500
	treeExcludePatterns = nil
}

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
			args:     []string{"--path", tmpDir, "--no-gitignore"},
			expected: []string{"file1.txt", "subdir/"},
		},
		{
			name:     "tree with sizes",
			args:     []string{"--path", tmpDir, "--sizes", "--no-gitignore"},
			expected: []string{"file1.txt"},
		},
		{
			name:     "tree with depth limit",
			args:     []string{"--path", tmpDir, "--depth", "1", "--no-gitignore"},
			expected: []string{"subdir/"},
		},
		{
			name:     "non-existent path",
			args:     []string{"--path", "/nonexistent/path"},
			hasError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetTreeFlags()
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

func TestTreeMaxEntries(t *testing.T) {
	tmpDir := t.TempDir()

	// Create many files
	for i := 0; i < 20; i++ {
		os.WriteFile(filepath.Join(tmpDir, "file"+string(rune('a'+i))+".txt"), []byte("content"), 0644)
	}

	t.Run("truncates at max entries", func(t *testing.T) {
		resetTreeFlags()
		cmd := newTreeCmd()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetArgs([]string{"--path", tmpDir, "--max-entries", "5", "--no-gitignore", "--no-default-excludes"})

		err := cmd.Execute()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		output := buf.String()
		if !strings.Contains(output, "truncated") {
			t.Error("expected truncation message")
		}
	})

	t.Run("json output includes truncated flag", func(t *testing.T) {
		resetTreeFlags()
		cmd := newTreeCmd()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetArgs([]string{"--path", tmpDir, "--max-entries", "5", "--no-gitignore", "--no-default-excludes", "--json"})

		err := cmd.Execute()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var result TreeResult
		if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
			t.Fatalf("failed to parse JSON: %v", err)
		}

		if !result.Truncated {
			t.Error("expected Truncated to be true")
		}
		if result.Total != 5 {
			t.Errorf("expected Total 5, got %d", result.Total)
		}
	})

	t.Run("unlimited when max-entries is 0", func(t *testing.T) {
		resetTreeFlags()
		cmd := newTreeCmd()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetArgs([]string{"--path", tmpDir, "--max-entries", "0", "--no-gitignore", "--no-default-excludes"})

		err := cmd.Execute()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		output := buf.String()
		if strings.Contains(output, "truncated") {
			t.Error("should not truncate when max-entries is 0")
		}
	})
}

func TestTreeDefaultExcludes(t *testing.T) {
	tmpDir := t.TempDir()

	// Create directories that should be excluded by default
	os.Mkdir(filepath.Join(tmpDir, "node_modules"), 0755)
	os.Mkdir(filepath.Join(tmpDir, "vendor"), 0755)
	os.Mkdir(filepath.Join(tmpDir, "__pycache__"), 0755)
	os.Mkdir(filepath.Join(tmpDir, "src"), 0755)
	os.WriteFile(filepath.Join(tmpDir, "node_modules", "package.json"), []byte("{}"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "src", "main.go"), []byte("package main"), 0644)

	t.Run("excludes default directories", func(t *testing.T) {
		resetTreeFlags()
		cmd := newTreeCmd()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetArgs([]string{"--path", tmpDir, "--no-gitignore"})

		err := cmd.Execute()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		output := buf.String()
		if strings.Contains(output, "node_modules") {
			t.Error("should exclude node_modules by default")
		}
		if strings.Contains(output, "vendor") {
			t.Error("should exclude vendor by default")
		}
		if strings.Contains(output, "__pycache__") {
			t.Error("should exclude __pycache__ by default")
		}
		if !strings.Contains(output, "src") {
			t.Error("should include src")
		}
	})

	t.Run("includes defaults with --no-default-excludes", func(t *testing.T) {
		resetTreeFlags()
		cmd := newTreeCmd()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetArgs([]string{"--path", tmpDir, "--no-gitignore", "--no-default-excludes"})

		err := cmd.Execute()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		output := buf.String()
		if !strings.Contains(output, "node_modules") {
			t.Error("should include node_modules with --no-default-excludes")
		}
		if !strings.Contains(output, "vendor") {
			t.Error("should include vendor with --no-default-excludes")
		}
	})
}

func TestTreeExcludePatterns(t *testing.T) {
	tmpDir := t.TempDir()

	os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte("package main"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "main_test.go"), []byte("package main"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "util.go"), []byte("package main"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "util_test.go"), []byte("package main"), 0644)

	t.Run("excludes by pattern", func(t *testing.T) {
		resetTreeFlags()
		cmd := newTreeCmd()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetArgs([]string{"--path", tmpDir, "--exclude", "_test\\.go$", "--no-gitignore"})

		err := cmd.Execute()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		output := buf.String()
		if strings.Contains(output, "_test.go") {
			t.Error("should exclude _test.go files")
		}
		if !strings.Contains(output, "main.go") {
			t.Error("should include main.go")
		}
		if !strings.Contains(output, "util.go") {
			t.Error("should include util.go")
		}
	})

	t.Run("multiple exclude patterns", func(t *testing.T) {
		resetTreeFlags()
		cmd := newTreeCmd()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetArgs([]string{"--path", tmpDir, "--exclude", "_test\\.go$", "--exclude", "^util", "--no-gitignore"})

		err := cmd.Execute()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		output := buf.String()
		if strings.Contains(output, "_test.go") {
			t.Error("should exclude _test.go files")
		}
		if strings.Contains(output, "util") {
			t.Error("should exclude util files")
		}
		if !strings.Contains(output, "main.go") {
			t.Error("should include main.go")
		}
	})

	t.Run("invalid regex returns error", func(t *testing.T) {
		resetTreeFlags()
		cmd := newTreeCmd()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetArgs([]string{"--path", tmpDir, "--exclude", "[invalid"})

		err := cmd.Execute()
		if err == nil {
			t.Error("expected error for invalid regex")
		}
	})
}

func TestTreeBuilderHelpers(t *testing.T) {
	t.Run("isDefaultExcluded", func(t *testing.T) {
		b := &treeBuilder{}

		if !b.isDefaultExcluded("node_modules") {
			t.Error("node_modules should be excluded")
		}
		if !b.isDefaultExcluded("vendor") {
			t.Error("vendor should be excluded")
		}
		if b.isDefaultExcluded("src") {
			t.Error("src should not be excluded")
		}
	})
}
