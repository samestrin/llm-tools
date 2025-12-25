package testhelpers

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGoldenDir(t *testing.T) {
	dir := GoldenDir()
	if dir == "" {
		t.Error("GoldenDir returned empty string")
	}
}

func TestGoldenFile(t *testing.T) {
	path := GoldenFile("test.golden")
	if path == "" {
		t.Error("GoldenFile returned empty string")
	}
	if filepath.Base(path) != "test.golden" {
		t.Errorf("expected basename test.golden, got %s", filepath.Base(path))
	}
}

func TestCreateTempDir(t *testing.T) {
	files := map[string]string{
		"file1.txt":     "content1",
		"sub/file2.txt": "content2",
	}

	dir := CreateTempDir(t, files)

	// Verify files were created
	content, err := os.ReadFile(filepath.Join(dir, "file1.txt"))
	if err != nil {
		t.Fatalf("failed to read file1.txt: %v", err)
	}
	if string(content) != "content1" {
		t.Errorf("expected content1, got %s", string(content))
	}

	content, err = os.ReadFile(filepath.Join(dir, "sub/file2.txt"))
	if err != nil {
		t.Fatalf("failed to read sub/file2.txt: %v", err)
	}
	if string(content) != "content2" {
		t.Errorf("expected content2, got %s", string(content))
	}
}

func TestCreateTempFile(t *testing.T) {
	path := CreateTempFile(t, "test.txt", "hello")

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read temp file: %v", err)
	}
	if string(content) != "hello" {
		t.Errorf("expected hello, got %s", string(content))
	}
}

func TestDiff(t *testing.T) {
	expected := "line1\nline2\nline3"
	actual := "line1\nchanged\nline3"

	result := diff(expected, actual)

	if result == "" {
		t.Error("diff returned empty string for different inputs")
	}
}

func TestDiff_Identical(t *testing.T) {
	text := "line1\nline2\nline3"
	result := diff(text, text)

	if result != "" {
		t.Errorf("diff returned non-empty string for identical inputs: %s", result)
	}
}
