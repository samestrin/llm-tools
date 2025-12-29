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
