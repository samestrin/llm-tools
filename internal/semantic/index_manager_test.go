package semantic

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestIndexManager_Index(t *testing.T) {
	// Create temp directory with Go files
	tmpDir := t.TempDir()

	// Write test files
	testFile := filepath.Join(tmpDir, "main.go")
	content := []byte(`package main

func Add(a, b int) int {
	return a + b
}

func Sub(a, b int) int {
	return a - b
}
`)
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Create in-memory storage
	storage, err := NewSQLiteStorage(":memory:", 4)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer storage.Close()

	// Create mock embedder
	embedder := &mockEmbedder{
		embedding: []float32{0.1, 0.2, 0.3, 0.4},
	}

	// Create chunker factory with Go chunker
	factory := NewChunkerFactory()
	factory.Register("go", NewGoChunker())

	// Create index manager
	mgr := NewIndexManager(storage, embedder, factory)

	// Index the directory
	result, err := mgr.Index(context.Background(), tmpDir, IndexOptions{})
	if err != nil {
		t.Fatalf("Index() error = %v", err)
	}

	if result.FilesProcessed != 1 {
		t.Errorf("Index() processed %d files, want 1", result.FilesProcessed)
	}

	if result.ChunksCreated < 2 {
		t.Errorf("Index() created %d chunks, want at least 2", result.ChunksCreated)
	}

	// Verify chunks are in storage
	chunks, err := storage.List(context.Background(), ListOptions{})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if len(chunks) != result.ChunksCreated {
		t.Errorf("Storage has %d chunks, index reported %d", len(chunks), result.ChunksCreated)
	}
}

func TestIndexManager_Update(t *testing.T) {
	tmpDir := t.TempDir()

	// Write initial file
	testFile := filepath.Join(tmpDir, "main.go")
	content := []byte(`package main

func Original() {}
`)
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	storage, err := NewSQLiteStorage(":memory:", 4)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer storage.Close()

	embedder := &mockEmbedder{
		embedding: []float32{0.1, 0.2, 0.3, 0.4},
	}

	factory := NewChunkerFactory()
	factory.Register("go", NewGoChunker())

	mgr := NewIndexManager(storage, embedder, factory)

	// Initial index
	_, err = mgr.Index(context.Background(), tmpDir, IndexOptions{})
	if err != nil {
		t.Fatalf("Initial Index() error = %v", err)
	}

	// Modify file
	newContent := []byte(`package main

func Updated() {}
func New() {}
`)
	if err := os.WriteFile(testFile, newContent, 0644); err != nil {
		t.Fatalf("Failed to update test file: %v", err)
	}

	// Update index
	result, err := mgr.Update(context.Background(), tmpDir, UpdateOptions{})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	// Should have detected the modified file
	if result.FilesUpdated != 1 {
		t.Errorf("Update() updated %d files, want 1", result.FilesUpdated)
	}
}

func TestIndexManager_Status(t *testing.T) {
	tmpDir := t.TempDir()

	// Write test file
	testFile := filepath.Join(tmpDir, "main.go")
	content := []byte(`package main
func Foo() {}
`)
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	storage, err := NewSQLiteStorage(":memory:", 4)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer storage.Close()

	embedder := &mockEmbedder{
		embedding: []float32{0.1, 0.2, 0.3, 0.4},
	}

	factory := NewChunkerFactory()
	factory.Register("go", NewGoChunker())

	mgr := NewIndexManager(storage, embedder, factory)

	// Index first
	mgr.Index(context.Background(), tmpDir, IndexOptions{})

	// Get status
	status, err := mgr.Status(context.Background())
	if err != nil {
		t.Fatalf("Status() error = %v", err)
	}

	if status.ChunksTotal < 1 {
		t.Errorf("Status() ChunksTotal = %d, want at least 1", status.ChunksTotal)
	}

	if status.FilesIndexed < 1 {
		t.Errorf("Status() FilesIndexed = %d, want at least 1", status.FilesIndexed)
	}
}

