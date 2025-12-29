package filesystem

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGetDiskUsage(t *testing.T) {
	tmpDir := t.TempDir()

	// Create files with known sizes
	os.WriteFile(filepath.Join(tmpDir, "file1.txt"), make([]byte, 1000), 0644)
	os.WriteFile(filepath.Join(tmpDir, "file2.txt"), make([]byte, 2000), 0644)
	os.MkdirAll(filepath.Join(tmpDir, "subdir"), 0755)
	os.WriteFile(filepath.Join(tmpDir, "subdir", "file3.txt"), make([]byte, 500), 0644)

	server, _ := NewServer([]string{tmpDir})

	result, err := server.handleGetDiskUsage(map[string]interface{}{
		"path": tmpDir,
	})
	if err != nil {
		t.Errorf("handleGetDiskUsage() error = %v", err)
		return
	}

	// Verify result contains size info
	if !strings.Contains(result, "total") && !strings.Contains(result, "size") {
		t.Errorf("handleGetDiskUsage() result should contain size info: %s", result)
	}
}

func TestFindLargeFiles(t *testing.T) {
	tmpDir := t.TempDir()

	// Create files of varying sizes
	os.WriteFile(filepath.Join(tmpDir, "small.txt"), make([]byte, 100), 0644)
	os.WriteFile(filepath.Join(tmpDir, "medium.txt"), make([]byte, 500), 0644)
	os.WriteFile(filepath.Join(tmpDir, "large.txt"), make([]byte, 1000), 0644)
	os.MkdirAll(filepath.Join(tmpDir, "sub"), 0755)
	os.WriteFile(filepath.Join(tmpDir, "sub", "huge.txt"), make([]byte, 2000), 0644)

	server, _ := NewServer([]string{tmpDir})

	tests := []struct {
		name         string
		args         map[string]interface{}
		wantContains []string
		wantErr      bool
	}{
		{
			name: "find files over 500 bytes",
			args: map[string]interface{}{
				"path":     tmpDir,
				"min_size": float64(500),
			},
			wantContains: []string{"large.txt", "huge.txt"},
			wantErr:      false,
		},
		{
			name: "limit results",
			args: map[string]interface{}{
				"path":  tmpDir,
				"limit": float64(2),
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := server.handleFindLargeFiles(tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("handleFindLargeFiles() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}

			for _, want := range tt.wantContains {
				if !strings.Contains(result, want) {
					t.Errorf("handleFindLargeFiles() = %v, want to contain %v", result, want)
				}
			}
		})
	}
}

func TestCompressFiles(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test files
	os.WriteFile(filepath.Join(tmpDir, "file1.txt"), []byte("content 1"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "file2.txt"), []byte("content 2"), 0644)

	server, _ := NewServer([]string{tmpDir})

	tests := []struct {
		name    string
		args    map[string]interface{}
		wantErr bool
	}{
		{
			name: "create zip archive",
			args: map[string]interface{}{
				"paths":  []interface{}{filepath.Join(tmpDir, "file1.txt"), filepath.Join(tmpDir, "file2.txt")},
				"output": filepath.Join(tmpDir, "archive.zip"),
				"format": "zip",
			},
			wantErr: false,
		},
		{
			name: "create tar.gz archive",
			args: map[string]interface{}{
				"paths":  []interface{}{filepath.Join(tmpDir, "file1.txt")},
				"output": filepath.Join(tmpDir, "archive.tar.gz"),
				"format": "tar.gz",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := server.handleCompressFiles(tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("handleCompressFiles() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}

			// Verify archive was created
			output := tt.args["output"].(string)
			if _, err := os.Stat(output); os.IsNotExist(err) {
				t.Errorf("Archive file was not created: %s", output)
			}

			_ = result
		})
	}
}

func TestExtractArchive(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a zip archive first
	zipPath := filepath.Join(tmpDir, "test.zip")
	zipFile, _ := os.Create(zipPath)
	zipWriter := zip.NewWriter(zipFile)
	w, _ := zipWriter.Create("extracted.txt")
	w.Write([]byte("extracted content"))
	zipWriter.Close()
	zipFile.Close()

	// Create a tar.gz archive
	tarPath := filepath.Join(tmpDir, "test.tar.gz")
	tarFile, _ := os.Create(tarPath)
	gzipWriter := gzip.NewWriter(tarFile)
	tarWriter := tar.NewWriter(gzipWriter)
	hdr := &tar.Header{
		Name: "extracted2.txt",
		Mode: 0644,
		Size: int64(len("extracted content 2")),
	}
	tarWriter.WriteHeader(hdr)
	tarWriter.Write([]byte("extracted content 2"))
	tarWriter.Close()
	gzipWriter.Close()
	tarFile.Close()

	server, _ := NewServer([]string{tmpDir})

	tests := []struct {
		name      string
		args      map[string]interface{}
		wantFiles []string
		wantErr   bool
	}{
		{
			name: "extract zip",
			args: map[string]interface{}{
				"archive":     zipPath,
				"destination": filepath.Join(tmpDir, "extracted_zip"),
			},
			wantFiles: []string{"extracted.txt"},
			wantErr:   false,
		},
		{
			name: "extract tar.gz",
			args: map[string]interface{}{
				"archive":     tarPath,
				"destination": filepath.Join(tmpDir, "extracted_tar"),
			},
			wantFiles: []string{"extracted2.txt"},
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := server.handleExtractArchive(tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("handleExtractArchive() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}

			// Verify extracted files exist
			dest := tt.args["destination"].(string)
			for _, file := range tt.wantFiles {
				if _, err := os.Stat(filepath.Join(dest, file)); os.IsNotExist(err) {
					t.Errorf("Extracted file not found: %s", file)
				}
			}

			_ = result
		})
	}
}

func TestSyncDirectories(t *testing.T) {
	tmpDir := t.TempDir()

	// Create source directory with files
	srcDir := filepath.Join(tmpDir, "source")
	os.MkdirAll(srcDir, 0755)
	os.WriteFile(filepath.Join(srcDir, "file1.txt"), []byte("content 1"), 0644)
	os.WriteFile(filepath.Join(srcDir, "file2.txt"), []byte("content 2"), 0644)
	os.MkdirAll(filepath.Join(srcDir, "subdir"), 0755)
	os.WriteFile(filepath.Join(srcDir, "subdir", "file3.txt"), []byte("content 3"), 0644)

	dstDir := filepath.Join(tmpDir, "dest")

	server, _ := NewServer([]string{tmpDir})

	result, err := server.handleSyncDirectories(map[string]interface{}{
		"source":      srcDir,
		"destination": dstDir,
	})
	if err != nil {
		t.Errorf("handleSyncDirectories() error = %v", err)
		return
	}

	// Verify files were synced
	files := []string{"file1.txt", "file2.txt", filepath.Join("subdir", "file3.txt")}
	for _, file := range files {
		if _, err := os.Stat(filepath.Join(dstDir, file)); os.IsNotExist(err) {
			t.Errorf("Synced file not found: %s", file)
		}
	}

	_ = result
}

func TestCompressFilesWithDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a directory with files
	subDir := filepath.Join(tmpDir, "mydir")
	os.MkdirAll(subDir, 0755)
	os.WriteFile(filepath.Join(subDir, "nested1.txt"), []byte("nested 1"), 0644)
	os.WriteFile(filepath.Join(subDir, "nested2.txt"), []byte("nested 2"), 0644)

	server, _ := NewServer([]string{tmpDir})

	// Test compressing a directory
	result, err := server.handleCompressFiles(map[string]interface{}{
		"paths":  []interface{}{subDir},
		"output": filepath.Join(tmpDir, "dir_archive.zip"),
		"format": "zip",
	})
	if err != nil {
		t.Errorf("handleCompressFiles() with directory error = %v", err)
	}

	// Verify archive was created
	if _, err := os.Stat(filepath.Join(tmpDir, "dir_archive.zip")); os.IsNotExist(err) {
		t.Errorf("Archive file was not created")
	}
	_ = result
}

func TestCompressFilesErrorCases(t *testing.T) {
	tmpDir := t.TempDir()
	server, _ := NewServer([]string{tmpDir})

	tests := []struct {
		name    string
		args    map[string]interface{}
		wantErr bool
	}{
		{
			name: "missing paths",
			args: map[string]interface{}{
				"output": filepath.Join(tmpDir, "test.zip"),
			},
			wantErr: true,
		},
		{
			name: "missing output",
			args: map[string]interface{}{
				"paths": []interface{}{filepath.Join(tmpDir, "file.txt")},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := server.handleCompressFiles(tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("handleCompressFiles() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestExtractArchiveErrorCases(t *testing.T) {
	tmpDir := t.TempDir()
	server, _ := NewServer([]string{tmpDir})

	tests := []struct {
		name    string
		args    map[string]interface{}
		wantErr bool
	}{
		{
			name: "missing archive",
			args: map[string]interface{}{
				"destination": filepath.Join(tmpDir, "dest"),
			},
			wantErr: true,
		},
		{
			name: "nonexistent archive",
			args: map[string]interface{}{
				"archive":     filepath.Join(tmpDir, "nonexistent.zip"),
				"destination": filepath.Join(tmpDir, "dest"),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := server.handleExtractArchive(tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("handleExtractArchive() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSyncDirectoriesErrorCases(t *testing.T) {
	tmpDir := t.TempDir()
	server, _ := NewServer([]string{tmpDir})

	tests := []struct {
		name    string
		args    map[string]interface{}
		wantErr bool
	}{
		{
			name: "missing source",
			args: map[string]interface{}{
				"destination": filepath.Join(tmpDir, "dest"),
			},
			wantErr: true,
		},
		{
			name: "missing destination",
			args: map[string]interface{}{
				"source": filepath.Join(tmpDir, "src"),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := server.handleSyncDirectories(tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("handleSyncDirectories() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGetDiskUsageErrorCases(t *testing.T) {
	tmpDir := t.TempDir()
	server, _ := NewServer([]string{tmpDir})

	tests := []struct {
		name    string
		args    map[string]interface{}
		wantErr bool
	}{
		{
			name:    "missing path",
			args:    map[string]interface{}{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := server.handleGetDiskUsage(tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("handleGetDiskUsage() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestFindLargeFilesErrorCases(t *testing.T) {
	tmpDir := t.TempDir()
	server, _ := NewServer([]string{tmpDir})

	tests := []struct {
		name    string
		args    map[string]interface{}
		wantErr bool
	}{
		{
			name:    "missing path",
			args:    map[string]interface{}{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := server.handleFindLargeFiles(tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("handleFindLargeFiles() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCompressTarGzWithDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a directory with nested files
	subDir := filepath.Join(tmpDir, "archive_dir")
	os.MkdirAll(filepath.Join(subDir, "nested"), 0755)
	os.WriteFile(filepath.Join(subDir, "file1.txt"), []byte("content 1"), 0644)
	os.WriteFile(filepath.Join(subDir, "nested", "file2.txt"), []byte("content 2"), 0644)

	server, _ := NewServer([]string{tmpDir})

	// Test tar.gz with directory
	result, err := server.handleCompressFiles(map[string]interface{}{
		"paths":  []interface{}{subDir},
		"output": filepath.Join(tmpDir, "dir_archive.tar.gz"),
		"format": "tar.gz",
	})
	if err != nil {
		t.Errorf("handleCompressFiles() with directory tar.gz error = %v", err)
	}

	// Verify archive was created
	if _, err := os.Stat(filepath.Join(tmpDir, "dir_archive.tar.gz")); os.IsNotExist(err) {
		t.Errorf("tar.gz archive file was not created")
	}

	_ = result
}

func TestCompressTarGzMultipleFiles(t *testing.T) {
	tmpDir := t.TempDir()

	// Create multiple files
	os.WriteFile(filepath.Join(tmpDir, "tar1.txt"), []byte("tar content 1"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "tar2.txt"), []byte("tar content 2"), 0644)

	server, _ := NewServer([]string{tmpDir})

	result, err := server.handleCompressFiles(map[string]interface{}{
		"paths":  []interface{}{filepath.Join(tmpDir, "tar1.txt"), filepath.Join(tmpDir, "tar2.txt")},
		"output": filepath.Join(tmpDir, "multi.tar.gz"),
		"format": "tar.gz",
	})
	if err != nil {
		t.Errorf("handleCompressFiles() with multiple tar.gz error = %v", err)
	}

	if _, err := os.Stat(filepath.Join(tmpDir, "multi.tar.gz")); os.IsNotExist(err) {
		t.Errorf("tar.gz archive file was not created")
	}

	_ = result
}
