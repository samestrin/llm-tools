package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCountCommand(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test markdown file with checkboxes
	mdContent := `# Test
- [x] Done task
- [ ] Pending task
- [x] Another done
`
	mdFile := filepath.Join(tmpDir, "test.md")
	os.WriteFile(mdFile, []byte(mdContent), 0644)

	// Create test files for line counting
	txtFile := filepath.Join(tmpDir, "lines.txt")
	os.WriteFile(txtFile, []byte("line1\nline2\nline3\n"), 0644)

	tests := []struct {
		name     string
		args     []string
		expected []string
		hasError bool
	}{
		{
			name:     "count checkboxes in file",
			args:     []string{"--path", mdFile, "--mode", "checkboxes"},
			expected: []string{"COUNT: 3", "CHECKED: 2", "UNCHECKED: 1"},
		},
		{
			name:     "count lines in file",
			args:     []string{"--path", txtFile, "--mode", "lines"},
			expected: []string{"COUNT: 3"},
		},
		{
			name:     "count files in directory",
			args:     []string{"--path", tmpDir, "--mode", "files"},
			expected: []string{"COUNT: 2"},
		},
		{
			name:     "count files recursive",
			args:     []string{"--path", tmpDir, "--mode", "files", "-r"},
			expected: []string{"COUNT:"},
		},
		{
			name:     "non-existent path",
			args:     []string{"--path", "/nonexistent/path", "--mode", "checkboxes"},
			hasError: true,
		},
		{
			name:     "invalid mode",
			args:     []string{"--path", tmpDir, "--mode", "invalid"},
			hasError: true,
		},
		{
			name:     "legacy checkboxes flag",
			args:     []string{"--path", mdFile, "--checkboxes"},
			expected: []string{"COUNT: 3", "CHECKED: 2", "UNCHECKED: 1"},
		},
		{
			name:     "legacy lines flag",
			args:     []string{"--path", txtFile, "--lines"},
			expected: []string{"COUNT: 3"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := newCountCmd()
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

func TestCountCheckboxesRecursive(t *testing.T) {
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "subdir")
	os.Mkdir(subDir, 0755)

	// Create markdown files in both directories
	os.WriteFile(filepath.Join(tmpDir, "root.md"), []byte("- [x] Root task\n"), 0644)
	os.WriteFile(filepath.Join(subDir, "sub.md"), []byte("- [ ] Sub task\n- [x] Done\n"), 0644)

	cmd := newCountCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--path", tmpDir, "--mode", "checkboxes", "-r"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "COUNT: 3") {
		t.Errorf("output %q should contain COUNT: 3", output)
	}
	if !strings.Contains(output, "CHECKED: 2") {
		t.Errorf("output %q should contain CHECKED: 2", output)
	}
}

func TestCountJSONOutput(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test markdown file with checkboxes
	mdContent := `# Test
- [x] Done task
- [ ] Pending task
- [x] Another done
`
	mdFile := filepath.Join(tmpDir, "test.md")
	os.WriteFile(mdFile, []byte(mdContent), 0644)

	cmd := newCountCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--path", mdFile, "--mode", "checkboxes", "--json"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	// JSON output should contain these keys
	expectedKeys := []string{`"count"`, `"checked"`, `"unchecked"`, `"percent"`}
	for _, key := range expectedKeys {
		if !strings.Contains(output, key) {
			t.Errorf("JSON output %q should contain key %s", output, key)
		}
	}
}

func TestCountMinimalOutput(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test markdown file with checkboxes
	mdContent := `# Test
- [x] Done task
- [ ] Pending task
- [x] Another done
`
	mdFile := filepath.Join(tmpDir, "test.md")
	os.WriteFile(mdFile, []byte(mdContent), 0644)

	cmd := newCountCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--path", mdFile, "--mode", "checkboxes", "--min"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	// Minimal output should be more compact but still have key info
	if !strings.Contains(output, "3") || !strings.Contains(output, "2") {
		t.Errorf("minimal output should contain counts, got: %q", output)
	}
}

func TestCountMinimalJSONOutput(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test markdown file with checkboxes
	mdContent := `# Test
- [x] Done task
- [ ] Pending task
- [x] Another done
`
	mdFile := filepath.Join(tmpDir, "test.md")
	os.WriteFile(mdFile, []byte(mdContent), 0644)

	cmd := newCountCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--path", mdFile, "--mode", "checkboxes", "--json", "--min"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	// Minimal JSON should use "count" (not abbreviated, for consistency with other tools)
	if !strings.Contains(output, `"count"`) {
		t.Errorf("minimal JSON output should use 'count' key, got: %q", output)
	}
	// Verify it's valid JSON with expected values
	if !strings.Contains(output, `"checked":2`) {
		t.Errorf("minimal JSON output should contain checked count, got: %q", output)
	}
}

// TestCountLinesInDirectory tests line counting in a directory
func TestCountLinesInDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	// Create files with different line counts
	os.WriteFile(filepath.Join(tmpDir, "a.txt"), []byte("line1\nline2\n"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "b.txt"), []byte("line1\nline2\nline3\n"), 0644)

	cmd := newCountCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--path", tmpDir, "--mode", "lines"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	// 2 + 3 = 5 lines total
	if !strings.Contains(output, "COUNT: 5") {
		t.Errorf("output %q should contain COUNT: 5", output)
	}
}

// TestCountLinesRecursive tests recursive line counting in directories
func TestCountLinesRecursive(t *testing.T) {
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "sub")
	os.Mkdir(subDir, 0755)

	// Create files with different line counts
	os.WriteFile(filepath.Join(tmpDir, "root.txt"), []byte("line1\nline2\n"), 0644)
	os.WriteFile(filepath.Join(subDir, "sub.txt"), []byte("line1\nline2\nline3\n"), 0644)

	cmd := newCountCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--path", tmpDir, "--mode", "lines", "-r"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	// 2 + 3 = 5 lines total
	if !strings.Contains(output, "COUNT: 5") {
		t.Errorf("output %q should contain COUNT: 5", output)
	}
}

