package commands

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFormatTDTableRoutedInput(t *testing.T) {
	tests := []struct {
		name            string
		input           string
		section         string
		expectTables    []string
		expectItems     int
		expectSections  int
	}{
		{
			name: "all sections from routed output",
			input: `{
				"quick_wins": [
					{"SEVERITY": "LOW", "FILE_LINE": "src/a.ts:1", "PROBLEM": "Minor issue", "EST_MINUTES": 15}
				],
				"backlog": [
					{"SEVERITY": "MEDIUM", "FILE_LINE": "src/b.ts:2", "PROBLEM": "Medium issue", "EST_MINUTES": 60}
				],
				"td_files": [
					{"SEVERITY": "HIGH", "FILE_LINE": "src/c.ts:3", "PROBLEM": "Major issue", "EST_MINUTES": 3000}
				]
			}`,
			section:        "all",
			expectTables:   []string{"quick_wins", "backlog", "td_files"},
			expectItems:    3,
			expectSections: 3,
		},
		{
			name: "single section - quick_wins only",
			input: `{
				"quick_wins": [
					{"SEVERITY": "LOW", "FILE_LINE": "src/a.ts:1", "PROBLEM": "Issue 1", "EST_MINUTES": 10},
					{"SEVERITY": "LOW", "FILE_LINE": "src/b.ts:2", "PROBLEM": "Issue 2", "EST_MINUTES": 20}
				],
				"backlog": [
					{"SEVERITY": "MEDIUM", "FILE_LINE": "src/c.ts:3", "PROBLEM": "Issue 3", "EST_MINUTES": 100}
				]
			}`,
			section:        "quick_wins",
			expectTables:   []string{"quick_wins"},
			expectItems:    2,
			expectSections: 1,
		},
		{
			name: "empty sections ignored",
			input: `{
				"quick_wins": [],
				"backlog": [
					{"SEVERITY": "MEDIUM", "FILE_LINE": "src/a.ts:1", "PROBLEM": "Issue", "EST_MINUTES": 50}
				],
				"td_files": []
			}`,
			section:        "all",
			expectTables:   []string{"backlog"},
			expectItems:    1,
			expectSections: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset flags
			formatTDTableContent = tt.input
			formatTDTableFile = ""
			formatTDTableSection = tt.section
			formatTDTableJSON = true
			formatTDTableMinimal = false

			cmd := newFormatTDTableCmd()
			buf := new(bytes.Buffer)
			cmd.SetOut(buf)
			cmd.SetArgs([]string{"--content", tt.input, "--section", tt.section, "--json"})

			err := cmd.Execute()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			var result FormatTDTableResult
			if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
				t.Fatalf("failed to parse output: %v\nOutput: %s", err, buf.String())
			}

			if result.Summary.TotalItems != tt.expectItems {
				t.Errorf("expected %d items, got %d", tt.expectItems, result.Summary.TotalItems)
			}

			if result.Summary.SectionsFormatted != tt.expectSections {
				t.Errorf("expected %d sections, got %d", tt.expectSections, result.Summary.SectionsFormatted)
			}

			for _, section := range tt.expectTables {
				if _, ok := result.Tables[section]; !ok {
					t.Errorf("expected table for section %s", section)
				}
			}
		})
	}
}

func TestFormatTDTableRawInput(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectItems int
	}{
		{
			name: "items wrapper",
			input: `{
				"items": [
					{"SEVERITY": "HIGH", "FILE_LINE": "src/auth.ts:45", "PROBLEM": "Missing validation"},
					{"SEVERITY": "LOW", "FILE_LINE": "src/utils.ts:10", "PROBLEM": "Unused import"}
				]
			}`,
			expectItems: 2,
		},
		{
			name: "rows wrapper",
			input: `{
				"rows": [
					{"SEVERITY": "MEDIUM", "PROBLEM": "Performance issue"}
				]
			}`,
			expectItems: 1,
		},
		{
			name: "raw array",
			input: `[
				{"SEVERITY": "HIGH", "PROBLEM": "Bug 1"},
				{"SEVERITY": "HIGH", "PROBLEM": "Bug 2"},
				{"SEVERITY": "HIGH", "PROBLEM": "Bug 3"}
			]`,
			expectItems: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := newFormatTDTableCmd()
			buf := new(bytes.Buffer)
			cmd.SetOut(buf)
			cmd.SetArgs([]string{"--content", tt.input, "--json"})

			err := cmd.Execute()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			var result FormatTDTableResult
			if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
				t.Fatalf("failed to parse output: %v", err)
			}

			if result.Summary.TotalItems != tt.expectItems {
				t.Errorf("expected %d items, got %d", tt.expectItems, result.Summary.TotalItems)
			}
		})
	}
}

