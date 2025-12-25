package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestAnalyzeDepsCommand(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "analyze-deps-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test markdown file with file references
	testFile := filepath.Join(tmpDir, "story.md")
	content := "# User Story: Add Authentication\n\n" +
		"## Implementation Tasks\n\n" +
		"1. Create `src/auth/AuthService.ts` for handling authentication\n" +
		"2. Modify `src/api/client.ts` to include auth headers\n" +
		"3. Update `src/config/settings.json` with auth config\n" +
		"4. Read `docs/api-spec.md` for reference\n" +
		"5. Add new file `tests/auth.test.ts`\n\n" +
		"## Technical Notes\n\n" +
		"- Reference `src/utils/helpers.ts` for utility functions\n" +
		"- See `README.md` for project setup\n" +
		"- Edit `package.json` to add dependencies\n\n" +
		"## Files Overview\n\n" +
		"The main changes will be in src/auth/ directory.\n"
	os.WriteFile(testFile, []byte(content), 0644)

	cmd := newAnalyzeDepsCmd()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{testFile})

	err = cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := stdout.String()

	// Check for expected outputs
	expectedPatterns := []string{
		"FILES_CREATE:",
		"FILES_MODIFY:",
		"FILES_READ:",
		"TOTAL_FILES:",
		"CONFIDENCE:",
	}

	for _, pattern := range expectedPatterns {
		if !bytes.Contains([]byte(output), []byte(pattern)) {
			t.Errorf("output missing %q, got: %s", pattern, output)
		}
	}
}

func TestAnalyzeDepsJSON(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "analyze-deps-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	testFile := filepath.Join(tmpDir, "task.md")
	content := "# Task\n\nCreate `new-file.ts` and modify `existing.ts`.\n"
	os.WriteFile(testFile, []byte(content), 0644)

	cmd := newAnalyzeDepsCmd()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{testFile, "--json"})

	err = cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := stdout.String()

	// Check for JSON structure
	expectedPatterns := []string{
		`"files_create"`,
		`"files_modify"`,
		`"files_read"`,
		`"total_files"`,
		`"confidence"`,
	}

	for _, pattern := range expectedPatterns {
		if !bytes.Contains([]byte(output), []byte(pattern)) {
			t.Errorf("JSON output missing %q, got: %s", pattern, output)
		}
	}
}

func TestAnalyzeDepsEmpty(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "analyze-deps-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	testFile := filepath.Join(tmpDir, "empty.md")
	content := "# Empty Story\n\nNo file references here at all.\n"
	os.WriteFile(testFile, []byte(content), 0644)

	cmd := newAnalyzeDepsCmd()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{testFile})

	err = cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := stdout.String()

	if !bytes.Contains([]byte(output), []byte("TOTAL_FILES: 0")) {
		t.Errorf("expected TOTAL_FILES: 0 for empty file, got: %s", output)
	}
	if !bytes.Contains([]byte(output), []byte("CONFIDENCE: low")) {
		t.Errorf("expected CONFIDENCE: low, got: %s", output)
	}
}

func TestAnalyzeDepsConfidenceLevels(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "analyze-deps-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	tests := []struct {
		name     string
		content  string
		wantConf string
	}{
		{
			name:     "low confidence (0 files)",
			content:  "# Story\n\nNo files.",
			wantConf: "low",
		},
		{
			name:     "medium confidence (1-2 files)",
			content:  "# Story\n\nModify `file.ts`.",
			wantConf: "medium",
		},
		{
			name:     "high confidence (3+ files)",
			content:  "# Story\n\nModify `a.ts`, `b.ts`, and `c.ts`.",
			wantConf: "high",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testFile := filepath.Join(tmpDir, "test.md")
			os.WriteFile(testFile, []byte(tt.content), 0644)

			cmd := newAnalyzeDepsCmd()
			var stdout bytes.Buffer
			cmd.SetOut(&stdout)
			cmd.SetArgs([]string{testFile})

			err := cmd.Execute()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			output := stdout.String()
			expected := "CONFIDENCE: " + tt.wantConf
			if !bytes.Contains([]byte(output), []byte(expected)) {
				t.Errorf("expected %q, got: %s", expected, output)
			}
		})
	}
}

func TestAnalyzeDepsNonexistentFile(t *testing.T) {
	cmd := newAnalyzeDepsCmd()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"/nonexistent/file.md"})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestIsValidFilePath(t *testing.T) {
	tests := []struct {
		path  string
		valid bool
	}{
		{"src/main.ts", true},
		{"file.go", true},
		{"config.json", true},
		{"styles.css", true},
		{"README.md", true},
		{"ab", false},          // too short
		{"noextension", false}, // no extension
		{"file.xyz", false},    // unknown extension
		{"", false},            // empty
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := isValidFilePath(tt.path)
			if result != tt.valid {
				t.Errorf("isValidFilePath(%q) = %v, want %v", tt.path, result, tt.valid)
			}
		})
	}
}
