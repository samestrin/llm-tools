package commands

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestForeachMissingTemplate(t *testing.T) {
	cmd := newForeachCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--files", "test.txt"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing template")
	}
	// Cobra will complain about required flag
}

func TestForeachNoFiles(t *testing.T) {
	tmpDir := t.TempDir()
	templateFile := filepath.Join(tmpDir, "template.md")
	os.WriteFile(templateFile, []byte("Process: [[CONTENT]]"), 0644)

	cmd := newForeachCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--template", templateFile})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for no files")
	}
	if !strings.Contains(err.Error(), "no files to process") {
		t.Errorf("expected 'no files to process' error, got: %v", err)
	}
}

func TestForeachTemplateNotFound(t *testing.T) {
	cmd := newForeachCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--template", "/nonexistent/template.md", "--files", "test.txt"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for non-existent template")
	}
	if !strings.Contains(err.Error(), "failed to read template") {
		t.Errorf("expected 'failed to read template' error, got: %v", err)
	}
}

func TestSubstituteTemplateVars(t *testing.T) {
	template := `File: [[FILENAME]]
Path: [[FILEPATH]]
Extension: [[EXTENSION]]
Dir: [[DIRNAME]]
Index: [[INDEX]]/[[TOTAL]]
Custom: [[LANG]]
Content:
[[CONTENT]]`

	variables := map[string]string{
		"LANG": "Go",
	}

	result := substituteTemplateVars(template, "/path/to/file.go", "package main", variables, 2, 5)

	if !strings.Contains(result, "File: file.go") {
		t.Error("expected FILENAME substitution")
	}
	if !strings.Contains(result, "Path: /path/to/file.go") {
		t.Error("expected FILEPATH substitution")
	}
	if !strings.Contains(result, "Extension: .go") {
		t.Error("expected EXTENSION substitution")
	}
	if !strings.Contains(result, "Dir: /path/to") {
		t.Error("expected DIRNAME substitution")
	}
	if !strings.Contains(result, "Index: 2/5") {
		t.Error("expected INDEX/TOTAL substitution")
	}
	if !strings.Contains(result, "Custom: Go") {
		t.Error("expected custom variable substitution")
	}
	if !strings.Contains(result, "package main") {
		t.Error("expected CONTENT substitution")
	}
}

func TestDetermineOutputPath(t *testing.T) {
	// Reset flags for test
	originalOutputDir := foreachOutputDir
	originalOutputPat := foreachOutputPat
	defer func() {
		foreachOutputDir = originalOutputDir
		foreachOutputPat = originalOutputPat
	}()

	tests := []struct {
		name      string
		inputPath string
		outputDir string
		outputPat string
		expected  string
	}{
		{
			name:      "no output options",
			inputPath: "/path/to/file.txt",
			outputDir: "",
			outputPat: "",
			expected:  "",
		},
		{
			name:      "output dir only",
			inputPath: "/path/to/file.txt",
			outputDir: "/output",
			outputPat: "",
			expected:  "/output/file.txt",
		},
		{
			name:      "output pattern only",
			inputPath: "/path/to/file.txt",
			outputDir: "",
			outputPat: "{{name}}-processed.md",
			expected:  "/path/to/file-processed.md",
		},
		{
			name:      "both output dir and pattern",
			inputPath: "/path/to/file.txt",
			outputDir: "/output",
			outputPat: "{{name}}-result{{ext}}",
			expected:  "/output/file-result.txt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			foreachOutputDir = tt.outputDir
			foreachOutputPat = tt.outputPat
			result := determineOutputPath(tt.inputPath)
			if result != tt.expected {
				t.Errorf("determineOutputPath(%q) = %q, want %q", tt.inputPath, result, tt.expected)
			}
		})
	}
}

