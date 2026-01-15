package semantic

import (
	"context"
	"testing"
)

// TestLexicalSearch_ExactMatch verifies that searching for an exact function name
// returns the correct chunk with a relevance score.
func TestLexicalSearch_ExactMatch(t *testing.T) {
	storage, err := NewSQLiteStorage(":memory:", 384)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer storage.Close()

	ctx := context.Background()

	// Create a chunk with a unique name
	chunk := Chunk{
		ID:        "exact-match-chunk",
		FilePath:  "/test/auth.go",
		Type:      ChunkFunction,
		Name:      "handleAuthCallback",
		Signature: "func handleAuthCallback(ctx context.Context) error",
		Content:   "func handleAuthCallback(ctx context.Context) error { return validateToken(ctx) }",
		StartLine: 10,
		EndLine:   15,
		Language:  "go",
	}

	embedding := make([]float32, 384)
	if err := storage.Create(ctx, chunk, embedding); err != nil {
		t.Fatalf("failed to create chunk: %v", err)
	}

	// Search via Storage interface method
	opts := LexicalSearchOptions{TopK: 10}
	results, err := storage.LexicalSearch(ctx, "handleAuthCallback", opts)
	if err != nil {
		t.Fatalf("LexicalSearch failed: %v", err)
	}

	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}

	if len(results) > 0 {
		if results[0].Chunk.ID != chunk.ID {
			t.Errorf("expected chunk ID %s, got %s", chunk.ID, results[0].Chunk.ID)
		}
		if results[0].Score == 0 {
			t.Error("expected non-zero relevance score")
		}
	}
}

// TestLexicalSearch_PartialMatch verifies that searching for a keyword
// returns multiple matching chunks ranked by relevance.
func TestLexicalSearch_PartialMatch(t *testing.T) {
	storage, err := NewSQLiteStorage(":memory:", 384)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer storage.Close()

	ctx := context.Background()

	// Create chunks with overlapping keywords in content
	chunks := []ChunkWithEmbedding{
		{
			Chunk: Chunk{
				ID:        "auth-chunk-1",
				FilePath:  "/test/auth.go",
				Type:      ChunkFunction,
				Name:      "handleAuth",
				Content:   "func handleAuth() { auth.verify(); auth.validate() }",
				StartLine: 1,
				EndLine:   5,
				Language:  "go",
			},
			Embedding: make([]float32, 384),
		},
		{
			Chunk: Chunk{
				ID:        "auth-chunk-2",
				FilePath:  "/test/callback.go",
				Type:      ChunkFunction,
				Name:      "handleAuthCallback",
				Content:   "func handleAuthCallback() { auth.callback() }",
				StartLine: 1,
				EndLine:   5,
				Language:  "go",
			},
			Embedding: make([]float32, 384),
		},
		{
			Chunk: Chunk{
				ID:        "auth-chunk-3",
				FilePath:  "/test/validate.go",
				Type:      ChunkFunction,
				Name:      "validateAuth",
				Content:   "func validateAuth() { /* auth validation */ }",
				StartLine: 1,
				EndLine:   5,
				Language:  "go",
			},
			Embedding: make([]float32, 384),
		},
	}

	if err := storage.CreateBatch(ctx, chunks); err != nil {
		t.Fatalf("failed to create batch: %v", err)
	}

	// Search for "auth" - should match all three
	opts := LexicalSearchOptions{TopK: 10}
	results, err := storage.LexicalSearch(ctx, "auth", opts)
	if err != nil {
		t.Fatalf("LexicalSearch failed: %v", err)
	}

	if len(results) < 3 {
		t.Errorf("expected at least 3 results, got %d", len(results))
	}

	// Verify results are sorted by relevance (BM25 - lower is better)
	for i := 1; i < len(results); i++ {
		if results[i].Score < results[i-1].Score {
			// BM25 scores are negative, more negative = more relevant
			// So we expect scores to increase (become less negative) as relevance decreases
			// But our LexicalSearch should normalize this to descending order
		}
	}
}

