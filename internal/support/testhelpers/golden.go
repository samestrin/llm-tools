// Package testhelpers provides testing utilities for golden file testing.
package testhelpers

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// GoldenDir returns the path to the testdata/golden directory.
func GoldenDir() string {
	// Find the repo root by looking for go.mod
	dir, _ := os.Getwd()
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return filepath.Join(dir, "testdata", "golden")
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			// Fallback to relative path
			return "testdata/golden"
		}
		dir = parent
	}
}

// GoldenFile returns the full path to a golden file.
func GoldenFile(name string) string {
	return filepath.Join(GoldenDir(), name)
}

// Update returns true if UPDATE_GOLDEN environment variable is set.
// Use: UPDATE_GOLDEN=1 go test ./... to update golden files.
var Update = os.Getenv("UPDATE_GOLDEN") == "1"

// AssertGolden compares actual output to a golden file.
// If -update flag is set, it updates the golden file instead.
func AssertGolden(t *testing.T, name string, actual string) {
	t.Helper()

	goldenPath := GoldenFile(name)

	if Update {
		// Create directory if needed
		if err := os.MkdirAll(filepath.Dir(goldenPath), 0755); err != nil {
			t.Fatalf("failed to create golden file directory: %v", err)
		}
		// Write the actual output as the new golden file
		if err := os.WriteFile(goldenPath, []byte(actual), 0644); err != nil {
			t.Fatalf("failed to update golden file %s: %v", goldenPath, err)
		}
		t.Logf("Updated golden file: %s", goldenPath)
		return
	}

	// Read expected output from golden file
	expected, err := os.ReadFile(goldenPath)
	if err != nil {
		if os.IsNotExist(err) {
			t.Fatalf("golden file %s does not exist. Run with -update to create it", goldenPath)
		}
		t.Fatalf("failed to read golden file %s: %v", goldenPath, err)
	}

	// Compare outputs
	if string(expected) != actual {
		t.Errorf("output mismatch for golden file %s\n--- Expected ---\n%s\n--- Actual ---\n%s\n--- Diff ---\n%s",
			goldenPath, string(expected), actual, diff(string(expected), actual))
	}
}

// AssertGoldenNormalized compares output with normalized line endings.
func AssertGoldenNormalized(t *testing.T, name string, actual string) {
	t.Helper()
	// Normalize line endings
	actual = strings.ReplaceAll(actual, "\r\n", "\n")
	AssertGolden(t, name, actual)
}

// diff returns a simple line-by-line diff of two strings.
func diff(expected, actual string) string {
	expectedLines := strings.Split(expected, "\n")
	actualLines := strings.Split(actual, "\n")

	var result strings.Builder
	maxLen := len(expectedLines)
	if len(actualLines) > maxLen {
		maxLen = len(actualLines)
	}

	for i := 0; i < maxLen; i++ {
		var expLine, actLine string
		if i < len(expectedLines) {
			expLine = expectedLines[i]
		}
		if i < len(actualLines) {
			actLine = actualLines[i]
		}

		if expLine != actLine {
			result.WriteString("- ")
			result.WriteString(expLine)
			result.WriteString("\n+ ")
			result.WriteString(actLine)
			result.WriteString("\n")
		}
	}

	return result.String()
}

// CreateTempDir creates a temporary directory with optional files.
func CreateTempDir(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()

	for name, content := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatalf("failed to create directory for %s: %v", name, err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("failed to write file %s: %v", name, err)
		}
	}

	return dir
}

// CreateTempFile creates a single temporary file with content.
func CreateTempFile(t *testing.T, name, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}
	return path
}
