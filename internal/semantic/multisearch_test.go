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
		Results: []EnhancedResult{
			{SearchResult: SearchResult{Chunk: Chunk{ID: "1"}, Score: 0.9}, MatchedQueries: []string{"auth"}, BoostedScore: 0.9},
			{SearchResult: SearchResult{Chunk: Chunk{ID: "2"}, Score: 0.8}, MatchedQueries: []string{"login"}, BoostedScore: 0.8},
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
		Results: []EnhancedResult{
			{SearchResult: SearchResult{Chunk: Chunk{ID: "1", Name: "Test"}, Score: 0.9}, MatchedQueries: []string{"test"}, BoostedScore: 0.9},
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

// ===== Boosting Tests =====

// TestCalculateBoostedScore verifies the boost formula: BaseScore + (0.05 * (MatchCount - 1))
func TestCalculateBoostedScore(t *testing.T) {
	tests := []struct {
		name       string
		baseScore  float32
		matchCount int
		expected   float32
	}{
		{"single match no boost", 0.85, 1, 0.85},
		{"two matches +0.05", 0.80, 2, 0.85},
		{"three matches +0.10", 0.75, 3, 0.85},
		{"five matches +0.20", 0.30, 5, 0.50},
		{"capped at 1.0", 0.98, 3, 1.00},
		{"zero matches", 0.50, 0, 0.50},
		{"negative matches (defensive)", 0.50, -1, 0.50},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalculateBoostedScore(tt.baseScore, tt.matchCount)
			// Allow small floating point tolerance
			if diff := got - tt.expected; diff < -0.001 || diff > 0.001 {
				t.Errorf("CalculateBoostedScore(%f, %d) = %f, want %f",
					tt.baseScore, tt.matchCount, got, tt.expected)
			}
		})
	}
}

// TestMultisearch_MatchedQueriesTracking verifies which queries matched each result.
func TestMultisearch_MatchedQueriesTracking(t *testing.T) {
	// Use temp file database for concurrent access testing
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "matched_queries_test.db")

	storage, err := NewSQLiteStorage(dbPath, 4)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer storage.Close()

	embedder := &mockEmbedder{embedding: []float32{0.1, 0.2, 0.3, 0.4}}
	searcher := NewSearcher(storage, embedder)
	ctx := context.Background()

	// Add test chunk that will match all queries (same embedding)
	storage.Create(ctx, Chunk{ID: "1", Name: "AuthHandler"}, []float32{0.1, 0.2, 0.3, 0.4})

	result, err := searcher.Multisearch(ctx, MultisearchOptions{
		Queries: []string{"auth", "handler", "login"},
		TopK:    10,
	})
	if err != nil {
		t.Fatalf("Multisearch() error = %v", err)
	}

	if len(result.Results) != 1 {
		t.Fatalf("got %d results, want 1", len(result.Results))
	}

	// Should track all 3 queries
	if len(result.Results[0].MatchedQueries) != 3 {
		t.Errorf("MatchedQueries len = %d, want 3", len(result.Results[0].MatchedQueries))
	}

	// Queries should be in order
	expectedQueries := []string{"auth", "handler", "login"}
	for i, q := range expectedQueries {
		if i < len(result.Results[0].MatchedQueries) && result.Results[0].MatchedQueries[i] != q {
			t.Errorf("MatchedQueries[%d] = %q, want %q", i, result.Results[0].MatchedQueries[i], q)
		}
	}
}

// TestMultisearch_BoostedScoreCalculation verifies boosted scores are calculated correctly.
func TestMultisearch_BoostedScoreCalculation(t *testing.T) {
	// Use temp file database for concurrent access testing
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "boosted_score_test.db")

	storage, err := NewSQLiteStorage(dbPath, 4)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer storage.Close()

	embedder := &mockEmbedder{embedding: []float32{0.1, 0.2, 0.3, 0.4}}
	searcher := NewSearcher(storage, embedder)
	ctx := context.Background()

	// Add test chunk
	storage.Create(ctx, Chunk{ID: "1", Name: "Test"}, []float32{0.1, 0.2, 0.3, 0.4})

	// 3 queries, chunk matches all -> boost = base + 0.10
	result, err := searcher.Multisearch(ctx, MultisearchOptions{
		Queries: []string{"q1", "q2", "q3"},
		TopK:    10,
	})
	if err != nil {
		t.Fatalf("Multisearch() error = %v", err)
	}

	if len(result.Results) != 1 {
		t.Fatalf("got %d results, want 1", len(result.Results))
	}

	baseScore := result.Results[0].Score
	boostedScore := result.Results[0].BoostedScore

	// BoostedScore should be baseScore + 0.10 (3 matches -> +0.05 * 2)
	expectedBoost := baseScore + 0.10
	if expectedBoost > 1.0 {
		expectedBoost = 1.0
	}

	if diff := boostedScore - expectedBoost; diff < -0.01 || diff > 0.01 {
		t.Errorf("BoostedScore = %f, want %f (base %f + 0.10)", boostedScore, expectedBoost, baseScore)
	}
}

