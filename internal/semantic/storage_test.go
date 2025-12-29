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
			_ = storage.Create(context.Background(), c, []float32{0.1, 0.2})
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
			{Chunk{ID: "t1", FilePath: "/a.go", Name: "High", Type: ChunkFunction, Language: "go"}, []float32{0.9, 0.1}},
			{Chunk{ID: "t2", FilePath: "/b.go", Name: "Low", Type: ChunkFunction, Language: "go"}, []float32{0.1, 0.9}},
		}

		for _, c := range chunks {
			_ = storage.Create(context.Background(), c.chunk, c.embedding)
		}

		queryEmbedding := []float32{0.95, 0.05}
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
			_ = storage.Create(context.Background(), c, []float32{0.1, 0.2})
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
			_ = storage.Create(context.Background(), chunk, []float32{0.1, 0.2})
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
}
