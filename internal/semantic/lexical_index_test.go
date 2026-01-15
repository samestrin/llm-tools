package semantic

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// TestLexicalIndex_NewInMemory verifies that an in-memory lexical index can be created.
func TestLexicalIndex_NewInMemory(t *testing.T) {
	idx, err := NewLexicalIndex(":memory:", 384)
	if err != nil {
		t.Fatalf("failed to create in-memory lexical index: %v", err)
	}
	defer idx.Close()

	// Verify index is usable
	ctx := context.Background()
	results, err := idx.Search(ctx, "test", LexicalSearchOptions{TopK: 10})
	if err != nil {
		t.Fatalf("search on empty index should not error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results on empty index, got %d", len(results))
	}
}

// TestLexicalIndex_NewWithPath verifies that a lexical index can be created at a file path.
func TestLexicalIndex_NewWithPath(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test-fts.db")

	idx, err := NewLexicalIndex(dbPath, 384)
	if err != nil {
		t.Fatalf("failed to create lexical index at path: %v", err)
	}
	defer idx.Close()

	// Verify the database file was created
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Error("database file was not created")
	}
}

// TestLexicalIndex_CreateParentDirs verifies that parent directories are created if missing.
func TestLexicalIndex_CreateParentDirs(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "nested", "dirs", "test-fts.db")

	idx, err := NewLexicalIndex(dbPath, 384)
	if err != nil {
		t.Fatalf("failed to create lexical index with nested path: %v", err)
	}
	defer idx.Close()

	// Verify the database file was created
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Error("database file was not created in nested directory")
	}
}

// TestLexicalIndex_IndexChunk verifies that chunks can be indexed.
func TestLexicalIndex_IndexChunk(t *testing.T) {
	idx, err := NewLexicalIndex(":memory:", 384)
	if err != nil {
		t.Fatalf("failed to create lexical index: %v", err)
	}
	defer idx.Close()

	ctx := context.Background()
	chunk := Chunk{
		ID:        "test-chunk-1",
		FilePath:  "/test/file.go",
		Type:      ChunkFunction,
		Name:      "handleAuth",
		Content:   "func handleAuth() { validate(token) }",
		StartLine: 1,
		EndLine:   5,
		Language:  "go",
	}

	if err := idx.IndexChunk(ctx, chunk); err != nil {
		t.Fatalf("failed to index chunk: %v", err)
	}

	// Search should find the chunk
	results, err := idx.Search(ctx, "handleAuth", LexicalSearchOptions{TopK: 10})
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}

	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}

	if len(results) > 0 && results[0].Chunk.ID != chunk.ID {
		t.Errorf("expected chunk ID %s, got %s", chunk.ID, results[0].Chunk.ID)
	}
}

// TestLexicalIndex_IndexBatch verifies that multiple chunks can be indexed at once.
func TestLexicalIndex_IndexBatch(t *testing.T) {
	idx, err := NewLexicalIndex(":memory:", 384)
	if err != nil {
		t.Fatalf("failed to create lexical index: %v", err)
	}
	defer idx.Close()

	ctx := context.Background()
	chunks := []Chunk{
		{ID: "chunk-1", FilePath: "/test/a.go", Type: ChunkFunction, Name: "funcA", Content: "func funcA() {}", Language: "go"},
		{ID: "chunk-2", FilePath: "/test/b.go", Type: ChunkFunction, Name: "funcB", Content: "func funcB() {}", Language: "go"},
		{ID: "chunk-3", FilePath: "/test/c.go", Type: ChunkFunction, Name: "funcC", Content: "func funcC() {}", Language: "go"},
	}

	if err := idx.IndexBatch(ctx, chunks); err != nil {
		t.Fatalf("failed to index batch: %v", err)
	}

	// Search should find all chunks with "func"
	results, err := idx.Search(ctx, "func", LexicalSearchOptions{TopK: 10})
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}

	if len(results) != 3 {
		t.Errorf("expected 3 results, got %d", len(results))
	}
}

// TestLexicalIndex_DeleteChunk verifies that chunks can be removed from the index.
func TestLexicalIndex_DeleteChunk(t *testing.T) {
	idx, err := NewLexicalIndex(":memory:", 384)
	if err != nil {
		t.Fatalf("failed to create lexical index: %v", err)
	}
	defer idx.Close()

	ctx := context.Background()
	chunk := Chunk{
		ID:       "delete-me",
		FilePath: "/test/file.go",
		Type:     ChunkFunction,
		Name:     "deleteTarget",
		Content:  "func deleteTarget() {}",
		Language: "go",
	}

	// Index then delete
	if err := idx.IndexChunk(ctx, chunk); err != nil {
		t.Fatalf("failed to index chunk: %v", err)
	}

	if err := idx.DeleteChunk(ctx, chunk.ID); err != nil {
		t.Fatalf("failed to delete chunk: %v", err)
	}

	// Search should return no results
	results, err := idx.Search(ctx, "deleteTarget", LexicalSearchOptions{TopK: 10})
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}

	if len(results) != 0 {
		t.Errorf("expected 0 results after delete, got %d", len(results))
	}
}

