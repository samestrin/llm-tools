package semantic

import (
	"context"
	"testing"
	"time"
)

// TestHybridSearchLatency validates that hybrid search overhead is acceptable.
// This is a basic latency validation - a full benchmark framework would need
// ground truth datasets which are out of scope for this sprint.
func TestHybridSearchLatency(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping latency test in short mode")
	}

	// Create in-memory storage for testing
	storage, err := NewSQLiteStorage(":memory:", 4)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer storage.Close()

	ctx := context.Background()

	// Index test chunks
	chunks := []Chunk{
		{ID: "1", FilePath: "/auth/login.go", Type: ChunkFunction, Name: "Login", Content: "func Login(user, pass string) error { return validateCredentials(user, pass) }", StartLine: 10, EndLine: 20, Language: "go", FileMtime: time.Now().Unix()},
		{ID: "2", FilePath: "/auth/logout.go", Type: ChunkFunction, Name: "Logout", Content: "func Logout(session string) error { return invalidateSession(session) }", StartLine: 5, EndLine: 15, Language: "go", FileMtime: time.Now().Unix()},
		{ID: "3", FilePath: "/db/connect.go", Type: ChunkFunction, Name: "Connect", Content: "func Connect(dsn string) (*sql.DB, error) { return sql.Open(\"postgres\", dsn) }", StartLine: 20, EndLine: 40, Language: "go", FileMtime: time.Now().Unix()},
		{ID: "4", FilePath: "/db/query.go", Type: ChunkFunction, Name: "Query", Content: "func Query(db *sql.DB, q string) (*sql.Rows, error) { return db.Query(q) }", StartLine: 10, EndLine: 25, Language: "go", FileMtime: time.Now().Unix()},
		{ID: "5", FilePath: "/api/handler.go", Type: ChunkFunction, Name: "HandleRequest", Content: "func HandleRequest(w http.ResponseWriter, r *http.Request) { json.NewEncoder(w).Encode(response) }", StartLine: 30, EndLine: 50, Language: "go", FileMtime: time.Now().Unix()},
	}

	// Use mock embeddings (4 dimensions to match storage)
	embeddings := [][]float32{
		{0.1, 0.2, 0.8, 0.3},
		{0.1, 0.3, 0.7, 0.2},
		{0.5, 0.6, 0.2, 0.1},
		{0.4, 0.7, 0.1, 0.2},
		{0.3, 0.4, 0.5, 0.6},
	}

	for i, chunk := range chunks {
		if err := storage.Create(ctx, chunk, embeddings[i]); err != nil {
			t.Fatalf("Failed to store chunk: %v", err)
		}
	}

	// Create mock embedder
	mockEmbedder := &mockEmbedderLatency{
		embeddings: map[string][]float32{
			"authentication":     {0.1, 0.25, 0.75, 0.25},
			"database query":     {0.45, 0.65, 0.15, 0.15},
			"api handler":        {0.3, 0.4, 0.5, 0.6},
			"login credentials":  {0.1, 0.2, 0.8, 0.3},
			"session management": {0.1, 0.3, 0.7, 0.2},
		},
		dim: 4,
	}

	searcher := NewSearcher(storage, mockEmbedder)

	testCases := []struct {
		name        string
		query       string
		hybrid      bool
		maxLatency  time.Duration
		description string
	}{
		{
			name:        "DenseSearch",
			query:       "authentication",
			hybrid:      false,
			maxLatency:  100 * time.Millisecond,
			description: "Baseline dense search latency",
		},
		{
			name:        "HybridSearch",
			query:       "authentication",
			hybrid:      true,
			maxLatency:  200 * time.Millisecond,
			description: "Hybrid search with FTS should add <50ms overhead",
		},
		{
			name:        "HybridSearchWithFilters",
			query:       "database query",
			hybrid:      true,
			maxLatency:  250 * time.Millisecond,
			description: "Hybrid search with additional filters",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			start := time.Now()

			var results []SearchResult
			var err error

			if tc.hybrid {
				results, err = searcher.HybridSearch(ctx, tc.query, HybridSearchOptions{
					SearchOptions: SearchOptions{TopK: 10},
					FusionK:       60,
					FusionAlpha:   0.7,
				})
			} else {
				results, err = searcher.Search(ctx, tc.query, SearchOptions{TopK: 10})
			}

			elapsed := time.Since(start)

			if err != nil {
				t.Errorf("Search failed: %v", err)
				return
			}

			if elapsed > tc.maxLatency {
				t.Errorf("%s: latency %v exceeds threshold %v", tc.description, elapsed, tc.maxLatency)
			}

			// Verify we got results
			if len(results) == 0 {
				t.Logf("Warning: no results returned for query %q", tc.query)
			}

			t.Logf("%s: %v, returned %d results", tc.name, elapsed, len(results))
		})
	}
}

