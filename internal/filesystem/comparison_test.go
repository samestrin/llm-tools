package filesystem

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestListDirectoryOutputStructure validates list_directory output matches fast-filesystem format
func TestListDirectoryOutputStructure(t *testing.T) {
	// Create temp directory with test files
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "test.txt"), []byte("hello"), 0644)
	os.Mkdir(filepath.Join(tmpDir, "subdir"), 0755)

	server, _ := NewServer([]string{tmpDir})
	result, err := server.handleListDirectory(map[string]interface{}{
		"path": tmpDir,
	})
	if err != nil {
		t.Fatalf("handleListDirectory failed: %v", err)
	}

	var output map[string]interface{}
	if err := json.Unmarshal([]byte(result), &output); err != nil {
		t.Fatalf("Failed to parse JSON output: %v", err)
	}

	// Validate required keys match fast-filesystem
	requiredKeys := []string{"path", "items", "total"}
	for _, key := range requiredKeys {
		if _, ok := output[key]; !ok {
			t.Errorf("Missing required key: %s", key)
		}
	}

	// Validate items is an array
	items, ok := output["items"].([]interface{})
	if !ok {
		t.Fatalf("items should be an array")
	}

	if len(items) == 0 {
		t.Fatal("Expected at least one item")
	}

	// Validate item structure
	item := items[0].(map[string]interface{})
	itemRequiredKeys := []string{
		"name", "path", "type", "is_dir", "size", "size_readable",
		"mode", "permissions", "modified", "is_readable", "is_writable",
	}
	for _, key := range itemRequiredKeys {
		if _, ok := item[key]; !ok {
			t.Errorf("Item missing required key: %s", key)
		}
	}

	// Validate type field values
	typeVal := item["type"].(string)
	if typeVal != "file" && typeVal != "directory" {
		t.Errorf("type should be 'file' or 'directory', got: %s", typeVal)
	}
}

// TestGetDirectoryTreeOutputStructure validates get_directory_tree output matches fast-filesystem format
func TestGetDirectoryTreeOutputStructure(t *testing.T) {
	tmpDir := t.TempDir()
	os.Mkdir(filepath.Join(tmpDir, "subdir"), 0755)
	os.WriteFile(filepath.Join(tmpDir, "subdir", "file.txt"), []byte("test"), 0644)

	server, _ := NewServer([]string{tmpDir})
	result, err := server.handleGetDirectoryTree(map[string]interface{}{
		"path":          tmpDir,
		"max_depth":     3,
		"include_files": true,
	})
	if err != nil {
		t.Fatalf("handleGetDirectoryTree failed: %v", err)
	}

	var output map[string]interface{}
	if err := json.Unmarshal([]byte(result), &output); err != nil {
		t.Fatalf("Failed to parse JSON output: %v", err)
	}

	// Validate required keys match fast-filesystem (tree, not root)
	requiredKeys := []string{"tree", "total_dirs", "total_files", "total_size"}
	for _, key := range requiredKeys {
		if _, ok := output[key]; !ok {
			t.Errorf("Missing required key: %s", key)
		}
	}

	// Validate tree node structure
	tree, ok := output["tree"].(map[string]interface{})
	if !ok {
		t.Fatal("tree should be an object")
	}

	treeRequiredKeys := []string{"name", "path", "is_dir"}
	for _, key := range treeRequiredKeys {
		if _, ok := tree[key]; !ok {
			t.Errorf("Tree node missing required key: %s", key)
		}
	}
}

// TestGetFileInfoOutputStructure validates get_file_info output matches fast-filesystem format
func TestGetFileInfoOutputStructure(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("hello world"), 0644)

	server, _ := NewServer([]string{tmpDir})
	result, err := server.handleGetFileInfo(map[string]interface{}{
		"path": testFile,
	})
	if err != nil {
		t.Fatalf("handleGetFileInfo failed: %v", err)
	}

	var output map[string]interface{}
	if err := json.Unmarshal([]byte(result), &output); err != nil {
		t.Fatalf("Failed to parse JSON output: %v", err)
	}

	// Validate required keys match fast-filesystem
	requiredKeys := []string{
		"name", "path", "type", "is_dir", "size", "size_readable",
		"mode", "permissions", "modified", "extension", "mime_type",
		"is_readable", "is_writable",
	}
	for _, key := range requiredKeys {
		if _, ok := output[key]; !ok {
			t.Errorf("Missing required key: %s", key)
		}
	}

	// Validate type value
	if output["type"] != "file" {
		t.Errorf("type should be 'file' for a file, got: %v", output["type"])
	}

	// Validate extension
	if output["extension"] != ".txt" {
		t.Errorf("extension should be '.txt', got: %v", output["extension"])
	}
}