// TestLexicalIndex_DeleteByFilePath verifies bulk deletion by file path.
func TestLexicalIndex_DeleteByFilePath(t *testing.T) {
	idx, err := NewLexicalIndex(":memory:", 384)
	if err != nil {
		t.Fatalf("failed to create lexical index: %v", err)
	}
	defer idx.Close()

	ctx := context.Background()
	chunks := []Chunk{
		{ID: "c1", FilePath: "/test/delete.go", Type: ChunkFunction, Name: "func1", Content: "func func1() {}", Language: "go"},
		{ID: "c2", FilePath: "/test/delete.go", Type: ChunkFunction, Name: "func2", Content: "func func2() {}", Language: "go"},
		{ID: "c3", FilePath: "/test/keep.go", Type: ChunkFunction, Name: "func3", Content: "func func3() {}", Language: "go"},
	}

	if err := idx.IndexBatch(ctx, chunks); err != nil {
		t.Fatalf("failed to index batch: %v", err)
	}

	// Delete all chunks from /test/delete.go
	deleted, err := idx.DeleteByFilePath(ctx, "/test/delete.go")
	if err != nil {
		t.Fatalf("failed to delete by file path: %v", err)
	}

	if deleted != 2 {
		t.Errorf("expected 2 chunks deleted, got %d", deleted)
	}

	// Search should only find func3
	results, err := idx.Search(ctx, "func", LexicalSearchOptions{TopK: 10})
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}

	if len(results) != 1 {
		t.Errorf("expected 1 result after delete, got %d", len(results))
	}

	if len(results) > 0 && results[0].Chunk.ID != "c3" {
		t.Errorf("expected chunk c3, got %s", results[0].Chunk.ID)
	}
}

// TestLexicalIndex_Search verifies search with various options.
func TestLexicalIndex_Search(t *testing.T) {
	idx, err := NewLexicalIndex(":memory:", 384)
	if err != nil {
		t.Fatalf("failed to create lexical index: %v", err)
	}
	defer idx.Close()

	ctx := context.Background()
	chunks := []Chunk{
		{ID: "fn1", FilePath: "/internal/auth/handler.go", Type: ChunkFunction, Name: "handleAuth", Content: "func handleAuth() { validate() }", Language: "go"},
		{ID: "st1", FilePath: "/internal/auth/types.go", Type: ChunkStruct, Name: "AuthRequest", Content: "type AuthRequest struct { Token string }", Language: "go"},
		{ID: "fn2", FilePath: "/internal/user/handler.go", Type: ChunkFunction, Name: "handleUser", Content: "func handleUser() { validate() }", Language: "go"},
	}

	if err := idx.IndexBatch(ctx, chunks); err != nil {
		t.Fatalf("failed to index batch: %v", err)
	}

	// Test type filter
	t.Run("TypeFilter", func(t *testing.T) {
		results, err := idx.Search(ctx, "type", LexicalSearchOptions{TopK: 10, Type: "struct"})
		if err != nil {
			t.Fatalf("search failed: %v", err)
		}
		if len(results) != 1 {
			t.Errorf("expected 1 struct result, got %d", len(results))
		}
		if len(results) > 0 && results[0].Chunk.Type != ChunkStruct {
			t.Errorf("expected struct type, got %s", results[0].Chunk.Type)
		}
	})

	// Test path filter
	t.Run("PathFilter", func(t *testing.T) {
		results, err := idx.Search(ctx, "validate", LexicalSearchOptions{TopK: 10, PathFilter: "/internal/auth"})
		if err != nil {
			t.Fatalf("search failed: %v", err)
		}
		if len(results) != 1 {
			t.Errorf("expected 1 result from /internal/auth, got %d", len(results))
		}
	})

	// Test TopK limit
	t.Run("TopKLimit", func(t *testing.T) {
		results, err := idx.Search(ctx, "func", LexicalSearchOptions{TopK: 1})
		if err != nil {
			t.Fatalf("search failed: %v", err)
		}
		if len(results) > 1 {
			t.Errorf("expected max 1 result with TopK=1, got %d", len(results))
		}
	})
}