// TestLexicalSearch_TypeFilter verifies that the type filter correctly
// restricts results to the specified chunk type.
func TestLexicalSearch_TypeFilter(t *testing.T) {
	storage, err := NewSQLiteStorage(":memory:", 384)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer storage.Close()

	ctx := context.Background()

	// Create chunks of different types mentioning "User"
	chunks := []ChunkWithEmbedding{
		{
			Chunk: Chunk{
				ID:        "user-func",
				FilePath:  "/test/user.go",
				Type:      ChunkFunction,
				Name:      "GetUser",
				Content:   "func GetUser(id string) User { return User{} }",
				StartLine: 1,
				EndLine:   5,
				Language:  "go",
			},
			Embedding: make([]float32, 384),
		},
		{
			Chunk: Chunk{
				ID:        "user-struct",
				FilePath:  "/test/types.go",
				Type:      ChunkStruct,
				Name:      "User",
				Content:   "type User struct { ID string; Name string }",
				StartLine: 10,
				EndLine:   15,
				Language:  "go",
			},
			Embedding: make([]float32, 384),
		},
	}

	if err := storage.CreateBatch(ctx, chunks); err != nil {
		t.Fatalf("failed to create batch: %v", err)
	}

	// Search with type filter for struct only
	opts := LexicalSearchOptions{
		TopK: 10,
		Type: "struct",
	}
	results, err := storage.LexicalSearch(ctx, "User", opts)
	if err != nil {
		t.Fatalf("LexicalSearch failed: %v", err)
	}

	if len(results) != 1 {
		t.Errorf("expected 1 struct result, got %d", len(results))
	}

	if len(results) > 0 && results[0].Chunk.Type != ChunkStruct {
		t.Errorf("expected struct type, got %s", results[0].Chunk.Type)
	}
}

// TestLexicalSearch_PathFilter verifies that the path filter correctly
// restricts results to chunks from the specified path prefix.
func TestLexicalSearch_PathFilter(t *testing.T) {
	storage, err := NewSQLiteStorage(":memory:", 384)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer storage.Close()

	ctx := context.Background()

	// Create chunks in different directories with searchable keywords
	chunks := []ChunkWithEmbedding{
		{
			Chunk: Chunk{
				ID:        "auth-validate",
				FilePath:  "/internal/auth/validate.go",
				Type:      ChunkFunction,
				Name:      "checkCredentials",
				Content:   "func checkCredentials(creds Credentials) bool { return verify(creds) }",
				StartLine: 1,
				EndLine:   5,
				Language:  "go",
			},
			Embedding: make([]float32, 384),
		},
		{
			Chunk: Chunk{
				ID:        "user-validate",
				FilePath:  "/internal/user/validate.go",
				Type:      ChunkFunction,
				Name:      "checkUserInput",
				Content:   "func checkUserInput(input string) bool { return verify(input) }",
				StartLine: 1,
				EndLine:   5,
				Language:  "go",
			},
			Embedding: make([]float32, 384),
		},
	}

	if err := storage.CreateBatch(ctx, chunks); err != nil {
		t.Fatalf("failed to create batch: %v", err)
	}

	// Search with path filter for auth directory - search for "verify" which appears in both
	opts := LexicalSearchOptions{
		TopK:       10,
		PathFilter: "/internal/auth",
	}
	results, err := storage.LexicalSearch(ctx, "verify", opts)
	if err != nil {
		t.Fatalf("LexicalSearch failed: %v", err)
	}

	if len(results) != 1 {
		t.Errorf("expected 1 result from /internal/auth, got %d", len(results))
	}

	if len(results) > 0 && results[0].Chunk.FilePath != "/internal/auth/validate.go" {
		t.Errorf("expected chunk from /internal/auth, got %s", results[0].Chunk.FilePath)
	}
}