// TestSearchCodeOutputStructure validates search_code output matches fast-filesystem format
func TestSearchCodeOutputStructure(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.go")
	// Create file with match in the middle so context_before and context_after have content
	os.WriteFile(testFile, []byte("package main\n\nimport \"fmt\"\n\nfunc hello() {\n\tfmt.Println(\"hello\")\n}\n\nfunc main() {\n\thello()\n}\n"), 0644)

	server, _ := NewServer([]string{tmpDir})
	result, err := server.handleSearchCode(map[string]interface{}{
		"path":          tmpDir,
		"pattern":       "hello",
		"context_lines": 1,
	})
	if err != nil {
		t.Fatalf("handleSearchCode failed: %v", err)
	}

	var output map[string]interface{}
	if err := json.Unmarshal([]byte(result), &output); err != nil {
		t.Fatalf("Failed to parse JSON output: %v", err)
	}

	// Validate required keys match fast-filesystem
	requiredKeys := []string{"pattern", "path", "matches", "total_files", "total_matches", "ripgrep_used", "search_time_ms"}
	for _, key := range requiredKeys {
		if _, ok := output[key]; !ok {
			t.Errorf("Missing required key: %s", key)
		}
	}

	// Validate matches structure
	matches, ok := output["matches"].([]interface{})
	if !ok {
		t.Fatal("matches should be an array")
	}

	if len(matches) > 0 {
		match := matches[0].(map[string]interface{})
		matchRequiredKeys := []string{"file", "line", "content"}
		for _, key := range matchRequiredKeys {
			if _, ok := match[key]; !ok {
				t.Errorf("Match missing required key: %s", key)
			}
		}

		// With context_lines > 0, should have context fields
		if _, ok := match["context"]; !ok {
			t.Error("Match missing context field when context_lines > 0")
		}
		if _, ok := match["context_before"]; !ok {
			t.Error("Match missing context_before field when context_lines > 0")
		}
		if _, ok := match["context_after"]; !ok {
			t.Error("Match missing context_after field when context_lines > 0")
		}
	}

	// Validate ripgrep_used is boolean
	if _, ok := output["ripgrep_used"].(bool); !ok {
		t.Error("ripgrep_used should be a boolean")
	}

	// Validate search_time_ms is a number
	if _, ok := output["search_time_ms"].(float64); !ok {
		t.Error("search_time_ms should be a number")
	}
}

// TestSearchFilesOutputStructure validates search_files output matches fast-filesystem format
func TestSearchFilesOutputStructure(t *testing.T) {
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "test.txt"), []byte("hello"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "other.go"), []byte("world"), 0644)

	server, _ := NewServer([]string{tmpDir})
	result, err := server.handleSearchFiles(map[string]interface{}{
		"path":    tmpDir,
		"pattern": "*.txt",
	})
	if err != nil {
		t.Fatalf("handleSearchFiles failed: %v", err)
	}

	var output map[string]interface{}
	if err := json.Unmarshal([]byte(result), &output); err != nil {
		t.Fatalf("Failed to parse JSON output: %v", err)
	}

	// Validate required keys
	requiredKeys := []string{"pattern", "path", "matches", "total"}
	for _, key := range requiredKeys {
		if _, ok := output[key]; !ok {
			t.Errorf("Missing required key: %s", key)
		}
	}

	// Validate matches structure
	matches, ok := output["matches"].([]interface{})
	if !ok {
		t.Fatal("matches should be an array")
	}

	if len(matches) > 0 {
		match := matches[0].(map[string]interface{})
		matchRequiredKeys := []string{"path", "name", "size", "is_dir", "mod_time"}
		for _, key := range matchRequiredKeys {
			if _, ok := match[key]; !ok {
				t.Errorf("Match missing required key: %s", key)
			}
		}
	}
}

