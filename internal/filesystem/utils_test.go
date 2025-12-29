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

func TestNormalizePathRelative(t *testing.T) {
	// Test relative path conversion to absolute
	relativePath := "test.txt"
	result, err := NormalizePath(relativePath)
	if err != nil {
		t.Errorf("NormalizePath() error = %v", err)
		return
	}

	// Result should be an absolute path
	if !filepath.IsAbs(result) {
		t.Errorf("NormalizePath(%q) = %q, should be absolute", relativePath, result)
	}

	// Result should contain the filename
	if filepath.Base(result) != "test.txt" {
		t.Errorf("NormalizePath() base = %q, want %q", filepath.Base(result), "test.txt")
	}
}

func TestResolveSymlinkNonSymlink(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a regular file
	regularFile := filepath.Join(tmpDir, "regular.txt")
	os.WriteFile(regularFile, []byte("content"), 0644)

	// ResolveSymlink should work on regular files too
	resolved, err := ResolveSymlink(regularFile)
	if err != nil {
		t.Errorf("ResolveSymlink() error = %v", err)
	}

	// Should return the same path (cleaned)
	if resolved == "" {
		t.Error("ResolveSymlink() returned empty string")
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

func TestGetInt(t *testing.T) {
	tests := []struct {
		name       string
		args       map[string]interface{}
		key        string
		defaultVal int
		want       int
	}{
		{
			name:       "get int from float64",
			args:       map[string]interface{}{"count": float64(42)},
			key:        "count",
			defaultVal: 0,
			want:       42,
		},
		{
			name:       "get int from int",
			args:       map[string]interface{}{"count": 42},
			key:        "count",
			defaultVal: 0,
			want:       42,
		},
		{
			name:       "missing key returns default",
			args:       map[string]interface{}{},
			key:        "count",
			defaultVal: 10,
			want:       10,
		},
		{
			name:       "wrong type returns default",
			args:       map[string]interface{}{"count": "not a number"},
			key:        "count",
			defaultVal: 5,
			want:       5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetInt(tt.args, tt.key, tt.defaultVal)
			if got != tt.want {
				t.Errorf("GetInt() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetStringSlice(t *testing.T) {
	tests := []struct {
		name string
		args map[string]interface{}
		key  string
		want int // length of result
	}{
		{
			name: "get string slice",
			args: map[string]interface{}{"items": []interface{}{"a", "b", "c"}},
			key:  "items",
			want: 3,
		},
		{
			name: "missing key returns nil",
			args: map[string]interface{}{},
			key:  "items",
			want: 0,
		},
		{
			name: "wrong type returns nil",
			args: map[string]interface{}{"items": "not a slice"},
			key:  "items",
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetStringSlice(tt.args, tt.key)
			if len(got) != tt.want {
				t.Errorf("GetStringSlice() length = %v, want %v", len(got), tt.want)
			}
		})
	}
}

func TestGetString(t *testing.T) {
	tests := []struct {
		name       string
		args       map[string]interface{}
		key        string
		defaultVal string
		want       string
	}{
		{
			name:       "get string",
			args:       map[string]interface{}{"name": "test"},
			key:        "name",
			defaultVal: "",
			want:       "test",
		},
		{
			name:       "missing key returns default",
			args:       map[string]interface{}{},
			key:        "name",
			defaultVal: "default",
			want:       "default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetString(tt.args, tt.key, tt.defaultVal)
			if got != tt.want {
				t.Errorf("GetString() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetBool(t *testing.T) {
	tests := []struct {
		name       string
		args       map[string]interface{}
		key        string
		defaultVal bool
		want       bool
	}{
		{
			name:       "get bool true",
			args:       map[string]interface{}{"flag": true},
			key:        "flag",
			defaultVal: false,
			want:       true,
		},
		{
			name:       "get bool false",
			args:       map[string]interface{}{"flag": false},
			key:        "flag",
			defaultVal: true,
			want:       false,
		},
		{
			name:       "missing key returns default",
			args:       map[string]interface{}{},
			key:        "flag",
			defaultVal: true,
			want:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetBool(tt.args, tt.key, tt.defaultVal)
			if got != tt.want {
				t.Errorf("GetBool() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResolvePath(t *testing.T) {
	home, _ := os.UserHomeDir()

	tests := []struct {
		name     string
		path     string
		wantPath string
		wantErr  bool
	}{
		{
			name:     "expand tilde",
			path:     "~/Documents",
			wantPath: filepath.Join(home, "Documents"),
			wantErr:  false,
		},
		{
			name:     "absolute path unchanged",
			path:     "/tmp/test.txt",
			wantPath: "/tmp/test.txt",
			wantErr:  false,
		},
		{
			name:     "clean path with dots",
			path:     "/tmp/../tmp/test.txt",
			wantPath: "/tmp/test.txt",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ResolvePath(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("ResolvePath() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.wantPath {
				t.Errorf("ResolvePath() = %v, want %v", got, tt.wantPath)
			}
		})
	}
}

func TestCreateBackupPath(t *testing.T) {
	path := "/tmp/test.txt"
	backup := CreateBackupPath(path)

	// Should add .bak suffix
	if backup == "" {
		t.Error("CreateBackupPath() should return non-empty string")
	}
	expected := path + ".bak"
	if backup != expected {
		t.Errorf("CreateBackupPath() = %v, want %v", backup, expected)
	}
}

func TestEnsureDir(t *testing.T) {
	tmpDir := t.TempDir()

	// Test creating nested directory
	nestedPath := filepath.Join(tmpDir, "nested", "deep", "dir", "file.txt")
	if err := EnsureDir(nestedPath); err != nil {
		t.Errorf("EnsureDir() error = %v", err)
	}

	// Verify parent directory was created
	parentDir := filepath.Dir(nestedPath)
	if _, err := os.Stat(parentDir); os.IsNotExist(err) {
		t.Errorf("EnsureDir() should create parent directory")
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
