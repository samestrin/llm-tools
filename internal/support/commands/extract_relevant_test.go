package commands

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExtractRelevantMissingContext(t *testing.T) {
	cmd := newExtractRelevantCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--path", "."})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing context")
	}
	// Cobra will complain about required flag
}

func TestExtractRelevantFileNotFound(t *testing.T) {
	// Temporarily set a fake API key for this test
	os.Setenv("OPENAI_API_KEY", "test-key-for-testing")
	defer os.Unsetenv("OPENAI_API_KEY")

	cmd := newExtractRelevantCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--path", "/nonexistent/path", "--context", "test"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for non-existent path")
	}
	if !strings.Contains(err.Error(), "path not found") {
		t.Errorf("expected 'path not found' error, got: %v", err)
	}
}

func TestExtractRelevantNoAPIKey(t *testing.T) {
	// Ensure no API key is set
	os.Unsetenv("OPENAI_API_KEY")
	os.Unsetenv("ANTHROPIC_API_KEY")

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.md")
	os.WriteFile(testFile, []byte("# Test Content\n\nSome text here."), 0644)

	cmd := newExtractRelevantCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--path", testFile, "--context", "test"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing API key")
	}
	if !strings.Contains(err.Error(), "API key is required") {
		t.Errorf("expected API key error, got: %v", err)
	}
}

func TestShouldExcludeDir(t *testing.T) {
	tests := []struct {
		name     string
		expected bool
	}{
		{".git", true},
		{"node_modules", true},
		{"vendor", true},
		{"src", false},
		{"lib", false},
		{"__pycache__", true},
		{"build", true},
		{"docs", false},
		{"tests", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := shouldExcludeDir(tt.name)
			if result != tt.expected {
				t.Errorf("shouldExcludeDir(%q) = %v, want %v", tt.name, result, tt.expected)
			}
		})
	}
}

func TestHasTextExtension(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
	}{
		{"file.go", true},
		{"file.py", true},
		{"file.js", true},
		{"file.ts", true},
		{"file.md", true},
		{"file.txt", true},
		{"file.json", true},
		{"file.yaml", true},
		{"file.exe", false},
		{"file.bin", false},
		{"file.jpg", false},
		{"file.png", false},
		{"Dockerfile", true},
		{"Makefile", true},
		{"README", true},
		{"file", false}, // Unknown extensionless file
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := hasTextExtension(tt.path)
			if result != tt.expected {
				t.Errorf("hasTextExtension(%q) = %v, want %v", tt.path, result, tt.expected)
			}
		})
	}
}

func TestExtractRelevantResultJSON(t *testing.T) {
	result := ExtractRelevantResult{
		Path:           "/test/path",
		Context:        "API endpoints",
		ExtractedParts: []string{"## file1.go\n\nSome content", "## file2.go\n\nMore content"},
		TotalFiles:     5,
		ProcessedFiles: 2,
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var parsed ExtractRelevantResult
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if parsed.Path != result.Path {
		t.Errorf("Path mismatch: got %s, want %s", parsed.Path, result.Path)
	}
	if parsed.Context != result.Context {
		t.Errorf("Context mismatch: got %s, want %s", parsed.Context, result.Context)
	}
	if len(parsed.ExtractedParts) != len(result.ExtractedParts) {
		t.Errorf("ExtractedParts length mismatch: got %d, want %d", len(parsed.ExtractedParts), len(result.ExtractedParts))
	}
}

func TestExtractRelevantResultWithError(t *testing.T) {
	result := ExtractRelevantResult{
		Path:    "/test/path",
		Context: "test context",
		Error:   "API call failed",
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	if !strings.Contains(string(data), "API call failed") {
		t.Error("expected error message in JSON output")
	}
}

func TestExtractRelevantFlags(t *testing.T) {
	cmd := newExtractRelevantCmd()

	// Check that flags exist
	if cmd.Flag("context") == nil {
		t.Error("expected --context flag")
	}
	if cmd.Flag("concurrency") == nil {
		t.Error("expected --concurrency flag")
	}
	if cmd.Flag("output") == nil {
		t.Error("expected --output flag")
	}
	if cmd.Flag("timeout") == nil {
		t.Error("expected --timeout flag")
	}
	if cmd.Flag("json") == nil {
		t.Error("expected --json flag")
	}
}

func TestExtractRelevantDefaultConcurrency(t *testing.T) {
	cmd := newExtractRelevantCmd()

	concurrencyFlag := cmd.Flag("concurrency")
	if concurrencyFlag == nil {
		t.Fatal("concurrency flag not found")
	}

	if concurrencyFlag.DefValue != "2" {
		t.Errorf("expected default concurrency 2, got %s", concurrencyFlag.DefValue)
	}
}

func TestExtractRelevantDefaultTimeout(t *testing.T) {
	cmd := newExtractRelevantCmd()

	timeoutFlag := cmd.Flag("timeout")
	if timeoutFlag == nil {
		t.Fatal("timeout flag not found")
	}

	if timeoutFlag.DefValue != "60" {
		t.Errorf("expected default timeout 60, got %s", timeoutFlag.DefValue)
	}
}
