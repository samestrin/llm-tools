package semantic

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
)

// ===== MultisearchOptions Type Tests =====

// TestMultisearchOptions_Structure verifies the MultisearchOptions struct fields.
func TestMultisearchOptions_Structure(t *testing.T) {
	opts := MultisearchOptions{
		Queries:   []string{"auth", "login", "session"},
		TopK:      10,
		Threshold: 0.5,
		Profiles:  []string{"code", "docs"},
	}

	if len(opts.Queries) != 3 {
		t.Errorf("Queries len = %d, want 3", len(opts.Queries))
	}
	if opts.TopK != 10 {
		t.Errorf("TopK = %d, want 10", opts.TopK)
	}
	if opts.Threshold != 0.5 {
		t.Errorf("Threshold = %f, want 0.5", opts.Threshold)
	}
	if len(opts.Profiles) != 2 {
		t.Errorf("Profiles len = %d, want 2", len(opts.Profiles))
	}
}

// TestMultisearchOptions_JSONSerialization verifies JSON serialization.
func TestMultisearchOptions_JSONSerialization(t *testing.T) {
	opts := MultisearchOptions{
		Queries:   []string{"auth", "login"},
		TopK:      10,
		Threshold: 0.5,
	}

	data, err := json.Marshal(opts)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var decoded MultisearchOptions
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if len(decoded.Queries) != 2 {
		t.Errorf("decoded Queries len = %d, want 2", len(decoded.Queries))
	}
	if decoded.TopK != 10 {
		t.Errorf("decoded TopK = %d, want 10", decoded.TopK)
	}
}

// TestMultisearchOptions_Validation verifies validation rules.
func TestMultisearchOptions_Validation(t *testing.T) {
	tests := []struct {
		name    string
		opts    MultisearchOptions
		wantErr string
	}{
		{
			name:    "empty queries",
			opts:    MultisearchOptions{Queries: []string{}},
			wantErr: "queries cannot be empty",
		},
		{
			name:    "nil queries",
			opts:    MultisearchOptions{Queries: nil},
			wantErr: "queries cannot be empty",
		},
		{
			name:    "exceeds max queries",
			opts:    MultisearchOptions{Queries: make([]string, 11)},
			wantErr: "query count exceeds maximum of 10",
		},
		{
			name:    "empty string in queries",
			opts:    MultisearchOptions{Queries: []string{"auth", "", "login"}},
			wantErr: "query at index 1 cannot be empty",
		},
		{
			name:    "invalid threshold high",
			opts:    MultisearchOptions{Queries: []string{"auth"}, Threshold: 1.5},
			wantErr: "threshold must be between 0.0 and 1.0",
		},
		{
			name:    "invalid threshold negative",
			opts:    MultisearchOptions{Queries: []string{"auth"}, Threshold: -0.1},
			wantErr: "threshold must be between 0.0 and 1.0",
		},
		{
			name:    "valid single query",
			opts:    MultisearchOptions{Queries: []string{"auth"}},
			wantErr: "",
		},
		{
			name:    "valid max queries",
			opts:    MultisearchOptions{Queries: []string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "10"}},
			wantErr: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.opts.Validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("Validate() error = %v, want nil", err)
				}
			} else {
				if err == nil {
					t.Errorf("Validate() error = nil, want %q", tt.wantErr)
				} else if err.Error() != tt.wantErr {
					t.Errorf("Validate() error = %q, want %q", err.Error(), tt.wantErr)
				}
			}
		})
	}
}

// ===== MultisearchResult Type Tests =====

// TestMultisearchResult_Structure verifies the MultisearchResult struct fields.
func TestMultisearchResult_Structure(t *testing.T) {
	result := MultisearchResult{
		Results: []SearchResult{
			{Chunk: Chunk{ID: "1"}, Score: 0.9},
			{Chunk: Chunk{ID: "2"}, Score: 0.8},
		},
		TotalQueries:   3,
		TotalResults:   2,
		QueriesMatched: map[string]int{"auth": 2, "login": 1},
	}

	if len(result.Results) != 2 {
		t.Errorf("Results len = %d, want 2", len(result.Results))
	}
	if result.TotalQueries != 3 {
		t.Errorf("TotalQueries = %d, want 3", result.TotalQueries)
	}
	if result.TotalResults != 2 {
		t.Errorf("TotalResults = %d, want 2", result.TotalResults)
	}
	if result.QueriesMatched["auth"] != 2 {
		t.Errorf("QueriesMatched[auth] = %d, want 2", result.QueriesMatched["auth"])
	}
}

