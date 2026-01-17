package semantic

import (
	"context"
	"fmt"
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

func (m *mockEmbedder) Model() string {
	return "mock-embedder"
}

// ===== Backward Compatibility Regression Tests =====
// These tests ensure that existing behavior is unchanged after adding hybrid search.

// TestSearch_DefaultBehavior_Unchanged verifies that Search() without hybrid options
// performs dense-only vector search (original behavior).
func TestSearch_DefaultBehavior_Unchanged(t *testing.T) {
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

	// Add test chunks
	storage.Create(ctx, Chunk{ID: "1", Name: "Func1", Content: "function one"}, []float32{0.1, 0.2, 0.3, 0.4})
	storage.Create(ctx, Chunk{ID: "2", Name: "Func2", Content: "function two"}, []float32{0.2, 0.3, 0.4, 0.5})

	// Search with basic SearchOptions (not HybridSearchOptions)
	results, err := searcher.Search(ctx, "test query", SearchOptions{
		TopK: 10,
	})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}

	// Verify we get results (dense search works)
	if len(results) != 2 {
		t.Errorf("Search() returned %d results, want 2", len(results))
	}

	// Verify results have valid scores (cosine similarity)
	for _, r := range results {
		if r.Score < 0 || r.Score > 1 {
			t.Errorf("Result score %f out of expected range [0,1]", r.Score)
		}
	}
}

// TestSearch_ExistingOptions_Work verifies that all SearchOptions fields work as before.
func TestSearch_ExistingOptions_Work(t *testing.T) {
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

	// Add test chunks with different types and paths
	storage.Create(ctx, Chunk{ID: "1", FilePath: "internal/a.go", Name: "FuncA", Type: ChunkFunction}, []float32{1, 0, 0, 0})
	storage.Create(ctx, Chunk{ID: "2", FilePath: "internal/b.go", Name: "MethodB", Type: ChunkMethod}, []float32{0.9, 0.1, 0, 0})
	storage.Create(ctx, Chunk{ID: "3", FilePath: "cmd/main.go", Name: "Main", Type: ChunkFunction}, []float32{0.5, 0.5, 0, 0})

	// Test TopK
	t.Run("TopK", func(t *testing.T) {
		results, err := searcher.Search(ctx, "test", SearchOptions{TopK: 2})
		if err != nil {
			t.Fatalf("Search() error = %v", err)
		}
		if len(results) != 2 {
			t.Errorf("TopK=2 returned %d results, want 2", len(results))
		}
	})

	// Test Threshold
	t.Run("Threshold", func(t *testing.T) {
		results, err := searcher.Search(ctx, "test", SearchOptions{TopK: 10, Threshold: 0.9})
		if err != nil {
			t.Fatalf("Search() error = %v", err)
		}
		// Only exact match should pass 0.9 threshold
		if len(results) != 2 {
			t.Errorf("Threshold=0.9 returned %d results, want 2", len(results))
		}
	})

	// Test Type filter
	t.Run("Type", func(t *testing.T) {
		results, err := searcher.Search(ctx, "test", SearchOptions{TopK: 10, Type: "function"})
		if err != nil {
			t.Fatalf("Search() error = %v", err)
		}
		if len(results) != 2 {
			t.Errorf("Type=function returned %d results, want 2", len(results))
		}
		for _, r := range results {
			if r.Chunk.Type != ChunkFunction {
				t.Errorf("Got chunk type %v, want function", r.Chunk.Type)
			}
		}
	})

	// Test PathFilter
	t.Run("PathFilter", func(t *testing.T) {
		results, err := searcher.Search(ctx, "test", SearchOptions{TopK: 10, PathFilter: "internal/"})
		if err != nil {
			t.Fatalf("Search() error = %v", err)
		}
		if len(results) != 2 {
			t.Errorf("PathFilter=internal/ returned %d results, want 2", len(results))
		}
	})
}

// TestSearch_MethodSignature_Unchanged verifies that Search() signature is backward compatible.
// This is a compile-time test - if it compiles, the signature is compatible.
func TestSearch_MethodSignature_Unchanged(t *testing.T) {
	storage, err := NewSQLiteStorage(":memory:", 4)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer storage.Close()

	embedder := &mockEmbedder{embedding: []float32{0.1, 0.2, 0.3, 0.4}}
	searcher := NewSearcher(storage, embedder)
	ctx := context.Background()

	// This call verifies the original signature:
	// Search(ctx context.Context, query string, opts SearchOptions) ([]SearchResult, error)
	var results []SearchResult
	var searchErr error
	results, searchErr = searcher.Search(ctx, "query", SearchOptions{})

	// Use variables to avoid unused warnings
	_ = results
	_ = searchErr
}

