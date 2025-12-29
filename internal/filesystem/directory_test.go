package filesystem

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestListDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test files and directories
	os.WriteFile(filepath.Join(tmpDir, "file1.txt"), []byte("content"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "file2.go"), []byte("content"), 0644)
	os.WriteFile(filepath.Join(tmpDir, ".hidden"), []byte("content"), 0644)
	os.Mkdir(filepath.Join(tmpDir, "subdir"), 0755)

	server, _ := NewServer([]string{tmpDir})

	tests := []struct {
		name         string
		args         map[string]interface{}
		wantContains []string
		wantMissing  []string
		wantErr      bool
	}{
		{
			name: "list all files",
			args: map[string]interface{}{
				"path": tmpDir,
			},
			wantContains: []string{"file1.txt", "file2.go", "subdir"},
			wantMissing:  []string{".hidden"}, // Hidden not shown by default
			wantErr:      false,
		},
		{
			name: "list with hidden files",
			args: map[string]interface{}{
				"path":        tmpDir,
				"show_hidden": true,
			},
			wantContains: []string{"file1.txt", ".hidden"},
			wantErr:      false,
		},
		{
			name: "list with pattern filter",
			args: map[string]interface{}{
				"path":    tmpDir,
				"pattern": "*.txt",
			},
			wantContains: []string{"file1.txt"},
			wantMissing:  []string{"file2.go"},
			wantErr:      false,
		},
		{
			name: "list with pagination",
			args: map[string]interface{}{
				"path":      tmpDir,
				"page":      float64(1),
				"page_size": float64(2),
			},
			wantErr: false,
		},
		{
			name: "list nonexistent directory",
			args: map[string]interface{}{
				"path": filepath.Join(tmpDir, "nonexistent"),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := server.handleListDirectory(tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("handleListDirectory() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}

			for _, want := range tt.wantContains {
				if !strings.Contains(result, want) {
					t.Errorf("handleListDirectory() = %v, want to contain %v", result, want)
				}
			}
			for _, missing := range tt.wantMissing {
				if strings.Contains(result, missing) {
					t.Errorf("handleListDirectory() = %v, should not contain %v", result, missing)
				}
			}
		})
	}
}

func TestGetDirectoryTree(t *testing.T) {
	tmpDir := t.TempDir()

	// Create nested structure
	os.MkdirAll(filepath.Join(tmpDir, "level1", "level2", "level3"), 0755)
	os.MkdirAll(filepath.Join(tmpDir, ".hidden_dir"), 0755)
	os.WriteFile(filepath.Join(tmpDir, "root.txt"), []byte("content"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "level1", "l1.txt"), []byte("content"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "level1", "level2", "l2.txt"), []byte("content"), 0644)
	os.WriteFile(filepath.Join(tmpDir, ".hidden_dir", "file.txt"), []byte("content"), 0644)

	server, _ := NewServer([]string{tmpDir})

	tests := []struct {
		name         string
		args         map[string]interface{}
		wantContains []string
		wantMissing  []string
		wantErr      bool
	}{
		{
			name: "full tree",
			args: map[string]interface{}{
				"path":      tmpDir,
				"max_depth": float64(10),
			},
			wantContains: []string{"level1", "level2", "level3"},
			wantErr:      false,
		},
		{
			name: "limited depth",
			args: map[string]interface{}{
				"path":      tmpDir,
				"max_depth": float64(1),
			},
			wantContains: []string{"level1"},
			wantMissing:  []string{"level3"},
			wantErr:      false,
		},
		{
			name: "include files",
			args: map[string]interface{}{
				"path":          tmpDir,
				"include_files": true,
			},
			wantContains: []string{"root.txt"},
			wantErr:      false,
		},
		{
			name: "show hidden",
			args: map[string]interface{}{
				"path":        tmpDir,
				"show_hidden": true,
			},
			wantContains: []string{".hidden_dir"},
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := server.handleGetDirectoryTree(tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("handleGetDirectoryTree() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}

			for _, want := range tt.wantContains {
				if !strings.Contains(result, want) {
					t.Errorf("handleGetDirectoryTree() = %v, want to contain %v", result, want)
				}
			}
			for _, missing := range tt.wantMissing {
				if strings.Contains(result, missing) {
					t.Errorf("handleGetDirectoryTree() = %v, should not contain %v", result, missing)
				}
			}
		})
	}
}

func TestListDirectorySort(t *testing.T) {
	tmpDir := t.TempDir()

	// Create files with different sizes and times
	os.WriteFile(filepath.Join(tmpDir, "aaa.txt"), []byte("a"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "bbb.txt"), []byte("bb"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "ccc.txt"), []byte("ccc"), 0644)

	server, _ := NewServer([]string{tmpDir})

	tests := []struct {
		name    string
		args    map[string]interface{}
		wantErr bool
	}{
		{
			name: "sort by name",
			args: map[string]interface{}{
				"path":    tmpDir,
				"sort_by": "name",
			},
			wantErr: false,
		},
		{
			name: "sort by size",
			args: map[string]interface{}{
				"path":    tmpDir,
				"sort_by": "size",
			},
			wantErr: false,
		},
		{
			name: "sort reverse",
			args: map[string]interface{}{
				"path":    tmpDir,
				"sort_by": "name",
				"reverse": true,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := server.handleListDirectory(tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("handleListDirectory() error = %v, wantErr %v", err, tt.wantErr)
			}
			_ = result
		})
	}
}