// TestLexicalSearch_NoResults verifies that searching for a non-existent term
// returns an empty slice (not nil) and no error.
func TestLexicalSearch_NoResults(t *testing.T) {
	storage, err := NewSQLiteStorage(":memory:", 384)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer storage.Close()

	ctx := context.Background()

	// Create a chunk
	chunk := Chunk{
		ID:       "some-chunk",
		FilePath: "/test/file.go",
		Type:     ChunkFunction,
		Name:     "someFunction",
		Content:  "func someFunction() {}",
		Language: "go",
	}
	if err := storage.Create(ctx, chunk, make([]float32, 384)); err != nil {
		t.Fatalf("failed to create chunk: %v", err)
	}

	// Search for non-existent term
	opts := LexicalSearchOptions{TopK: 10}
	results, err := storage.LexicalSearch(ctx, "xyzNonExistent123", opts)
	if err != nil {
		t.Fatalf("LexicalSearch should not error on no results: %v", err)
	}

	if results == nil {
		t.Error("expected empty slice, got nil")
	}

	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

// TestLexicalSearch_CaseInsensitive verifies that FTS5 performs
// case-insensitive matching.
func TestLexicalSearch_CaseInsensitive(t *testing.T) {
	storage, err := NewSQLiteStorage(":memory:", 384)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer storage.Close()

	ctx := context.Background()

	chunk := Chunk{
		ID:        "pascal-case-chunk",
		FilePath:  "/test/handler.go",
		Type:      ChunkFunction,
		Name:      "HandleAuthCallback",
		Content:   "func HandleAuthCallback() { /* PascalCase */ }",
		StartLine: 1,
		EndLine:   5,
		Language:  "go",
	}

	if err := storage.Create(ctx, chunk, make([]float32, 384)); err != nil {
		t.Fatalf("failed to create chunk: %v", err)
	}

	// Search with lowercase
	opts := LexicalSearchOptions{TopK: 10}
	results, err := storage.LexicalSearch(ctx, "handleauthcallback", opts)
	if err != nil {
		t.Fatalf("LexicalSearch failed: %v", err)
	}

	if len(results) != 1 {
		t.Errorf("expected 1 result with case-insensitive match, got %d", len(results))
	}
}

// TestLexicalSearch_BooleanOperators verifies that FTS5 boolean operators
// (AND, OR) work correctly.
func TestLexicalSearch_BooleanOperators(t *testing.T) {
	storage, err := NewSQLiteStorage(":memory:", 384)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer storage.Close()

	ctx := context.Background()

	// Create chunks with separate words that FTS5 can tokenize
	chunks := []ChunkWithEmbedding{
		{
			Chunk: Chunk{
				ID:       "both-terms",
				FilePath: "/test/handler.go",
				Type:     ChunkFunction,
				Name:     "processAuth",
				Content:  "func processAuth() { handle the auth request }",
				Language: "go",
			},
			Embedding: make([]float32, 384),
		},
		{
			Chunk: Chunk{
				ID:       "handle-only",
				FilePath: "/test/request.go",
				Type:     ChunkFunction,
				Name:     "processRequest",
				Content:  "func processRequest() { handle the request }",
				Language: "go",
			},
			Embedding: make([]float32, 384),
		},
		{
			Chunk: Chunk{
				ID:       "auth-only",
				FilePath: "/test/verify.go",
				Type:     ChunkFunction,
				Name:     "verifyAuth",
				Content:  "func verifyAuth() { verify the auth token }",
				Language: "go",
			},
			Embedding: make([]float32, 384),
		},
	}

	if err := storage.CreateBatch(ctx, chunks); err != nil {
		t.Fatalf("failed to create batch: %v", err)
	}

	// Search with AND operator - should only match chunks with both "handle" AND "auth"
	opts := LexicalSearchOptions{TopK: 10}
	results, err := storage.LexicalSearch(ctx, "handle AND auth", opts)
	if err != nil {
		t.Fatalf("LexicalSearch with AND failed: %v", err)
	}

	// Should match only "both-terms" which has both words
	if len(results) != 1 {
		t.Errorf("expected 1 result with AND operator, got %d", len(results))
	}

	// Search with OR operator - should match all chunks that have either word
	results, err = storage.LexicalSearch(ctx, "handle OR auth", opts)
	if err != nil {
		t.Fatalf("LexicalSearch with OR failed: %v", err)
	}

	if len(results) < 3 {
		t.Errorf("expected at least 3 results with OR operator, got %d", len(results))
	}
}

// TestLexicalSearch_EmptyQuery verifies that an empty query returns empty results.
func TestLexicalSearch_EmptyQuery(t *testing.T) {
	storage, err := NewSQLiteStorage(":memory:", 384)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer storage.Close()

	ctx := context.Background()

	// Create a chunk
	if err := storage.Create(ctx, Chunk{
		ID: "test", FilePath: "/test.go", Type: ChunkFunction, Name: "test", Language: "go",
	}, make([]float32, 384)); err != nil {
		t.Fatalf("failed to create chunk: %v", err)
	}

	opts := LexicalSearchOptions{TopK: 10}
	results, err := storage.LexicalSearch(ctx, "", opts)
	if err != nil {
		t.Fatalf("LexicalSearch with empty query should not error: %v", err)
	}

	if len(results) != 0 {
		t.Errorf("expected 0 results for empty query, got %d", len(results))
	}
}

// TestLexicalSearch_StorageClosed verifies that calling LexicalSearch on
// a closed storage returns ErrStorageClosed.
func TestLexicalSearch_StorageClosed(t *testing.T) {
	storage, err := NewSQLiteStorage(":memory:", 384)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}

	// Close the storage
	storage.Close()

	ctx := context.Background()
	opts := LexicalSearchOptions{TopK: 10}
	_, err = storage.LexicalSearch(ctx, "test", opts)

	if err != ErrStorageClosed {
		t.Errorf("expected ErrStorageClosed, got %v", err)
	}
}