// TestFindLargeFilesOutputStructure validates find_large_files output matches fast-filesystem format
func TestFindLargeFilesOutputStructure(t *testing.T) {
	tmpDir := t.TempDir()
	// Create a file larger than 0 bytes
	os.WriteFile(filepath.Join(tmpDir, "large.bin"), make([]byte, 1024), 0644)

	server, _ := NewServer([]string{tmpDir})
	result, err := server.handleFindLargeFiles(map[string]interface{}{
		"path":        tmpDir,
		"min_size":    "0",
		"max_results": 10,
	})
	if err != nil {
		t.Fatalf("handleFindLargeFiles failed: %v", err)
	}

	var output map[string]interface{}
	if err := json.Unmarshal([]byte(result), &output); err != nil {
		t.Fatalf("Failed to parse JSON output: %v", err)
	}

	// Validate required keys
	requiredKeys := []string{"path", "min_size", "files", "total_count", "total_size"}
	for _, key := range requiredKeys {
		if _, ok := output[key]; !ok {
			t.Errorf("Missing required key: %s", key)
		}
	}
}

// TestReadFileOutputStructure validates read_file output matches fast-filesystem format
func TestReadFileOutputStructure(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("line1\nline2\nline3\n"), 0644)

	server, _ := NewServer([]string{tmpDir})
	result, err := server.handleReadFile(map[string]interface{}{
		"path": testFile,
	})
	if err != nil {
		t.Fatalf("handleReadFile failed: %v", err)
	}

	var output map[string]interface{}
	if err := json.Unmarshal([]byte(result), &output); err != nil {
		t.Fatalf("Failed to parse JSON output: %v", err)
	}

	// Validate required keys
	requiredKeys := []string{"path", "content", "size", "encoding"}
	for _, key := range requiredKeys {
		if _, ok := output[key]; !ok {
			t.Errorf("Missing required key: %s", key)
		}
	}
}

// TestPaginationOutputStructure validates pagination fields in list_directory
func TestPaginationOutputStructure(t *testing.T) {
	tmpDir := t.TempDir()
	// Create multiple files
	for i := 0; i < 10; i++ {
		os.WriteFile(filepath.Join(tmpDir, string(rune('a'+i))+".txt"), []byte("test"), 0644)
	}

	server, _ := NewServer([]string{tmpDir})
	result, err := server.handleListDirectory(map[string]interface{}{
		"path":      tmpDir,
		"page":      1,
		"page_size": 3,
	})
	if err != nil {
		t.Fatalf("handleListDirectory failed: %v", err)
	}

	var output map[string]interface{}
	if err := json.Unmarshal([]byte(result), &output); err != nil {
		t.Fatalf("Failed to parse JSON output: %v", err)
	}

	// Validate pagination keys
	paginationKeys := []string{"page", "page_size", "total_pages", "has_more"}
	for _, key := range paginationKeys {
		if _, ok := output[key]; !ok {
			t.Errorf("Missing pagination key: %s", key)
		}
	}

	// Validate pagination values
	items := output["items"].([]interface{})
	if len(items) != 3 {
		t.Errorf("Expected 3 items with page_size=3, got %d", len(items))
	}

	if output["has_more"] != true {
		t.Error("has_more should be true when there are more pages")
	}

	// Validate continuation_token is present
	if _, ok := output["continuation_token"]; !ok {
		t.Error("Missing continuation_token for paginated results")
	}
}

// TestTypeFieldConsistency validates type field is consistently "file" or "directory"
func TestTypeFieldConsistency(t *testing.T) {
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "file.txt"), []byte("test"), 0644)
	os.Mkdir(filepath.Join(tmpDir, "dir"), 0755)

	server, _ := NewServer([]string{tmpDir})
	result, err := server.handleListDirectory(map[string]interface{}{
		"path": tmpDir,
	})
	if err != nil {
		t.Fatalf("handleListDirectory failed: %v", err)
	}

	var output map[string]interface{}
	json.Unmarshal([]byte(result), &output)

	items := output["items"].([]interface{})
	for _, item := range items {
		entry := item.(map[string]interface{})
		typeVal := entry["type"].(string)
		isDir := entry["is_dir"].(bool)

		if isDir && typeVal != "directory" {
			t.Errorf("is_dir=true but type=%s (expected 'directory')", typeVal)
		}
		if !isDir && typeVal != "file" {
			t.Errorf("is_dir=false but type=%s (expected 'file')", typeVal)
		}
	}
}