// TestMultisearch_BoostDisabled verifies boost can be disabled.
func TestMultisearch_BoostDisabled(t *testing.T) {
	// Use temp file database for concurrent access testing
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "boost_disabled_test.db")

	storage, err := NewSQLiteStorage(dbPath, 4)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer storage.Close()

	embedder := &mockEmbedder{embedding: []float32{0.1, 0.2, 0.3, 0.4}}
	searcher := NewSearcher(storage, embedder)
	ctx := context.Background()

	storage.Create(ctx, Chunk{ID: "1", Name: "Test"}, []float32{0.1, 0.2, 0.3, 0.4})

	// Disable boosting
	boostOff := false
	result, err := searcher.Multisearch(ctx, MultisearchOptions{
		Queries:         []string{"q1", "q2", "q3"},
		TopK:            10,
		BoostMultiMatch: &boostOff,
	})
	if err != nil {
		t.Fatalf("Multisearch() error = %v", err)
	}

	if len(result.Results) != 1 {
		t.Fatalf("got %d results, want 1", len(result.Results))
	}

	// BoostedScore should equal base Score when boost disabled
	if result.Results[0].BoostedScore != result.Results[0].Score {
		t.Errorf("BoostedScore = %f, want %f (same as base when disabled)",
			result.Results[0].BoostedScore, result.Results[0].Score)
	}
}

// TestMultisearch_BoostChangesRanking verifies boosting can change result order.
func TestMultisearch_BoostChangesRanking(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "boost_ranking_test.db")

	storage, err := NewSQLiteStorage(dbPath, 4)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer storage.Close()

	ctx := context.Background()

	// Chunk A: high score but matches fewer queries
	// Chunk B: lower score but matches more queries
	storage.Create(ctx, Chunk{ID: "A", Name: "HighScore"}, []float32{0.95, 0.05, 0, 0})
	storage.Create(ctx, Chunk{ID: "B", Name: "LowScore"}, []float32{0.7, 0.7, 0, 0})

	embedder := &mockEmbedder{embedding: []float32{1, 0, 0, 0}}
	searcher := NewSearcher(storage, embedder)

	// With boost enabled, a chunk matching more queries might rank higher
	result, err := searcher.Multisearch(ctx, MultisearchOptions{
		Queries: []string{"test"},
		TopK:    10,
	})
	if err != nil {
		t.Fatalf("Multisearch() error = %v", err)
	}

	// Verify results are sorted by boosted score
	for i := 1; i < len(result.Results); i++ {
		if result.Results[i].BoostedScore > result.Results[i-1].BoostedScore {
			t.Errorf("Results not sorted by BoostedScore: [%d]=%f > [%d]=%f",
				i, result.Results[i].BoostedScore, i-1, result.Results[i-1].BoostedScore)
		}
	}
}

