package filesystem

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCopyFile(t *testing.T) {
	tmpDir := t.TempDir()

	server, _ := NewServer([]string{tmpDir})

	tests := []struct {
		name    string
		setup   func() (string, string)
		wantErr bool
	}{
		{
			name: "copy file",
			setup: func() (string, string) {
				src := filepath.Join(tmpDir, "source.txt")
				dst := filepath.Join(tmpDir, "dest.txt")
				os.WriteFile(src, []byte("content"), 0644)
				return src, dst
			},
			wantErr: false,
		},
		{
			name: "copy to nested directory",
			setup: func() (string, string) {
				src := filepath.Join(tmpDir, "src2.txt")
				dst := filepath.Join(tmpDir, "nested", "deep", "dest2.txt")
				os.WriteFile(src, []byte("content"), 0644)
				return src, dst
			},
			wantErr: false,
		},
		{
			name: "copy directory",
			setup: func() (string, string) {
				src := filepath.Join(tmpDir, "srcdir")
				os.MkdirAll(src, 0755)
				os.WriteFile(filepath.Join(src, "file.txt"), []byte("content"), 0644)
				dst := filepath.Join(tmpDir, "dstdir")
				return src, dst
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src, dst := tt.setup()

			result, err := server.handleCopyFile(map[string]interface{}{
				"source":      src,
				"destination": dst,
			})
			if (err != nil) != tt.wantErr {
				t.Errorf("handleCopyFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}

			// Verify destination exists
			if _, err := os.Stat(dst); os.IsNotExist(err) {
				t.Errorf("Destination file was not created")
			}

			// Verify result
			if !strings.Contains(result, "success") && !strings.Contains(result, "true") {
				t.Errorf("Result should indicate success: %s", result)
			}
		})
	}
}

func TestMoveFile(t *testing.T) {
	tmpDir := t.TempDir()

	server, _ := NewServer([]string{tmpDir})

	tests := []struct {
		name    string
		setup   func() (string, string)
		wantErr bool
	}{
		{
			name: "move file",
			setup: func() (string, string) {
				src := filepath.Join(tmpDir, "move_src.txt")
				dst := filepath.Join(tmpDir, "move_dst.txt")
				os.WriteFile(src, []byte("move content"), 0644)
				return src, dst
			},
			wantErr: false,
		},
		{
			name: "rename file",
			setup: func() (string, string) {
				src := filepath.Join(tmpDir, "old_name.txt")
				dst := filepath.Join(tmpDir, "new_name.txt")
				os.WriteFile(src, []byte("rename content"), 0644)
				return src, dst
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src, dst := tt.setup()

			result, err := server.handleMoveFile(map[string]interface{}{
				"source":      src,
				"destination": dst,
			})
			if (err != nil) != tt.wantErr {
				t.Errorf("handleMoveFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}

			// Verify source no longer exists
			if _, err := os.Stat(src); !os.IsNotExist(err) {
				t.Errorf("Source file should no longer exist")
			}

			// Verify destination exists
			if _, err := os.Stat(dst); os.IsNotExist(err) {
				t.Errorf("Destination file was not created")
			}

			_ = result
		})
	}
}

func TestDeleteFile(t *testing.T) {
	tmpDir := t.TempDir()

	server, _ := NewServer([]string{tmpDir})

	tests := []struct {
		name    string
		setup   func() string
		args    map[string]interface{}
		wantErr bool
	}{
		{
			name: "delete file",
			setup: func() string {
				path := filepath.Join(tmpDir, "delete_me.txt")
				os.WriteFile(path, []byte("delete"), 0644)
				return path
			},
			wantErr: false,
		},
		{
			name: "delete directory",
			setup: func() string {
				path := filepath.Join(tmpDir, "delete_dir")
				os.MkdirAll(path, 0755)
				os.WriteFile(filepath.Join(path, "file.txt"), []byte("content"), 0644)
				return path
			},
			args:    map[string]interface{}{"recursive": true},
			wantErr: false,
		},
		{
			name: "delete nonexistent file",
			setup: func() string {
				return filepath.Join(tmpDir, "nonexistent.txt")
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := tt.setup()
			args := map[string]interface{}{"path": path}
			if tt.args != nil {
				for k, v := range tt.args {
					args[k] = v
				}
			}

			result, err := server.handleDeleteFile(args)
			if (err != nil) != tt.wantErr {
				t.Errorf("handleDeleteFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}

			// Verify file no longer exists
			if _, err := os.Stat(path); !os.IsNotExist(err) {
				t.Errorf("File should have been deleted")
			}

			_ = result
		})
	}
}

func TestBatchFileOperations(t *testing.T) {
	tmpDir := t.TempDir()

	server, _ := NewServer([]string{tmpDir})

	tests := []struct {
		name    string
		setup   func()
		args    map[string]interface{}
		wantErr bool
	}{
		{
			name: "batch copy operations",
			setup: func() {
				os.WriteFile(filepath.Join(tmpDir, "batch1.txt"), []byte("1"), 0644)
				os.WriteFile(filepath.Join(tmpDir, "batch2.txt"), []byte("2"), 0644)
			},
			args: map[string]interface{}{
				"operations": []interface{}{
					map[string]interface{}{
						"operation":   "copy",
						"source":      filepath.Join(tmpDir, "batch1.txt"),
						"destination": filepath.Join(tmpDir, "batch1_copy.txt"),
					},
					map[string]interface{}{
						"operation":   "copy",
						"source":      filepath.Join(tmpDir, "batch2.txt"),
						"destination": filepath.Join(tmpDir, "batch2_copy.txt"),
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()

			result, err := server.handleBatchFileOperations(tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("handleBatchFileOperations() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}

			// Verify result indicates success
			if !strings.Contains(result, "success") && !strings.Contains(result, "completed") {
				t.Errorf("Result should indicate success: %s", result)
			}
		})
	}
}
