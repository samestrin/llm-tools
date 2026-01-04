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

func TestCreateDirectories(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name        string
		opts        CreateDirectoriesOptions
		wantSuccess int
		wantFailed  int
		wantErr     bool
	}{
		{
			name: "create multiple directories",
			opts: CreateDirectoriesOptions{
				Paths: []string{
					filepath.Join(tmpDir, "multi1"),
					filepath.Join(tmpDir, "multi2"),
					filepath.Join(tmpDir, "multi3"),
				},
				Recursive: true,
			},
			wantSuccess: 3,
			wantFailed:  0,
			wantErr:     false,
		},
		{
			name: "create nested directories",
			opts: CreateDirectoriesOptions{
				Paths: []string{
					filepath.Join(tmpDir, "nested1", "deep", "dir"),
					filepath.Join(tmpDir, "nested2", "deep", "dir"),
				},
				Recursive: true,
			},
			wantSuccess: 2,
			wantFailed:  0,
			wantErr:     false,
		},
		{
			name: "partial failure without recursive",
			opts: CreateDirectoriesOptions{
				Paths: []string{
					filepath.Join(tmpDir, "simple_ok"),
					filepath.Join(tmpDir, "parent_missing", "child"),
				},
				Recursive: false,
			},
			wantSuccess: 1,
			wantFailed:  1,
			wantErr:     false,
		},
		{
			name: "empty paths",
			opts: CreateDirectoriesOptions{
				Paths: []string{},
			},
			wantErr: true,
		},
		{
			name: "nil paths",
			opts: CreateDirectoriesOptions{
				Paths: nil,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := CreateDirectories(tt.opts)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateDirectories() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}

			if result.Success != tt.wantSuccess {
				t.Errorf("CreateDirectories() success = %d, want %d", result.Success, tt.wantSuccess)
			}
			if result.Failed != tt.wantFailed {
				t.Errorf("CreateDirectories() failed = %d, want %d", result.Failed, tt.wantFailed)
			}

			// Verify successful directories exist
			for i, path := range tt.opts.Paths {
				_, statErr := os.Stat(path)
				if i < tt.wantSuccess && os.IsNotExist(statErr) {
					t.Errorf("Directory %d should have been created: %s", i, path)
				}
			}
		})
	}
}

func TestCreateDirectoriesPathValidation(t *testing.T) {
	tmpDir := t.TempDir()

	// Test with allowed dirs restriction
	opts := CreateDirectoriesOptions{
		Paths: []string{
			filepath.Join(tmpDir, "allowed"),
			"/tmp/outside",
		},
		Recursive:   true,
		AllowedDirs: []string{tmpDir},
	}

	result, err := CreateDirectories(opts)
	if err != nil {
		t.Fatalf("CreateDirectories() error = %v", err)
	}

	// First should succeed, second should fail
	if result.Success != 1 {
		t.Errorf("Expected 1 success, got %d", result.Success)
	}
	if result.Failed != 1 {
		t.Errorf("Expected 1 failure, got %d", result.Failed)
	}

	// Verify allowed path was created
	if _, err := os.Stat(filepath.Join(tmpDir, "allowed")); os.IsNotExist(err) {
		t.Error("Allowed directory should have been created")
	}
}

func TestCreateDirectoriesResultMessage(t *testing.T) {
	tmpDir := t.TempDir()

	opts := CreateDirectoriesOptions{
		Paths: []string{
			filepath.Join(tmpDir, "msg_test1"),
			filepath.Join(tmpDir, "msg_test2"),
		},
		Recursive: true,
	}

	result, err := CreateDirectories(opts)
	if err != nil {
		t.Fatalf("CreateDirectories() error = %v", err)
	}

	if result.Message == "" {
		t.Error("Expected non-empty message")
	}

	if len(result.Directories) != 2 {
		t.Errorf("Expected 2 directory results, got %d", len(result.Directories))
	}
}
