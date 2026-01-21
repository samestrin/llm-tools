package semantic

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// trackingEmbedder tracks EmbedBatch calls for testing
type trackingEmbedder struct {
	embedding  []float32
	calls      []int // Size of each EmbedBatch call
	callsMutex sync.Mutex
}

func (t *trackingEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	return t.embedding, nil
}

func (t *trackingEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	t.callsMutex.Lock()
	t.calls = append(t.calls, len(texts))
	t.callsMutex.Unlock()

	result := make([][]float32, len(texts))
	for i := range texts {
		result[i] = t.embedding
	}
	return result, nil
}

func (t *trackingEmbedder) Dimensions() int {
	return len(t.embedding)
}

func (t *trackingEmbedder) Model() string {
	return "tracking-embedder"
}

func (t *trackingEmbedder) CallCount() int {
	t.callsMutex.Lock()
	defer t.callsMutex.Unlock()
	return len(t.calls)
}

func (t *trackingEmbedder) CallSizes() []int {
	t.callsMutex.Lock()
	defer t.callsMutex.Unlock()
	copied := make([]int, len(t.calls))
	copy(copied, t.calls)
	return copied
}

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

func TestIndexManager_WithParallelBatching(t *testing.T) {
	// Create temp directory with a file that will produce multiple chunks
	tmpDir := t.TempDir()

	// Write a test file with many functions to test parallel batching
	testFile := filepath.Join(tmpDir, "main.go")
	content := []byte(`package main

func Func1() int { return 1 }
func Func2() int { return 2 }
func Func3() int { return 3 }
func Func4() int { return 4 }
func Func5() int { return 5 }
func Func6() int { return 6 }
func Func7() int { return 7 }
func Func8() int { return 8 }
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

	// Index with batch size of 2 and parallel of 4
	result, err := mgr.Index(context.Background(), tmpDir, IndexOptions{
		BatchSize: 2,
		Parallel:  4,
	})
	if err != nil {
		t.Fatalf("Index() with parallel batching error = %v", err)
	}

	if result.FilesProcessed != 1 {
		t.Errorf("Index() processed %d files, want 1", result.FilesProcessed)
	}

	// Should have created chunks for each function
	if result.ChunksCreated < 8 {
		t.Errorf("Index() created %d chunks, want at least 8", result.ChunksCreated)
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

func TestIndexManager_ParallelZeroIsSequential(t *testing.T) {
	// Parallel = 0 should behave the same as sequential (no parallelism)
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

	// Parallel = 0 with batch size should use sequential batching
	result, err := mgr.Index(context.Background(), tmpDir, IndexOptions{
		BatchSize: 2,
		Parallel:  0,
	})
	if err != nil {
		t.Fatalf("Index() with Parallel=0 error = %v", err)
	}

	if result.ChunksCreated < 3 {
		t.Errorf("Index() created %d chunks, want at least 3", result.ChunksCreated)
	}
}

func TestIndexManager_ParallelOneIsSequential(t *testing.T) {
	// Parallel = 1 should also behave sequentially
	tmpDir := t.TempDir()

	testFile := filepath.Join(tmpDir, "main.go")
	content := []byte(`package main

func A() {}
func B() {}
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

	// Parallel = 1 with batch size should use sequential batching
	result, err := mgr.Index(context.Background(), tmpDir, IndexOptions{
		BatchSize: 1,
		Parallel:  1,
	})
	if err != nil {
		t.Fatalf("Index() with Parallel=1 error = %v", err)
	}

	if result.ChunksCreated < 2 {
		t.Errorf("Index() created %d chunks, want at least 2", result.ChunksCreated)
	}
}

func TestIndexManager_ParallelWithoutBatchSize(t *testing.T) {
	// Parallel with BatchSize=0 should send all at once (ignores parallel)
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

	// Parallel without batch size (BatchSize=0) should work but ignore parallel
	result, err := mgr.Index(context.Background(), tmpDir, IndexOptions{
		BatchSize: 0,
		Parallel:  4,
	})
	if err != nil {
		t.Fatalf("Index() with Parallel but no BatchSize error = %v", err)
	}

	if result.ChunksCreated < 3 {
		t.Errorf("Index() created %d chunks, want at least 3", result.ChunksCreated)
	}
}

func TestIndexManager_ParallelHighWorkerCount(t *testing.T) {
	// Test with more workers than batches (edge case)
	tmpDir := t.TempDir()

	testFile := filepath.Join(tmpDir, "main.go")
	content := []byte(`package main

func A() {}
func B() {}
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

	// More workers (10) than batches (~2 with batch size 1)
	result, err := mgr.Index(context.Background(), tmpDir, IndexOptions{
		BatchSize: 1,
		Parallel:  10,
	})
	if err != nil {
		t.Fatalf("Index() with high parallel count error = %v", err)
	}

	if result.ChunksCreated < 2 {
		t.Errorf("Index() created %d chunks, want at least 2", result.ChunksCreated)
	}
}

func TestIndexManager_ExcludeFiles(t *testing.T) {
	// Test that --exclude now works on files too, not just directories
	tmpDir := t.TempDir()

	// Write test files - one should be excluded
	os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte("package main\nfunc Main() {}"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "helper.go"), []byte("package main\nfunc Helper() {}"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "main_test.go"), []byte("package main\nfunc TestMain() {}"), 0644)

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

	// Index with file pattern exclusion
	result, err := mgr.Index(context.Background(), tmpDir, IndexOptions{
		Excludes: []string{"*_test.go"},
	})
	if err != nil {
		t.Fatalf("Index() error = %v", err)
	}

	// Should only process main.go and helper.go (2 files, not 3)
	if result.FilesProcessed != 2 {
		t.Errorf("Index() with file exclude processed %d files, want 2", result.FilesProcessed)
	}
}

func TestIndexManager_ExcludeTests(t *testing.T) {
	// Test --exclude-tests flag
	tmpDir := t.TempDir()

	// Create test directory structure
	testsDir := filepath.Join(tmpDir, "__tests__")
	os.MkdirAll(testsDir, 0755)

	// Write source files
	os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte("package main\nfunc Main() {}"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "util.go"), []byte("package main\nfunc Util() {}"), 0644)

	// Write test files (should be excluded)
	os.WriteFile(filepath.Join(tmpDir, "main_test.go"), []byte("package main\nfunc TestMain() {}"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "util_test.go"), []byte("package main\nfunc TestUtil() {}"), 0644)
	os.WriteFile(filepath.Join(testsDir, "integration.go"), []byte("package tests\nfunc Integration() {}"), 0644)

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

	// Index with ExcludeTests enabled
	result, err := mgr.Index(context.Background(), tmpDir, IndexOptions{
		ExcludeTests: true,
	})
	if err != nil {
		t.Fatalf("Index() error = %v", err)
	}

	// Should only process main.go and util.go (2 files)
	// main_test.go, util_test.go are excluded by pattern
	// __tests__/integration.go is excluded by directory
	if result.FilesProcessed != 2 {
		t.Errorf("Index() with ExcludeTests processed %d files, want 2", result.FilesProcessed)
	}
}

func TestIndexManager_ExcludeTestsWithJSPatterns(t *testing.T) {
	// Test --exclude-tests with JavaScript/TypeScript patterns
	tmpDir := t.TempDir()

	// Create __tests__ directory
	testsDir := filepath.Join(tmpDir, "__tests__")
	os.MkdirAll(testsDir, 0755)

	// Write source files
	os.WriteFile(filepath.Join(tmpDir, "app.ts"), []byte("function app() {}"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "utils.ts"), []byte("function utils() {}"), 0644)

	// Write test files (should be excluded)
	os.WriteFile(filepath.Join(tmpDir, "app.test.ts"), []byte("test('app', () => {})"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "utils.spec.ts"), []byte("describe('utils', () => {})"), 0644)
	os.WriteFile(filepath.Join(testsDir, "e2e.ts"), []byte("test('e2e', () => {})"), 0644)

	storage, err := NewSQLiteStorage(":memory:", 4)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer storage.Close()

	embedder := &mockEmbedder{
		embedding: []float32{0.1, 0.2, 0.3, 0.4},
	}

	factory := NewChunkerFactory()
	// Register JS chunker for .ts files
	jsChunker := NewJSChunker()
	factory.Register("ts", jsChunker)

	mgr := NewIndexManager(storage, embedder, factory)

	// Index with ExcludeTests enabled
	result, err := mgr.Index(context.Background(), tmpDir, IndexOptions{
		ExcludeTests: true,
	})
	if err != nil {
		t.Fatalf("Index() error = %v", err)
	}

	// Should only process app.ts and utils.ts (2 files)
	if result.FilesProcessed != 2 {
		t.Errorf("Index() with ExcludeTests (JS) processed %d files, want 2", result.FilesProcessed)
	}
}

func TestTestFilePatterns(t *testing.T) {
	// Verify TestFilePatterns contains expected patterns
	expectedPatterns := []string{
		"*_test.go",
		"*.test.ts",
		"*.spec.js",
		"test_*.py",
	}

	for _, expected := range expectedPatterns {
		found := false
		for _, pattern := range TestFilePatterns {
			if pattern == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("TestFilePatterns missing expected pattern: %s", expected)
		}
	}
}

func TestTestDirPatterns(t *testing.T) {
	// Verify TestDirPatterns contains expected patterns
	expectedPatterns := []string{
		"__tests__",
		"test",
		"tests",
		"testdata",
	}

	for _, expected := range expectedPatterns {
		found := false
		for _, pattern := range TestDirPatterns {
			if pattern == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("TestDirPatterns missing expected pattern: %s", expected)
		}
	}
}

func TestIndexManager_EmbedBatchSize(t *testing.T) {
	// Create temp directory with multiple Go files
	tmpDir := t.TempDir()

	// Create 3 files, each with 2 functions (2 chunks per file = 6 total chunks)
	files := []struct {
		name    string
		content string
	}{
		{"file1.go", "package main\nfunc A1() int { return 1 }\nfunc A2() int { return 2 }"},
		{"file2.go", "package main\nfunc B1() int { return 1 }\nfunc B2() int { return 2 }"},
		{"file3.go", "package main\nfunc C1() int { return 1 }\nfunc C2() int { return 2 }"},
	}

	for _, f := range files {
		if err := os.WriteFile(filepath.Join(tmpDir, f.name), []byte(f.content), 0644); err != nil {
			t.Fatalf("Failed to write %s: %v", f.name, err)
		}
	}

	// Create storage
	storage, err := NewSQLiteStorage(":memory:", 4)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer storage.Close()

	// Create tracking embedder
	embedder := &trackingEmbedder{
		embedding: []float32{0.1, 0.2, 0.3, 0.4},
	}

	// Create chunker factory
	factory := NewChunkerFactory()
	factory.Register("go", NewGoChunker())

	mgr := NewIndexManager(storage, embedder, factory)

	// Index with EmbedBatchSize of 4 (should batch 6 chunks into 2 API calls: 4 + 2)
	result, err := mgr.Index(context.Background(), tmpDir, IndexOptions{
		EmbedBatchSize: 4,
	})
	if err != nil {
		t.Fatalf("Index() with EmbedBatchSize error = %v", err)
	}

	if result.FilesProcessed != 3 {
		t.Errorf("Index() processed %d files, want 3", result.FilesProcessed)
	}

	// Should have 6 chunks (2 per file × 3 files)
	if result.ChunksCreated < 6 {
		t.Errorf("Index() created %d chunks, want at least 6", result.ChunksCreated)
	}

	// With EmbedBatchSize=4 and 6 chunks, we expect 2 API calls (4 + 2)
	// instead of 3 calls (one per file)
	callCount := embedder.CallCount()
	if callCount != 2 {
		t.Errorf("EmbedBatch called %d times, want 2 (batching 6 chunks with batch size 4)", callCount)
	}

	callSizes := embedder.CallSizes()
	if len(callSizes) != 2 || callSizes[0] != 4 || callSizes[1] != 2 {
		t.Errorf("EmbedBatch call sizes = %v, want [4, 2]", callSizes)
	}
}

func TestIndexManager_EmbedBatchSize_ReducesAPICalls(t *testing.T) {
	factory := NewChunkerFactory()
	factory.Register("go", NewGoChunker())

	// Test 1: Without EmbedBatchSize (per-file batching)
	tmpDir1 := t.TempDir()
	for i := 0; i < 5; i++ {
		content := []byte("package main\nfunc FuncA" + string(rune('0'+i)) + "() int { return " + string(rune('0'+i)) + " }")
		if err := os.WriteFile(filepath.Join(tmpDir1, "file"+string(rune('A'+i))+".go"), content, 0644); err != nil {
			t.Fatalf("Failed to write file: %v", err)
		}
	}
	storage1, _ := NewSQLiteStorage(":memory:", 4)
	defer storage1.Close()
	embedder1 := &trackingEmbedder{embedding: []float32{0.1, 0.2, 0.3, 0.4}}
	mgr1 := NewIndexManager(storage1, embedder1, factory)

	_, err := mgr1.Index(context.Background(), tmpDir1, IndexOptions{})
	if err != nil {
		t.Fatalf("Index() without EmbedBatchSize error = %v", err)
	}
	perFileCallCount := embedder1.CallCount()

	// Test 2: With EmbedBatchSize=10 (all chunks in 1 API call)
	tmpDir2 := t.TempDir()
	for i := 0; i < 5; i++ {
		content := []byte("package main\nfunc FuncB" + string(rune('0'+i)) + "() int { return " + string(rune('0'+i)) + " }")
		if err := os.WriteFile(filepath.Join(tmpDir2, "file"+string(rune('A'+i))+".go"), content, 0644); err != nil {
			t.Fatalf("Failed to write file: %v", err)
		}
	}
	storage2, _ := NewSQLiteStorage(":memory:", 4)
	defer storage2.Close()
	embedder2 := &trackingEmbedder{embedding: []float32{0.1, 0.2, 0.3, 0.4}}
	mgr2 := NewIndexManager(storage2, embedder2, factory)

	_, err = mgr2.Index(context.Background(), tmpDir2, IndexOptions{
		EmbedBatchSize: 10,
	})
	if err != nil {
		t.Fatalf("Index() with EmbedBatchSize error = %v", err)
	}
	batchedCallCount := embedder2.CallCount()

	// Per-file should have 5 calls (one per file)
	if perFileCallCount != 5 {
		t.Errorf("Per-file batching made %d API calls, want 5", perFileCallCount)
	}

	// Cross-file batching should have 1 call
	if batchedCallCount != 1 {
		t.Errorf("Cross-file batching made %d API calls, want 1", batchedCallCount)
	}

	// Verify reduction
	if batchedCallCount >= perFileCallCount {
		t.Errorf("EmbedBatchSize should reduce API calls: per-file=%d, batched=%d", perFileCallCount, batchedCallCount)
	}
}

func TestIndexManager_EmbedBatchSize_ZeroIsPerFile(t *testing.T) {
	// EmbedBatchSize=0 should use per-file batching (original behavior)
	tmpDir := t.TempDir()

	// Create 2 files
	os.WriteFile(filepath.Join(tmpDir, "a.go"), []byte("package main\nfunc A() {}"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "b.go"), []byte("package main\nfunc B() {}"), 0644)

	storage, _ := NewSQLiteStorage(":memory:", 4)
	defer storage.Close()
	embedder := &trackingEmbedder{embedding: []float32{0.1, 0.2, 0.3, 0.4}}
	factory := NewChunkerFactory()
	factory.Register("go", NewGoChunker())
	mgr := NewIndexManager(storage, embedder, factory)

	_, err := mgr.Index(context.Background(), tmpDir, IndexOptions{
		EmbedBatchSize: 0, // Zero means per-file
	})
	if err != nil {
		t.Fatalf("Index() error = %v", err)
	}

	// Should have 2 calls (one per file) since EmbedBatchSize=0
	if embedder.CallCount() != 2 {
		t.Errorf("EmbedBatch called %d times, want 2 (per-file batching)", embedder.CallCount())
	}
}

func TestIndexManager_EmbedBatchSize_WithStorageBatching(t *testing.T) {
	// Test that EmbedBatchSize works correctly with BatchSize (storage batching)
	tmpDir := t.TempDir()

	// Create 2 files with 3 functions each = 6 chunks
	os.WriteFile(filepath.Join(tmpDir, "a.go"), []byte("package main\nfunc A1() {}\nfunc A2() {}\nfunc A3() {}"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "b.go"), []byte("package main\nfunc B1() {}\nfunc B2() {}\nfunc B3() {}"), 0644)

	storage, _ := NewSQLiteStorage(":memory:", 4)
	defer storage.Close()
	embedder := &trackingEmbedder{embedding: []float32{0.1, 0.2, 0.3, 0.4}}
	factory := NewChunkerFactory()
	factory.Register("go", NewGoChunker())
	mgr := NewIndexManager(storage, embedder, factory)

	result, err := mgr.Index(context.Background(), tmpDir, IndexOptions{
		EmbedBatchSize: 3, // Embed 3 chunks per API call
		BatchSize:      2, // Store 2 vectors per upsert
	})
	if err != nil {
		t.Fatalf("Index() error = %v", err)
	}

	// Should have at least 6 chunks
	if result.ChunksCreated < 6 {
		t.Errorf("Index() created %d chunks, want at least 6", result.ChunksCreated)
	}

	// With 6 chunks and EmbedBatchSize=3, we expect 2 embed API calls
	if embedder.CallCount() != 2 {
		t.Errorf("EmbedBatch called %d times, want 2", embedder.CallCount())
	}

	// Verify all chunks are stored
	chunks, _ := storage.List(context.Background(), ListOptions{})
	if len(chunks) != result.ChunksCreated {
		t.Errorf("Storage has %d chunks, want %d", len(chunks), result.ChunksCreated)
	}
}

func TestIndexManager_UploadProgressCallback(t *testing.T) {
	// Test that upload progress callback is called during indexing
	// With the new incremental commit design, we get "indexing" phase events
	tmpDir := t.TempDir()

	// Create 2 files with 3 functions each = 6 chunks
	os.WriteFile(filepath.Join(tmpDir, "a.go"), []byte("package main\nfunc A1() {}\nfunc A2() {}\nfunc A3() {}"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "b.go"), []byte("package main\nfunc B1() {}\nfunc B2() {}\nfunc B3() {}"), 0644)

	storage, _ := NewSQLiteStorage(":memory:", 4)
	defer storage.Close()
	embedder := &trackingEmbedder{embedding: []float32{0.1, 0.2, 0.3, 0.4}}
	factory := NewChunkerFactory()
	factory.Register("go", NewGoChunker())
	mgr := NewIndexManager(storage, embedder, factory)

	// Track callback events
	var uploadEvents []UploadProgressEvent
	var mu sync.Mutex

	result, err := mgr.Index(context.Background(), tmpDir, IndexOptions{
		EmbedBatchSize: 3, // Process ~3 chunks per batch
		OnUploadProgress: func(event UploadProgressEvent) {
			mu.Lock()
			uploadEvents = append(uploadEvents, event)
			mu.Unlock()
		},
	})
	if err != nil {
		t.Fatalf("Index() error = %v", err)
	}

	// Verify we got progress events
	if len(uploadEvents) == 0 {
		t.Fatal("Expected upload progress events, got none")
	}

	// Check that we have indexing phase events (new incremental commit design)
	hasIndexing := false
	for _, e := range uploadEvents {
		if e.Phase == "indexing" {
			hasIndexing = true
		}
	}

	if !hasIndexing {
		t.Error("Expected indexing phase events, got none")
	}

	// Verify the last event has correct totals
	lastEvent := uploadEvents[len(uploadEvents)-1]
	if lastEvent.Current != lastEvent.Total {
		t.Errorf("Last event Current (%d) != Total (%d)", lastEvent.Current, lastEvent.Total)
	}
	if lastEvent.ChunksUploaded != result.ChunksCreated {
		t.Errorf("Last event ChunksUploaded (%d) != result.ChunksCreated (%d)", lastEvent.ChunksUploaded, result.ChunksCreated)
	}
}

func TestIndexManager_IncrementalCommit(t *testing.T) {
	// Test that incremental commit design allows resuming interrupted indexing
	tmpDir := t.TempDir()

	// Create 4 files
	os.WriteFile(filepath.Join(tmpDir, "a.go"), []byte("package main\nfunc A1() {}\nfunc A2() {}"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "b.go"), []byte("package main\nfunc B1() {}\nfunc B2() {}"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "c.go"), []byte("package main\nfunc C1() {}\nfunc C2() {}"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "d.go"), []byte("package main\nfunc D1() {}\nfunc D2() {}"), 0644)

	storage, _ := NewSQLiteStorage(":memory:", 4)
	defer storage.Close()
	embedder := &trackingEmbedder{embedding: []float32{0.1, 0.2, 0.3, 0.4}}
	factory := NewChunkerFactory()
	factory.Register("go", NewGoChunker())
	mgr := NewIndexManager(storage, embedder, factory)

	// First indexing
	result1, err := mgr.Index(context.Background(), tmpDir, IndexOptions{
		EmbedBatchSize: 4, // ~4 chunks per batch
	})
	if err != nil {
		t.Fatalf("First Index() error = %v", err)
	}

	// Re-run without force - should skip all files
	result2, err := mgr.Index(context.Background(), tmpDir, IndexOptions{
		EmbedBatchSize: 4,
	})
	if err != nil {
		t.Fatalf("Second Index() error = %v", err)
	}

	// Second run should have processed 0 files (all skipped)
	if result2.FilesProcessed != 0 {
		t.Errorf("Expected 0 files processed on resume, got %d", result2.FilesProcessed)
	}

	// But unchanged should match first run's processed
	if result2.FilesUnchanged != result1.FilesProcessed {
		t.Errorf("Expected %d files unchanged, got %d", result1.FilesProcessed, result2.FilesUnchanged)
	}
}