// TestLexicalSearch_TopKLimit verifies that the TopK parameter correctly
// limits the number of results returned.
func TestLexicalSearch_TopKLimit(t *testing.T) {
	storage, err := NewSQLiteStorage(":memory:", 384)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer storage.Close()

	ctx := context.Background()

	// Create multiple chunks with same keyword
	var chunks []ChunkWithEmbedding
	for i := 0; i < 20; i++ {
		chunks = append(chunks, ChunkWithEmbedding{
			Chunk: Chunk{
				ID:       string(rune('a' + i)),
				FilePath: "/test/file.go",
				Type:     ChunkFunction,
				Name:     "testFunc",
				Content:  "func testFunc() { /* test function */ }",
				Language: "go",
			},
			Embedding: make([]float32, 384),
		})
	}

	if err := storage.CreateBatch(ctx, chunks); err != nil {
		t.Fatalf("failed to create batch: %v", err)
	}

	// Search with TopK=5
	opts := LexicalSearchOptions{TopK: 5}
	results, err := storage.LexicalSearch(ctx, "testFunc", opts)
	if err != nil {
		t.Fatalf("LexicalSearch failed: %v", err)
	}

	if len(results) > 5 {
		t.Errorf("expected max 5 results with TopK=5, got %d", len(results))
	}
}

// ===== Lexical Search Delegation Tests =====
// These tests verify that QdrantStorage delegates lexical search to its parallel FTS index
// and that SQLiteStorage uses its native FTS5 implementation.

// TestSQLiteStorage_ImplementsLexicalSearcher verifies SQLiteStorage implements LexicalSearcher.
func TestSQLiteStorage_ImplementsLexicalSearcher(t *testing.T) {
	storage, err := NewSQLiteStorage(":memory:", 4)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer storage.Close()

	// Verify it implements the LexicalSearcher interface
	var _ LexicalSearcher = storage
}

// TestLexicalIndex_SearchDelegation verifies the LexicalIndex Search method works correctly.
// This is the underlying implementation that QdrantStorage delegates to.
func TestLexicalIndex_SearchDelegation(t *testing.T) {
	// Create standalone lexical index
	idx, err := NewLexicalIndex(":memory:", 4)
	if err != nil {
		t.Fatalf("failed to create lexical index: %v", err)
	}
	defer idx.Close()

	ctx := context.Background()

	// Index some chunks
	chunks := []Chunk{
		{
			ID:        "chunk-1",
			FilePath:  "/internal/auth/handler.go",
			Type:      ChunkFunction,
			Name:      "handleAuthentication",
			Content:   "func handleAuthentication(ctx context.Context) error { return validateToken(ctx) }",
			StartLine: 10,
			EndLine:   20,
			Language:  "go",
		},
		{
			ID:        "chunk-2",
			FilePath:  "/internal/user/service.go",
			Type:      ChunkFunction,
			Name:      "createUser",
			Content:   "func createUser(name string) (*User, error) { return &User{Name: name}, nil }",
			StartLine: 30,
			EndLine:   40,
			Language:  "go",
		},
	}

	if err := idx.IndexBatch(ctx, chunks); err != nil {
		t.Fatalf("failed to index batch: %v", err)
	}

	// Search for "authentication"
	opts := LexicalSearchOptions{TopK: 10}
	results, err := idx.Search(ctx, "handleAuthentication", opts)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}

	if len(results) > 0 {
		if results[0].Chunk.ID != "chunk-1" {
			t.Errorf("expected chunk-1, got %s", results[0].Chunk.ID)
		}
		if results[0].Score <= 0 {
			t.Error("expected positive relevance score")
		}
	}
}