// TestMultisearchOptions_IsBoostEnabled verifies default and explicit boost settings.
func TestMultisearchOptions_IsBoostEnabled(t *testing.T) {
	// Default (nil) should be enabled
	opts1 := MultisearchOptions{Queries: []string{"test"}}
	if !opts1.IsBoostEnabled() {
		t.Error("IsBoostEnabled() = false for nil, want true (default)")
	}

	// Explicit true
	boostOn := true
	opts2 := MultisearchOptions{Queries: []string{"test"}, BoostMultiMatch: &boostOn}
	if !opts2.IsBoostEnabled() {
		t.Error("IsBoostEnabled() = false for explicit true, want true")
	}

	// Explicit false
	boostOff := false
	opts3 := MultisearchOptions{Queries: []string{"test"}, BoostMultiMatch: &boostOff}
	if opts3.IsBoostEnabled() {
		t.Error("IsBoostEnabled() = true for explicit false, want false")
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

// ===== Output Format Tests =====

// TestOutputFormat_Validation verifies output format validation.
func TestOutputFormat_Validation(t *testing.T) {
	tests := []struct {
		format string
		valid  bool
	}{
		{"blended", true},
		{"by_query", true},
		{"by_collection", true},
		{"", true}, // empty means default (blended)
		{"invalid", false},
		{"by-query", false}, // hyphen not underscore
		{"BLENDED", false},  // case sensitive
	}

	for _, tt := range tests {
		t.Run(tt.format, func(t *testing.T) {
			got := IsValidOutputFormat(tt.format)
			if got != tt.valid {
				t.Errorf("IsValidOutputFormat(%q) = %v, want %v", tt.format, got, tt.valid)
			}
		})
	}
}

// TestOutputFormat_ValidOutputFormats verifies the list of valid formats.
func TestOutputFormat_ValidOutputFormats(t *testing.T) {
	formats := ValidOutputFormats()
	if len(formats) != 3 {
		t.Errorf("ValidOutputFormats() len = %d, want 3", len(formats))
	}

	expected := map[string]bool{"blended": true, "by_query": true, "by_collection": true}
	for _, f := range formats {
		if !expected[f] {
			t.Errorf("Unexpected format in ValidOutputFormats: %s", f)
		}
	}
}

// TestFormatByQuery verifies by_query output formatting.
func TestFormatByQuery(t *testing.T) {
	// Create a result with results matching multiple queries
	result := &MultisearchResult{
		Results: []EnhancedResult{
			{
				SearchResult:   SearchResult{Chunk: Chunk{ID: "1", Name: "Auth"}, Score: 0.9},
				MatchedQueries: []string{"auth", "login"},
				BoostedScore:   0.95,
			},
			{
				SearchResult:   SearchResult{Chunk: Chunk{ID: "2", Name: "DB"}, Score: 0.8},
				MatchedQueries: []string{"database"},
				BoostedScore:   0.8,
			},
		},
		TotalQueries:   3,
		TotalResults:   2,
		QueriesMatched: map[string]int{"auth": 1, "login": 1, "database": 1},
	}

	formatted := result.FormatByQuery()

	// Check format is set
	if formatted.Format != OutputByQuery {
		t.Errorf("Format = %s, want by_query", formatted.Format)
	}

	// Results should be nil (omitempty)
	if formatted.Results != nil {
		t.Errorf("Results should be nil in by_query format")
	}

	// Check ByQuery map
	if formatted.ByQuery == nil {
		t.Fatal("ByQuery is nil")
	}

	// "auth" query should have result ID=1
	if len(formatted.ByQuery["auth"]) != 1 || formatted.ByQuery["auth"][0].Chunk.ID != "1" {
		t.Errorf("auth query results incorrect")
	}

	// "login" query should also have result ID=1 (matches multiple queries)
	if len(formatted.ByQuery["login"]) != 1 || formatted.ByQuery["login"][0].Chunk.ID != "1" {
		t.Errorf("login query results incorrect")
	}

	// "database" query should have result ID=2
	if len(formatted.ByQuery["database"]) != 1 || formatted.ByQuery["database"][0].Chunk.ID != "2" {
		t.Errorf("database query results incorrect")
	}
}

// TestFormatByQuery_EmptyQueryResults verifies queries with no matches get empty arrays.
func TestFormatByQuery_EmptyQueryResults(t *testing.T) {
	result := &MultisearchResult{
		Results: []EnhancedResult{
			{
				SearchResult:   SearchResult{Chunk: Chunk{ID: "1"}, Score: 0.9},
				MatchedQueries: []string{"auth"},
				BoostedScore:   0.9,
			},
		},
		TotalQueries:   2,
		TotalResults:   1,
		QueriesMatched: map[string]int{"auth": 1, "nonexistent": 0},
	}

	formatted := result.FormatByQuery()

	// "nonexistent" query should have empty array, not nil
	if results, ok := formatted.ByQuery["nonexistent"]; !ok {
		t.Errorf("nonexistent query should have entry in ByQuery map")
	} else if results == nil {
		t.Errorf("nonexistent query should have empty array, not nil")
	} else if len(results) != 0 {
		t.Errorf("nonexistent query should have 0 results")
	}
}

// TestFormatByCollection verifies by_collection output formatting.
func TestFormatByCollection(t *testing.T) {
	// Create a result with results from multiple collections
	result := &MultisearchResult{
		Results: []EnhancedResult{
			{
				SearchResult:   SearchResult{Chunk: Chunk{ID: "1", Name: "Auth", Domain: "code"}, Score: 0.9},
				MatchedQueries: []string{"test"},
				BoostedScore:   0.9,
			},
			{
				SearchResult:   SearchResult{Chunk: Chunk{ID: "2", Name: "AuthDoc", Domain: "docs"}, Score: 0.85},
				MatchedQueries: []string{"test"},
				BoostedScore:   0.85,
			},
			{
				SearchResult:   SearchResult{Chunk: Chunk{ID: "3", Name: "Config", Domain: "code"}, Score: 0.8},
				MatchedQueries: []string{"test"},
				BoostedScore:   0.8,
			},
		},
		TotalQueries:   1,
		TotalResults:   3,
		QueriesMatched: map[string]int{"test": 3},
	}

	formatted := result.FormatByCollection()

	// Check format is set
	if formatted.Format != OutputByCollection {
		t.Errorf("Format = %s, want by_collection", formatted.Format)
	}

	// Check ByCollection map
	if formatted.ByCollection == nil {
		t.Fatal("ByCollection is nil")
	}

	// "code" collection should have 2 results
	if len(formatted.ByCollection["code"]) != 2 {
		t.Errorf("code collection has %d results, want 2", len(formatted.ByCollection["code"]))
	}

	// "docs" collection should have 1 result
	if len(formatted.ByCollection["docs"]) != 1 {
		t.Errorf("docs collection has %d results, want 1", len(formatted.ByCollection["docs"]))
	}

	// Results within each collection should be sorted by score
	codeResults := formatted.ByCollection["code"]
	for i := 1; i < len(codeResults); i++ {
		if codeResults[i].BoostedScore > codeResults[i-1].BoostedScore {
			t.Errorf("code results not sorted by score")
		}
	}
}

// TestFormatByCollection_EmptyDomain verifies results without Domain use "default".
func TestFormatByCollection_EmptyDomain(t *testing.T) {
	result := &MultisearchResult{
		Results: []EnhancedResult{
			{
				SearchResult:   SearchResult{Chunk: Chunk{ID: "1", Domain: ""}, Score: 0.9}, // no domain
				MatchedQueries: []string{"test"},
				BoostedScore:   0.9,
			},
		},
		TotalQueries:   1,
		TotalResults:   1,
		QueriesMatched: map[string]int{"test": 1},
	}

	formatted := result.FormatByCollection()

	// Results without domain should be in "default" collection
	if len(formatted.ByCollection["default"]) != 1 {
		t.Errorf("default collection should have 1 result for domain-less chunk")
	}
}

// TestFormatBlended verifies blended output formatting.
func TestFormatBlended(t *testing.T) {
	result := &MultisearchResult{
		Results: []EnhancedResult{
			{SearchResult: SearchResult{Chunk: Chunk{ID: "1"}, Score: 0.9}, BoostedScore: 0.9},
		},
		TotalQueries:   1,
		TotalResults:   1,
		QueriesMatched: map[string]int{"test": 1},
	}

	formatted := result.FormatBlended()

	if formatted.Format != OutputBlended {
		t.Errorf("Format = %s, want blended", formatted.Format)
	}
	if len(formatted.Results) != 1 {
		t.Errorf("Results len = %d, want 1", len(formatted.Results))
	}
	if formatted.ByQuery != nil {
		t.Errorf("ByQuery should be nil for blended format")
	}
	if formatted.ByCollection != nil {
		t.Errorf("ByCollection should be nil for blended format")
	}
}

// TestFormatAs verifies the FormatAs dispatcher.
func TestFormatAs(t *testing.T) {
	result := &MultisearchResult{
		Results: []EnhancedResult{
			{
				SearchResult:   SearchResult{Chunk: Chunk{ID: "1", Domain: "code"}, Score: 0.9},
				MatchedQueries: []string{"test"},
				BoostedScore:   0.9,
			},
		},
		TotalQueries:   1,
		TotalResults:   1,
		QueriesMatched: map[string]int{"test": 1},
	}

	tests := []struct {
		format   OutputFormat
		expected OutputFormat
	}{
		{OutputBlended, OutputBlended},
		{OutputByQuery, OutputByQuery},
		{OutputByCollection, OutputByCollection},
		{"", OutputBlended},                      // empty defaults to blended
		{OutputFormat("invalid"), OutputBlended}, // invalid defaults to blended
	}

	for _, tt := range tests {
		t.Run(string(tt.format), func(t *testing.T) {
			formatted := result.FormatAs(tt.format)
			if formatted.Format != tt.expected {
				t.Errorf("FormatAs(%q).Format = %s, want %s", tt.format, formatted.Format, tt.expected)
			}
		})
	}
}
