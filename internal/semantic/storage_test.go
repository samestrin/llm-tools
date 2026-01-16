package semantic

import (
	"context"
	"testing"
)

// StorageTestSuite runs a standard set of tests against any Storage implementation
func StorageTestSuite(t *testing.T, newStorage func() (Storage, func())) {
	t.Run("Create", func(t *testing.T) {
		storage, cleanup := newStorage()
		defer cleanup()

		chunk := Chunk{
			ID:        "test-123",
			FilePath:  "/path/to/file.go",
			Type:      ChunkFunction,
			Name:      "TestFunc",
			Content:   "func TestFunc() {}",
			StartLine: 1,
			EndLine:   1,
			Language:  "go",
		}
		embedding := []float32{0.1, 0.2, 0.3, 0.4}

		err := storage.Create(context.Background(), chunk, embedding)
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}
	})

	t.Run("Read", func(t *testing.T) {
		storage, cleanup := newStorage()
		defer cleanup()

		chunk := Chunk{
			ID:        "test-read",
			FilePath:  "/path/to/file.go",
			Type:      ChunkFunction,
			Name:      "ReadFunc",
			Content:   "func ReadFunc() {}",
			StartLine: 1,
			EndLine:   1,
			Language:  "go",
		}
		embedding := []float32{0.1, 0.2, 0.3, 0.4}

		_ = storage.Create(context.Background(), chunk, embedding)

		result, err := storage.Read(context.Background(), "test-read")
		if err != nil {
			t.Fatalf("Read() error = %v", err)
		}
		if result.ID != chunk.ID {
			t.Errorf("Read() ID = %v, want %v", result.ID, chunk.ID)
		}
		if result.Name != chunk.Name {
			t.Errorf("Read() Name = %v, want %v", result.Name, chunk.Name)
		}
	})

	t.Run("ReadNotFound", func(t *testing.T) {
		storage, cleanup := newStorage()
		defer cleanup()

		_, err := storage.Read(context.Background(), "nonexistent")
		if err == nil {
			t.Error("Read() should return error for nonexistent chunk")
		}
	})

	t.Run("Update", func(t *testing.T) {
		storage, cleanup := newStorage()
		defer cleanup()

		chunk := Chunk{
			ID:        "test-update",
			FilePath:  "/path/to/file.go",
			Type:      ChunkFunction,
			Name:      "UpdateFunc",
			Content:   "func UpdateFunc() {}",
			StartLine: 1,
			EndLine:   1,
			Language:  "go",
		}
		embedding := []float32{0.1, 0.2, 0.3, 0.4}

		_ = storage.Create(context.Background(), chunk, embedding)

		// Update the chunk
		chunk.Content = "func UpdateFunc() { updated }"
		newEmbedding := []float32{0.5, 0.6, 0.7, 0.8}

		err := storage.Update(context.Background(), chunk, newEmbedding)
		if err != nil {
			t.Fatalf("Update() error = %v", err)
		}

		// Verify update
		result, _ := storage.Read(context.Background(), "test-update")
		if result.Content != chunk.Content {
			t.Errorf("Update() Content = %v, want %v", result.Content, chunk.Content)
		}
	})

	t.Run("Delete", func(t *testing.T) {
		storage, cleanup := newStorage()
		defer cleanup()

		chunk := Chunk{
			ID:        "test-delete",
			FilePath:  "/path/to/file.go",
			Type:      ChunkFunction,
			Name:      "DeleteFunc",
			Content:   "func DeleteFunc() {}",
			StartLine: 1,
			EndLine:   1,
			Language:  "go",
		}
		embedding := []float32{0.1, 0.2, 0.3, 0.4}

		_ = storage.Create(context.Background(), chunk, embedding)

		err := storage.Delete(context.Background(), "test-delete")
		if err != nil {
			t.Fatalf("Delete() error = %v", err)
		}

		// Verify deletion
		_, err = storage.Read(context.Background(), "test-delete")
		if err == nil {
			t.Error("Delete() chunk should not be found after deletion")
		}
	})

	t.Run("List", func(t *testing.T) {
		storage, cleanup := newStorage()
		defer cleanup()

		// Create multiple chunks
		for i := 0; i < 5; i++ {
			chunk := Chunk{
				ID:        "test-list-" + string(rune('a'+i)),
				FilePath:  "/path/to/file.go",
				Type:      ChunkFunction,
				Name:      "ListFunc" + string(rune('A'+i)),
				Content:   "func ListFunc() {}",
				StartLine: i,
				EndLine:   i,
				Language:  "go",
			}
			_ = storage.Create(context.Background(), chunk, []float32{0.1, 0.2, 0.3, 0.4})
		}

		chunks, err := storage.List(context.Background(), ListOptions{})
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}
		if len(chunks) != 5 {
			t.Errorf("List() returned %d chunks, want 5", len(chunks))
		}
	})

	t.Run("ListWithLimit", func(t *testing.T) {
		storage, cleanup := newStorage()
		defer cleanup()

		for i := 0; i < 10; i++ {
			chunk := Chunk{
				ID:        "test-limit-" + string(rune('a'+i)),
				FilePath:  "/path/to/file.go",
				Type:      ChunkFunction,
				Name:      "LimitFunc" + string(rune('A'+i)),
				Content:   "func LimitFunc() {}",
				StartLine: i,
				EndLine:   i,
				Language:  "go",
			}
			_ = storage.Create(context.Background(), chunk, []float32{0.1, 0.2, 0.3, 0.4})
		}

		chunks, err := storage.List(context.Background(), ListOptions{Limit: 5})
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}
		if len(chunks) != 5 {
			t.Errorf("List() with limit returned %d chunks, want 5", len(chunks))
		}
	})

	t.Run("ListByFilePath", func(t *testing.T) {
		storage, cleanup := newStorage()
		defer cleanup()

		chunks := []Chunk{
			{ID: "fp-1", FilePath: "/path/a.go", Type: ChunkFunction, Name: "A", Language: "go"},
			{ID: "fp-2", FilePath: "/path/a.go", Type: ChunkFunction, Name: "B", Language: "go"},
			{ID: "fp-3", FilePath: "/path/b.go", Type: ChunkFunction, Name: "C", Language: "go"},
		}

		for _, c := range chunks {
			_ = storage.Create(context.Background(), c, []float32{0.1, 0.2, 0.3, 0.4})
		}

		results, err := storage.List(context.Background(), ListOptions{FilePath: "/path/a.go"})
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}
		if len(results) != 2 {
			t.Errorf("List() by file path returned %d chunks, want 2", len(results))
		}
	})

	t.Run("Search", func(t *testing.T) {
		storage, cleanup := newStorage()
		defer cleanup()

		// Create chunks with different embeddings
		chunks := []struct {
			chunk     Chunk
			embedding []float32
		}{
			{Chunk{ID: "s1", FilePath: "/auth.go", Name: "Auth", Type: ChunkFunction, Language: "go"}, []float32{0.9, 0.1, 0.0, 0.0}},
			{Chunk{ID: "s2", FilePath: "/db.go", Name: "Database", Type: ChunkFunction, Language: "go"}, []float32{0.1, 0.9, 0.0, 0.0}},
			{Chunk{ID: "s3", FilePath: "/auth2.go", Name: "AuthMiddleware", Type: ChunkFunction, Language: "go"}, []float32{0.8, 0.2, 0.0, 0.0}},
		}

		for _, c := range chunks {
			_ = storage.Create(context.Background(), c.chunk, c.embedding)
		}

		// Search with embedding similar to auth functions
		queryEmbedding := []float32{0.85, 0.15, 0.0, 0.0}
		results, err := storage.Search(context.Background(), queryEmbedding, SearchOptions{TopK: 2})
		if err != nil {
			t.Fatalf("Search() error = %v", err)
		}

		if len(results) != 2 {
			t.Errorf("Search() returned %d results, want 2", len(results))
		}

		// First result should be most similar (Auth or AuthMiddleware)
		if results[0].Chunk.Name != "Auth" && results[0].Chunk.Name != "AuthMiddleware" {
			t.Errorf("Search() first result = %v, want Auth or AuthMiddleware", results[0].Chunk.Name)
		}
	})

	t.Run("SearchWithThreshold", func(t *testing.T) {
		storage, cleanup := newStorage()
		defer cleanup()

		chunks := []struct {
			chunk     Chunk
			embedding []float32
		}{
			{Chunk{ID: "t1", FilePath: "/a.go", Name: "High", Type: ChunkFunction, Language: "go"}, []float32{0.9, 0.1, 0.0, 0.0}},
			{Chunk{ID: "t2", FilePath: "/b.go", Name: "Low", Type: ChunkFunction, Language: "go"}, []float32{0.1, 0.9, 0.0, 0.0}},
		}

		for _, c := range chunks {
			_ = storage.Create(context.Background(), c.chunk, c.embedding)
		}

		queryEmbedding := []float32{0.95, 0.05, 0.0, 0.0}
		results, err := storage.Search(context.Background(), queryEmbedding, SearchOptions{TopK: 10, Threshold: 0.8})
		if err != nil {
			t.Fatalf("Search() error = %v", err)
		}

		// Only high-similarity results should be returned
		for _, r := range results {
			if r.Score < 0.8 {
				t.Errorf("Search() returned result with score %v, below threshold 0.8", r.Score)
			}
		}
	})

	t.Run("DeleteByFilePath", func(t *testing.T) {
		storage, cleanup := newStorage()
		defer cleanup()

		chunks := []Chunk{
			{ID: "del-1", FilePath: "/path/delete.go", Type: ChunkFunction, Name: "A", Language: "go"},
			{ID: "del-2", FilePath: "/path/delete.go", Type: ChunkFunction, Name: "B", Language: "go"},
			{ID: "del-3", FilePath: "/path/keep.go", Type: ChunkFunction, Name: "C", Language: "go"},
		}

		for _, c := range chunks {
			_ = storage.Create(context.Background(), c, []float32{0.1, 0.2, 0.3, 0.4})
		}

		count, err := storage.DeleteByFilePath(context.Background(), "/path/delete.go")
		if err != nil {
			t.Fatalf("DeleteByFilePath() error = %v", err)
		}
		if count != 2 {
			t.Errorf("DeleteByFilePath() deleted %d, want 2", count)
		}

		// Verify keep.go still exists
		results, _ := storage.List(context.Background(), ListOptions{FilePath: "/path/keep.go"})
		if len(results) != 1 {
			t.Errorf("DeleteByFilePath() should not have deleted /path/keep.go")
		}
	})

	t.Run("Stats", func(t *testing.T) {
		storage, cleanup := newStorage()
		defer cleanup()

		for i := 0; i < 3; i++ {
			chunk := Chunk{
				ID:       "stats-" + string(rune('a'+i)),
				FilePath: "/file" + string(rune('1'+i)) + ".go",
				Type:     ChunkFunction,
				Name:     "Func" + string(rune('A'+i)),
				Language: "go",
			}
			_ = storage.Create(context.Background(), chunk, []float32{0.1, 0.2, 0.3, 0.4})
		}

		stats, err := storage.Stats(context.Background())
		if err != nil {
			t.Fatalf("Stats() error = %v", err)
		}

		if stats.ChunksTotal != 3 {
			t.Errorf("Stats() ChunksTotal = %d, want 3", stats.ChunksTotal)
		}
		if stats.FilesIndexed != 3 {
			t.Errorf("Stats() FilesIndexed = %d, want 3", stats.FilesIndexed)
		}
	})

	t.Run("FileHash_SetAndGet", func(t *testing.T) {
		storage, cleanup := newStorage()
		defer cleanup()

		filePath := "/path/to/file.go"
		hash := "abc123def456"

		// Initially should return empty string
		got, err := storage.GetFileHash(context.Background(), filePath)
		if err != nil {
			t.Fatalf("GetFileHash() error = %v", err)
		}
		if got != "" {
			t.Errorf("GetFileHash() = %q, want empty string for unindexed file", got)
		}

		// Set the hash
		err = storage.SetFileHash(context.Background(), filePath, hash)
		if err != nil {
			t.Fatalf("SetFileHash() error = %v", err)
		}

		// Get should return the hash
		got, err = storage.GetFileHash(context.Background(), filePath)
		if err != nil {
			t.Fatalf("GetFileHash() error = %v", err)
		}
		if got != hash {
			t.Errorf("GetFileHash() = %q, want %q", got, hash)
		}
	})

	t.Run("FileHash_Update", func(t *testing.T) {
		storage, cleanup := newStorage()
		defer cleanup()

		filePath := "/path/to/file.go"
		hash1 := "hash1"
		hash2 := "hash2"

		// Set initial hash
		_ = storage.SetFileHash(context.Background(), filePath, hash1)

		// Update hash
		err := storage.SetFileHash(context.Background(), filePath, hash2)
		if err != nil {
			t.Fatalf("SetFileHash() update error = %v", err)
		}

		// Should return updated hash
		got, err := storage.GetFileHash(context.Background(), filePath)
		if err != nil {
			t.Fatalf("GetFileHash() error = %v", err)
		}
		if got != hash2 {
			t.Errorf("GetFileHash() = %q, want %q after update", got, hash2)
		}
	})

	t.Run("FileHash_ClearedOnClear", func(t *testing.T) {
		storage, cleanup := newStorage()
		defer cleanup()

		filePath := "/path/to/file.go"
		hash := "somehash"

		// Set a hash
		_ = storage.SetFileHash(context.Background(), filePath, hash)

		// Clear storage
		err := storage.Clear(context.Background())
		if err != nil {
			t.Fatalf("Clear() error = %v", err)
		}

		// Hash should be gone
		got, err := storage.GetFileHash(context.Background(), filePath)
		if err != nil {
			t.Fatalf("GetFileHash() error = %v", err)
		}
		if got != "" {
			t.Errorf("GetFileHash() = %q, want empty string after Clear()", got)
		}
	})
}

