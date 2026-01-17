package commands

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestParseStreamPipeDelimited tests pipe-delimited parsing
func TestParseStreamPipeDelimited(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test fixture with pipe-delimited data
	pipeData := `ID|CATEGORY|EST_MINUTES|DESCRIPTION
TD-001|performance|30|Optimize database queries
TD-002|security|120|Add input validation
TD-003|refactoring|15|Extract helper function
`
	pipeFile := filepath.Join(tmpDir, "td_stream.txt")
	os.WriteFile(pipeFile, []byte(pipeData), 0644)

	tests := []struct {
		name           string
		args           []string
		expectedFormat string
		expectedRows   int
		expectedCols   []string
		hasError       bool
	}{
		{
			name:           "parse pipe-delimited with auto-detect",
			args:           []string{"--file", pipeFile, "--format", "auto"},
			expectedFormat: "pipe",
			expectedRows:   3,
			expectedCols:   []string{"ID", "CATEGORY", "EST_MINUTES", "DESCRIPTION"},
		},
		{
			name:           "parse pipe-delimited explicit format",
			args:           []string{"--file", pipeFile, "--format", "pipe"},
			expectedFormat: "pipe",
			expectedRows:   3,
			expectedCols:   []string{"ID", "CATEGORY", "EST_MINUTES", "DESCRIPTION"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := newParseStreamCmd()
			buf := new(bytes.Buffer)
			cmd.SetOut(buf)
			cmd.SetArgs(append(tt.args, "--json"))

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

			var result ParseStreamResult
			if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
				t.Fatalf("failed to parse JSON output: %v\nOutput: %s", err, buf.String())
			}

			if result.Format != tt.expectedFormat {
				t.Errorf("format = %q, want %q", result.Format, tt.expectedFormat)
			}

			if result.RowCount != tt.expectedRows {
				t.Errorf("row_count = %d, want %d", result.RowCount, tt.expectedRows)
			}

			if len(result.Headers) != len(tt.expectedCols) {
				t.Errorf("headers length = %d, want %d", len(result.Headers), len(tt.expectedCols))
			}
		})
	}
}

// TestParseStreamCustomDelimiter tests custom delimiter support
func TestParseStreamCustomDelimiter(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test fixture with semicolon delimiter
	semiData := `ID;CATEGORY;EST_MINUTES
TD-001;performance;30
TD-002;security;120
`
	semiFile := filepath.Join(tmpDir, "semi_delim.txt")
	os.WriteFile(semiFile, []byte(semiData), 0644)

	cmd := newParseStreamCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--file", semiFile, "--delimiter", ";", "--json"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result ParseStreamResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	if result.Delimiter != ";" {
		t.Errorf("delimiter = %q, want %q", result.Delimiter, ";")
	}

	if result.RowCount != 2 {
		t.Errorf("row_count = %d, want %d", result.RowCount, 2)
	}
}

// TestParseStreamExplicitHeaders tests providing headers explicitly
func TestParseStreamExplicitHeaders(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test fixture without header row
	noHeaderData := `TD-001|performance|30
TD-002|security|120
`
	noHeaderFile := filepath.Join(tmpDir, "no_header.txt")
	os.WriteFile(noHeaderFile, []byte(noHeaderData), 0644)

	cmd := newParseStreamCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--file", noHeaderFile, "--headers", "ID,CATEGORY,MINUTES", "--json"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result ParseStreamResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	if len(result.Headers) != 3 {
		t.Errorf("headers length = %d, want 3", len(result.Headers))
	}

	// With explicit headers, both lines should be data rows
	if result.RowCount != 2 {
		t.Errorf("row_count = %d, want 2", result.RowCount)
	}
}