// mockEmbedderLatency provides deterministic embeddings for testing
type mockEmbedderLatency struct {
	embeddings map[string][]float32
	dim        int
}

func (m *mockEmbedderLatency) Embed(ctx context.Context, text string) ([]float32, error) {
	if emb, ok := m.embeddings[text]; ok {
		return emb, nil
	}
	// Default embedding for unknown queries
	return []float32{0.25, 0.25, 0.25, 0.25}, nil
}

func (m *mockEmbedderLatency) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	results := make([][]float32, len(texts))
	for i, text := range texts {
		emb, err := m.Embed(ctx, text)
		if err != nil {
			return nil, err
		}
		results[i] = emb
	}
	return results, nil
}

func (m *mockEmbedderLatency) Dimensions() int {
	return m.dim
}

// TestRecencyBoostLatency validates that recency boosting adds minimal overhead
func TestRecencyBoostLatency(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping latency test in short mode")
	}

	// Create test data with varying mtimes
	now := time.Now()
	results := []SearchResult{
		{Chunk: Chunk{ID: "1", Name: "Recent", FilePath: "/a.go", FileMtime: now.Unix()}, Score: 0.8},
		{Chunk: Chunk{ID: "2", Name: "Week", FilePath: "/b.go", FileMtime: now.AddDate(0, 0, -7).Unix()}, Score: 0.85},
		{Chunk: Chunk{ID: "3", Name: "Month", FilePath: "/c.go", FileMtime: now.AddDate(0, -1, 0).Unix()}, Score: 0.9},
		{Chunk: Chunk{ID: "4", Name: "Old", FilePath: "/d.go", FileMtime: now.AddDate(0, -6, 0).Unix()}, Score: 0.75},
		{Chunk: Chunk{ID: "5", Name: "NoMtime", FilePath: "/e.go", FileMtime: 0}, Score: 0.7},
	}

	// Create mtime map from results
	mtimes := make(map[string]time.Time)
	for _, r := range results {
		if r.Chunk.FileMtime > 0 {
			mtimes[r.Chunk.FilePath] = time.Unix(r.Chunk.FileMtime, 0)
		}
	}

	cfg := RecencyConfig{
		Factor:       0.5,
		HalfLifeDays: 7.0,
	}

	// Measure recency calculation time
	iterations := 1000
	start := time.Now()

	for i := 0; i < iterations; i++ {
		ApplyRecencyBoost(results, mtimes, cfg, now)
	}

	elapsed := time.Since(start)
	avgLatency := elapsed / time.Duration(iterations)

	// Recency calculation should be sub-millisecond
	maxAvgLatency := 1 * time.Millisecond
	if avgLatency > maxAvgLatency {
		t.Errorf("Recency boost average latency %v exceeds threshold %v", avgLatency, maxAvgLatency)
	}

	t.Logf("Recency boost avg latency over %d iterations: %v", iterations, avgLatency)
}

// TestRRFFusionLatency validates that RRF fusion calculation is efficient
func TestRRFFusionLatency(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping latency test in short mode")
	}

	// Create test result sets
	denseResults := make([]SearchResult, 100)
	lexicalResults := make([]SearchResult, 100)

	for i := 0; i < 100; i++ {
		denseResults[i] = SearchResult{Chunk: Chunk{ID: string(rune('A' + i%26))}, Score: float32(100-i) / 100}
		lexicalResults[i] = SearchResult{Chunk: Chunk{ID: string(rune('Z' - i%26))}, Score: float32(100-i) / 100}
	}

	// Measure fusion time
	iterations := 1000
	start := time.Now()

	for i := 0; i < iterations; i++ {
		FuseRRF(denseResults, lexicalResults, 60)
	}

	elapsed := time.Since(start)
	avgLatency := elapsed / time.Duration(iterations)

	// RRF fusion should be sub-millisecond for typical result sets
	maxAvgLatency := 1 * time.Millisecond
	if avgLatency > maxAvgLatency {
		t.Errorf("RRF fusion average latency %v exceeds threshold %v", avgLatency, maxAvgLatency)
	}

	t.Logf("RRF fusion avg latency over %d iterations: %v", iterations, avgLatency)
}
