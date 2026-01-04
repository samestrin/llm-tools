package filesystem

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExecuteHandler(t *testing.T) {
	tmpDir := t.TempDir()
	server, _ := NewServer([]string{tmpDir})

	// Create test file
	testFile := filepath.Join(tmpDir, "handler_test.txt")
	os.WriteFile(testFile, []byte("test content"), 0644)

	tests := []struct {
		name         string
		toolName     string
		args         map[string]interface{}
		wantContains string
		wantErr      bool
	}{
		{
			name:         "llm_filesystem_list_allowed_directories",
			toolName:     "llm_filesystem_list_allowed_directories",
			args:         map[string]interface{}{},
			wantContains: "allowed_directories",
			wantErr:      false,
		},
		{
			name:     "llm_filesystem_read_file",
			toolName: "llm_filesystem_read_file",
			args: map[string]interface{}{
				"path": testFile,
			},
			wantContains: "test content",
			wantErr:      false,
		},
		{
			name:     "llm_filesystem_get_file_info",
			toolName: "llm_filesystem_get_file_info",
			args: map[string]interface{}{
				"path": testFile,
			},
			wantContains: "handler_test.txt",
			wantErr:      false,
		},
		{
			name:     "llm_filesystem_list_directory",
			toolName: "llm_filesystem_list_directory",
			args: map[string]interface{}{
				"path": tmpDir,
			},
			wantContains: "items",
			wantErr:      false,
		},
		{
			name:     "llm_filesystem_get_directory_tree",
			toolName: "llm_filesystem_get_directory_tree",
			args: map[string]interface{}{
				"path": tmpDir,
			},
			wantContains: "tree",
			wantErr:      false,
		},
		{
			name:     "llm_filesystem_get_disk_usage",
			toolName: "llm_filesystem_get_disk_usage",
			args: map[string]interface{}{
				"path": tmpDir,
			},
			wantContains: "total_size",
			wantErr:      false,
		},
		{
			name:     "unknown tool",
			toolName: "unknown_tool",
			args:     map[string]interface{}{},
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := server.ExecuteHandler(tt.toolName, tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("ExecuteHandler(%s) error = %v, wantErr %v", tt.toolName, err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			if !strings.Contains(result, tt.wantContains) {
				t.Errorf("ExecuteHandler(%s) = %v, want to contain %v", tt.toolName, result, tt.wantContains)
			}
		})
	}
}

func TestExecuteHandlerAllTools(t *testing.T) {
	tmpDir := t.TempDir()
	server, _ := NewServer([]string{tmpDir})

	// Create test files for various operations
	testFile := filepath.Join(tmpDir, "all_tools_test.txt")
	os.WriteFile(testFile, []byte("original content"), 0644)

	// Test each tool to ensure dispatch works
	toolTests := []struct {
		toolName string
		args     map[string]interface{}
	}{
		{"llm_filesystem_write_file", map[string]interface{}{"path": filepath.Join(tmpDir, "write_test.txt"), "content": "test"}},
		{"llm_filesystem_create_directory", map[string]interface{}{"path": filepath.Join(tmpDir, "test_dir")}},
		{"llm_filesystem_create_directories", map[string]interface{}{"paths": []interface{}{filepath.Join(tmpDir, "test_dir1"), filepath.Join(tmpDir, "test_dir2")}}},
		{"llm_filesystem_search_files", map[string]interface{}{"path": tmpDir, "pattern": "*.txt"}},
		{"llm_filesystem_search_code", map[string]interface{}{"path": tmpDir, "pattern": "content"}},
		{"llm_filesystem_find_large_files", map[string]interface{}{"path": tmpDir}},
		{"llm_filesystem_extract_lines", map[string]interface{}{"path": testFile, "start_line": float64(1), "end_line": float64(1)}},
	}

	for _, tt := range toolTests {
		t.Run(tt.toolName, func(t *testing.T) {
			_, err := server.ExecuteHandler(tt.toolName, tt.args)
			if err != nil {
				t.Errorf("ExecuteHandler(%s) error = %v", tt.toolName, err)
			}
		})
	}
}

func TestExecuteHandlerEditTools(t *testing.T) {
	tmpDir := t.TempDir()
	server, _ := NewServer([]string{tmpDir})

	// Create test file for edit operations
	testFile := filepath.Join(tmpDir, "edit_dispatch_test.txt")
	os.WriteFile(testFile, []byte("Hello World"), 0644)

	tests := []struct {
		toolName string
		args     map[string]interface{}
		wantErr  bool
	}{
		{
			toolName: "llm_filesystem_edit_block",
			args: map[string]interface{}{
				"path":       testFile,
				"old_string": "World",
				"new_string": "Go",
			},
			wantErr: false,
		},
		{
			toolName: "llm_filesystem_edit_file",
			args: map[string]interface{}{
				"path":      testFile,
				"operation": "replace",
				"line":      float64(1),
				"content":   "Replaced line",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.toolName, func(t *testing.T) {
			// Recreate file for each test
			os.WriteFile(testFile, []byte("Hello World"), 0644)

			_, err := server.ExecuteHandler(tt.toolName, tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("ExecuteHandler(%s) error = %v, wantErr %v", tt.toolName, err, tt.wantErr)
			}
		})
	}
}

func TestExecuteHandlerFileOps(t *testing.T) {
	tmpDir := t.TempDir()
	server, _ := NewServer([]string{tmpDir})

	// Test move file via ExecuteHandler
	srcFile := filepath.Join(tmpDir, "move_test.txt")
	dstFile := filepath.Join(tmpDir, "moved.txt")
	os.WriteFile(srcFile, []byte("move content"), 0644)

	_, err := server.ExecuteHandler("llm_filesystem_move_file", map[string]interface{}{
		"source":      srcFile,
		"destination": dstFile,
	})
	if err != nil {
		t.Errorf("llm_filesystem_move_file error = %v", err)
	}

	// Test batch file operations
	os.WriteFile(filepath.Join(tmpDir, "batch_src.txt"), []byte("batch"), 0644)
	_, err = server.ExecuteHandler("llm_filesystem_batch_file_operations", map[string]interface{}{
		"operations": []interface{}{
			map[string]interface{}{
				"operation":   "copy",
				"source":      filepath.Join(tmpDir, "batch_src.txt"),
				"destination": filepath.Join(tmpDir, "batch_dst.txt"),
			},
		},
	})
	if err != nil {
		t.Errorf("llm_filesystem_batch_file_operations error = %v", err)
	}

	// Test delete file
	deleteFile := filepath.Join(tmpDir, "to_delete.txt")
	os.WriteFile(deleteFile, []byte("delete"), 0644)
	_, err = server.ExecuteHandler("llm_filesystem_delete_file", map[string]interface{}{
		"path": deleteFile,
	})
	if err != nil {
		t.Errorf("llm_filesystem_delete_file error = %v", err)
	}
}

func TestExecuteHandlerAdvanced(t *testing.T) {
	tmpDir := t.TempDir()
	server, _ := NewServer([]string{tmpDir})

	// Create test files
	os.WriteFile(filepath.Join(tmpDir, "compress1.txt"), []byte("file1"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "compress2.txt"), []byte("file2"), 0644)

	// Test compress files
	_, err := server.ExecuteHandler("llm_filesystem_compress_files", map[string]interface{}{
		"paths":  []interface{}{filepath.Join(tmpDir, "compress1.txt"), filepath.Join(tmpDir, "compress2.txt")},
		"output": filepath.Join(tmpDir, "archive.zip"),
		"format": "zip",
	})
	if err != nil {
		t.Errorf("llm_filesystem_compress_files error = %v", err)
	}

	// Test extract archive
	_, err = server.ExecuteHandler("llm_filesystem_extract_archive", map[string]interface{}{
		"archive":     filepath.Join(tmpDir, "archive.zip"),
		"destination": filepath.Join(tmpDir, "extracted"),
	})
	if err != nil {
		t.Errorf("llm_filesystem_extract_archive error = %v", err)
	}

	// Test sync directories
	os.MkdirAll(filepath.Join(tmpDir, "src_dir"), 0755)
	os.WriteFile(filepath.Join(tmpDir, "src_dir", "sync.txt"), []byte("sync"), 0644)
	_, err = server.ExecuteHandler("llm_filesystem_sync_directories", map[string]interface{}{
		"source":      filepath.Join(tmpDir, "src_dir"),
		"destination": filepath.Join(tmpDir, "dst_dir"),
	})
	if err != nil {
		t.Errorf("llm_filesystem_sync_directories error = %v", err)
	}
}

func TestExecuteHandlerSafeEdit(t *testing.T) {
	tmpDir := t.TempDir()
	server, _ := NewServer([]string{tmpDir})

	testFile := filepath.Join(tmpDir, "safe_edit_test.txt")
	os.WriteFile(testFile, []byte("Original content here"), 0644)

	// Test safe_edit via handler
	_, err := server.ExecuteHandler("llm_filesystem_safe_edit", map[string]interface{}{
		"path":       testFile,
		"old_string": "Original",
		"new_string": "Modified",
	})
	if err != nil {
		t.Errorf("llm_filesystem_safe_edit error = %v", err)
	}

	// Verify file content changed
	content, _ := os.ReadFile(testFile)
	if !strings.Contains(string(content), "Modified") {
		t.Errorf("File should contain 'Modified', got: %s", string(content))
	}
}

func TestExecuteHandlerEditBlocks(t *testing.T) {
	tmpDir := t.TempDir()
	server, _ := NewServer([]string{tmpDir})

	testFile := filepath.Join(tmpDir, "edit_blocks_test.txt")
	os.WriteFile(testFile, []byte("Hello World\nGoodbye World"), 0644)

	_, err := server.ExecuteHandler("llm_filesystem_edit_blocks", map[string]interface{}{
		"path": testFile,
		"edits": []interface{}{
			map[string]interface{}{
				"old_string": "Hello",
				"new_string": "Hi",
			},
			map[string]interface{}{
				"old_string": "Goodbye",
				"new_string": "Bye",
			},
		},
	})
	if err != nil {
		t.Errorf("llm_filesystem_edit_blocks error = %v", err)
	}

	content, _ := os.ReadFile(testFile)
	if !strings.Contains(string(content), "Hi") || !strings.Contains(string(content), "Bye") {
		t.Errorf("File content = %s, want to contain 'Hi' and 'Bye'", string(content))
	}
}

func TestExecuteHandlerMultipleBlocks(t *testing.T) {
	tmpDir := t.TempDir()
	server, _ := NewServer([]string{tmpDir})

	testFile := filepath.Join(tmpDir, "multi_blocks_test.txt")
	os.WriteFile(testFile, []byte("Test content"), 0644)

	_, err := server.ExecuteHandler("llm_filesystem_edit_multiple_blocks", map[string]interface{}{
		"path": testFile,
		"edits": []interface{}{
			map[string]interface{}{
				"old_string": "Test",
				"new_string": "Demo",
			},
		},
	})
	if err != nil {
		t.Errorf("llm_filesystem_edit_multiple_blocks error = %v", err)
	}
}

func TestExecuteHandlerReadMultiple(t *testing.T) {
	tmpDir := t.TempDir()
	server, _ := NewServer([]string{tmpDir})

	file1 := filepath.Join(tmpDir, "read1.txt")
	file2 := filepath.Join(tmpDir, "read2.txt")
	os.WriteFile(file1, []byte("content 1"), 0644)
	os.WriteFile(file2, []byte("content 2"), 0644)

	result, err := server.ExecuteHandler("llm_filesystem_read_multiple_files", map[string]interface{}{
		"paths": []interface{}{file1, file2},
	})
	if err != nil {
		t.Errorf("llm_filesystem_read_multiple_files error = %v", err)
	}
	if !strings.Contains(result, "content 1") || !strings.Contains(result, "content 2") {
		t.Errorf("Result should contain both contents: %s", result)
	}
}

func TestExecuteHandlerExtractLines(t *testing.T) {
	tmpDir := t.TempDir()
	server, _ := NewServer([]string{tmpDir})

	testFile := filepath.Join(tmpDir, "extract.txt")
	os.WriteFile(testFile, []byte("line1\nline2\nline3\nline4\n"), 0644)

	result, err := server.ExecuteHandler("llm_filesystem_extract_lines", map[string]interface{}{
		"path":       testFile,
		"start_line": float64(2),
		"end_line":   float64(3),
	})
	if err != nil {
		t.Errorf("llm_filesystem_extract_lines error = %v", err)
	}
	if !strings.Contains(result, "line2") {
		t.Errorf("Result should contain line2: %s", result)
	}
}

func TestExecuteHandlerSearchAndReplace(t *testing.T) {
	tmpDir := t.TempDir()
	server, _ := NewServer([]string{tmpDir})

	testFile := filepath.Join(tmpDir, "replace_test.txt")
	os.WriteFile(testFile, []byte("foo bar foo"), 0644)

	result, err := server.ExecuteHandler("llm_filesystem_search_and_replace", map[string]interface{}{
		"path":        tmpDir,
		"pattern":     "foo",
		"replacement": "baz",
	})
	if err != nil {
		t.Errorf("llm_filesystem_search_and_replace error = %v", err)
	}

	_ = result
}
