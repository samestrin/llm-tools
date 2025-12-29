package filesystem

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteFile(t *testing.T) {
	tmpDir := t.TempDir()
	server, _ := NewServer([]string{tmpDir})

	tests := []struct {
		name        string
		args        map[string]interface{}
		wantContent string
		wantErr     bool
	}{
		{
			name: "create new file",
			args: map[string]interface{}{
				"path":    filepath.Join(tmpDir, "new.txt"),
				"content": "new content",
			},
			wantContent: "new content",
			wantErr:     false,
		},
		{
			name: "overwrite existing file",
			args: map[string]interface{}{
				"path":    filepath.Join(tmpDir, "overwrite.txt"),
				"content": "overwritten",
			},
			wantContent: "overwritten",
			wantErr:     false,
		},
		{
			name: "create with parent directories",
			args: map[string]interface{}{
				"path":        filepath.Join(tmpDir, "sub", "dir", "file.txt"),
				"content":     "nested content",
				"create_dirs": true,
			},
			wantContent: "nested content",
			wantErr:     false,
		},
		{
			name: "append mode",
			args: map[string]interface{}{
				"path":    filepath.Join(tmpDir, "append.txt"),
				"content": " appended",
				"append":  true,
			},
			wantErr: false,
		},
	}

	// Create file for overwrite test
	os.WriteFile(filepath.Join(tmpDir, "overwrite.txt"), []byte("original"), 0644)

	// Create file for append test
	os.WriteFile(filepath.Join(tmpDir, "append.txt"), []byte("original"), 0644)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := server.handleWriteFile(tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("handleWriteFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}

			// Verify file was written
			path := tt.args["path"].(string)
			content, err := os.ReadFile(path)
			if err != nil {
				t.Errorf("Failed to read written file: %v", err)
				return
			}

			if tt.name == "append mode" {
				if !strings.Contains(string(content), "appended") {
					t.Errorf("Append failed: got %q", string(content))
				}
			} else if string(content) != tt.wantContent {
				t.Errorf("File content = %q, want %q", string(content), tt.wantContent)
			}

			_ = result // Suppress unused warning
		})
	}
}