// TestCountLinesFileWithoutTrailingNewline tests files without trailing newline
func TestCountLinesFileWithoutTrailingNewline(t *testing.T) {
	tmpDir := t.TempDir()

	// Create file without trailing newline
	os.WriteFile(filepath.Join(tmpDir, "no_newline.txt"), []byte("line1\nline2"), 0644)

	cmd := newCountCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--path", filepath.Join(tmpDir, "no_newline.txt"), "--mode", "lines"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	// Should count 2 lines even without trailing newline
	if !strings.Contains(output, "COUNT: 2") {
		t.Errorf("output %q should contain COUNT: 2", output)
	}
}

// TestCountLinesHumanReadableOutput tests human-readable output for line counting
func TestCountLinesHumanReadableOutput(t *testing.T) {
	tmpDir := t.TempDir()

	os.WriteFile(filepath.Join(tmpDir, "test.txt"), []byte("line1\nline2\nline3\n"), 0644)

	cmd := newCountCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	// Neither --json nor --min means human-readable
	cmd.SetArgs([]string{"--path", filepath.Join(tmpDir, "test.txt"), "--mode", "lines"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	// Human-readable should have COUNT:
	if !strings.Contains(output, "COUNT:") {
		t.Errorf("output %q should contain COUNT:", output)
	}
}

// TestCountFilesWithPattern tests file counting with glob pattern
func TestCountFilesWithPattern(t *testing.T) {
	tmpDir := t.TempDir()

	// Create files of different types
	os.WriteFile(filepath.Join(tmpDir, "a.txt"), []byte("content"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "b.txt"), []byte("content"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "c.md"), []byte("content"), 0644)

	cmd := newCountCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--path", tmpDir, "--mode", "files", "--pattern", "*.txt"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	// Should count only .txt files
	if !strings.Contains(output, "COUNT: 2") {
		t.Errorf("output %q should contain COUNT: 2", output)
	}
}