func TestIndexManager_WithExcludes(t *testing.T) {
	tmpDir := t.TempDir()

	// Create directory structure
	srcDir := filepath.Join(tmpDir, "src")
	vendorDir := filepath.Join(tmpDir, "vendor")
	os.MkdirAll(srcDir, 0755)
	os.MkdirAll(vendorDir, 0755)

	// Write files
	os.WriteFile(filepath.Join(srcDir, "main.go"), []byte("package main\nfunc Main() {}"), 0644)
	os.WriteFile(filepath.Join(vendorDir, "lib.go"), []byte("package vendor\nfunc Lib() {}"), 0644)

	storage, err := NewSQLiteStorage(":memory:", 4)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer storage.Close()

	embedder := &mockEmbedder{
		embedding: []float32{0.1, 0.2, 0.3, 0.4},
	}

	factory := NewChunkerFactory()
	factory.Register("go", NewGoChunker())

	mgr := NewIndexManager(storage, embedder, factory)

	// Index with vendor excluded
	result, err := mgr.Index(context.Background(), tmpDir, IndexOptions{
		Excludes: []string{"vendor"},
	})
	if err != nil {
		t.Fatalf("Index() error = %v", err)
	}

	// Should only process src/main.go
	if result.FilesProcessed != 1 {
		t.Errorf("Index() with excludes processed %d files, want 1", result.FilesProcessed)
	}
}

func TestIndexManager_WithIncludes(t *testing.T) {
	tmpDir := t.TempDir()

	// Write different file types
	os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte("package main\nfunc Main() {}"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "test.go"), []byte("package main\nfunc Test() {}"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "script.py"), []byte("def foo(): pass"), 0644)

	storage, err := NewSQLiteStorage(":memory:", 4)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer storage.Close()

	embedder := &mockEmbedder{
		embedding: []float32{0.1, 0.2, 0.3, 0.4},
	}

	factory := NewChunkerFactory()
	factory.Register("go", NewGoChunker())
	// Note: No Python chunker registered

	mgr := NewIndexManager(storage, embedder, factory)

	// Index with explicit includes
	result, err := mgr.Index(context.Background(), tmpDir, IndexOptions{
		Includes: []string{"*.go"},
	})
	if err != nil {
		t.Fatalf("Index() error = %v", err)
	}

	// Should only process .go files
	if result.FilesProcessed != 2 {
		t.Errorf("Index() with includes processed %d files, want 2", result.FilesProcessed)
	}
}

func TestIndexManager_EmptyDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	storage, err := NewSQLiteStorage(":memory:", 4)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer storage.Close()

	embedder := &mockEmbedder{
		embedding: []float32{0.1, 0.2, 0.3, 0.4},
	}

	factory := NewChunkerFactory()
	factory.Register("go", NewGoChunker())

	mgr := NewIndexManager(storage, embedder, factory)

	result, err := mgr.Index(context.Background(), tmpDir, IndexOptions{})
	if err != nil {
		t.Fatalf("Index() error = %v", err)
	}

	if result.FilesProcessed != 0 {
		t.Errorf("Index() on empty dir processed %d files, want 0", result.FilesProcessed)
	}

	if result.ChunksCreated != 0 {
		t.Errorf("Index() on empty dir created %d chunks, want 0", result.ChunksCreated)
	}
}

func TestIndexManager_InvalidPath(t *testing.T) {
	storage, err := NewSQLiteStorage(":memory:", 4)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer storage.Close()

	embedder := &mockEmbedder{
		embedding: []float32{0.1, 0.2, 0.3, 0.4},
	}

	factory := NewChunkerFactory()

	mgr := NewIndexManager(storage, embedder, factory)

	_, err = mgr.Index(context.Background(), "/nonexistent/path/12345", IndexOptions{})
	if err == nil {
		t.Error("Index() should return error for invalid path")
	}
}