func TestWriteFileBackup(t *testing.T) {
	tmpDir := t.TempDir()
	server, _ := NewServer([]string{tmpDir})

	// Create existing file
	existingFile := filepath.Join(tmpDir, "backup_test.txt")
	originalContent := "original content"
	if err := os.WriteFile(existingFile, []byte(originalContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Write with backup
	args := map[string]interface{}{
		"path":    existingFile,
		"content": "new content",
		"backup":  true,
	}

	_, err := server.handleLargeWriteFile(args) // Use large write for backup support
	if err != nil {
		t.Errorf("handleLargeWriteFile() error = %v", err)
		return
	}

	// Check backup exists
	matches, _ := filepath.Glob(existingFile + ".bak*")
	if len(matches) == 0 {
		t.Error("No backup file created")
	}
}

func TestWriteFilePathSecurity(t *testing.T) {
	tmpDir := t.TempDir()
	server, _ := NewServer([]string{tmpDir})

	tests := []struct {
		name    string
		args    map[string]interface{}
		wantErr bool
	}{
		{
			name: "write to allowed path",
			args: map[string]interface{}{
				"path":    filepath.Join(tmpDir, "allowed.txt"),
				"content": "content",
			},
			wantErr: false,
		},
		{
			name: "write outside allowed dirs",
			args: map[string]interface{}{
				"path":    "/tmp/outside.txt",
				"content": "content",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := server.handleWriteFile(tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("handleWriteFile() security error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGetFileInfo(t *testing.T) {
	tmpDir := t.TempDir()
	server, _ := NewServer([]string{tmpDir})

	// Create test file
	testFile := filepath.Join(tmpDir, "info_test.txt")
	testContent := "test content for info"
	if err := os.WriteFile(testFile, []byte(testContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Create test directory
	testDir := filepath.Join(tmpDir, "info_test_dir")
	if err := os.MkdirAll(testDir, 0755); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name         string
		args         map[string]interface{}
		wantContains []string
		wantErr      bool
	}{
		{
			name: "get file info",
			args: map[string]interface{}{
				"path": testFile,
			},
			wantContains: []string{"info_test.txt", "path", "size", "mode"},
			wantErr:      false,
		},
		{
			name: "get directory info",
			args: map[string]interface{}{
				"path": testDir,
			},
			wantContains: []string{"info_test_dir", "is_dir"},
			wantErr:      false,
		},
		{
			name: "missing path",
			args: map[string]interface{}{},
			wantErr: true,
		},
		{
			name: "nonexistent file",
			args: map[string]interface{}{
				"path": filepath.Join(tmpDir, "nonexistent.txt"),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := server.handleGetFileInfo(tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("handleGetFileInfo() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}

			for _, want := range tt.wantContains {
				if !strings.Contains(result, want) {
					t.Errorf("handleGetFileInfo() = %v, want to contain %v", result, want)
				}
			}
		})
	}
}

func TestCreateDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	server, _ := NewServer([]string{tmpDir})

	tests := []struct {
		name    string
		args    map[string]interface{}
		wantErr bool
	}{
		{
			name: "create simple directory",
			args: map[string]interface{}{
				"path": filepath.Join(tmpDir, "new_dir"),
			},
			wantErr: false,
		},
		{
			name: "create nested directory",
			args: map[string]interface{}{
				"path":      filepath.Join(tmpDir, "nested", "deep", "dir"),
				"recursive": true,
			},
			wantErr: false,
		},
		{
			name: "create without recursive flag",
			args: map[string]interface{}{
				"path":      filepath.Join(tmpDir, "parent_missing", "child"),
				"recursive": false,
			},
			wantErr: true,
		},
		{
			name: "missing path",
			args: map[string]interface{}{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := server.handleCreateDirectory(tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("handleCreateDirectory() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}

			// Verify directory exists
			path := tt.args["path"].(string)
			if _, err := os.Stat(path); os.IsNotExist(err) {
				t.Errorf("Directory was not created: %s", path)
			}

			// Verify result contains success indicators
			if !strings.Contains(result, "created") {
				t.Errorf("handleCreateDirectory() result should indicate creation: %s", result)
			}
		})
	}
}

func TestLargeWriteFile(t *testing.T) {
	tmpDir := t.TempDir()
	server, _ := NewServer([]string{tmpDir})

	tests := []struct {
		name        string
		args        map[string]interface{}
		wantContent string
		wantErr     bool
	}{
		{
			name: "create new large file",
			args: map[string]interface{}{
				"path":    filepath.Join(tmpDir, "large.txt"),
				"content": "large content test",
			},
			wantContent: "large content test",
			wantErr:     false,
		},
		{
			name: "verify write",
			args: map[string]interface{}{
				"path":         filepath.Join(tmpDir, "verified.txt"),
				"content":      "verified content",
				"verify_write": true,
			},
			wantContent: "verified content",
			wantErr:     false,
		},
		{
			name: "missing path",
			args: map[string]interface{}{
				"content": "content",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := server.handleLargeWriteFile(tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("handleLargeWriteFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}

			// Verify file content
			path := tt.args["path"].(string)
			content, err := os.ReadFile(path)
			if err != nil {
				t.Errorf("Failed to read file: %v", err)
				return
			}
			if string(content) != tt.wantContent {
				t.Errorf("File content = %q, want %q", string(content), tt.wantContent)
			}

			_ = result
		})
	}
}

func TestWriteFileErrorCases(t *testing.T) {
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
			_, err := server.handleWriteFile(tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("handleWriteFile() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestWriteEmptyContent(t *testing.T) {
	tmpDir := t.TempDir()
	server, _ := NewServer([]string{tmpDir})

	// Writing empty content is allowed
	testFile := filepath.Join(tmpDir, "empty.txt")
	_, err := server.handleWriteFile(map[string]interface{}{
		"path":    testFile,
		"content": "",
	})
	if err != nil {
		t.Errorf("handleWriteFile() with empty content error = %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(testFile); os.IsNotExist(err) {
		t.Error("Empty file was not created")
	}
}

func TestWriteFileWithMode(t *testing.T) {
	tmpDir := t.TempDir()
	server, _ := NewServer([]string{tmpDir})

	testFile := filepath.Join(tmpDir, "mode_test.txt")

	result, err := server.handleWriteFile(map[string]interface{}{
		"path":    testFile,
		"content": "mode content",
		"mode":    float64(0600),
	})
	if err != nil {
		t.Errorf("handleWriteFile() with mode error = %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(testFile); os.IsNotExist(err) {
		t.Error("File was not created")
	}

	_ = result
}