// TestParseStreamMarkdownChecklist tests markdown checklist parsing
func TestParseStreamMarkdownChecklist(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test fixture with markdown checklists
	checklistData := `# Tasks
- [x] Completed task one
- [ ] Pending task two
- [X] Another completed task
  - [ ] Nested sub-task
`
	checklistFile := filepath.Join(tmpDir, "checklist.md")
	os.WriteFile(checklistFile, []byte(checklistData), 0644)

	cmd := newParseStreamCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--file", checklistFile, "--format", "markdown-checklist", "--json"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result ParseStreamResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	if result.Format != "markdown-checklist" {
		t.Errorf("format = %q, want %q", result.Format, "markdown-checklist")
	}

	// Should find 4 checklist items
	if result.RowCount != 4 {
		t.Errorf("row_count = %d, want 4", result.RowCount)
	}

	// Verify first row has correct structure
	if len(result.Rows) > 0 {
		firstRow := result.Rows[0]
		if checked, ok := firstRow["checked"].(bool); !ok || !checked {
			t.Errorf("first row checked = %v, want true", firstRow["checked"])
		}
	}
}

// TestParseStreamEmptyFields tests handling of empty fields
func TestParseStreamEmptyFields(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test fixture with empty fields
	emptyFieldData := `ID|CATEGORY|EST_MINUTES
TD-001||30
||120
`
	emptyFile := filepath.Join(tmpDir, "empty_fields.txt")
	os.WriteFile(emptyFile, []byte(emptyFieldData), 0644)

	cmd := newParseStreamCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--file", emptyFile, "--format", "pipe", "--json"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result ParseStreamResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	// Empty fields should be preserved
	if len(result.Rows) != 2 {
		t.Errorf("rows count = %d, want 2", len(result.Rows))
	}

	// First row should have empty CATEGORY
	if len(result.Rows) > 0 {
		firstRow := result.Rows[0]
		if cat, ok := firstRow["CATEGORY"].(string); !ok || cat != "" {
			t.Errorf("CATEGORY = %q, want empty string", firstRow["CATEGORY"])
		}
	}
}

// TestParseStreamInconsistentColumns tests handling of rows with wrong column count
func TestParseStreamInconsistentColumns(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test fixture with inconsistent columns
	inconsistentData := `ID|CATEGORY|EST_MINUTES
TD-001|performance
TD-002|security|120|extra_field
TD-003|refactoring|15
`
	inconsistentFile := filepath.Join(tmpDir, "inconsistent.txt")
	os.WriteFile(inconsistentFile, []byte(inconsistentData), 0644)

	cmd := newParseStreamCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--file", inconsistentFile, "--format", "pipe", "--json"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result ParseStreamResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	// Should still parse all rows, but have parse errors
	if len(result.ParseErrors) < 2 {
		t.Errorf("expected at least 2 parse errors for inconsistent columns, got %d", len(result.ParseErrors))
	}

	// Valid row should still be in results
	if result.RowCount != 3 {
		t.Errorf("row_count = %d, want 3 (all rows parsed with warnings)", result.RowCount)
	}
}

// TestParseStreamEmptyInput tests handling of empty input
func TestParseStreamEmptyInput(t *testing.T) {
	tmpDir := t.TempDir()

	emptyFile := filepath.Join(tmpDir, "empty.txt")
	os.WriteFile(emptyFile, []byte(""), 0644)

	cmd := newParseStreamCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--file", emptyFile, "--json"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result ParseStreamResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	if result.RowCount != 0 {
		t.Errorf("row_count = %d, want 0 for empty input", result.RowCount)
	}
}

// TestParseStreamFileNotFound tests error handling for missing file
func TestParseStreamFileNotFound(t *testing.T) {
	cmd := newParseStreamCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--file", "/nonexistent/path/file.txt", "--json"})

	err := cmd.Execute()
	if err == nil {
		t.Errorf("expected error for missing file, got nil")
	}

	if !strings.Contains(err.Error(), "not found") && !strings.Contains(err.Error(), "no such file") {
		t.Errorf("error message should mention file not found: %v", err)
	}
}

