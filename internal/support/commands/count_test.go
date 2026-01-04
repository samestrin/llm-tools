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
	// Minimal JSON should use abbreviated keys (count -> c per KeyAbbreviations)
	if !strings.Contains(output, `"c"`) {
		t.Errorf("minimal JSON output should use abbreviated key 'c' for count, got: %q", output)
	}
	// Verify it's valid JSON with expected values
	if !strings.Contains(output, `"checked":2`) && !strings.Contains(output, `"chk":2`) {
		t.Errorf("minimal JSON output should contain checked count, got: %q", output)
	}
}
