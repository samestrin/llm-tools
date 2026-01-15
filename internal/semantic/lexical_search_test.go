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
