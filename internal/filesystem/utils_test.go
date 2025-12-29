package filesystem

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidatePath(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name        string
		path        string
		allowedDirs []string
		wantErr     bool
	}{
		{
			name:        "allowed path",
			path:        filepath.Join(tmpDir, "test.txt"),
			allowedDirs: []string{tmpDir},
			wantErr:     false,
		},
		{
			name:        "path outside allowed dirs",
			path:        "/etc/passwd",
			allowedDirs: []string{tmpDir},
			wantErr:     true,
		},
		{
			name:        "path with .. traversal",
			path:        filepath.Join(tmpDir, "..", "etc", "passwd"),
			allowedDirs: []string{tmpDir},
			wantErr:     true,
		},
		{
			name:        "empty allowed dirs allows all",
			path:        "/tmp/anything",
			allowedDirs: []string{},
			wantErr:     false,
		},
		{
			name:        "nested allowed path",
			path:        filepath.Join(tmpDir, "subdir", "file.txt"),
			allowedDirs: []string{tmpDir},
			wantErr:     false,
		},
		{
			name:        "multiple allowed dirs",
			path:        "/home/user/file.txt",
			allowedDirs: []string{"/tmp", "/home/user"},
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePath(tt.path, tt.allowedDirs)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePath() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestNormalizePath(t *testing.T) {
	home, _ := os.UserHomeDir()

	tests := []struct {
		name     string
		path     string
		wantPath string
		wantErr  bool
	}{
		{
			name:     "absolute path unchanged",
			path:     "/tmp/test.txt",
			wantPath: "/tmp/test.txt",
			wantErr:  false,
		},
		{
			name:     "expand tilde",
			path:     "~/test.txt",
			wantPath: filepath.Join(home, "test.txt"),
			wantErr:  false,
		},
		{
			name:     "clean double slashes",
			path:     "/tmp//test.txt",
			wantPath: "/tmp/test.txt",
			wantErr:  false,
		},
		{
			name:     "resolve dot segments",
			path:     "/tmp/./subdir/../test.txt",
			wantPath: "/tmp/test.txt",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizePath(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("NormalizePath() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.wantPath {
				t.Errorf("NormalizePath() = %v, want %v", got, tt.wantPath)
			}
		})
	}
}

func TestIsPathAllowed(t *testing.T) {
	tests := []struct {
		name        string
		path        string
		allowedDirs []string
		want        bool
	}{
		{
			name:        "path in allowed dir",
			path:        "/home/user/file.txt",
			allowedDirs: []string{"/home/user"},
			want:        true,
		},
		{
			name:        "path not in allowed dir",
			path:        "/etc/passwd",
			allowedDirs: []string{"/home/user"},
			want:        false,
		},
		{
			name:        "empty allowed dirs means all allowed",
			path:        "/any/path",
			allowedDirs: []string{},
			want:        true,
		},
		{
			name:        "exact match",
			path:        "/home/user",
			allowedDirs: []string{"/home/user"},
			want:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsPathAllowed(tt.path, tt.allowedDirs); got != tt.want {
				t.Errorf("IsPathAllowed() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSymlinkHandling(t *testing.T) {
	tmpDir := t.TempDir()

	// Resolve the tmpDir itself (on macOS /var is a symlink to /private/var)
	resolvedTmpDir, err := filepath.EvalSymlinks(tmpDir)
	if err != nil {
		resolvedTmpDir = tmpDir
	}

	// Create a real file
	realFile := filepath.Join(tmpDir, "real.txt")
	if err := os.WriteFile(realFile, []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a symlink to it
	symlink := filepath.Join(tmpDir, "link.txt")
	if err := os.Symlink(realFile, symlink); err != nil {
		t.Skip("symlinks not supported")
	}

	tests := []struct {
		name        string
		path        string
		allowedDirs []string
		wantErr     bool
	}{
		{
			name:        "symlink within allowed dir",
			path:        symlink,
			allowedDirs: []string{resolvedTmpDir}, // Use resolved path
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolved, err := ResolveSymlink(tt.path)
			if err != nil {
				t.Skipf("symlink resolution not available: %v", err)
			}
			err = ValidatePath(resolved, tt.allowedDirs)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePath(resolved symlink) error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