// TestMultisearchResult_JSONSerialization verifies JSON serialization.
func TestMultisearchResult_JSONSerialization(t *testing.T) {
	result := MultisearchResult{
		Results: []SearchResult{
			{Chunk: Chunk{ID: "1", Name: "Test"}, Score: 0.9},
		},
		TotalQueries:   1,
		TotalResults:   1,
		QueriesMatched: map[string]int{"test": 1},
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var decoded MultisearchResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if len(decoded.Results) != 1 {
		t.Errorf("decoded Results len = %d, want 1", len(decoded.Results))
	}
	if decoded.TotalQueries != 1 {
		t.Errorf("decoded TotalQueries = %d, want 1", decoded.TotalQueries)
	}
}

// ===== Multisearch Function Tests =====

// TestMultisearch_SingleQuery verifies Multisearch works with a single query.
func TestMultisearch_SingleQuery(t *testing.T) {
	storage, err := NewSQLiteStorage(":memory:", 4)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer storage.Close()

	embedder := &mockEmbedder{embedding: []float32{0.1, 0.2, 0.3, 0.4}}
	searcher := NewSearcher(storage, embedder)
	ctx := context.Background()

	// Add test data
	storage.Create(ctx, Chunk{ID: "1", Name: "Auth"}, []float32{0.1, 0.2, 0.3, 0.4})

	result, err := searcher.Multisearch(ctx, MultisearchOptions{
		Queries: []string{"authentication"},
		TopK:    10,
	})
	if err != nil {
		t.Fatalf("Multisearch() error = %v", err)
	}

	if result == nil {
		t.Fatal("Multisearch() returned nil result")
	}
	if result.TotalQueries != 1 {
		t.Errorf("TotalQueries = %d, want 1", result.TotalQueries)
	}
	if len(result.Results) != 1 {
		t.Errorf("Results len = %d, want 1", len(result.Results))
	}
}

// TestMultisearch_MultipleQueries verifies Multisearch works with multiple queries.
func TestMultisearch_MultipleQueries(t *testing.T) {
	// Use temp file database for concurrent access testing
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "multisearch_multi_test.db")

	storage, err := NewSQLiteStorage(dbPath, 4)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer storage.Close()

	embedder := &mockEmbedder{embedding: []float32{0.1, 0.2, 0.3, 0.4}}
	searcher := NewSearcher(storage, embedder)
	ctx := context.Background()

	// Add test data
	storage.Create(ctx, Chunk{ID: "1", Name: "Auth"}, []float32{0.1, 0.2, 0.3, 0.4})
	storage.Create(ctx, Chunk{ID: "2", Name: "Login"}, []float32{0.1, 0.2, 0.3, 0.4})

	result, err := searcher.Multisearch(ctx, MultisearchOptions{
		Queries: []string{"authentication", "login", "session"},
		TopK:    10,
	})
	if err != nil {
		t.Fatalf("Multisearch() error = %v", err)
	}

	if result == nil {
		t.Fatal("Multisearch() returned nil result")
	}
	if result.TotalQueries != 3 {
		t.Errorf("TotalQueries = %d, want 3", result.TotalQueries)
	}
}

// TestMultisearch_Deduplication verifies that duplicate chunks are deduplicated.
func TestMultisearch_Deduplication(t *testing.T) {
	// Use temp file database for concurrent access testing
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "multisearch_dedup_test.db")

	storage, err := NewSQLiteStorage(dbPath, 4)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer storage.Close()

	embedder := &mockEmbedder{embedding: []float32{0.1, 0.2, 0.3, 0.4}}
	searcher := NewSearcher(storage, embedder)
	ctx := context.Background()

	// Add test data - same chunk will match multiple queries
	storage.Create(ctx, Chunk{ID: "auth-chunk", Name: "AuthHandler"}, []float32{0.1, 0.2, 0.3, 0.4})

	result, err := searcher.Multisearch(ctx, MultisearchOptions{
		Queries: []string{"auth", "login", "handler"},
		TopK:    10,
	})
	if err != nil {
		t.Fatalf("Multisearch() error = %v", err)
	}

	// Each chunk should appear only once in results (deduplicated)
	if len(result.Results) != 1 {
		t.Errorf("Results len = %d, want 1 (deduplicated)", len(result.Results))
	}
}

