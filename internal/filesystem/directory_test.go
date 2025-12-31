package filesystem

import (
	"encoding/json"
	"fmt"
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
	os.Mkdir(filepath.Join(tmpDir, "dir1"), 0755)

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
			name: "sort by type",
			args: map[string]interface{}{
				"path":    tmpDir,
				"sort_by": "type",
			},
			wantErr: false,
		},
		{
			name: "sort by time",
			args: map[string]interface{}{
				"path":    tmpDir,
				"sort_by": "time",
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

func TestListDirectoryMissingPath(t *testing.T) {
	tmpDir := t.TempDir()
	server, _ := NewServer([]string{tmpDir})

	_, err := server.handleListDirectory(map[string]interface{}{})
	if err == nil {
		t.Error("handleListDirectory() should error when path is missing")
	}
}

func TestListDirectoryPaginationEnforced(t *testing.T) {
	tmpDir := t.TempDir()

	// Create 12 test files
	for i := 1; i <= 12; i++ {
		os.WriteFile(filepath.Join(tmpDir, fmt.Sprintf("file%02d.txt", i)), []byte("content"), 0644)
	}

	server, _ := NewServer([]string{tmpDir})

	t.Run("page_size=5 returns exactly 5 items", func(t *testing.T) {
		result, err := server.handleListDirectory(map[string]interface{}{
			"path":      tmpDir,
			"page":      float64(1),
			"page_size": float64(5),
		})
		if err != nil {
			t.Fatalf("handleListDirectory() error = %v", err)
		}

		var resp ListDirectoryResult
		if err := json.Unmarshal([]byte(result), &resp); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}

		if len(resp.Entries) != 5 {
			t.Errorf("expected 5 entries, got %d", len(resp.Entries))
		}
		if resp.Total != 12 {
			t.Errorf("expected total=12, got %d", resp.Total)
		}
		if resp.TotalPages != 3 {
			t.Errorf("expected total_pages=3, got %d", resp.TotalPages)
		}
		if resp.Page != 1 {
			t.Errorf("expected page=1, got %d", resp.Page)
		}
	})

	t.Run("page=2 returns items 6-10", func(t *testing.T) {
		result, err := server.handleListDirectory(map[string]interface{}{
			"path":      tmpDir,
			"page":      float64(2),
			"page_size": float64(5),
		})
		if err != nil {
			t.Fatalf("handleListDirectory() error = %v", err)
		}

		var resp ListDirectoryResult
		if err := json.Unmarshal([]byte(result), &resp); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}

		if len(resp.Entries) != 5 {
			t.Errorf("expected 5 entries, got %d", len(resp.Entries))
		}
		if resp.Page != 2 {
			t.Errorf("expected page=2, got %d", resp.Page)
		}
	})

	t.Run("last page returns remaining items", func(t *testing.T) {
		result, err := server.handleListDirectory(map[string]interface{}{
			"path":      tmpDir,
			"page":      float64(3),
			"page_size": float64(5),
		})
		if err != nil {
			t.Fatalf("handleListDirectory() error = %v", err)
		}

		var resp ListDirectoryResult
		if err := json.Unmarshal([]byte(result), &resp); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}

		if len(resp.Entries) != 2 {
			t.Errorf("expected 2 entries on last page, got %d", len(resp.Entries))
		}
	})

	t.Run("page_size=0 returns all items", func(t *testing.T) {
		result, err := server.handleListDirectory(map[string]interface{}{
			"path":      tmpDir,
			"page_size": float64(0),
		})
		if err != nil {
			t.Fatalf("handleListDirectory() error = %v", err)
		}

		var resp ListDirectoryResult
		if err := json.Unmarshal([]byte(result), &resp); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}

		if len(resp.Entries) != 12 {
			t.Errorf("expected all 12 entries with page_size=0, got %d", len(resp.Entries))
		}
	})

	t.Run("page out of range returns empty list", func(t *testing.T) {
		result, err := server.handleListDirectory(map[string]interface{}{
			"path":      tmpDir,
			"page":      float64(10),
			"page_size": float64(5),
		})
		if err != nil {
			t.Fatalf("handleListDirectory() error = %v", err)
		}

		var resp ListDirectoryResult
		if err := json.Unmarshal([]byte(result), &resp); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}

		if len(resp.Entries) != 0 {
			t.Errorf("expected 0 entries for out-of-range page, got %d", len(resp.Entries))
		}
	})
}

