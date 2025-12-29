package filesystem

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test file
	testContent := "line 1\nline 2\nline 3\nline 4\nline 5\n"
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte(testContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Create empty file
	emptyFile := filepath.Join(tmpDir, "empty.txt")
	if err := os.WriteFile(emptyFile, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	server, _ := NewServer([]string{tmpDir})

	tests := []struct {
		name        string
		args        map[string]interface{}
		wantContent string
		wantErr     bool
	}{
		{
			name: "read entire file",
			args: map[string]interface{}{
				"path": testFile,
			},
			wantContent: testContent,
			wantErr:     false,
		},
		{
			name: "read with line range",
			args: map[string]interface{}{
				"path":       testFile,
				"line_start": float64(2),
				"line_count": float64(2),
			},
			wantContent: "line 2\nline 3\n",
			wantErr:     false,
		},
		{
			name: "read empty file",
			args: map[string]interface{}{
				"path": emptyFile,
			},
			wantContent: "",
			wantErr:     false,
		},
		{
			name: "read missing file",
			args: map[string]interface{}{
				"path": filepath.Join(tmpDir, "nonexistent.txt"),
			},
			wantErr: true,
		},
		{
			name: "read with byte offset",
			args: map[string]interface{}{
				"path":         testFile,
				"start_offset": float64(7), // Start at "line 2"
				"max_size":     float64(6), // Read "line 2"
			},
			wantContent: "line 2",
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := server.handleReadFile(tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("handleReadFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !strings.Contains(result, tt.wantContent) {
				t.Errorf("handleReadFile() = %v, want content %v", result, tt.wantContent)
			}
		})
	}
}

func TestReadMultipleFiles(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test files
	file1 := filepath.Join(tmpDir, "file1.txt")
	file2 := filepath.Join(tmpDir, "file2.txt")
	if err := os.WriteFile(file1, []byte("content 1"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file2, []byte("content 2"), 0644); err != nil {
		t.Fatal(err)
	}

	server, _ := NewServer([]string{tmpDir})

	tests := []struct {
		name          string
		args          map[string]interface{}
		wantContains  []string
		wantErr       bool
	}{
		{
			name: "read multiple files",
			args: map[string]interface{}{
				"paths": []interface{}{file1, file2},
			},
			wantContains: []string{"content 1", "content 2"},
			wantErr:      false,
		},
		{
			name: "read with one missing file",
			args: map[string]interface{}{
				"paths": []interface{}{file1, filepath.Join(tmpDir, "missing.txt")},
			},
			// Should still return content for file1, with error for missing
			wantContains: []string{"content 1"},
			wantErr:      false, // Partial success is not an error
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := server.handleReadMultipleFiles(tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("handleReadMultipleFiles() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			for _, want := range tt.wantContains {
				if !strings.Contains(result, want) {
					t.Errorf("handleReadMultipleFiles() = %v, want to contain %v", result, want)
				}
			}
		})
	}
}

func TestReadFilePathSecurity(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test file in allowed dir
	allowedFile := filepath.Join(tmpDir, "allowed.txt")
	if err := os.WriteFile(allowedFile, []byte("allowed content"), 0644); err != nil {
		t.Fatal(err)
	}

	server, _ := NewServer([]string{tmpDir})

	tests := []struct {
		name    string
		args    map[string]interface{}
		wantErr bool
	}{
		{
			name: "allowed path",
			args: map[string]interface{}{
				"path": allowedFile,
			},
			wantErr: false,
		},
		{
			name: "path outside allowed dirs",
			args: map[string]interface{}{
				"path": "/etc/passwd",
			},
			wantErr: true,
		},
		{
			name: "path traversal attempt",
			args: map[string]interface{}{
				"path": filepath.Join(tmpDir, "..", "etc", "passwd"),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := server.handleReadFile(tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("handleReadFile() security error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