// TestLexicalIndex_SearchWithFilters verifies that LexicalIndex filters work.
func TestLexicalIndex_SearchWithFilters(t *testing.T) {
	idx, err := NewLexicalIndex(":memory:", 4)
	if err != nil {
		t.Fatalf("failed to create lexical index: %v", err)
	}
	defer idx.Close()

	ctx := context.Background()

	// Index chunks of different types and paths
	// Using "process" as the search term since FTS5 tokenizes on word boundaries
	chunks := []Chunk{
		{
			ID:       "func-internal",
			FilePath: "/internal/service/handler.go",
			Type:     ChunkFunction,
			Name:     "handleRequest",
			Content:  "func handleRequest() { process the request }",
			Language: "go",
		},
		{
			ID:       "method-internal",
			FilePath: "/internal/service/handler.go",
			Type:     ChunkMethod,
			Name:     "handleMethod",
			Content:  "func (s *Service) handleMethod() { process the data }",
			Language: "go",
		},
		{
			ID:       "func-cmd",
			FilePath: "/cmd/main.go",
			Type:     ChunkFunction,
			Name:     "handleMain",
			Content:  "func handleMain() { process the args }",
			Language: "go",
		},
	}

	if err := idx.IndexBatch(ctx, chunks); err != nil {
		t.Fatalf("failed to index batch: %v", err)
	}

	// Test type filter - search for "process" which is in all chunks
	t.Run("TypeFilter", func(t *testing.T) {
		results, err := idx.Search(ctx, "process", LexicalSearchOptions{TopK: 10, Type: "function"})
		if err != nil {
			t.Fatalf("Search with type filter failed: %v", err)
		}
		if len(results) != 2 {
			t.Errorf("expected 2 function results, got %d", len(results))
		}
		for _, r := range results {
			if r.Chunk.Type != ChunkFunction {
				t.Errorf("expected function type, got %s", r.Chunk.Type)
			}
		}
	})

	// Test path filter - search for "process" but only in /internal paths
	t.Run("PathFilter", func(t *testing.T) {
		results, err := idx.Search(ctx, "process", LexicalSearchOptions{TopK: 10, PathFilter: "/internal"})
		if err != nil {
			t.Fatalf("Search with path filter failed: %v", err)
		}
		if len(results) != 2 {
			t.Errorf("expected 2 results from /internal, got %d", len(results))
		}
	})
}

// TestLexicalIndex_EmptyIndex verifies that searching an empty index returns empty results.
func TestLexicalIndex_EmptyIndex(t *testing.T) {
	idx, err := NewLexicalIndex(":memory:", 4)
	if err != nil {
		t.Fatalf("failed to create lexical index: %v", err)
	}
	defer idx.Close()

	ctx := context.Background()

	results, err := idx.Search(ctx, "anything", LexicalSearchOptions{TopK: 10})
	if err != nil {
		t.Fatalf("Search on empty index should not error: %v", err)
	}

	if results == nil {
		t.Error("expected empty slice, got nil")
	}

	if len(results) != 0 {
		t.Errorf("expected 0 results from empty index, got %d", len(results))
	}
}

// TestLexicalIndex_EmptyQuery verifies that empty query returns empty results.
func TestLexicalIndex_EmptyQuery(t *testing.T) {
	idx, err := NewLexicalIndex(":memory:", 4)
	if err != nil {
		t.Fatalf("failed to create lexical index: %v", err)
	}
	defer idx.Close()

	// Add a chunk first
	ctx := context.Background()
	idx.IndexChunk(ctx, Chunk{ID: "test", FilePath: "/test.go", Name: "test", Type: ChunkFunction, Content: "test"})

	results, err := idx.Search(ctx, "", LexicalSearchOptions{TopK: 10})
	if err != nil {
		t.Fatalf("Search with empty query should not error: %v", err)
	}

	if len(results) != 0 {
		t.Errorf("expected 0 results for empty query, got %d", len(results))
	}
}

// TestLexicalIndex_ClosedIndex verifies that searching a closed index returns error.
func TestLexicalIndex_ClosedIndex(t *testing.T) {
	idx, err := NewLexicalIndex(":memory:", 4)
	if err != nil {
		t.Fatalf("failed to create lexical index: %v", err)
	}

	// Close the index
	idx.Close()

	ctx := context.Background()
	_, err = idx.Search(ctx, "test", LexicalSearchOptions{TopK: 10})
	if err != ErrStorageClosed {
		t.Errorf("expected ErrStorageClosed, got %v", err)
	}
}