func TestFormatTDTableMarkdownOutput(t *testing.T) {
	input := `{
		"items": [
			{"SEVERITY": "HIGH", "FILE_LINE": "src/auth.ts:45", "PROBLEM": "Missing validation", "FIX": "Add zod", "EST_MINUTES": 30}
		]
	}`

	cmd := newFormatTDTableCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--content", input, "--json"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result FormatTDTableResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse output: %v", err)
	}

	table := result.Tables["items"]

	// Check header row
	if !strings.Contains(table, "| Severity |") {
		t.Error("expected Severity column header")
	}
	if !strings.Contains(table, "| File Line |") {
		t.Error("expected File Line column header")
	}

	// Check separator row
	if !strings.Contains(table, "|------|") {
		t.Error("expected separator row")
	}

	// Check data row
	if !strings.Contains(table, "| HIGH |") {
		t.Error("expected HIGH severity in data")
	}
	if !strings.Contains(table, "src/auth.ts:45") {
		t.Error("expected file line in data")
	}
}

func TestFormatTDTableColumnOrdering(t *testing.T) {
	// Test that columns appear in preferred order
	input := `[{"EST_MINUTES": 30, "PROBLEM": "Issue", "SEVERITY": "HIGH", "FILE_LINE": "src/a.ts:1", "CUSTOM_FIELD": "value"}]`

	cmd := newFormatTDTableCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--content", input, "--json"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result FormatTDTableResult
	json.Unmarshal(buf.Bytes(), &result)

	table := result.Tables["items"]

	// SEVERITY should come before FILE_LINE in the header
	severityPos := strings.Index(table, "Severity")
	fileLinePos := strings.Index(table, "File Line")
	problemPos := strings.Index(table, "Problem")
	customPos := strings.Index(table, "Custom Field")

	if severityPos > fileLinePos {
		t.Error("SEVERITY should come before FILE_LINE")
	}
	if fileLinePos > problemPos {
		t.Error("FILE_LINE should come before PROBLEM")
	}
	if problemPos > customPos {
		t.Error("PROBLEM should come before custom fields")
	}
}

func TestFormatTDTableEscaping(t *testing.T) {
	// Test that pipe characters and newlines are escaped
	input := `[{"PROBLEM": "Issue with | pipe", "DESCRIPTION": "Line1\nLine2"}]`

	cmd := newFormatTDTableCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--content", input, "--json"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result FormatTDTableResult
	json.Unmarshal(buf.Bytes(), &result)

	table := result.Tables["items"]

	if strings.Contains(table, "Issue with | pipe") {
		t.Error("pipe character should be escaped")
	}
	if !strings.Contains(table, "Issue with \\| pipe") {
		t.Error("escaped pipe should be present")
	}
	if strings.Contains(table, "\n") && strings.Contains(table, "Line1\nLine2") {
		t.Error("newlines in content should be escaped")
	}
}

func TestFormatTDTableNumericFormatting(t *testing.T) {
	input := `[
		{"EST_MINUTES": 30},
		{"EST_MINUTES": 30.5},
		{"EST_MINUTES": "45"}
	]`

	cmd := newFormatTDTableCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--content", input, "--json"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result FormatTDTableResult
	json.Unmarshal(buf.Bytes(), &result)

	table := result.Tables["items"]

	// Whole numbers should not have decimals
	if !strings.Contains(table, "| 30 |") {
		t.Error("expected whole number 30 without decimals")
	}
	// Decimals should be formatted
	if !strings.Contains(table, "| 30.5 |") {
		t.Error("expected decimal 30.5")
	}
}

func TestFormatTDTableFileInput(t *testing.T) {
	// Create temp file
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "input.json")

	content := `{"items": [{"SEVERITY": "HIGH", "PROBLEM": "Test"}]}`
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	cmd := newFormatTDTableCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--file", tmpFile, "--json"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result FormatTDTableResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse output: %v", err)
	}

	if result.Summary.TotalItems != 1 {
		t.Errorf("expected 1 item, got %d", result.Summary.TotalItems)
	}
}