// TestLexicalIndex_Clear verifies that the index can be cleared.
func TestLexicalIndex_Clear(t *testing.T) {
	idx, err := NewLexicalIndex(":memory:", 384)
	if err != nil {
		t.Fatalf("failed to create lexical index: %v", err)
	}
	defer idx.Close()

	ctx := context.Background()
	chunk := Chunk{
		ID:       "clear-test",
		FilePath: "/test/file.go",
		Type:     ChunkFunction,
		Name:     "clearMe",
		Content:  "func clearMe() {}",
		Language: "go",
	}

	if err := idx.IndexChunk(ctx, chunk); err != nil {
		t.Fatalf("failed to index chunk: %v", err)
	}

	// Clear the index
	if err := idx.Clear(ctx); err != nil {
		t.Fatalf("failed to clear index: %v", err)
	}

	// Search should return no results
	results, err := idx.Search(ctx, "clearMe", LexicalSearchOptions{TopK: 10})
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}

	if len(results) != 0 {
		t.Errorf("expected 0 results after clear, got %d", len(results))
	}
}

// TestLexicalIndex_Reopen verifies that data persists across close/reopen.
func TestLexicalIndex_Reopen(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "reopen-test.db")

	// Create and populate index
	idx, err := NewLexicalIndex(dbPath, 384)
	if err != nil {
		t.Fatalf("failed to create lexical index: %v", err)
	}

	ctx := context.Background()
	chunk := Chunk{
		ID:       "persist-test",
		FilePath: "/test/file.go",
		Type:     ChunkFunction,
		Name:     "persistedFunc",
		Content:  "func persistedFunc() {}",
		Language: "go",
	}

	if err := idx.IndexChunk(ctx, chunk); err != nil {
		t.Fatalf("failed to index chunk: %v", err)
	}
	idx.Close()

	// Reopen and verify data persists
	idx2, err := NewLexicalIndex(dbPath, 384)
	if err != nil {
		t.Fatalf("failed to reopen lexical index: %v", err)
	}
	defer idx2.Close()

	results, err := idx2.Search(ctx, "persistedFunc", LexicalSearchOptions{TopK: 10})
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}

	if len(results) != 1 {
		t.Errorf("expected 1 result after reopen, got %d", len(results))
	}
}

// TestLexicalIndex_Closed verifies that operations on closed index return ErrStorageClosed.
func TestLexicalIndex_Closed(t *testing.T) {
	idx, err := NewLexicalIndex(":memory:", 384)
	if err != nil {
		t.Fatalf("failed to create lexical index: %v", err)
	}

	// Close the index
	idx.Close()

	ctx := context.Background()

	// All operations should return ErrStorageClosed
	if _, err := idx.Search(ctx, "test", LexicalSearchOptions{}); err != ErrStorageClosed {
		t.Errorf("expected ErrStorageClosed on Search, got %v", err)
	}

	if err := idx.IndexChunk(ctx, Chunk{ID: "test"}); err != ErrStorageClosed {
		t.Errorf("expected ErrStorageClosed on IndexChunk, got %v", err)
	}

	if err := idx.IndexBatch(ctx, []Chunk{{ID: "test"}}); err != ErrStorageClosed {
		t.Errorf("expected ErrStorageClosed on IndexBatch, got %v", err)
	}

	if err := idx.DeleteChunk(ctx, "test"); err != ErrStorageClosed {
		t.Errorf("expected ErrStorageClosed on DeleteChunk, got %v", err)
	}

	if _, err := idx.DeleteByFilePath(ctx, "/test"); err != ErrStorageClosed {
		t.Errorf("expected ErrStorageClosed on DeleteByFilePath, got %v", err)
	}

	if err := idx.Clear(ctx); err != ErrStorageClosed {
		t.Errorf("expected ErrStorageClosed on Clear, got %v", err)
	}
}

// ===== Helper function tests =====

// TestSanitizeCollectionName verifies collection name sanitization for filenames.
func TestSanitizeCollectionName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"simple", "simple"},
		{"with-hyphen", "with-hyphen"},
		{"with_underscore", "with_underscore"},
		{"mixed-123_test", "mixed-123_test"},
		{"path/traversal", "pathtraversal"},
		{"../../etc/passwd", "etcpasswd"},
		{"special!@#chars", "specialchars"},
		{"spaces and tabs", "spacesandtabs"},
		{"", "default"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := sanitizeCollectionName(tt.input)
			if result != tt.expected {
				t.Errorf("sanitizeCollectionName(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// TestGetFTSPath verifies FTS database path generation.
func TestGetFTSPath(t *testing.T) {
	// Test with custom data dir
	t.Run("CustomDataDir", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := getFTSPath("test_collection", tmpDir)
		expected := filepath.Join(tmpDir, "qdrant-fts-test_collection.db")
		if path != expected {
			t.Errorf("getFTSPath = %q, want %q", path, expected)
		}
	})

	// Test with default data dir (home dir)
	t.Run("DefaultDataDir", func(t *testing.T) {
		home, err := os.UserHomeDir()
		if err != nil {
			t.Skip("cannot get home dir")
		}
		path := getFTSPath("my_collection", "")
		expected := filepath.Join(home, ".llm-semantic", "qdrant-fts-my_collection.db")
		if path != expected {
			t.Errorf("getFTSPath = %q, want %q", path, expected)
		}
	})
}