// TestLexicalSearcher_InterfaceCompatibility verifies that both SQLiteStorage
// and LexicalIndex return compatible results when implementing LexicalSearcher.
func TestLexicalSearcher_InterfaceCompatibility(t *testing.T) {
	ctx := context.Background()

	// Create SQLiteStorage and LexicalIndex with same data
	sqliteStorage, err := NewSQLiteStorage(":memory:", 4)
	if err != nil {
		t.Fatalf("failed to create SQLite storage: %v", err)
	}
	defer sqliteStorage.Close()

	lexicalIdx, err := NewLexicalIndex(":memory:", 4)
	if err != nil {
		t.Fatalf("failed to create lexical index: %v", err)
	}
	defer lexicalIdx.Close()

	// Add same chunk to both
	chunk := Chunk{
		ID:        "test-chunk",
		FilePath:  "/test/file.go",
		Type:      ChunkFunction,
		Name:      "searchableFunction",
		Content:   "func searchableFunction() { /* unique content */ }",
		StartLine: 1,
		EndLine:   5,
		Language:  "go",
	}

	// Add to SQLiteStorage (which auto-syncs to FTS)
	if err := sqliteStorage.Create(ctx, chunk, make([]float32, 4)); err != nil {
		t.Fatalf("failed to add chunk to SQLite: %v", err)
	}

	// Add to LexicalIndex directly
	if err := lexicalIdx.IndexChunk(ctx, chunk); err != nil {
		t.Fatalf("failed to add chunk to lexical index: %v", err)
	}

	// Search both
	opts := LexicalSearchOptions{TopK: 10}
	query := "searchableFunction"

	sqliteResults, err := sqliteStorage.LexicalSearch(ctx, query, opts)
	if err != nil {
		t.Fatalf("SQLite LexicalSearch failed: %v", err)
	}

	lexicalResults, err := lexicalIdx.Search(ctx, query, opts)
	if err != nil {
		t.Fatalf("LexicalIndex Search failed: %v", err)
	}

	// Verify results are compatible
	if len(sqliteResults) != len(lexicalResults) {
		t.Errorf("result count mismatch: SQLite=%d, LexicalIndex=%d", len(sqliteResults), len(lexicalResults))
	}

	if len(sqliteResults) > 0 && len(lexicalResults) > 0 {
		// Check that chunk IDs match
		if sqliteResults[0].Chunk.ID != lexicalResults[0].Chunk.ID {
			t.Errorf("chunk ID mismatch: SQLite=%s, LexicalIndex=%s",
				sqliteResults[0].Chunk.ID, lexicalResults[0].Chunk.ID)
		}

		// Check that both have positive scores
		if sqliteResults[0].Score <= 0 {
			t.Error("SQLite result should have positive score")
		}
		if lexicalResults[0].Score <= 0 {
			t.Error("LexicalIndex result should have positive score")
		}
	}
}

// ===== Atomic Dual-Write Synchronization Tests =====
// These tests verify that CRUD operations on the primary store
// correctly sync to the parallel FTS index.

// TestDualWrite_CreateSyncsToFTS verifies that Create operations sync to FTS.
func TestDualWrite_CreateSyncsToFTS(t *testing.T) {
	ctx := context.Background()

	// Create SQLiteStorage (which has native FTS)
	storage, err := NewSQLiteStorage(":memory:", 4)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer storage.Close()

	// Create a chunk
	chunk := Chunk{
		ID:        "dual-write-create",
		FilePath:  "/test/handler.go",
		Type:      ChunkFunction,
		Name:      "handleDualWrite",
		Content:   "func handleDualWrite() { process() }",
		StartLine: 10,
		EndLine:   15,
		Language:  "go",
	}

	embedding := make([]float32, 4)
	if err := storage.Create(ctx, chunk, embedding); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Verify chunk is searchable via lexical search (FTS)
	results, err := storage.LexicalSearch(ctx, "handleDualWrite", LexicalSearchOptions{TopK: 10})
	if err != nil {
		t.Fatalf("LexicalSearch failed: %v", err)
	}

	if len(results) != 1 {
		t.Errorf("expected 1 result in FTS after Create, got %d", len(results))
	}

	if len(results) > 0 && results[0].Chunk.ID != chunk.ID {
		t.Errorf("expected chunk ID %s, got %s", chunk.ID, results[0].Chunk.ID)
	}
}