func TestFormatTDTableTextOutput(t *testing.T) {
	input := `{
		"quick_wins": [{"SEVERITY": "LOW", "PROBLEM": "Quick fix"}],
		"backlog": [{"SEVERITY": "MEDIUM", "PROBLEM": "Backlog item"}]
	}`

	cmd := newFormatTDTableCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--content", input})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()

	// Check section headers
	if !strings.Contains(output, "Quick Wins") {
		t.Error("expected Quick Wins header")
	}
	if !strings.Contains(output, "Backlog") {
		t.Error("expected Backlog header")
	}
	// Check summary
	if !strings.Contains(output, "Total: 2 items") {
		t.Error("expected total items in summary")
	}
}

func TestFormatTDTableMinimalOutput(t *testing.T) {
	input := `{"items": [{"SEVERITY": "HIGH", "PROBLEM": "Issue"}]}`

	cmd := newFormatTDTableCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--content", input, "--min"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()

	// Minimal should not have section headers
	if strings.Contains(output, "### ") {
		t.Error("minimal output should not have section headers")
	}
	// Should have table content
	if !strings.Contains(output, "| Severity |") {
		t.Error("should have table content")
	}
}

func TestFormatTDTableInvalidSection(t *testing.T) {
	cmd := newFormatTDTableCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--content", `[{"PROBLEM": "test"}]`, "--section", "invalid"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for invalid section")
	}
	if !strings.Contains(err.Error(), "invalid section") {
		t.Errorf("expected 'invalid section' error, got: %v", err)
	}
}

func TestFormatTDTableNoInput(t *testing.T) {
	// Reset flags
	formatTDTableContent = ""
	formatTDTableFile = ""

	cmd := newFormatTDTableCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for no input")
	}
	if !strings.Contains(err.Error(), "no input provided") {
		t.Errorf("expected 'no input provided' error, got: %v", err)
	}
}

func TestFormatTDTableInvalidJSON(t *testing.T) {
	cmd := newFormatTDTableCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--content", "not valid json"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
	if !strings.Contains(err.Error(), "could not parse") {
		t.Errorf("expected parse error, got: %v", err)
	}
}

func TestFormatTDTableEmptyInput(t *testing.T) {
	input := `{"items": []}`

	cmd := newFormatTDTableCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--content", input, "--json"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result FormatTDTableResult
	json.Unmarshal(buf.Bytes(), &result)

	if result.Summary.TotalItems != 0 {
		t.Errorf("expected 0 items, got %d", result.Summary.TotalItems)
	}
	if result.Summary.SectionsFormatted != 0 {
		t.Errorf("expected 0 sections, got %d", result.Summary.SectionsFormatted)
	}
}

func TestFormatTDTableNilValues(t *testing.T) {
	input := `[{"SEVERITY": null, "PROBLEM": "Test", "EXTRA": null}]`

	cmd := newFormatTDTableCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--content", input, "--json"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result FormatTDTableResult
	json.Unmarshal(buf.Bytes(), &result)

	// Should handle nil values without error
	if result.Summary.TotalItems != 1 {
		t.Errorf("expected 1 item, got %d", result.Summary.TotalItems)
	}
}

func TestFormatColumnHeader(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"SEVERITY", "Severity"},
		{"FILE_LINE", "File Line"},
		{"EST_MINUTES", "Est Minutes"},
		{"PROBLEM", "Problem"},
		{"ID", "Id"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := formatColumnHeader(tt.input)
			if result != tt.expected {
				t.Errorf("formatColumnHeader(%s) = %s, want %s", tt.input, result, tt.expected)
			}
		})
	}
}

func TestFormatCellValue(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected string
	}{
		{"nil", nil, ""},
		{"string", "test", "test"},
		{"string with pipe", "a | b", "a \\| b"},
		{"int", 42, "42"},
		{"float whole", float64(30), "30"},
		{"float decimal", 30.5, "30.5"},
		{"json.Number", json.Number("123"), "123"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatCellValue(tt.input)
			if result != tt.expected {
				t.Errorf("formatCellValue(%v) = %s, want %s", tt.input, result, tt.expected)
			}
		})
	}
}
