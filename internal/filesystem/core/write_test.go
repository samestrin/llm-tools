package core

import (
	"os"
	"path/filepath"
	"testing"
)

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
