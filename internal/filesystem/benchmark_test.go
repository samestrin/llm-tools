package filesystem

import (
	"os"
	"path/filepath"
	"testing"
)

// BenchmarkNewServer tests cold start performance
func BenchmarkNewServer(b *testing.B) {
	tmpDir := b.TempDir()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := NewServer([]string{tmpDir})
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkReadFile tests file read performance
func BenchmarkReadFile(b *testing.B) {
	tmpDir := b.TempDir()
	server, _ := NewServer([]string{tmpDir})

	// Create a test file with content
	testFile := filepath.Join(tmpDir, "benchmark.txt")
	content := make([]byte, 10000) // 10KB file
	for i := range content {
		content[i] = byte('a' + (i % 26))
	}
	os.WriteFile(testFile, content, 0644)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := server.handleReadFile(map[string]interface{}{
			"path": testFile,
		})
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkWriteFile tests file write performance
func BenchmarkWriteFile(b *testing.B) {
	tmpDir := b.TempDir()
	server, _ := NewServer([]string{tmpDir})

	content := make([]byte, 10000) // 10KB content
	for i := range content {
		content[i] = byte('a' + (i % 26))
	}
	contentStr := string(content)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		testFile := filepath.Join(tmpDir, "benchmark_write.txt")
		_, err := server.handleWriteFile(map[string]interface{}{
			"path":    testFile,
			"content": contentStr,
		})
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkListDirectory tests directory listing performance
func BenchmarkListDirectory(b *testing.B) {
	tmpDir := b.TempDir()
	server, _ := NewServer([]string{tmpDir})

	// Create multiple files
	for i := 0; i < 100; i++ {
		os.WriteFile(filepath.Join(tmpDir, "file"+string(rune('a'+i%26))+".txt"), []byte("content"), 0644)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := server.handleListDirectory(map[string]interface{}{
			"path": tmpDir,
		})
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkSearchCode tests code search performance
func BenchmarkSearchCode(b *testing.B) {
	tmpDir := b.TempDir()
	server, _ := NewServer([]string{tmpDir})

	// Create files with searchable content
	for i := 0; i < 50; i++ {
		content := "func TestFunction() {\n\treturn nil\n}\n"
		os.WriteFile(filepath.Join(tmpDir, "file"+string(rune('a'+i%26))+".go"), []byte(content), 0644)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := server.handleSearchCode(map[string]interface{}{
			"path":    tmpDir,
			"pattern": "TestFunction",
		})
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkEditBlock tests edit block performance
func BenchmarkEditBlock(b *testing.B) {
	tmpDir := b.TempDir()
	server, _ := NewServer([]string{tmpDir})

	testFile := filepath.Join(tmpDir, "edit_bench.txt")
	originalContent := "hello world hello world hello world"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Reset file before each edit
		os.WriteFile(testFile, []byte(originalContent), 0644)

		_, err := server.handleEditBlock(map[string]interface{}{
			"path":       testFile,
			"old_string": "hello",
			"new_string": "goodbye",
		})
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkGetDirectoryTree tests tree generation performance
func BenchmarkGetDirectoryTree(b *testing.B) {
	tmpDir := b.TempDir()
	server, _ := NewServer([]string{tmpDir})

	// Create nested directory structure
	for i := 0; i < 5; i++ {
		subDir := filepath.Join(tmpDir, "dir"+string(rune('a'+i)))
		os.MkdirAll(subDir, 0755)
		for j := 0; j < 10; j++ {
			os.WriteFile(filepath.Join(subDir, "file"+string(rune('0'+j))+".txt"), []byte("content"), 0644)
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := server.handleGetDirectoryTree(map[string]interface{}{
			"path": tmpDir,
		})
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkCopyFile tests file copy performance
func BenchmarkCopyFile(b *testing.B) {
	tmpDir := b.TempDir()
	server, _ := NewServer([]string{tmpDir})

	// Create a source file
	srcFile := filepath.Join(tmpDir, "source.txt")
	content := make([]byte, 50000) // 50KB file
	os.WriteFile(srcFile, content, 0644)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dstFile := filepath.Join(tmpDir, "dest.txt")
		_, err := server.handleCopyFile(map[string]interface{}{
			"source":      srcFile,
			"destination": dstFile,
		})
		if err != nil {
			b.Fatal(err)
		}
		os.Remove(dstFile)
	}
}