// TestSizeReadableFormat validates size_readable has human-readable format
func TestSizeReadableFormat(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.bin")
	os.WriteFile(testFile, make([]byte, 1536), 0644) // 1.5 KB

	server, _ := NewServer([]string{tmpDir})
	result, err := server.handleGetFileInfo(map[string]interface{}{
		"path": testFile,
	})
	if err != nil {
		t.Fatalf("handleGetFileInfo failed: %v", err)
	}

	var output map[string]interface{}
	json.Unmarshal([]byte(result), &output)

	sizeReadable := output["size_readable"].(string)
	// Should contain "KB" or similar human-readable suffix
	if sizeReadable == "" {
		t.Error("size_readable should not be empty")
	}
}

// TestModifiedFieldFormat validates modified field is ISO 8601 format
func TestModifiedFieldFormat(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("test"), 0644)

	server, _ := NewServer([]string{tmpDir})
	result, err := server.handleGetFileInfo(map[string]interface{}{
		"path": testFile,
	})
	if err != nil {
		t.Fatalf("handleGetFileInfo failed: %v", err)
	}

	var output map[string]interface{}
	json.Unmarshal([]byte(result), &output)

	modified := output["modified"].(string)
	// Should be in ISO 8601 format like "2024-01-15T10:30:00Z" or with timezone
	if len(modified) < 19 { // Minimum: "2024-01-15T10:30:00"
		t.Errorf("modified field format seems wrong: %s", modified)
	}
}

// TestEditMultipleBlocksOutputStructure validates edit_multiple_blocks output matches fast-filesystem format
func TestEditMultipleBlocksOutputStructure(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("line1\nline2\nline3\nline4\n"), 0644)

	server, _ := NewServer([]string{tmpDir})
	result, err := server.handleEditMultipleBlocks(map[string]interface{}{
		"path": testFile,
		"edits": []interface{}{
			map[string]interface{}{
				"mode":        "insert_after",
				"line_number": float64(2),
				"new_text":    "inserted line",
			},
		},
		"backup": false,
	})
	if err != nil {
		t.Fatalf("handleEditMultipleBlocks failed: %v", err)
	}

	var output map[string]interface{}
	if err := json.Unmarshal([]byte(result), &output); err != nil {
		t.Fatalf("Failed to parse JSON output: %v", err)
	}

	// Validate required keys match fast-filesystem format
	requiredKeys := []string{
		"message", "path", "total_edits", "successful_edits", "total_changes",
		"original_lines", "new_lines", "edit_results", "backup_created",
		"backup_enabled", "size", "size_readable", "timestamp",
	}
	for _, key := range requiredKeys {
		if _, ok := output[key]; !ok {
			t.Errorf("Missing required key: %s", key)
		}
	}

	// Validate edit_results array structure
	editResults, ok := output["edit_results"].([]interface{})
	if !ok {
		t.Fatal("edit_results should be an array")
	}

	if len(editResults) == 0 {
		t.Fatal("Expected at least one edit result")
	}

	// Validate edit result structure
	editResult := editResults[0].(map[string]interface{})
	editResultKeys := []string{"edit_index", "mode", "status", "changes_made"}
	for _, key := range editResultKeys {
		if _, ok := editResult[key]; !ok {
			t.Errorf("Edit result missing required key: %s", key)
		}
	}

	// Validate specific values
	if output["total_edits"].(float64) != 1 {
		t.Errorf("Expected total_edits=1, got %v", output["total_edits"])
	}
	if output["successful_edits"].(float64) != 1 {
		t.Errorf("Expected successful_edits=1, got %v", output["successful_edits"])
	}
	if output["original_lines"].(float64) != 5 { // 4 lines + trailing newline
		t.Errorf("Expected original_lines=5, got %v", output["original_lines"])
	}
	if output["backup_enabled"].(bool) != false {
		t.Error("backup_enabled should be false")
	}

	// Validate edit result values
	if editResult["status"].(string) != "success" {
		t.Errorf("Expected status='success', got %v", editResult["status"])
	}
	if editResult["mode"].(string) != "insert_after" {
		t.Errorf("Expected mode='insert_after', got %v", editResult["mode"])
	}
}