func TestIndexManager_WithBatchSize(t *testing.T) {
	// Create temp directory with a file that will produce multiple chunks
	tmpDir := t.TempDir()

	// Write a test file with multiple functions (each becomes a chunk)
	testFile := filepath.Join(tmpDir, "main.go")
	content := []byte(`package main

func Func1() int { return 1 }
func Func2() int { return 2 }
func Func3() int { return 3 }
func Func4() int { return 4 }
func Func5() int { return 5 }
`)
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Create storage
	storage, err := NewSQLiteStorage(":memory:", 4)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer storage.Close()

	// Create mock embedder
	embedder := &mockEmbedder{
		embedding: []float32{0.1, 0.2, 0.3, 0.4},
	}

	// Create chunker factory
	factory := NewChunkerFactory()
	factory.Register("go", NewGoChunker())

	mgr := NewIndexManager(storage, embedder, factory)

	// Index with batch size of 2 (should work fine with 5 functions)
	result, err := mgr.Index(context.Background(), tmpDir, IndexOptions{
		BatchSize: 2,
	})
	if err != nil {
		t.Fatalf("Index() with BatchSize error = %v", err)
	}

	if result.FilesProcessed != 1 {
		t.Errorf("Index() processed %d files, want 1", result.FilesProcessed)
	}

	// Should have created chunks for each function
	if result.ChunksCreated < 5 {
		t.Errorf("Index() created %d chunks, want at least 5", result.ChunksCreated)
	}

	// Verify all chunks are in storage
	chunks, err := storage.List(context.Background(), ListOptions{})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if len(chunks) != result.ChunksCreated {
		t.Errorf("Storage has %d chunks, index reported %d", len(chunks), result.ChunksCreated)
	}
}

func TestIndexManager_BatchSizeZeroIsUnlimited(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()

	testFile := filepath.Join(tmpDir, "main.go")
	content := []byte(`package main

func A() {}
func B() {}
func C() {}
`)
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	storage, err := NewSQLiteStorage(":memory:", 4)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer storage.Close()

	embedder := &mockEmbedder{
		embedding: []float32{0.1, 0.2, 0.3, 0.4},
	}

	factory := NewChunkerFactory()
	factory.Register("go", NewGoChunker())

	mgr := NewIndexManager(storage, embedder, factory)

	// BatchSize = 0 means unlimited (all chunks in one batch)
	result, err := mgr.Index(context.Background(), tmpDir, IndexOptions{
		BatchSize: 0,
	})
	if err != nil {
		t.Fatalf("Index() with BatchSize=0 error = %v", err)
	}

	if result.ChunksCreated < 3 {
		t.Errorf("Index() created %d chunks, want at least 3", result.ChunksCreated)
	}
}

func TestIndexManager_BatchSizeOne(t *testing.T) {
	// Test edge case: batch size of 1 (each vector sent individually)
	tmpDir := t.TempDir()

	testFile := filepath.Join(tmpDir, "main.go")
	content := []byte(`package main

func X() {}
func Y() {}
`)
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	storage, err := NewSQLiteStorage(":memory:", 4)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer storage.Close()

	embedder := &mockEmbedder{
		embedding: []float32{0.1, 0.2, 0.3, 0.4},
	}

	factory := NewChunkerFactory()
	factory.Register("go", NewGoChunker())

	mgr := NewIndexManager(storage, embedder, factory)

	// BatchSize = 1 means each vector sent separately
	result, err := mgr.Index(context.Background(), tmpDir, IndexOptions{
		BatchSize: 1,
	})
	if err != nil {
		t.Fatalf("Index() with BatchSize=1 error = %v", err)
	}

	if result.ChunksCreated < 2 {
		t.Errorf("Index() created %d chunks, want at least 2", result.ChunksCreated)
	}

	// Verify chunks in storage
	chunks, err := storage.List(context.Background(), ListOptions{})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if len(chunks) != result.ChunksCreated {
		t.Errorf("Storage has %d chunks, index reported %d", len(chunks), result.ChunksCreated)
	}
}

func TestDefaultBatchSize(t *testing.T) {
	// Verify the default batch size constant is reasonable
	if DefaultBatchSize <= 0 {
		t.Errorf("DefaultBatchSize = %d, want positive value", DefaultBatchSize)
	}
	if DefaultBatchSize > 1000 {
		t.Errorf("DefaultBatchSize = %d, seems too large (want <= 1000)", DefaultBatchSize)
	}
}