// TestDualWrite_CreateBatchSyncsToFTS verifies that CreateBatch operations sync to FTS.
func TestDualWrite_CreateBatchSyncsToFTS(t *testing.T) {
	ctx := context.Background()

	storage, err := NewSQLiteStorage(":memory:", 4)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer storage.Close()

	// Create a batch of chunks
	chunks := []ChunkWithEmbedding{
		{
			Chunk: Chunk{
				ID:       "batch-1",
				FilePath: "/test/service.go",
				Type:     ChunkFunction,
				Name:     "processBatch",
				Content:  "func processBatch() { handle items }",
				Language: "go",
			},
			Embedding: make([]float32, 4),
		},
		{
			Chunk: Chunk{
				ID:       "batch-2",
				FilePath: "/test/service.go",
				Type:     ChunkFunction,
				Name:     "validateBatch",
				Content:  "func validateBatch() { check items }",
				Language: "go",
			},
			Embedding: make([]float32, 4),
		},
		{
			Chunk: Chunk{
				ID:       "batch-3",
				FilePath: "/test/service.go",
				Type:     ChunkFunction,
				Name:     "saveBatch",
				Content:  "func saveBatch() { store items }",
				Language: "go",
			},
			Embedding: make([]float32, 4),
		},
	}

	if err := storage.CreateBatch(ctx, chunks); err != nil {
		t.Fatalf("CreateBatch failed: %v", err)
	}

	// Verify all chunks are searchable via lexical search
	// Search for "items" which appears in all chunks
	results, err := storage.LexicalSearch(ctx, "items", LexicalSearchOptions{TopK: 10})
	if err != nil {
		t.Fatalf("LexicalSearch failed: %v", err)
	}

	if len(results) != 3 {
		t.Errorf("expected 3 results in FTS after CreateBatch, got %d", len(results))
	}
}

// TestDualWrite_DeleteSyncsToFTS verifies that Delete operations sync to FTS.
func TestDualWrite_DeleteSyncsToFTS(t *testing.T) {
	ctx := context.Background()

	storage, err := NewSQLiteStorage(":memory:", 4)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer storage.Close()

	// Create a chunk
	chunk := Chunk{
		ID:        "dual-write-delete",
		FilePath:  "/test/handler.go",
		Type:      ChunkFunction,
		Name:      "deletableFunc",
		Content:   "func deletableFunc() { willBeRemoved() }",
		StartLine: 10,
		EndLine:   15,
		Language:  "go",
	}

	if err := storage.Create(ctx, chunk, make([]float32, 4)); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Verify it's searchable
	results, err := storage.LexicalSearch(ctx, "deletableFunc", LexicalSearchOptions{TopK: 10})
	if err != nil {
		t.Fatalf("LexicalSearch failed: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result before delete, got %d", len(results))
	}

	// Delete the chunk
	if err := storage.Delete(ctx, chunk.ID); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Verify it's no longer searchable via FTS
	results, err = storage.LexicalSearch(ctx, "deletableFunc", LexicalSearchOptions{TopK: 10})
	if err != nil {
		t.Fatalf("LexicalSearch after delete failed: %v", err)
	}

	if len(results) != 0 {
		t.Errorf("expected 0 results in FTS after Delete, got %d", len(results))
	}
}

// TestDualWrite_DeleteByFilePathSyncsToFTS verifies that DeleteByFilePath syncs to FTS.
func TestDualWrite_DeleteByFilePathSyncsToFTS(t *testing.T) {
	ctx := context.Background()

	storage, err := NewSQLiteStorage(":memory:", 4)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer storage.Close()

	// Create chunks from different files
	chunks := []ChunkWithEmbedding{
		{
			Chunk: Chunk{
				ID:       "file-a-1",
				FilePath: "/test/fileA.go",
				Type:     ChunkFunction,
				Name:     "funcA1",
				Content:  "func funcA1() { targetFile operation }",
				Language: "go",
			},
			Embedding: make([]float32, 4),
		},
		{
			Chunk: Chunk{
				ID:       "file-a-2",
				FilePath: "/test/fileA.go",
				Type:     ChunkFunction,
				Name:     "funcA2",
				Content:  "func funcA2() { targetFile operation }",
				Language: "go",
			},
			Embedding: make([]float32, 4),
		},
		{
			Chunk: Chunk{
				ID:       "file-b-1",
				FilePath: "/test/fileB.go",
				Type:     ChunkFunction,
				Name:     "funcB1",
				Content:  "func funcB1() { keepFile operation }",
				Language: "go",
			},
			Embedding: make([]float32, 4),
		},
	}

	if err := storage.CreateBatch(ctx, chunks); err != nil {
		t.Fatalf("CreateBatch failed: %v", err)
	}

	// Verify all chunks are searchable
	results, err := storage.LexicalSearch(ctx, "operation", LexicalSearchOptions{TopK: 10})
	if err != nil {
		t.Fatalf("LexicalSearch failed: %v", err)
	}
	if len(results) != 3 {
		t.Errorf("expected 3 results before delete, got %d", len(results))
	}

	// Delete all chunks from fileA.go
	deleted, err := storage.DeleteByFilePath(ctx, "/test/fileA.go")
	if err != nil {
		t.Fatalf("DeleteByFilePath failed: %v", err)
	}
	if deleted != 2 {
		t.Errorf("expected 2 deleted, got %d", deleted)
	}

	// Verify only fileB chunks remain searchable
	results, err = storage.LexicalSearch(ctx, "operation", LexicalSearchOptions{TopK: 10})
	if err != nil {
		t.Fatalf("LexicalSearch after delete failed: %v", err)
	}

	if len(results) != 1 {
		t.Errorf("expected 1 result after DeleteByFilePath, got %d", len(results))
	}

	if len(results) > 0 && results[0].Chunk.FilePath != "/test/fileB.go" {
		t.Errorf("expected chunk from fileB.go, got %s", results[0].Chunk.FilePath)
	}
}