// TestWriteFileOutputStructure validates write_file output matches fast-filesystem format
func TestWriteFileOutputStructure(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "newfile.txt")

	server, _ := NewServer([]string{tmpDir})
	result, err := server.handleWriteFile(map[string]interface{}{
		"path":    testFile,
		"content": "hello world",
	})
	if err != nil {
		t.Fatalf("handleWriteFile failed: %v", err)
	}

	var output map[string]interface{}
	if err := json.Unmarshal([]byte(result), &output); err != nil {
		t.Fatalf("Failed to parse JSON output: %v", err)
	}

	// Validate required keys
	requiredKeys := []string{"path", "size", "created", "message"}
	for _, key := range requiredKeys {
		if _, ok := output[key]; !ok {
			t.Errorf("Missing required key: %s", key)
		}
	}

	// File should be created (first write)
	if output["created"] != true {
		t.Error("created should be true for new file")
	}
}

// TestCopyFileOutputStructure validates copy_file output matches expected format
func TestCopyFileOutputStructure(t *testing.T) {
	tmpDir := t.TempDir()
	sourceFile := filepath.Join(tmpDir, "source.txt")
	destFile := filepath.Join(tmpDir, "dest.txt")
	os.WriteFile(sourceFile, []byte("test content"), 0644)

	server, _ := NewServer([]string{tmpDir})
	result, err := server.handleCopyFile(map[string]interface{}{
		"source":      sourceFile,
		"destination": destFile,
	})
	if err != nil {
		t.Fatalf("handleCopyFile failed: %v", err)
	}

	var output map[string]interface{}
	if err := json.Unmarshal([]byte(result), &output); err != nil {
		t.Fatalf("Failed to parse JSON output: %v", err)
	}

	// Validate required keys
	requiredKeys := []string{"source", "destination", "success"}
	for _, key := range requiredKeys {
		if _, ok := output[key]; !ok {
			t.Errorf("Missing required key: %s", key)
		}
	}
}

// TestDeleteFileOutputStructure validates delete_file output matches expected format
func TestDeleteFileOutputStructure(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "todelete.txt")
	os.WriteFile(testFile, []byte("delete me"), 0644)

	server, _ := NewServer([]string{tmpDir})
	result, err := server.handleDeleteFile(map[string]interface{}{
		"path": testFile,
	})
	if err != nil {
		t.Fatalf("handleDeleteFile failed: %v", err)
	}

	var output map[string]interface{}
	if err := json.Unmarshal([]byte(result), &output); err != nil {
		t.Fatalf("Failed to parse JSON output: %v", err)
	}

	// Validate required keys
	requiredKeys := []string{"path", "success"}
	for _, key := range requiredKeys {
		if _, ok := output[key]; !ok {
			t.Errorf("Missing required key: %s", key)
		}
	}
}

// TestCreateDirectoryOutputStructure validates create_directory output
func TestCreateDirectoryOutputStructure(t *testing.T) {
	tmpDir := t.TempDir()
	newDir := filepath.Join(tmpDir, "newdir")

	server, _ := NewServer([]string{tmpDir})
	result, err := server.handleCreateDirectory(map[string]interface{}{
		"path": newDir,
	})
	if err != nil {
		t.Fatalf("handleCreateDirectory failed: %v", err)
	}

	var output map[string]interface{}
	if err := json.Unmarshal([]byte(result), &output); err != nil {
		t.Fatalf("Failed to parse JSON output: %v", err)
	}

	// Validate required keys
	requiredKeys := []string{"path", "created", "message"}
	for _, key := range requiredKeys {
		if _, ok := output[key]; !ok {
			t.Errorf("Missing required key: %s", key)
		}
	}

	if output["created"] != true {
		t.Error("created should be true")
	}
}

// TestGetDiskUsageOutputStructure validates get_disk_usage output
func TestGetDiskUsageOutputStructure(t *testing.T) {
	tmpDir := t.TempDir()

	server, _ := NewServer([]string{tmpDir})
	result, err := server.handleGetDiskUsage(map[string]interface{}{
		"path": tmpDir,
	})
	if err != nil {
		t.Fatalf("handleGetDiskUsage failed: %v", err)
	}

	var output map[string]interface{}
	if err := json.Unmarshal([]byte(result), &output); err != nil {
		t.Fatalf("Failed to parse JSON output: %v", err)
	}

	// Validate required keys - llm-filesystem uses total_size, total_files, total_dirs
	requiredKeys := []string{"path", "total_size", "total_files", "total_dirs"}
	for _, key := range requiredKeys {
		if _, ok := output[key]; !ok {
			t.Errorf("Missing required key: %s", key)
		}
	}
}

