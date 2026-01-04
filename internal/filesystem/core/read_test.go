package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSizeExceededError(t *testing.T) {
	err := &SizeExceededError{
		Message: "File size (95000 bytes) exceeds max_size (70000 bytes)",
		Path:    "/path/to/file.md",
		Size:    95000,
		MaxSize: 70000,
	}

	t.Run("Error() returns message", func(t *testing.T) {
		if err.Error() != err.Message {
			t.Errorf("Error() = %q, want %q", err.Error(), err.Message)
		}
	})

	t.Run("ToJSON() returns valid JSON with error flag", func(t *testing.T) {
		jsonStr := err.ToJSON()

		var result map[string]interface{}
		if unmarshalErr := json.Unmarshal([]byte(jsonStr), &result); unmarshalErr != nil {
			t.Fatalf("ToJSON() returned invalid JSON: %v", unmarshalErr)
		}

		// Check error flag
		if errorFlag, ok := result["error"].(bool); !ok || !errorFlag {
			t.Error("Expected error=true in JSON output")
		}

		// Check message
		if msg, ok := result["message"].(string); !ok || msg != err.Message {
			t.Errorf("Expected message=%q, got %v", err.Message, result["message"])
		}

		// Check path
		if path, ok := result["path"].(string); !ok || path != err.Path {
			t.Errorf("Expected path=%q, got %v", err.Path, result["path"])
		}

		// Check size (JSON numbers are float64)
		if size, ok := result["size"].(float64); !ok || int64(size) != err.Size {
			t.Errorf("Expected size=%d, got %v", err.Size, result["size"])
		}

		// Check max_size
		if maxSize, ok := result["max_size"].(float64); !ok || int64(maxSize) != err.MaxSize {
			t.Errorf("Expected max_size=%d, got %v", err.MaxSize, result["max_size"])
		}
	})
}

func TestTotalSizeExceededError(t *testing.T) {
	err := &TotalSizeExceededError{
		Message:      "Total size (83008 bytes) exceeds max_total_size (70000 bytes)",
		TotalSize:    83008,
		MaxTotalSize: 70000,
		Files: []FileSizeEntry{
			{Path: "file1.md", Size: 19706},
			{Path: "file2.md", Size: 63302},
		},
	}

	t.Run("Error() returns message", func(t *testing.T) {
		if err.Error() != err.Message {
			t.Errorf("Error() = %q, want %q", err.Error(), err.Message)
		}
	})

	t.Run("ToJSON() returns valid JSON with error flag", func(t *testing.T) {
		jsonStr := err.ToJSON()

		var result map[string]interface{}
		if unmarshalErr := json.Unmarshal([]byte(jsonStr), &result); unmarshalErr != nil {
			t.Fatalf("ToJSON() returned invalid JSON: %v", unmarshalErr)
		}

		// Check error flag
		if errorFlag, ok := result["error"].(bool); !ok || !errorFlag {
			t.Error("Expected error=true in JSON output")
		}

		// Check message
		if msg, ok := result["message"].(string); !ok || msg != err.Message {
			t.Errorf("Expected message=%q, got %v", err.Message, result["message"])
		}

		// Check total_size
		if totalSize, ok := result["total_size"].(float64); !ok || int64(totalSize) != err.TotalSize {
			t.Errorf("Expected total_size=%d, got %v", err.TotalSize, result["total_size"])
		}

		// Check max_total_size
		if maxTotalSize, ok := result["max_total_size"].(float64); !ok || int64(maxTotalSize) != err.MaxTotalSize {
			t.Errorf("Expected max_total_size=%d, got %v", err.MaxTotalSize, result["max_total_size"])
		}

		// Check files array
		files, ok := result["files"].([]interface{})
		if !ok {
			t.Fatalf("Expected files array, got %T", result["files"])
		}
		if len(files) != 2 {
			t.Errorf("Expected 2 files, got %d", len(files))
		}
	})
}