// TestMultisearch_TopKLimit verifies TopK limits total results.
func TestMultisearch_TopKLimit(t *testing.T) {
	storage, err := NewSQLiteStorage(":memory:", 4)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer storage.Close()

	embedder := &mockEmbedder{embedding: []float32{0.1, 0.2, 0.3, 0.4}}
	searcher := NewSearcher(storage, embedder)
	ctx := context.Background()

	// Add many test chunks
	for i := 0; i < 20; i++ {
		storage.Create(ctx, Chunk{ID: string(rune('a' + i)), Name: "Func"}, []float32{0.1, 0.2, 0.3, 0.4})
	}

	result, err := searcher.Multisearch(ctx, MultisearchOptions{
		Queries: []string{"test"},
		TopK:    5,
	})
	if err != nil {
		t.Fatalf("Multisearch() error = %v", err)
	}

	if len(result.Results) > 5 {
		t.Errorf("Results len = %d, want <= 5", len(result.Results))
	}
}

// TestMultisearch_ValidationErrors verifies validation errors are returned.
func TestMultisearch_ValidationErrors(t *testing.T) {
	storage, err := NewSQLiteStorage(":memory:", 4)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer storage.Close()

	embedder := &mockEmbedder{embedding: []float32{0.1, 0.2, 0.3, 0.4}}
	searcher := NewSearcher(storage, embedder)
	ctx := context.Background()

	tests := []struct {
		name    string
		opts    MultisearchOptions
		wantErr string
	}{
		{
			name:    "empty queries",
			opts:    MultisearchOptions{Queries: []string{}},
			wantErr: "queries cannot be empty",
		},
		{
			name:    "exceeds max",
			opts:    MultisearchOptions{Queries: make([]string, 11)},
			wantErr: "query count exceeds maximum of 10",
		},
		{
			name:    "empty string",
			opts:    MultisearchOptions{Queries: []string{"auth", ""}},
			wantErr: "query at index 1 cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Fill empty strings for exceeds max test
			if tt.name == "exceeds max" {
				for i := range tt.opts.Queries {
					tt.opts.Queries[i] = "query"
				}
			}

			_, err := searcher.Multisearch(ctx, tt.opts)
			if err == nil {
				t.Errorf("Multisearch() error = nil, want %q", tt.wantErr)
			} else if err.Error() != tt.wantErr {
				t.Errorf("Multisearch() error = %q, want %q", err.Error(), tt.wantErr)
			}
		})
	}
}

// TestMultisearch_ResultsSortedByScore verifies results are sorted by score descending.
func TestMultisearch_ResultsSortedByScore(t *testing.T) {
	storage, err := NewSQLiteStorage(":memory:", 4)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer storage.Close()

	// Use embedder that returns varying embeddings based on position
	embedder := &mockEmbedder{embedding: []float32{1, 0, 0, 0}}
	searcher := NewSearcher(storage, embedder)
	ctx := context.Background()

	// Add chunks with different embeddings (different similarity to query)
	storage.Create(ctx, Chunk{ID: "high", Name: "High"}, []float32{1, 0, 0, 0})      // score 1.0
	storage.Create(ctx, Chunk{ID: "medium", Name: "Med"}, []float32{0.7, 0.7, 0, 0}) // score ~0.7
	storage.Create(ctx, Chunk{ID: "low", Name: "Low"}, []float32{0.1, 0.9, 0, 0})    // score ~0.1

	result, err := searcher.Multisearch(ctx, MultisearchOptions{
		Queries: []string{"test"},
		TopK:    10,
	})
	if err != nil {
		t.Fatalf("Multisearch() error = %v", err)
	}

	// Verify results are sorted by score descending
	for i := 1; i < len(result.Results); i++ {
		if result.Results[i].Score > result.Results[i-1].Score {
			t.Errorf("Results not sorted: score[%d]=%f > score[%d]=%f",
				i, result.Results[i].Score, i-1, result.Results[i-1].Score)
		}
	}
}