// TestSearchResult_Fields_Unchanged verifies SearchResult struct fields are preserved.
func TestSearchResult_Fields_Unchanged(t *testing.T) {
	result := SearchResult{
		Chunk: Chunk{
			ID:        "test-id",
			FilePath:  "test.go",
			Name:      "TestFunc",
			Type:      ChunkFunction,
			Signature: "func TestFunc()",
			Content:   "content",
			StartLine: 1,
			EndLine:   10,
			Language:  "go",
		},
		Score: 0.95,
	}

	// Verify all expected fields are accessible
	if result.Chunk.ID != "test-id" {
		t.Error("Chunk.ID field missing or changed")
	}
	if result.Chunk.FilePath != "test.go" {
		t.Error("Chunk.FilePath field missing or changed")
	}
	if result.Chunk.Name != "TestFunc" {
		t.Error("Chunk.Name field missing or changed")
	}
	if result.Chunk.Type != ChunkFunction {
		t.Error("Chunk.Type field missing or changed")
	}
	if result.Score != 0.95 {
		t.Error("Score field missing or changed")
	}
}

// TestHybridSearch_NotCalledByDefault verifies that HybridSearch is separate from Search.
func TestHybridSearch_NotCalledByDefault(t *testing.T) {
	storage, err := NewSQLiteStorage(":memory:", 4)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer storage.Close()

	embedder := &mockEmbedder{embedding: []float32{0.1, 0.2, 0.3, 0.4}}
	searcher := NewSearcher(storage, embedder)
	ctx := context.Background()

	// Add a chunk
	storage.Create(ctx, Chunk{ID: "1", Name: "Test"}, []float32{0.1, 0.2, 0.3, 0.4})

	// Standard Search should work without HybridSearch
	results, err := searcher.Search(ctx, "test", SearchOptions{TopK: 5})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(results) != 1 {
		t.Errorf("Search() returned %d results, want 1", len(results))
	}

	// HybridSearch is a separate method that can also be called
	hybridResults, err := searcher.HybridSearch(ctx, "test", HybridSearchOptions{
		SearchOptions: SearchOptions{TopK: 5},
	})
	if err != nil {
		t.Fatalf("HybridSearch() error = %v", err)
	}
	if len(hybridResults) != 1 {
		t.Errorf("HybridSearch() returned %d results, want 1", len(hybridResults))
	}
}

// TestTopK_BehaviorPreserved verifies that TopK limits results correctly.
func TestTopK_BehaviorPreserved(t *testing.T) {
	storage, err := NewSQLiteStorage(":memory:", 4)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer storage.Close()

	embedder := &mockEmbedder{embedding: []float32{0.1, 0.2, 0.3, 0.4}}
	searcher := NewSearcher(storage, embedder)
	ctx := context.Background()

	// Add many chunks
	for i := 0; i < 20; i++ {
		storage.Create(ctx, Chunk{ID: string(rune('a' + i)), Name: "Chunk"}, []float32{0.1, 0.2, 0.3, 0.4})
	}

	tests := []struct {
		topK     int
		expected int
	}{
		{topK: 5, expected: 5},
		{topK: 10, expected: 10},
		{topK: 1, expected: 1},
		{topK: 0, expected: 20}, // 0 means no limit
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("TopK=%d", tt.topK), func(t *testing.T) {
			results, err := searcher.Search(ctx, "test", SearchOptions{TopK: tt.topK})
			if err != nil {
				t.Fatalf("Search() error = %v", err)
			}
			if len(results) != tt.expected {
				t.Errorf("TopK=%d returned %d results, want %d", tt.topK, len(results), tt.expected)
			}
		})
	}
}

// ===== Enhanced Search Output Tests =====
// These tests verify the enhanced search output fields: relevance, preview, domain