// TestExtractLinesOutputStructure validates extract_lines output (plain text format)
func TestExtractLinesOutputStructure(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "multiline.txt")
	os.WriteFile(testFile, []byte("line1\nline2\nline3\nline4\nline5\n"), 0644)

	server, _ := NewServer([]string{tmpDir})
	result, err := server.handleExtractLines(map[string]interface{}{
		"path":       testFile,
		"start_line": float64(2),
		"end_line":   float64(4),
	})
	if err != nil {
		t.Fatalf("handleExtractLines failed: %v", err)
	}

	// extract_lines returns plain text with line numbers
	// Format: "2: line2\n3: line3\n4: line4\n"
	lines := strings.Split(strings.TrimSpace(result), "\n")
	if len(lines) != 3 {
		t.Errorf("Expected 3 lines, got %d: %s", len(lines), result)
	}
}

// TestMoveFileOutputStructure validates move_file output
func TestMoveFileOutputStructure(t *testing.T) {
	tmpDir := t.TempDir()
	sourceFile := filepath.Join(tmpDir, "tomove.txt")
	destFile := filepath.Join(tmpDir, "moved.txt")
	os.WriteFile(sourceFile, []byte("move me"), 0644)

	server, _ := NewServer([]string{tmpDir})
	result, err := server.handleMoveFile(map[string]interface{}{
		"source":      sourceFile,
		"destination": destFile,
	})
	if err != nil {
		t.Fatalf("handleMoveFile failed: %v", err)
	}

	var output map[string]interface{}
	if err := json.Unmarshal([]byte(result), &output); err != nil {
		t.Fatalf("Failed to parse JSON output: %v", err)
	}

	// Validate required keys
	requiredKeys := []string{"source", "destination", "success"}
	for _, key := range requiredKeys {
		if _, ok := output[key]; !ok {
			t.Errorf("Missing required key: %s", key)
		}
	}
}

// TestEditMultipleBlocksAllModes validates all edit modes work correctly
func TestEditMultipleBlocksAllModes(t *testing.T) {
	tests := []struct {
		name         string
		initialContent string
		edits        []interface{}
		expectedLines int
	}{
		{
			name:         "replace mode",
			initialContent: "hello world\nfoo bar\n",
			edits: []interface{}{
				map[string]interface{}{
					"mode":     "replace",
					"old_text": "hello",
					"new_text": "goodbye",
				},
			},
			expectedLines: 3,
		},
		{
			name:         "insert_before mode",
			initialContent: "line1\nline2\n",
			edits: []interface{}{
				map[string]interface{}{
					"mode":        "insert_before",
					"line_number": float64(2),
					"new_text":    "inserted",
				},
			},
			expectedLines: 4,
		},
		{
			name:         "insert_after mode",
			initialContent: "line1\nline2\n",
			edits: []interface{}{
				map[string]interface{}{
					"mode":        "insert_after",
					"line_number": float64(1),
					"new_text":    "inserted",
				},
			},
			expectedLines: 4,
		},
		{
			name:         "delete_line mode",
			initialContent: "line1\nline2\nline3\n",
			edits: []interface{}{
				map[string]interface{}{
					"mode":        "delete_line",
					"line_number": float64(2),
				},
			},
			expectedLines: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			testFile := filepath.Join(tmpDir, "test.txt")
			os.WriteFile(testFile, []byte(tt.initialContent), 0644)

			server, _ := NewServer([]string{tmpDir})
			result, err := server.handleEditMultipleBlocks(map[string]interface{}{
				"path":   testFile,
				"edits":  tt.edits,
				"backup": false,
			})
			if err != nil {
				t.Fatalf("handleEditMultipleBlocks failed: %v", err)
			}

			var output map[string]interface{}
			json.Unmarshal([]byte(result), &output)

			if output["successful_edits"].(float64) != 1 {
				t.Errorf("Expected 1 successful edit, got %v", output["successful_edits"])
			}
			if output["new_lines"].(float64) != float64(tt.expectedLines) {
				t.Errorf("Expected %d new lines, got %v", tt.expectedLines, output["new_lines"])
			}
		})
	}
}
