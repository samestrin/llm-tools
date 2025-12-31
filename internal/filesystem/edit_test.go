package filesystem

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEditBlock(t *testing.T) {
	tmpDir := t.TempDir()

	server, _ := NewServer([]string{tmpDir})

	tests := []struct {
		name        string
		setup       func() string
		args        map[string]interface{}
		wantContent string
		wantErr     bool
	}{
		{
			name: "simple string replacement",
			setup: func() string {
				path := filepath.Join(tmpDir, "test1.txt")
				os.WriteFile(path, []byte("Hello World"), 0644)
				return path
			},
			args: map[string]interface{}{
				"old_string": "World",
				"new_string": "Go",
			},
			wantContent: "Hello Go",
			wantErr:     false,
		},
		{
			name: "multi-line replacement",
			setup: func() string {
				path := filepath.Join(tmpDir, "test2.txt")
				os.WriteFile(path, []byte("func old() {\n\treturn nil\n}"), 0644)
				return path
			},
			args: map[string]interface{}{
				"old_string": "func old() {\n\treturn nil\n}",
				"new_string": "func new() {\n\treturn err\n}",
			},
			wantContent: "func new() {\n\treturn err\n}",
			wantErr:     false,
		},
		{
			name: "old string not found",
			setup: func() string {
				path := filepath.Join(tmpDir, "test3.txt")
				os.WriteFile(path, []byte("Hello World"), 0644)
				return path
			},
			args: map[string]interface{}{
				"old_string": "Goodbye",
				"new_string": "Hi",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := tt.setup()
			tt.args["path"] = path

			result, err := server.handleEditBlock(tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("handleEditBlock() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}

			// Verify result contains success
			if !strings.Contains(result, "success") && !strings.Contains(result, "true") {
				t.Errorf("handleEditBlock() result should indicate success")
			}

			// Verify file content
			content, _ := os.ReadFile(path)
			if string(content) != tt.wantContent {
				t.Errorf("File content = %q, want %q", string(content), tt.wantContent)
			}
		})
	}
}

func TestEditBlocks(t *testing.T) {
	tmpDir := t.TempDir()

	server, _ := NewServer([]string{tmpDir})

	tests := []struct {
		name        string
		setup       func() string
		args        map[string]interface{}
		wantContent string
		wantErr     bool
	}{
		{
			name: "multiple replacements",
			setup: func() string {
				path := filepath.Join(tmpDir, "multi1.txt")
				os.WriteFile(path, []byte("Hello World\nGoodbye World"), 0644)
				return path
			},
			args: map[string]interface{}{
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
			},
			wantContent: "Hi World\nBye World",
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := tt.setup()
			tt.args["path"] = path

			result, err := server.handleEditBlocks(tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("handleEditBlocks() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}

			_ = result

			// Verify file content
			content, _ := os.ReadFile(path)
			if string(content) != tt.wantContent {
				t.Errorf("File content = %q, want %q", string(content), tt.wantContent)
			}
		})
	}
}

func TestSafeEdit(t *testing.T) {
	tmpDir := t.TempDir()

	server, _ := NewServer([]string{tmpDir})

	tests := []struct {
		name        string
		setup       func() string
		args        map[string]interface{}
		wantContent string
		wantBackup  bool
		wantErr     bool
	}{
		{
			name: "edit with backup",
			setup: func() string {
				path := filepath.Join(tmpDir, "safe1.txt")
				os.WriteFile(path, []byte("Original content"), 0644)
				return path
			},
			args: map[string]interface{}{
				"old_string": "Original",
				"new_string": "Modified",
				"backup":     true,
			},
			wantContent: "Modified content",
			wantBackup:  true,
			wantErr:     false,
		},
		{
			name: "edit with dry run",
			setup: func() string {
				path := filepath.Join(tmpDir, "safe2.txt")
				os.WriteFile(path, []byte("Original content"), 0644)
				return path
			},
			args: map[string]interface{}{
				"old_string": "Original",
				"new_string": "Modified",
				"dry_run":    true,
			},
			wantContent: "Original content", // Should not change in dry run
			wantBackup:  false,
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := tt.setup()
			tt.args["path"] = path

			result, err := server.handleSafeEdit(tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("handleSafeEdit() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}

			_ = result

			// Verify file content
			content, _ := os.ReadFile(path)
			if string(content) != tt.wantContent {
				t.Errorf("File content = %q, want %q", string(content), tt.wantContent)
			}

			// Check for backup file
			if tt.wantBackup {
				matches, _ := filepath.Glob(path + ".bak.*")
				if len(matches) == 0 {
					t.Error("Expected backup file to be created")
				}
			}
		})
	}
}

func TestSearchAndReplace(t *testing.T) {
	tmpDir := t.TempDir()

	server, _ := NewServer([]string{tmpDir})

	tests := []struct {
		name         string
		setup        func() []string
		args         map[string]interface{}
		wantContains []string
		wantErr      bool
	}{
		{
			name: "simple replace across files",
			setup: func() []string {
				path1 := filepath.Join(tmpDir, "sr1.txt")
				path2 := filepath.Join(tmpDir, "sr2.txt")
				os.WriteFile(path1, []byte("foo bar foo"), 0644)
				os.WriteFile(path2, []byte("foo baz"), 0644)
				return []string{path1, path2}
			},
			args: map[string]interface{}{
				"path":        tmpDir,
				"pattern":     "foo",
				"replacement": "qux",
			},
			wantContains: []string{"modified"},
			wantErr:      false,
		},
		{
			name: "regex replace",
			setup: func() []string {
				path := filepath.Join(tmpDir, "sr3.txt")
				os.WriteFile(path, []byte("func test1() {}\nfunc test2() {}"), 0644)
				return []string{path}
			},
			args: map[string]interface{}{
				"path":        tmpDir,
				"pattern":     `func test(\d+)`,
				"replacement": "func example$1",
				"regex":       true,
			},
			wantContains: []string{"modified"},
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_ = tt.setup()

			result, err := server.handleSearchAndReplace(tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("handleSearchAndReplace() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}

			for _, want := range tt.wantContains {
				if !strings.Contains(strings.ToLower(result), want) {
					t.Errorf("handleSearchAndReplace() = %v, want to contain %v", result, want)
				}
			}
		})
	}
}

func TestEditMultipleBlocks(t *testing.T) {
	tmpDir := t.TempDir()
	server, _ := NewServer([]string{tmpDir})

	tests := []struct {
		name        string
		setup       func() string
		args        map[string]interface{}
		wantContent string
		wantErr     bool
	}{
		{
			name: "replace mode with old_text/new_text",
			setup: func() string {
				path := filepath.Join(tmpDir, "multi1.txt")
				os.WriteFile(path, []byte("Hello World"), 0644)
				return path
			},
			args: map[string]interface{}{
				"backup": false,
				"edits": []interface{}{
					map[string]interface{}{
						"old_text": "Hello",
						"new_text": "Hi",
						"mode":     "replace",
					},
				},
			},
			wantContent: "Hi World",
			wantErr:     false,
		},
		{
			name: "replace mode with old_string/new_string (backwards compat)",
			setup: func() string {
				path := filepath.Join(tmpDir, "multi2.txt")
				os.WriteFile(path, []byte("Hello World"), 0644)
				return path
			},
			args: map[string]interface{}{
				"backup": false,
				"edits": []interface{}{
					map[string]interface{}{
						"old_string": "Hello",
						"new_string": "Hi",
					},
				},
			},
			wantContent: "Hi World",
			wantErr:     false,
		},
		{
			name: "insert_before mode",
			setup: func() string {
				path := filepath.Join(tmpDir, "multi3.txt")
				os.WriteFile(path, []byte("line1\nline2\nline3"), 0644)
				return path
			},
			args: map[string]interface{}{
				"backup": false,
				"edits": []interface{}{
					map[string]interface{}{
						"line_number": float64(2),
						"new_text":    "inserted",
						"mode":        "insert_before",
					},
				},
			},
			wantContent: "line1\ninserted\nline2\nline3",
			wantErr:     false,
		},
		{
			name: "insert_after mode",
			setup: func() string {
				path := filepath.Join(tmpDir, "multi4.txt")
				os.WriteFile(path, []byte("line1\nline2\nline3"), 0644)
				return path
			},
			args: map[string]interface{}{
				"backup": false,
				"edits": []interface{}{
					map[string]interface{}{
						"line_number": float64(2),
						"new_text":    "inserted",
						"mode":        "insert_after",
					},
				},
			},
			wantContent: "line1\nline2\ninserted\nline3",
			wantErr:     false,
		},
		{
			name: "delete_line mode",
			setup: func() string {
				path := filepath.Join(tmpDir, "multi5.txt")
				os.WriteFile(path, []byte("line1\nline2\nline3"), 0644)
				return path
			},
			args: map[string]interface{}{
				"backup": false,
				"edits": []interface{}{
					map[string]interface{}{
						"line_number": float64(2),
						"mode":        "delete_line",
					},
				},
			},
			wantContent: "line1\nline3",
			wantErr:     false,
		},
		{
			name: "multiple mixed operations",
			setup: func() string {
				path := filepath.Join(tmpDir, "multi6.txt")
				os.WriteFile(path, []byte("Hello World\nline2\nline3"), 0644)
				return path
			},
			args: map[string]interface{}{
				"backup": false,
				"edits": []interface{}{
					map[string]interface{}{
						"old_text": "Hello",
						"new_text": "Hi",
						"mode":     "replace",
					},
					map[string]interface{}{
						"line_number": float64(2),
						"new_text":    "inserted",
						"mode":        "insert_after",
					},
				},
			},
			wantContent: "Hi World\nline2\ninserted\nline3",
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := tt.setup()
			tt.args["path"] = path

			result, err := server.handleEditMultipleBlocks(tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("handleEditMultipleBlocks() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}

			if !strings.Contains(result, "success") && !strings.Contains(result, "true") {
				t.Errorf("handleEditMultipleBlocks() should indicate success, got %s", result)
			}

			content, _ := os.ReadFile(path)
			if string(content) != tt.wantContent {
				t.Errorf("File content = %q, want %q", string(content), tt.wantContent)
			}
		})
	}
}

func TestEditFile(t *testing.T) {
	tmpDir := t.TempDir()

	server, _ := NewServer([]string{tmpDir})

	tests := []struct {
		name        string
		setup       func() string
		args        map[string]interface{}
		wantContent string
		wantErr     bool
	}{
		{
			name: "insert at line",
			setup: func() string {
				path := filepath.Join(tmpDir, "ef1.txt")
				os.WriteFile(path, []byte("line1\nline2\nline3"), 0644)
				return path
			},
			args: map[string]interface{}{
				"operation": "insert",
				"line":      float64(2),
				"content":   "inserted",
			},
			wantContent: "line1\ninserted\nline2\nline3",
			wantErr:     false,
		},
		{
			name: "delete line",
			setup: func() string {
				path := filepath.Join(tmpDir, "ef2.txt")
				os.WriteFile(path, []byte("line1\nline2\nline3"), 0644)
				return path
			},
			args: map[string]interface{}{
				"operation": "delete",
				"line":      float64(2),
			},
			wantContent: "line1\nline3",
			wantErr:     false,
		},
		{
			name: "replace line",
			setup: func() string {
				path := filepath.Join(tmpDir, "ef3.txt")
				os.WriteFile(path, []byte("line1\nline2\nline3"), 0644)
				return path
			},
			args: map[string]interface{}{
				"operation": "replace",
				"line":      float64(2),
				"content":   "replaced",
			},
			wantContent: "line1\nreplaced\nline3",
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := tt.setup()
			tt.args["path"] = path

			result, err := server.handleEditFile(tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("handleEditFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}

			_ = result

			// Verify file content
			content, _ := os.ReadFile(path)
			if string(content) != tt.wantContent {
				t.Errorf("File content = %q, want %q", string(content), tt.wantContent)
			}
		})
	}
}

func TestEditBlockErrorCases(t *testing.T) {
	tmpDir := t.TempDir()
	server, _ := NewServer([]string{tmpDir})

	tests := []struct {
		name    string
		args    map[string]interface{}
		wantErr bool
	}{
		{
			name:    "missing path",
			args:    map[string]interface{}{},
			wantErr: true,
		},
		{
			name: "missing old_string",
			args: map[string]interface{}{
				"path":       filepath.Join(tmpDir, "test.txt"),
				"new_string": "new",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := server.handleEditBlock(tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("handleEditBlock() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestEditFileErrorCases(t *testing.T) {
	tmpDir := t.TempDir()
	server, _ := NewServer([]string{tmpDir})

	tests := []struct {
		name    string
		args    map[string]interface{}
		wantErr bool
	}{
		{
			name:    "missing path",
			args:    map[string]interface{}{},
			wantErr: true,
		},
		{
			name: "missing operation",
			args: map[string]interface{}{
				"path": filepath.Join(tmpDir, "test.txt"),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := server.handleEditFile(tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("handleEditFile() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestEditBlocksErrorCases(t *testing.T) {
	tmpDir := t.TempDir()
	server, _ := NewServer([]string{tmpDir})

	tests := []struct {
		name    string
		args    map[string]interface{}
		wantErr bool
	}{
		{
			name:    "missing path",
			args:    map[string]interface{}{},
			wantErr: true,
		},
		{
			name: "missing edits",
			args: map[string]interface{}{
				"path": filepath.Join(tmpDir, "test.txt"),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := server.handleEditBlocks(tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("handleEditBlocks() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSafeEditErrorCases(t *testing.T) {
	tmpDir := t.TempDir()
	server, _ := NewServer([]string{tmpDir})

	tests := []struct {
		name    string
		args    map[string]interface{}
		wantErr bool
	}{
		{
			name:    "missing path",
			args:    map[string]interface{}{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := server.handleSafeEdit(tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("handleSafeEdit() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSearchAndReplaceErrorCases(t *testing.T) {
	tmpDir := t.TempDir()
	server, _ := NewServer([]string{tmpDir})

	tests := []struct {
		name    string
		args    map[string]interface{}
		wantErr bool
	}{
		{
			name:    "missing path",
			args:    map[string]interface{}{},
			wantErr: true,
		},
		{
			name: "missing pattern",
			args: map[string]interface{}{
				"path":        tmpDir,
				"replacement": "new",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := server.handleSearchAndReplace(tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("handleSearchAndReplace() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSearchAndReplaceDryRun(t *testing.T) {
	tmpDir := t.TempDir()
	server, _ := NewServer([]string{tmpDir})

	// Create test file
	testFile := filepath.Join(tmpDir, "dryrun.txt")
	originalContent := "hello world hello"
	os.WriteFile(testFile, []byte(originalContent), 0644)

	// Run with dry_run = true
	result, err := server.handleSearchAndReplace(map[string]interface{}{
		"path":        tmpDir,
		"pattern":     "hello",
		"replacement": "goodbye",
		"dry_run":     true,
	})
	if err != nil {
		t.Errorf("handleSearchAndReplace() dry_run error = %v", err)
	}

	// Verify file was NOT modified
	content, _ := os.ReadFile(testFile)
	if string(content) != originalContent {
		t.Errorf("Dry run should not modify file, got %s", content)
	}

	// Result should show changes would be made
	if !strings.Contains(result, "Modified") {
		t.Errorf("Result should indicate modifications: %s", result)
	}
}

func TestSearchAndReplaceFileTypes(t *testing.T) {
	tmpDir := t.TempDir()
	server, _ := NewServer([]string{tmpDir})

	// Create files with different extensions
	goFile := filepath.Join(tmpDir, "test.go")
	txtFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(goFile, []byte("foo bar foo"), 0644)
	os.WriteFile(txtFile, []byte("foo bar foo"), 0644)

	// Only replace in .go files
	_, err := server.handleSearchAndReplace(map[string]interface{}{
		"path":        tmpDir,
		"pattern":     "foo",
		"replacement": "baz",
		"file_types":  []interface{}{".go"},
	})
	if err != nil {
		t.Errorf("handleSearchAndReplace() file_types error = %v", err)
	}

	// Verify .go file was modified
	goContent, _ := os.ReadFile(goFile)
	if !strings.Contains(string(goContent), "baz") {
		t.Errorf(".go file should be modified")
	}

	// Verify .txt file was NOT modified
	txtContent, _ := os.ReadFile(txtFile)
	if strings.Contains(string(txtContent), "baz") {
		t.Errorf(".txt file should not be modified")
	}
}

func TestSearchAndReplaceInvalidRegex(t *testing.T) {
	tmpDir := t.TempDir()
	server, _ := NewServer([]string{tmpDir})

	_, err := server.handleSearchAndReplace(map[string]interface{}{
		"path":        tmpDir,
		"pattern":     "[invalid(regex",
		"replacement": "new",
		"regex":       true,
	})
	if err == nil {
		t.Error("handleSearchAndReplace() should error on invalid regex")
	}
}

func TestEditFileAppendMode(t *testing.T) {
	tmpDir := t.TempDir()
	server, _ := NewServer([]string{tmpDir})

	testFile := filepath.Join(tmpDir, "append_test.txt")
	os.WriteFile(testFile, []byte("line1\nline2\nline3"), 0644)

	// Test append operation
	result, err := server.handleEditFile(map[string]interface{}{
		"path":      testFile,
		"operation": "insert",
		"line":      float64(4),
		"content":   "line4",
	})
	if err != nil {
		t.Errorf("handleEditFile() append error = %v", err)
	}

	content, _ := os.ReadFile(testFile)
	if !strings.Contains(string(content), "line4") {
		t.Errorf("File should contain line4: %s", string(content))
	}

	_ = result
}