func TestReadFileSizeLimit(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a file that exceeds the default size limit
	largeContent := strings.Repeat("a", 100000) // 100KB
	largeFile := filepath.Join(tmpDir, "large.txt")
	if err := os.WriteFile(largeFile, []byte(largeContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a small file
	smallContent := "small content"
	smallFile := filepath.Join(tmpDir, "small.txt")
	if err := os.WriteFile(smallFile, []byte(smallContent), 0644); err != nil {
		t.Fatal(err)
	}

	t.Run("file exceeds default size limit returns error", func(t *testing.T) {
		_, err := ReadFile(ReadFileOptions{
			Path:             largeFile,
			AllowedDirs:      []string{tmpDir},
			SizeCheckMaxSize: 0, // Use default (70000)
		})

		if err == nil {
			t.Fatal("Expected error for file exceeding size limit")
		}

		sizeErr, ok := err.(*SizeExceededError)
		if !ok {
			t.Fatalf("Expected SizeExceededError, got %T: %v", err, err)
		}

		if sizeErr.Size != 100000 {
			t.Errorf("Expected size=100000, got %d", sizeErr.Size)
		}
		if sizeErr.MaxSize != DefaultMaxSize {
			t.Errorf("Expected max_size=%d, got %d", DefaultMaxSize, sizeErr.MaxSize)
		}
	})

	t.Run("file within size limit succeeds", func(t *testing.T) {
		result, err := ReadFile(ReadFileOptions{
			Path:             smallFile,
			AllowedDirs:      []string{tmpDir},
			SizeCheckMaxSize: 0, // Use default
		})

		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		if result.Content != smallContent {
			t.Errorf("Expected content=%q, got %q", smallContent, result.Content)
		}
	})

	t.Run("custom size limit", func(t *testing.T) {
		_, err := ReadFile(ReadFileOptions{
			Path:             smallFile,
			AllowedDirs:      []string{tmpDir},
			SizeCheckMaxSize: 5, // 5 bytes - should fail
		})

		if err == nil {
			t.Fatal("Expected error for file exceeding custom size limit")
		}

		sizeErr, ok := err.(*SizeExceededError)
		if !ok {
			t.Fatalf("Expected SizeExceededError, got %T: %v", err, err)
		}

		if sizeErr.MaxSize != 5 {
			t.Errorf("Expected max_size=5, got %d", sizeErr.MaxSize)
		}
	})

	t.Run("negative one size limit disables checking", func(t *testing.T) {
		result, err := ReadFile(ReadFileOptions{
			Path:             largeFile,
			AllowedDirs:      []string{tmpDir},
			SizeCheckMaxSize: -1, // No limit (-1 = disabled)
		})

		if err != nil {
			t.Fatalf("Unexpected error with size limit disabled: %v", err)
		}

		if len(result.Content) != 100000 {
			t.Errorf("Expected content length=100000, got %d", len(result.Content))
		}
	})

	t.Run("large explicit limit allows large file", func(t *testing.T) {
		result, err := ReadFile(ReadFileOptions{
			Path:             largeFile,
			AllowedDirs:      []string{tmpDir},
			SizeCheckMaxSize: 200000, // Larger than file
		})

		if err != nil {
			t.Fatalf("Unexpected error with large limit: %v", err)
		}

		if len(result.Content) != 100000 {
			t.Errorf("Expected content length=100000, got %d", len(result.Content))
		}
	})
}

func TestReadMultipleFilesSizeLimit(t *testing.T) {
	tmpDir := t.TempDir()

	// Create files with known sizes
	file1Content := strings.Repeat("a", 40000) // 40KB
	file1 := filepath.Join(tmpDir, "file1.txt")
	if err := os.WriteFile(file1, []byte(file1Content), 0644); err != nil {
		t.Fatal(err)
	}

	file2Content := strings.Repeat("b", 50000) // 50KB
	file2 := filepath.Join(tmpDir, "file2.txt")
	if err := os.WriteFile(file2, []byte(file2Content), 0644); err != nil {
		t.Fatal(err)
	}

	smallFile := filepath.Join(tmpDir, "small.txt")
	if err := os.WriteFile(smallFile, []byte("small"), 0644); err != nil {
		t.Fatal(err)
	}

	t.Run("combined size exceeds default limit returns error", func(t *testing.T) {
		_, err := ReadMultipleFiles(ReadMultipleFilesOptions{
			Paths:                 []string{file1, file2},
			AllowedDirs:           []string{tmpDir},
			SizeCheckMaxTotalSize: 0, // Use default (70000)
		})

		if err == nil {
			t.Fatal("Expected error for combined files exceeding size limit")
		}

		sizeErr, ok := err.(*TotalSizeExceededError)
		if !ok {
			t.Fatalf("Expected TotalSizeExceededError, got %T: %v", err, err)
		}

		if sizeErr.TotalSize != 90000 {
			t.Errorf("Expected total_size=90000, got %d", sizeErr.TotalSize)
		}
		if sizeErr.MaxTotalSize != DefaultMaxSize {
			t.Errorf("Expected max_total_size=%d, got %d", DefaultMaxSize, sizeErr.MaxTotalSize)
		}
		if len(sizeErr.Files) != 2 {
			t.Errorf("Expected 2 files in error, got %d", len(sizeErr.Files))
		}
	})

	t.Run("combined size within limit succeeds", func(t *testing.T) {
		result, err := ReadMultipleFiles(ReadMultipleFilesOptions{
			Paths:                 []string{smallFile},
			AllowedDirs:           []string{tmpDir},
			SizeCheckMaxTotalSize: 0, // Use default
		})

		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		if result.Success != 1 {
			t.Errorf("Expected success=1, got %d", result.Success)
		}
	})

	t.Run("custom total size limit", func(t *testing.T) {
		_, err := ReadMultipleFiles(ReadMultipleFilesOptions{
			Paths:                 []string{file1},
			AllowedDirs:           []string{tmpDir},
			SizeCheckMaxTotalSize: 30000, // 30KB - should fail for 40KB file
		})

		if err == nil {
			t.Fatal("Expected error for files exceeding custom size limit")
		}

		sizeErr, ok := err.(*TotalSizeExceededError)
		if !ok {
			t.Fatalf("Expected TotalSizeExceededError, got %T: %v", err, err)
		}

		if sizeErr.MaxTotalSize != 30000 {
			t.Errorf("Expected max_total_size=30000, got %d", sizeErr.MaxTotalSize)
		}
	})

	t.Run("negative one total size limit disables checking", func(t *testing.T) {
		result, err := ReadMultipleFiles(ReadMultipleFilesOptions{
			Paths:                 []string{file1, file2},
			AllowedDirs:           []string{tmpDir},
			SizeCheckMaxTotalSize: -1, // No limit
		})

		if err != nil {
			t.Fatalf("Unexpected error with size limit disabled: %v", err)
		}

		if result.Success != 2 {
			t.Errorf("Expected success=2, got %d", result.Success)
		}
	})

	t.Run("handles missing files in size check gracefully", func(t *testing.T) {
		_, err := ReadMultipleFiles(ReadMultipleFilesOptions{
			Paths:                 []string{smallFile, filepath.Join(tmpDir, "nonexistent.txt")},
			AllowedDirs:           []string{tmpDir},
			SizeCheckMaxTotalSize: 0, // Use default
		})

		// Should not error at size check stage - missing files are handled during read
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
	})

	t.Run("handles directories in file list gracefully", func(t *testing.T) {
		subDir := filepath.Join(tmpDir, "subdir")
		os.Mkdir(subDir, 0755)

		_, err := ReadMultipleFiles(ReadMultipleFilesOptions{
			Paths:                 []string{smallFile, subDir},
			AllowedDirs:           []string{tmpDir},
			SizeCheckMaxTotalSize: 0, // Use default
		})

		// Should not error at size check stage - directories are handled during read
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
	})
}

func TestDefaultMaxSize(t *testing.T) {
	if DefaultMaxSize != 70000 {
		t.Errorf("Expected DefaultMaxSize=70000, got %d", DefaultMaxSize)
	}
}

func TestReadFileValidation(t *testing.T) {
	t.Run("empty path returns error", func(t *testing.T) {
		_, err := ReadFile(ReadFileOptions{
			Path: "",
		})
		if err == nil {
			t.Error("Expected error for empty path")
		}
	})
}

func TestReadMultipleFilesValidation(t *testing.T) {
	t.Run("empty paths returns error", func(t *testing.T) {
		_, err := ReadMultipleFiles(ReadMultipleFilesOptions{
			Paths: []string{},
		})
		if err == nil {
			t.Error("Expected error for empty paths")
		}
	})

	t.Run("nil paths returns error", func(t *testing.T) {
		_, err := ReadMultipleFiles(ReadMultipleFilesOptions{
			Paths: nil,
		})
		if err == nil {
			t.Error("Expected error for nil paths")
		}
	})
}

func TestReadFileByLines(t *testing.T) {
	tmpDir := t.TempDir()

	content := "line 1\nline 2\nline 3\nline 4\nline 5\n"
	testFile := filepath.Join(tmpDir, "lines.txt")
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	t.Run("read from specific line", func(t *testing.T) {
		result, err := ReadFile(ReadFileOptions{
			Path:             testFile,
			AllowedDirs:      []string{tmpDir},
			LineStart:        2,
			LineCount:        2,
			SizeCheckMaxSize: 0,
		})

		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		if !strings.Contains(result.Content, "line 2") {
			t.Error("Expected content to contain 'line 2'")
		}
		if !strings.Contains(result.Content, "line 3") {
			t.Error("Expected content to contain 'line 3'")
		}
		if strings.Contains(result.Content, "line 1") {
			t.Error("Content should not contain 'line 1'")
		}
	})
}

func TestExtractLinesBasic(t *testing.T) {
	tmpDir := t.TempDir()

	content := "line 1\nline 2\nline 3\nline 4\nline 5\n"
	testFile := filepath.Join(tmpDir, "extract.txt")
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	t.Run("extract by line numbers", func(t *testing.T) {
		result, err := ExtractLines(ExtractLinesOptions{
			Path:        testFile,
			AllowedDirs: []string{tmpDir},
			LineNumbers: []int{1, 3, 5},
		})

		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		if !strings.Contains(result.Content, "line 1") {
			t.Error("Expected content to contain 'line 1'")
		}
		if !strings.Contains(result.Content, "line 3") {
			t.Error("Expected content to contain 'line 3'")
		}
		if !strings.Contains(result.Content, "line 5") {
			t.Error("Expected content to contain 'line 5'")
		}
	})

	t.Run("extract by range", func(t *testing.T) {
		result, err := ExtractLines(ExtractLinesOptions{
			Path:        testFile,
			AllowedDirs: []string{tmpDir},
			StartLine:   2,
			EndLine:     4,
		})

		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		if !strings.Contains(result.Content, "line 2") {
			t.Error("Expected content to contain 'line 2'")
		}
		if !strings.Contains(result.Content, "line 4") {
			t.Error("Expected content to contain 'line 4'")
		}
	})

	t.Run("extract by pattern", func(t *testing.T) {
		result, err := ExtractLines(ExtractLinesOptions{
			Path:        testFile,
			AllowedDirs: []string{tmpDir},
			Pattern:     "line 3",
		})

		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		if !strings.Contains(result.Content, "line 3") {
			t.Error("Expected content to contain 'line 3'")
		}
	})

	t.Run("empty path returns error", func(t *testing.T) {
		_, err := ExtractLines(ExtractLinesOptions{
			Path: "",
		})
		if err == nil {
			t.Error("Expected error for empty path")
		}
	})
}
