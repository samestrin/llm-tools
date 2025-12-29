package semantic

import (
	"context"
	"testing"
)

func TestSearcher_Search(t *testing.T) {
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

	// Create searcher
	searcher := NewSearcher(storage, embedder)

	// Add some test data
	ctx := context.Background()
	chunks := []Chunk{
		{ID: "1", FilePath: "main.go", Name: "Add", Type: ChunkFunction, Content: "func Add(a, b int) int { return a + b }"},
		{ID: "2", FilePath: "main.go", Name: "Sub", Type: ChunkFunction, Content: "func Sub(a, b int) int { return a - b }"},
		{ID: "3", FilePath: "util.go", Name: "Helper", Type: ChunkFunction, Content: "func Helper() {}"},
	}

	for _, chunk := range chunks {
		if err := storage.Create(ctx, chunk, []float32{0.1, 0.2, 0.3, 0.4}); err != nil {
			t.Fatalf("Failed to create chunk: %v", err)
		}
	}

	// Perform search
	results, err := searcher.Search(ctx, "add two numbers", SearchOptions{TopK: 5})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}

	if len(results) == 0 {
		t.Error("Search() returned no results")
	}

	// Results should be sorted by score
	for i := 1; i < len(results); i++ {
		if results[i].Score > results[i-1].Score {
			t.Errorf("Results not sorted by score: %v > %v", results[i].Score, results[i-1].Score)
		}
	}
}

func TestSearcher_SearchWithThreshold(t *testing.T) {
	storage, err := NewSQLiteStorage(":memory:", 4)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer storage.Close()

	embedder := &mockEmbedder{
		embedding: []float32{1, 0, 0, 0},
	}

	searcher := NewSearcher(storage, embedder)
	ctx := context.Background()

	// Add chunks with different embeddings
	storage.Create(ctx, Chunk{ID: "1", Name: "Exact"}, []float32{1, 0, 0, 0})       // Score 1.0
	storage.Create(ctx, Chunk{ID: "2", Name: "Similar"}, []float32{0.9, 0.1, 0, 0}) // Score ~0.99
	storage.Create(ctx, Chunk{ID: "3", Name: "Different"}, []float32{0, 1, 0, 0})   // Score 0.0

	// Search with high threshold
	results, err := searcher.Search(ctx, "test", SearchOptions{
		TopK:      10,
		Threshold: 0.5,
	})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}

	// Should only return high-scoring results
	if len(results) != 2 {
		t.Errorf("Search() returned %d results, want 2 (above threshold)", len(results))
	}
}

func TestSearcher_SearchWithTypeFilter(t *testing.T) {
	storage, err := NewSQLiteStorage(":memory:", 4)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer storage.Close()

	embedder := &mockEmbedder{
		embedding: []float32{0.1, 0.2, 0.3, 0.4},
	}

	searcher := NewSearcher(storage, embedder)
	ctx := context.Background()

	// Add different chunk types
	storage.Create(ctx, Chunk{ID: "1", Name: "Func1", Type: ChunkFunction}, []float32{0.1, 0.2, 0.3, 0.4})
	storage.Create(ctx, Chunk{ID: "2", Name: "Method1", Type: ChunkMethod}, []float32{0.1, 0.2, 0.3, 0.4})
	storage.Create(ctx, Chunk{ID: "3", Name: "Struct1", Type: ChunkStruct}, []float32{0.1, 0.2, 0.3, 0.4})

	// Search only functions
	results, err := searcher.Search(ctx, "test", SearchOptions{
		TopK: 10,
		Type: "function",
	})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}

	if len(results) != 1 {
		t.Errorf("Search() with type filter returned %d results, want 1", len(results))
	}
}

func TestSearcher_SearchWithPathFilter(t *testing.T) {
	storage, err := NewSQLiteStorage(":memory:", 4)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer storage.Close()

	embedder := &mockEmbedder{
		embedding: []float32{0.1, 0.2, 0.3, 0.4},
	}

	searcher := NewSearcher(storage, embedder)
	ctx := context.Background()

	// Add chunks from different paths
	storage.Create(ctx, Chunk{ID: "1", FilePath: "internal/foo/a.go", Name: "Foo"}, []float32{0.1, 0.2, 0.3, 0.4})
	storage.Create(ctx, Chunk{ID: "2", FilePath: "internal/bar/b.go", Name: "Bar"}, []float32{0.1, 0.2, 0.3, 0.4})
	storage.Create(ctx, Chunk{ID: "3", FilePath: "cmd/main.go", Name: "Main"}, []float32{0.1, 0.2, 0.3, 0.4})

	// Search only in internal/foo
	results, err := searcher.Search(ctx, "test", SearchOptions{
		TopK:       10,
		PathFilter: "internal/foo",
	})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}

	if len(results) != 1 {
		t.Errorf("Search() with path filter returned %d results, want 1", len(results))
	}
}

func TestSearcher_EmptyQuery(t *testing.T) {
	storage, err := NewSQLiteStorage(":memory:", 4)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer storage.Close()

	embedder := &mockEmbedder{
		embedding: []float32{0.1, 0.2, 0.3, 0.4},
	}

	searcher := NewSearcher(storage, embedder)

	_, err = searcher.Search(context.Background(), "", SearchOptions{TopK: 5})
	if err == nil {
		t.Error("Search() with empty query should return error")
	}
}

func TestSearcher_EmbedderError(t *testing.T) {
	storage, err := NewSQLiteStorage(":memory:", 4)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer storage.Close()

	embedder := &mockEmbedder{
		err: context.Canceled,
	}

	searcher := NewSearcher(storage, embedder)

	_, err = searcher.Search(context.Background(), "test query", SearchOptions{TopK: 5})
	if err == nil {
		t.Error("Search() should propagate embedder error")
	}
}

// mockEmbedder is a test double for Embedder
type mockEmbedder struct {
	embedding []float32
	err       error
}

func (m *mockEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.embedding, nil
}

func (m *mockEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	if m.err != nil {
		return nil, m.err
	}
	result := make([][]float32, len(texts))
	for i := range texts {
		result[i] = m.embedding
	}
	return result, nil
}

func (m *mockEmbedder) Dimensions() int {
	return len(m.embedding)
}