func TestUniqueStrings(t *testing.T) {
	input := []string{"a", "b", "a", "c", "b", "d"}
	result := uniqueStrings(input)

	if len(result) != 4 {
		t.Errorf("expected 4 unique strings, got %d", len(result))
	}

	// Check order is preserved
	expected := []string{"a", "b", "c", "d"}
	for i, s := range expected {
		if result[i] != s {
			t.Errorf("expected result[%d] = %q, got %q", i, s, result[i])
		}
	}
}

func TestForeachResultJSON(t *testing.T) {
	result := ForeachResult{
		TotalFiles:     5,
		ProcessedFiles: 3,
		SkippedFiles:   1,
		FailedFiles:    1,
		Results: []ForeachFileRes{
			{InputFile: "a.txt", OutputFile: "out/a.txt", Status: "success"},
			{InputFile: "b.txt", Status: "skipped"},
			{InputFile: "c.txt", Status: "failed", Error: "LLM error"},
		},
		ProcessingTime: 2.5,
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var parsed ForeachResult
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if parsed.TotalFiles != result.TotalFiles {
		t.Errorf("TotalFiles mismatch")
	}
	if parsed.ProcessedFiles != result.ProcessedFiles {
		t.Errorf("ProcessedFiles mismatch")
	}
	if len(parsed.Results) != len(result.Results) {
		t.Errorf("Results length mismatch")
	}
}

func TestCheckUnsubstitutedVars(t *testing.T) {
	tests := []struct {
		content  string
		expected []string
	}{
		{"No variables here", nil},
		{"[[VAR1]] and [[VAR2]]", []string{"VAR1", "VAR2"}},
		{"[[CONTENT]] is replaced", []string{"CONTENT"}},
		{"Multiple [[A]] and [[A]] same", []string{"A", "A"}},
	}

	for _, tt := range tests {
		t.Run(tt.content, func(t *testing.T) {
			result := checkUnsubstitutedVars(tt.content)
			if len(result) != len(tt.expected) {
				t.Errorf("expected %d unsubstituted vars, got %d", len(tt.expected), len(result))
			}
		})
	}
}

func TestForeachFlags(t *testing.T) {
	cmd := newForeachCmd()

	requiredFlags := []string{"template"}
	optionalFlags := []string{"files", "glob", "output-dir", "output-pattern", "llm", "var", "parallel", "skip-existing", "timeout", "json"}

	for _, f := range requiredFlags {
		if cmd.Flag(f) == nil {
			t.Errorf("expected --%s flag", f)
		}
	}

	for _, f := range optionalFlags {
		if cmd.Flag(f) == nil {
			t.Errorf("expected --%s flag", f)
		}
	}
}

func TestForeachDefaultParallel(t *testing.T) {
	cmd := newForeachCmd()

	parallelFlag := cmd.Flag("parallel")
	if parallelFlag == nil {
		t.Fatal("parallel flag not found")
	}

	if parallelFlag.DefValue != "1" {
		t.Errorf("expected default parallel 1, got %s", parallelFlag.DefValue)
	}
}

func TestForeachDefaultTimeout(t *testing.T) {
	cmd := newForeachCmd()

	timeoutFlag := cmd.Flag("timeout")
	if timeoutFlag == nil {
		t.Fatal("timeout flag not found")
	}

	if timeoutFlag.DefValue != "120" {
		t.Errorf("expected default timeout 120, got %s", timeoutFlag.DefValue)
	}
}

func TestUpdateCounts(t *testing.T) {
	result := &ForeachResult{}

	updateCounts(result, ForeachFileRes{Status: "success"})
	if result.ProcessedFiles != 1 {
		t.Error("expected ProcessedFiles to be 1")
	}

	updateCounts(result, ForeachFileRes{Status: "skipped"})
	if result.SkippedFiles != 1 {
		t.Error("expected SkippedFiles to be 1")
	}

	updateCounts(result, ForeachFileRes{Status: "failed"})
	if result.FailedFiles != 1 {
		t.Error("expected FailedFiles to be 1")
	}
}