func TestGetDirectoryTreeMissingPath(t *testing.T) {
	tmpDir := t.TempDir()
	server, _ := NewServer([]string{tmpDir})

	_, err := server.handleGetDirectoryTree(map[string]interface{}{})
	if err == nil {
		t.Error("handleGetDirectoryTree() should error when path is missing")
	}
}

func TestGetDirectoryTreeRecursionDepth(t *testing.T) {
	tmpDir := t.TempDir()

	// Create nested structure: level1/level2/level3/level4
	os.MkdirAll(filepath.Join(tmpDir, "level1", "level2", "level3", "level4"), 0755)
	os.WriteFile(filepath.Join(tmpDir, "root.txt"), []byte("content"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "level1", "l1.txt"), []byte("content"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "level1", "level2", "l2.txt"), []byte("content"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "level1", "level2", "level3", "l3.txt"), []byte("content"), 0644)

	server, _ := NewServer([]string{tmpDir})

	t.Run("depth=2 shows 2 levels deep", func(t *testing.T) {
		result, err := server.handleGetDirectoryTree(map[string]interface{}{
			"path":      tmpDir,
			"max_depth": float64(2),
		})
		if err != nil {
			t.Fatalf("handleGetDirectoryTree() error = %v", err)
		}

		var resp DirectoryTreeResult
		if err := json.Unmarshal([]byte(result), &resp); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}

		// Root should have children (level1)
		if resp.Root == nil {
			t.Fatal("expected root node")
		}
		if len(resp.Root.Children) == 0 {
			t.Error("expected root to have children")
		}

		// Find level1
		var level1 *TreeNode
		for _, child := range resp.Root.Children {
			if child.Name == "level1" {
				level1 = child
				break
			}
		}
		if level1 == nil {
			t.Fatal("expected to find level1 in root children")
		}

		// level1 should have children (level2)
		if len(level1.Children) == 0 {
			t.Error("expected level1 to have children at depth=2")
		}

		// Find level2
		var level2 *TreeNode
		for _, child := range level1.Children {
			if child.Name == "level2" {
				level2 = child
				break
			}
		}
		if level2 == nil {
			t.Fatal("expected to find level2 in level1 children")
		}

		// level2 should NOT have children at depth=2 (we're at limit)
		// Actually depth=2 means we recurse 2 levels from root, so level2's children wouldn't be populated
		// The depth logic starts at 0 and stops when depth >= maxDepth
		// So depth=2: root is depth 0, level1 is depth 1, level2 is depth 2 (no children)
	})

	t.Run("depth=0 returns only root with no children", func(t *testing.T) {
		result, err := server.handleGetDirectoryTree(map[string]interface{}{
			"path":      tmpDir,
			"max_depth": float64(0),
		})
		if err != nil {
			t.Fatalf("handleGetDirectoryTree() error = %v", err)
		}

		var resp DirectoryTreeResult
		if err := json.Unmarshal([]byte(result), &resp); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}

		if resp.Root == nil {
			t.Fatal("expected root node")
		}
		if len(resp.Root.Children) != 0 {
			t.Errorf("expected 0 children at depth=0, got %d", len(resp.Root.Children))
		}
	})

	t.Run("tree structure has nested children", func(t *testing.T) {
		result, err := server.handleGetDirectoryTree(map[string]interface{}{
			"path":      tmpDir,
			"max_depth": float64(5),
		})
		if err != nil {
			t.Fatalf("handleGetDirectoryTree() error = %v", err)
		}

		// Result should contain level4 (deeply nested)
		if !strings.Contains(result, "level4") {
			t.Error("expected result to contain level4 at max_depth=5")
		}
	})
}