// TestSearch_EnhancedOutput_RelevanceLabels verifies that search results include relevance labels.
func TestSearch_EnhancedOutput_RelevanceLabels(t *testing.T) {
	storage, err := NewSQLiteStorage(":memory:", 4)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer storage.Close()

	// Use specific embeddings for predictable cosine similarity
	embedder := &mockEmbedder{embedding: []float32{1, 0, 0, 0}}
	searcher := NewSearcher(storage, embedder)
	ctx := context.Background()

	// Add chunks with different similarity to query
	storage.Create(ctx, Chunk{ID: "high", Name: "High", Signature: "func High()"}, []float32{1, 0, 0, 0})           // Score 1.0
	storage.Create(ctx, Chunk{ID: "medium", Name: "Medium", Signature: "func Medium()"}, []float32{0.7, 0.7, 0, 0}) // Score ~0.7
	storage.Create(ctx, Chunk{ID: "low", Name: "Low", Signature: "func Low()"}, []float32{0.1, 0.9, 0, 0})          // Score ~0.1

	results, err := searcher.Search(ctx, "test", SearchOptions{TopK: 10})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}

	if len(results) != 3 {
		t.Fatalf("Search() returned %d results, want 3", len(results))
	}

	// Verify all results have relevance labels
	for _, r := range results {
		if r.Relevance == "" {
			t.Errorf("Result %q missing relevance label", r.Chunk.Name)
		}
		// Relevance should be one of: high, medium, low
		if r.Relevance != "high" && r.Relevance != "medium" && r.Relevance != "low" {
			t.Errorf("Result %q has invalid relevance: %q", r.Chunk.Name, r.Relevance)
		}
	}
}

// TestSearch_EnhancedOutput_Preview verifies that search results include preview.
func TestSearch_EnhancedOutput_Preview(t *testing.T) {
	storage, err := NewSQLiteStorage(":memory:", 4)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer storage.Close()

	embedder := &mockEmbedder{embedding: []float32{0.1, 0.2, 0.3, 0.4}}
	searcher := NewSearcher(storage, embedder)
	ctx := context.Background()

	// Add chunks with signatures and content
	storage.Create(ctx, Chunk{
		ID:        "1",
		Name:      "Add",
		Signature: "func Add(a, b int) int",
		Content:   "func Add(a, b int) int { return a + b }",
	}, []float32{0.1, 0.2, 0.3, 0.4})

	results, err := searcher.Search(ctx, "test", SearchOptions{TopK: 10})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("Search() returned %d results, want 1", len(results))
	}

	// Verify preview is set (should be signature since it's available)
	if results[0].Preview == "" {
		t.Error("Result missing preview")
	}
	if results[0].Preview != "func Add(a, b int) int" {
		t.Errorf("Preview = %q, want %q", results[0].Preview, "func Add(a, b int) int")
	}
}

// TestSearch_EnhancedOutput_Domain verifies that search results include domain.
func TestSearch_EnhancedOutput_Domain(t *testing.T) {
	storage, err := NewSQLiteStorage(":memory:", 4)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer storage.Close()

	embedder := &mockEmbedder{embedding: []float32{0.1, 0.2, 0.3, 0.4}}
	searcher := NewSearcher(storage, embedder)
	ctx := context.Background()

	// Add a chunk (domain should default to "code")
	storage.Create(ctx, Chunk{
		ID:   "1",
		Name: "Test",
	}, []float32{0.1, 0.2, 0.3, 0.4})

	results, err := searcher.Search(ctx, "test", SearchOptions{TopK: 10})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("Search() returned %d results, want 1", len(results))
	}

	// Verify domain is set to default "code"
	if results[0].Chunk.Domain != "code" {
		t.Errorf("Chunk.Domain = %q, want %q", results[0].Chunk.Domain, "code")
	}
}

