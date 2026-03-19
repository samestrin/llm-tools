package core

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteMultipleFiles(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("happy path", func(t *testing.T) {
		result, err := WriteMultipleFiles(WriteMultipleFilesOptions{
			Files: []WriteFileEntry{
				{Path: filepath.Join(tmpDir, "a.txt"), Content: "content a"},
				{Path: filepath.Join(tmpDir, "b.txt"), Content: "content b"},
			},
		})
		if err != nil {
			t.Fatalf("WriteMultipleFiles() error = %v", err)
		}
		if result.Success != 2 {
			t.Errorf("Success = %d, want 2", result.Success)
		}
		if result.Failed != 0 {
			t.Errorf("Failed = %d, want 0", result.Failed)
		}
		// Verify files exist with correct content
		for _, entry := range []struct{ path, content string }{
			{filepath.Join(tmpDir, "a.txt"), "content a"},
			{filepath.Join(tmpDir, "b.txt"), "content b"},
		} {
			got, err := os.ReadFile(entry.path)
			if err != nil {
				t.Errorf("ReadFile(%s) error = %v", entry.path, err)
			} else if string(got) != entry.content {
				t.Errorf("File %s content = %q, want %q", entry.path, got, entry.content)
			}
		}
	})

	t.Run("partial failure", func(t *testing.T) {
		result, err := WriteMultipleFiles(WriteMultipleFilesOptions{
			Files: []WriteFileEntry{
				{Path: filepath.Join(tmpDir, "ok.txt"), Content: "ok"},
				{Path: "", Content: "bad"}, // empty path should fail
			},
		})
		if err != nil {
			t.Fatalf("WriteMultipleFiles() error = %v", err)
		}
		if result.Success != 1 {
			t.Errorf("Success = %d, want 1", result.Success)
		}
		if result.Failed != 1 {
			t.Errorf("Failed = %d, want 1", result.Failed)
		}
	})

	t.Run("empty input", func(t *testing.T) {
		_, err := WriteMultipleFiles(WriteMultipleFilesOptions{
			Files: []WriteFileEntry{},
		})
		if err == nil {
			t.Error("Expected error for empty files")
		}
	})

	t.Run("auto mkdir", func(t *testing.T) {
		result, err := WriteMultipleFiles(WriteMultipleFilesOptions{
			Files: []WriteFileEntry{
				{Path: filepath.Join(tmpDir, "deep", "nested", "file.txt"), Content: "nested"},
			},
		})
		if err != nil {
			t.Fatalf("WriteMultipleFiles() error = %v", err)
		}
		if result.Success != 1 {
			t.Errorf("Success = %d, want 1", result.Success)
		}
		got, _ := os.ReadFile(filepath.Join(tmpDir, "deep", "nested", "file.txt"))
		if string(got) != "nested" {
			t.Errorf("Content = %q, want %q", got, "nested")
		}
	})
}

func TestCreateDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name    string
		opts    CreateDirectoryOptions
		wantErr bool
	}{
		{
			name: "create simple directory",
			opts: CreateDirectoryOptions{
				Path:      filepath.Join(tmpDir, "simple"),
				Recursive: true,
			},
			wantErr: false,
		},
		{
			name: "create nested directory with recursive",
			opts: CreateDirectoryOptions{
				Path:      filepath.Join(tmpDir, "nested", "deep", "dir"),
				Recursive: true,
			},
			wantErr: false,
		},
		{
			name: "create without recursive fails for nested",
			opts: CreateDirectoryOptions{
				Path:      filepath.Join(tmpDir, "nonexistent", "child"),
				Recursive: false,
			},
			wantErr: true,
		},
		{
			name: "empty path",
			opts: CreateDirectoryOptions{
				Path: "",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := CreateDirectory(tt.opts)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateDirectory() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}

			// Verify directory exists
			if _, err := os.Stat(tt.opts.Path); os.IsNotExist(err) {
				t.Errorf("Directory was not created: %s", tt.opts.Path)
			}

			if !result.Created {
				t.Error("Expected Created to be true")
			}
		})
	}
}