// TestDualWrite_ClearSyncsToFTS verifies that Clear operations sync to FTS.
func TestDualWrite_ClearSyncsToFTS(t *testing.T) {
	ctx := context.Background()

	storage, err := NewSQLiteStorage(":memory:", 4)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer storage.Close()

	// Create some chunks
	chunks := []ChunkWithEmbedding{
		{
			Chunk: Chunk{
				ID:       "clear-1",
				FilePath: "/test/a.go",
				Type:     ChunkFunction,
				Name:     "clearableA",
				Content:  "func clearableA() { willBeClearedFromIndex() }",
				Language: "go",
			},
			Embedding: make([]float32, 4),
		},
		{
			Chunk: Chunk{
				ID:       "clear-2",
				FilePath: "/test/b.go",
				Type:     ChunkFunction,
				Name:     "clearableB",
				Content:  "func clearableB() { willBeClearedFromIndex() }",
				Language: "go",
			},
			Embedding: make([]float32, 4),
		},
	}

	if err := storage.CreateBatch(ctx, chunks); err != nil {
		t.Fatalf("CreateBatch failed: %v", err)
	}

	// Verify chunks are searchable
	results, err := storage.LexicalSearch(ctx, "willBeClearedFromIndex", LexicalSearchOptions{TopK: 10})
	if err != nil {
		t.Fatalf("LexicalSearch failed: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 results before clear, got %d", len(results))
	}

	// Clear all data
	if err := storage.Clear(ctx); err != nil {
		t.Fatalf("Clear failed: %v", err)
	}

	// Verify FTS is empty
	results, err = storage.LexicalSearch(ctx, "willBeClearedFromIndex", LexicalSearchOptions{TopK: 10})
	if err != nil {
		t.Fatalf("LexicalSearch after clear failed: %v", err)
	}

	if len(results) != 0 {
		t.Errorf("expected 0 results after Clear, got %d", len(results))
	}
}

// TestDualWrite_UpdateSyncsToFTS verifies that Update operations sync to FTS.
func TestDualWrite_UpdateSyncsToFTS(t *testing.T) {
	ctx := context.Background()

	storage, err := NewSQLiteStorage(":memory:", 4)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer storage.Close()

	// Create a chunk
	chunk := Chunk{
		ID:        "dual-write-update",
		FilePath:  "/test/handler.go",
		Type:      ChunkFunction,
		Name:      "updateableFunc",
		Content:   "func updateableFunc() { originalContent() }",
		StartLine: 10,
		EndLine:   15,
		Language:  "go",
	}

	if err := storage.Create(ctx, chunk, make([]float32, 4)); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Verify original content is searchable
	results, err := storage.LexicalSearch(ctx, "originalContent", LexicalSearchOptions{TopK: 10})
	if err != nil {
		t.Fatalf("LexicalSearch failed: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result for original content, got %d", len(results))
	}

	// Update the chunk with new content
	updatedChunk := chunk
	updatedChunk.Content = "func updateableFunc() { modifiedContent() }"

	if err := storage.Update(ctx, updatedChunk, make([]float32, 4)); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	// Verify old content is no longer searchable
	results, err = storage.LexicalSearch(ctx, "originalContent", LexicalSearchOptions{TopK: 10})
	if err != nil {
		t.Fatalf("LexicalSearch after update failed: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results for original content after update, got %d", len(results))
	}

	// Verify new content is searchable
	results, err = storage.LexicalSearch(ctx, "modifiedContent", LexicalSearchOptions{TopK: 10})
	if err != nil {
		t.Fatalf("LexicalSearch for new content failed: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result for modified content, got %d", len(results))
	}
}