// TestHybridSearch_EnhancedOutput verifies HybridSearch also populates enhanced fields.
func TestHybridSearch_EnhancedOutput(t *testing.T) {
	storage, err := NewSQLiteStorage(":memory:", 4)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer storage.Close()

	embedder := &mockEmbedder{embedding: []float32{0.1, 0.2, 0.3, 0.4}}
	searcher := NewSearcher(storage, embedder)
	ctx := context.Background()

	// Add a chunk with signature
	storage.Create(ctx, Chunk{
		ID:        "1",
		Name:      "Add",
		Signature: "func Add(a, b int) int",
		Content:   "func Add(a, b int) int { return a + b }",
	}, []float32{0.1, 0.2, 0.3, 0.4})

	results, err := searcher.HybridSearch(ctx, "test", HybridSearchOptions{
		SearchOptions: SearchOptions{TopK: 10},
	})
	if err != nil {
		t.Fatalf("HybridSearch() error = %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("HybridSearch() returned %d results, want 1", len(results))
	}

	// Verify enhanced fields
	if results[0].Relevance == "" {
		t.Error("HybridSearch result missing relevance label")
	}
	if results[0].Preview == "" {
		t.Error("HybridSearch result missing preview")
	}
	if results[0].Chunk.Domain != "code" {
		t.Errorf("HybridSearch Chunk.Domain = %q, want %q", results[0].Chunk.Domain, "code")
	}
}

// TestSearch_PercentileFallback verifies that percentile-based labeling works when no calibration.
func TestSearch_PercentileFallback(t *testing.T) {
	storage, err := NewSQLiteStorage(":memory:", 4)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer storage.Close()

	// Use specific embeddings for predictable cosine similarity
	embedder := &mockEmbedder{embedding: []float32{1, 0, 0, 0}}
	searcher := NewSearcher(storage, embedder)
	ctx := context.Background()

	// Add 10 chunks with varying similarity (no calibration metadata set)
	for i := 0; i < 10; i++ {
		emb := []float32{float32(10-i) / 10, float32(i) / 10, 0, 0}
		storage.Create(ctx, Chunk{
			ID:   fmt.Sprintf("chunk-%d", i),
			Name: fmt.Sprintf("Func%d", i),
		}, emb)
	}

	results, err := searcher.Search(ctx, "test", SearchOptions{TopK: 10})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}

	// Verify all results have relevance labels from percentile fallback
	highCount := 0
	mediumCount := 0
	lowCount := 0
	for _, r := range results {
		switch r.Relevance {
		case "high":
			highCount++
		case "medium":
			mediumCount++
		case "low":
			lowCount++
		default:
			t.Errorf("Invalid relevance %q for result %q", r.Relevance, r.Chunk.Name)
		}
	}

	// With 10 results and percentile distribution (20% high, 50% medium, 30% low):
	// - High: 2 results (top 20%)
	// - Medium: 5 results (middle 50%)
	// - Low: 3 results (bottom 30%)
	if highCount < 1 {
		t.Errorf("Expected at least 1 high result, got %d", highCount)
	}
	if mediumCount < 1 {
		t.Errorf("Expected at least 1 medium result, got %d", mediumCount)
	}
	if lowCount < 1 {
		t.Errorf("Expected at least 1 low result, got %d", lowCount)
	}
}

// TestThreshold_BehaviorPreserved verifies that threshold filtering works correctly.
func TestThreshold_BehaviorPreserved(t *testing.T) {
	storage, err := NewSQLiteStorage(":memory:", 4)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer storage.Close()

	// Use specific embeddings for predictable cosine similarity
	embedder := &mockEmbedder{embedding: []float32{1, 0, 0, 0}}
	searcher := NewSearcher(storage, embedder)
	ctx := context.Background()

	// Add chunks with different similarity to query
	storage.Create(ctx, Chunk{ID: "exact", Name: "Exact"}, []float32{1, 0, 0, 0})       // Score 1.0
	storage.Create(ctx, Chunk{ID: "high", Name: "High"}, []float32{0.9, 0.1, 0, 0})     // Score ~0.99
	storage.Create(ctx, Chunk{ID: "medium", Name: "Medium"}, []float32{0.7, 0.7, 0, 0}) // Score ~0.7
	storage.Create(ctx, Chunk{ID: "low", Name: "Low"}, []float32{0.1, 0.9, 0, 0})       // Score ~0.1
	storage.Create(ctx, Chunk{ID: "zero", Name: "Zero"}, []float32{0, 1, 0, 0})         // Score 0.0

	tests := []struct {
		threshold float32
		minCount  int
		maxCount  int
	}{
		{threshold: 0.0, minCount: 5, maxCount: 5},  // All results
		{threshold: 0.5, minCount: 3, maxCount: 3},  // Medium and above
		{threshold: 0.9, minCount: 2, maxCount: 2},  // High and exact
		{threshold: 0.99, minCount: 1, maxCount: 2}, // Only exact (or near-exact)
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("Threshold=%.2f", tt.threshold), func(t *testing.T) {
			results, err := searcher.Search(ctx, "test", SearchOptions{
				TopK:      10,
				Threshold: tt.threshold,
			})
			if err != nil {
				t.Fatalf("Search() error = %v", err)
			}
			if len(results) < tt.minCount || len(results) > tt.maxCount {
				t.Errorf("Threshold=%.2f returned %d results, want between %d and %d",
					tt.threshold, len(results), tt.minCount, tt.maxCount)
			}
		})
	}
}