// MemoryStorageTestSuite runs a standard set of tests for memory storage methods
func MemoryStorageTestSuite(t *testing.T, newStorage func() (Storage, func())) {
	t.Run("StoreMemory", func(t *testing.T) {
		storage, cleanup := newStorage()
		defer cleanup()

		entry := NewMemoryEntry("How should auth tokens be handled?", "Use JWT with 24h expiry")
		embedding := []float32{0.1, 0.2, 0.3, 0.4}

		err := storage.StoreMemory(context.Background(), *entry, embedding)
		if err != nil {
			t.Fatalf("StoreMemory() error = %v", err)
		}
	})

	t.Run("GetMemory", func(t *testing.T) {
		storage, cleanup := newStorage()
		defer cleanup()

		entry := NewMemoryEntry("Test question?", "Test answer")
		embedding := []float32{0.1, 0.2, 0.3, 0.4}

		_ = storage.StoreMemory(context.Background(), *entry, embedding)

		result, err := storage.GetMemory(context.Background(), entry.ID)
		if err != nil {
			t.Fatalf("GetMemory() error = %v", err)
		}
		if result.ID != entry.ID {
			t.Errorf("GetMemory() ID = %v, want %v", result.ID, entry.ID)
		}
		if result.Question != entry.Question {
			t.Errorf("GetMemory() Question = %v, want %v", result.Question, entry.Question)
		}
		if result.Answer != entry.Answer {
			t.Errorf("GetMemory() Answer = %v, want %v", result.Answer, entry.Answer)
		}
	})

	t.Run("GetMemoryNotFound", func(t *testing.T) {
		storage, cleanup := newStorage()
		defer cleanup()

		_, err := storage.GetMemory(context.Background(), "nonexistent-id")
		if err != ErrMemoryNotFound {
			t.Errorf("GetMemory() error = %v, want ErrMemoryNotFound", err)
		}
	})

	t.Run("StoreMemory_Upsert", func(t *testing.T) {
		storage, cleanup := newStorage()
		defer cleanup()

		entry := NewMemoryEntry("Question for upsert?", "Original answer")
		embedding := []float32{0.1, 0.2, 0.3, 0.4}

		// Store initially
		_ = storage.StoreMemory(context.Background(), *entry, embedding)

		// Store again with same ID but updated content
		entry.Answer = "Updated answer"
		entry.Occurrences = 5
		err := storage.StoreMemory(context.Background(), *entry, embedding)
		if err != nil {
			t.Fatalf("StoreMemory() upsert error = %v", err)
		}

		// Verify update
		result, _ := storage.GetMemory(context.Background(), entry.ID)
		if result.Answer != "Updated answer" {
			t.Errorf("StoreMemory() upsert Answer = %q, want %q", result.Answer, "Updated answer")
		}
		if result.Occurrences != 5 {
			t.Errorf("StoreMemory() upsert Occurrences = %d, want 5", result.Occurrences)
		}
	})

	t.Run("DeleteMemory", func(t *testing.T) {
		storage, cleanup := newStorage()
		defer cleanup()

		entry := NewMemoryEntry("Question to delete?", "Answer to delete")
		embedding := []float32{0.1, 0.2, 0.3, 0.4}

		_ = storage.StoreMemory(context.Background(), *entry, embedding)

		err := storage.DeleteMemory(context.Background(), entry.ID)
		if err != nil {
			t.Fatalf("DeleteMemory() error = %v", err)
		}

		// Verify deletion
		_, err = storage.GetMemory(context.Background(), entry.ID)
		if err != ErrMemoryNotFound {
			t.Error("DeleteMemory() entry should not be found after deletion")
		}
	})

	t.Run("ListMemory", func(t *testing.T) {
		storage, cleanup := newStorage()
		defer cleanup()

		// Create multiple entries
		entries := []*MemoryEntry{
			NewMemoryEntry("Question 1?", "Answer 1"),
			NewMemoryEntry("Question 2?", "Answer 2"),
			NewMemoryEntry("Question 3?", "Answer 3"),
		}

		for _, e := range entries {
			_ = storage.StoreMemory(context.Background(), *e, []float32{0.1, 0.2, 0.3, 0.4})
		}

		results, err := storage.ListMemory(context.Background(), MemoryListOptions{})
		if err != nil {
			t.Fatalf("ListMemory() error = %v", err)
		}
		if len(results) != 3 {
			t.Errorf("ListMemory() returned %d entries, want 3", len(results))
		}
	})

	t.Run("ListMemory_WithLimit", func(t *testing.T) {
		storage, cleanup := newStorage()
		defer cleanup()

		for i := 0; i < 10; i++ {
			entry := NewMemoryEntry("Question "+string(rune('A'+i))+"?", "Answer")
			_ = storage.StoreMemory(context.Background(), *entry, []float32{0.1, 0.2, 0.3, 0.4})
		}

		results, err := storage.ListMemory(context.Background(), MemoryListOptions{Limit: 5})
		if err != nil {
			t.Fatalf("ListMemory() error = %v", err)
		}
		if len(results) != 5 {
			t.Errorf("ListMemory() with limit returned %d entries, want 5", len(results))
		}
	})

	t.Run("ListMemory_ByStatus", func(t *testing.T) {
		storage, cleanup := newStorage()
		defer cleanup()

		entry1 := NewMemoryEntry("Pending question?", "Answer")
		entry1.Status = MemoryStatusPending

		entry2 := NewMemoryEntry("Promoted question?", "Answer")
		entry2.Status = MemoryStatusPromoted

		_ = storage.StoreMemory(context.Background(), *entry1, []float32{0.1, 0.2, 0.3, 0.4})
		_ = storage.StoreMemory(context.Background(), *entry2, []float32{0.1, 0.2, 0.3, 0.4})

		results, err := storage.ListMemory(context.Background(), MemoryListOptions{
			Status: MemoryStatusPromoted,
		})
		if err != nil {
			t.Fatalf("ListMemory() error = %v", err)
		}
		if len(results) != 1 {
			t.Errorf("ListMemory() by status returned %d entries, want 1", len(results))
			return // Avoid panic on empty results
		}
		if results[0].Status != MemoryStatusPromoted {
			t.Errorf("ListMemory() status = %q, want %q", results[0].Status, MemoryStatusPromoted)
		}
	})

	t.Run("SearchMemory", func(t *testing.T) {
		storage, cleanup := newStorage()
		defer cleanup()

		// Create entries with different embeddings
		entries := []struct {
			entry     *MemoryEntry
			embedding []float32
		}{
			{NewMemoryEntry("How to authenticate?", "Use JWT"), []float32{0.9, 0.1, 0.0, 0.0}},
			{NewMemoryEntry("How to connect database?", "Use postgres"), []float32{0.1, 0.9, 0.0, 0.0}},
			{NewMemoryEntry("How to handle auth errors?", "Return 401"), []float32{0.8, 0.2, 0.0, 0.0}},
		}

		for _, e := range entries {
			_ = storage.StoreMemory(context.Background(), *e.entry, e.embedding)
		}

		// Search with embedding similar to auth questions
		queryEmbedding := []float32{0.85, 0.15, 0.0, 0.0}
		results, err := storage.SearchMemory(context.Background(), queryEmbedding, MemorySearchOptions{TopK: 2})
		if err != nil {
			t.Fatalf("SearchMemory() error = %v", err)
		}

		if len(results) != 2 {
			t.Errorf("SearchMemory() returned %d results, want 2", len(results))
		}

		// Results should be sorted by score (highest first)
		if len(results) >= 2 && results[0].Score < results[1].Score {
			t.Errorf("SearchMemory() results not sorted by score: %v < %v", results[0].Score, results[1].Score)
		}
	})

	t.Run("SearchMemory_WithThreshold", func(t *testing.T) {
		storage, cleanup := newStorage()
		defer cleanup()

		entries := []struct {
			entry     *MemoryEntry
			embedding []float32
		}{
			{NewMemoryEntry("High match?", "Answer"), []float32{0.9, 0.1, 0.0, 0.0}},
			{NewMemoryEntry("Low match?", "Answer"), []float32{0.1, 0.9, 0.0, 0.0}},
		}

		for _, e := range entries {
			_ = storage.StoreMemory(context.Background(), *e.entry, e.embedding)
		}

		queryEmbedding := []float32{0.95, 0.05, 0.0, 0.0}
		results, err := storage.SearchMemory(context.Background(), queryEmbedding, MemorySearchOptions{
			TopK:      10,
			Threshold: 0.8,
		})
		if err != nil {
			t.Fatalf("SearchMemory() error = %v", err)
		}

		// Only high-similarity results should be returned
		for _, r := range results {
			if r.Score < 0.8 {
				t.Errorf("SearchMemory() returned result with score %v, below threshold 0.8", r.Score)
			}
		}
	})

	t.Run("StoreMemoryBatch", func(t *testing.T) {
		storage, cleanup := newStorage()
		defer cleanup()

		batch := []MemoryWithEmbedding{
			{Entry: *NewMemoryEntry("Batch Q1?", "A1"), Embedding: []float32{0.1, 0.2, 0.3, 0.4}},
			{Entry: *NewMemoryEntry("Batch Q2?", "A2"), Embedding: []float32{0.3, 0.4, 0.5, 0.6}},
			{Entry: *NewMemoryEntry("Batch Q3?", "A3"), Embedding: []float32{0.5, 0.6, 0.7, 0.8}},
		}

		err := storage.StoreMemoryBatch(context.Background(), batch)
		if err != nil {
			t.Fatalf("StoreMemoryBatch() error = %v", err)
		}

		// Verify all entries were stored
		results, err := storage.ListMemory(context.Background(), MemoryListOptions{})
		if err != nil {
			t.Fatalf("ListMemory() error = %v", err)
		}
		if len(results) != 3 {
			t.Errorf("StoreMemoryBatch() stored %d entries, want 3", len(results))
		}
	})
}