// TestParseStreamMinimalOutput tests --min flag output
func TestParseStreamMinimalOutput(t *testing.T) {
	tmpDir := t.TempDir()

	pipeData := `ID|CATEGORY
TD-001|performance
TD-002|security
`
	pipeFile := filepath.Join(tmpDir, "minimal.txt")
	os.WriteFile(pipeFile, []byte(pipeData), 0644)

	cmd := newParseStreamCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--file", pipeFile, "--format", "pipe", "--json", "--min"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Minimal output should still be valid JSON
	var result ParseStreamResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("minimal output should be valid JSON: %v", err)
	}
}

// TestParseStreamAlternativeListMarkers tests *, + list markers for checklists
func TestParseStreamAlternativeListMarkers(t *testing.T) {
	tmpDir := t.TempDir()

	checklistData := `* [x] Asterisk checked
* [ ] Asterisk unchecked
+ [x] Plus checked
+ [ ] Plus unchecked
`
	checklistFile := filepath.Join(tmpDir, "alt_markers.md")
	os.WriteFile(checklistFile, []byte(checklistData), 0644)

	cmd := newParseStreamCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--file", checklistFile, "--format", "markdown-checklist", "--json"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result ParseStreamResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	if result.RowCount != 4 {
		t.Errorf("row_count = %d, want 4 (all list markers supported)", result.RowCount)
	}
}

// TestParseStreamStdin tests reading from stdin
func TestParseStreamStdin(t *testing.T) {
	// This test would need to be adjusted to actually test stdin
	// For now, we test the --content flag alternative
	cmd := newParseStreamCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	// Using --content flag as stdin alternative for testing
	content := `ID|CATEGORY
TD-001|performance
`
	cmd.SetArgs([]string{"--content", content, "--format", "pipe", "--json"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result ParseStreamResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	if result.RowCount != 1 {
		t.Errorf("row_count = %d, want 1", result.RowCount)
	}
}

// TestParseStreamHumanReadableOutput tests human-readable output mode
func TestParseStreamHumanReadableOutput(t *testing.T) {
	tmpDir := t.TempDir()

	pipeData := `ID|CATEGORY|EST_MINUTES
TD-001|performance|30
TD-002|security|120
`
	pipeFile := filepath.Join(tmpDir, "human.txt")
	os.WriteFile(pipeFile, []byte(pipeData), 0644)

	cmd := newParseStreamCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--file", pipeFile, "--format", "pipe"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	// Should contain human-readable labels
	if !strings.Contains(output, "FORMAT") && !strings.Contains(output, "ROW_COUNT") {
		t.Errorf("expected human-readable output, got: %s", output)
	}
}

// TestParseStreamAutoDetectMarkdown tests auto-detection of markdown checklist format
func TestParseStreamAutoDetectMarkdown(t *testing.T) {
	tmpDir := t.TempDir()

	checklistData := `- [x] Task one
- [ ] Task two
- [x] Task three
`
	checklistFile := filepath.Join(tmpDir, "checklist.md")
	os.WriteFile(checklistFile, []byte(checklistData), 0644)

	cmd := newParseStreamCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--file", checklistFile, "--format", "auto", "--json"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result ParseStreamResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	if result.Format != "markdown-checklist" {
		t.Errorf("auto-detected format = %q, want markdown-checklist", result.Format)
	}
}

// TestParseStreamNoInput tests error when no input provided
func TestParseStreamNoInput(t *testing.T) {
	cmd := newParseStreamCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--json"})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error when no input provided")
	}
}

// TestParseStreamWhitespaceOnlyInput tests handling of whitespace-only input
func TestParseStreamWhitespaceOnlyInput(t *testing.T) {
	tmpDir := t.TempDir()

	whitespaceFile := filepath.Join(tmpDir, "whitespace.txt")
	os.WriteFile(whitespaceFile, []byte("   \n\n   \n"), 0644)

	cmd := newParseStreamCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--file", whitespaceFile, "--format", "pipe", "--json"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result ParseStreamResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	if result.RowCount != 0 {
		t.Errorf("row_count = %d, want 0 for whitespace-only input", result.RowCount)
	}
}
