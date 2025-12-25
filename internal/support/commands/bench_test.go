// Package commands provides benchmark tests for performance verification.
//
// Performance Targets:
// - Cold start: < 20ms (vs Python ~200ms)
// - listdir (1000 files): < 50ms
// - multigrep (10 keywords, 500 files): < 500ms
// - tree (depth 5): < 100ms
package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// createBenchmarkFiles creates test files for benchmarking
func createBenchmarkFiles(b *testing.B, count int) string {
	b.Helper()
	dir, err := os.MkdirTemp("", "bench")
	if err != nil {
		b.Fatalf("failed to create temp dir: %v", err)
	}

	// Create nested directories and files
	for i := 0; i < count; i++ {
		subdir := filepath.Join(dir, "dir"+string(rune('a'+i%26)))
		os.MkdirAll(subdir, 0755)

		filename := filepath.Join(subdir, "file"+string(rune('0'+i%10))+".go")
		content := []byte("package main\n\nfunc main() {\n\tprintln(\"hello\")\n}\n")
		if err := os.WriteFile(filename, content, 0644); err != nil {
			b.Fatalf("failed to create file: %v", err)
		}
	}

	return dir
}

// BenchmarkListdir benchmarks the listdir command with many files
// Target: 1000 files in < 50ms
func BenchmarkListdir1000(b *testing.B) {
	dir := createBenchmarkFiles(b, 1000)
	defer os.RemoveAll(dir)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cmd := newListdirCmd()
		buf := &bytes.Buffer{}
		cmd.SetOut(buf)
		cmd.SetErr(buf)
		cmd.SetArgs([]string{dir})
		if err := cmd.Execute(); err != nil {
			b.Fatalf("listdir failed: %v", err)
		}
	}
}

// BenchmarkTree benchmarks the tree command at depth 5
// Target: depth 5 in < 100ms
func BenchmarkTreeDepth5(b *testing.B) {
	// Create nested directory structure
	dir, err := os.MkdirTemp("", "bench-tree")
	if err != nil {
		b.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(dir)

	// Create 5 levels of nested directories with files
	createNestedDirs(b, dir, 5, 5) // 5 subdirs per level, 5 levels deep

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cmd := newTreeCmd()
		buf := &bytes.Buffer{}
		cmd.SetOut(buf)
		cmd.SetErr(buf)
		cmd.SetArgs([]string{dir, "--depth", "5"})
		if err := cmd.Execute(); err != nil {
			b.Fatalf("tree failed: %v", err)
		}
	}
}

// createNestedDirs creates nested directory structure for tree benchmarks
func createNestedDirs(b *testing.B, base string, breadth, depth int) {
	if depth == 0 {
		return
	}

	for i := 0; i < breadth; i++ {
		subdir := filepath.Join(base, "dir"+string(rune('a'+i)))
		if err := os.MkdirAll(subdir, 0755); err != nil {
			b.Fatalf("failed to create dir: %v", err)
		}

		// Create a file in each directory
		filename := filepath.Join(subdir, "file.go")
		if err := os.WriteFile(filename, []byte("package main\n"), 0644); err != nil {
			b.Fatalf("failed to create file: %v", err)
		}

		createNestedDirs(b, subdir, breadth, depth-1)
	}
}

// BenchmarkHash benchmarks the hash command
func BenchmarkHash(b *testing.B) {
	// Create a file to hash
	dir, _ := os.MkdirTemp("", "bench-hash")
	defer os.RemoveAll(dir)

	filename := filepath.Join(dir, "test.txt")
	// Create a 1MB file
	data := make([]byte, 1024*1024)
	for i := range data {
		data[i] = byte(i % 256)
	}
	os.WriteFile(filename, data, 0644)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cmd := newHashCmd()
		buf := &bytes.Buffer{}
		cmd.SetOut(buf)
		cmd.SetErr(buf)
		cmd.SetArgs([]string{filename})
		if err := cmd.Execute(); err != nil {
			b.Fatalf("hash failed: %v", err)
		}
	}
}

// BenchmarkMath benchmarks the math expression evaluator
func BenchmarkMath(b *testing.B) {
	expressions := []string{
		"2 + 3",
		"(10 + 5) * 2 - 8 / 4",
		"sqrt(16) + pow(2, 3)",
		"abs(-5) + max(1, 2, 3)",
	}

	for _, expr := range expressions {
		b.Run(expr, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				cmd := newMathCmd()
				buf := &bytes.Buffer{}
				cmd.SetOut(buf)
				cmd.SetErr(buf)
				cmd.SetArgs([]string{expr})
				if err := cmd.Execute(); err != nil {
					b.Fatalf("math failed: %v", err)
				}
			}
		})
	}
}

// BenchmarkCount benchmarks the count command
func BenchmarkCount(b *testing.B) {
	// Create a file with checkboxes
	dir, _ := os.MkdirTemp("", "bench-count")
	defer os.RemoveAll(dir)

	filename := filepath.Join(dir, "test.md")
	content := `# Test File

## Section 1
- [ ] Todo 1
- [x] Done 1
- [ ] Todo 2
- [x] Done 2
- [ ] Todo 3

## Section 2
- [x] Done 3
- [ ] Todo 4
- [x] Done 4
`
	os.WriteFile(filename, []byte(content), 0644)

	b.Run("checkboxes", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			cmd := newCountCmd()
			buf := &bytes.Buffer{}
			cmd.SetOut(buf)
			cmd.SetErr(buf)
			cmd.SetArgs([]string{"--mode", "checkboxes", filename})
			if err := cmd.Execute(); err != nil {
				b.Fatalf("count failed: %v", err)
			}
		}
	})

	b.Run("lines", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			cmd := newCountCmd()
			buf := &bytes.Buffer{}
			cmd.SetOut(buf)
			cmd.SetErr(buf)
			cmd.SetArgs([]string{"--mode", "lines", filename})
			if err := cmd.Execute(); err != nil {
				b.Fatalf("count failed: %v", err)
			}
		}
	})
}

// BenchmarkStats benchmarks the stats command
func BenchmarkStats(b *testing.B) {
	dir := createBenchmarkFiles(b, 100)
	defer os.RemoveAll(dir)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cmd := newStatsCmd()
		buf := &bytes.Buffer{}
		cmd.SetOut(buf)
		cmd.SetErr(buf)
		cmd.SetArgs([]string{dir})
		if err := cmd.Execute(); err != nil {
			b.Fatalf("stats failed: %v", err)
		}
	}
}
