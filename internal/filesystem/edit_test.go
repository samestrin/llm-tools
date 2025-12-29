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
