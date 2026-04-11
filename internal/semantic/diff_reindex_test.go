package semantic

import (
	"context"
	"crypto/sha256"
	"fmt"
	"testing"
)

func TestComputeContentHash(t *testing.T) {
	chunk := Chunk{Content: "func hello() {}"}
	hash := chunk.ComputeContentHash()

	if hash == "" {
		t.Fatal("ComputeContentHash returned empty string")
	}

	// Should be deterministic
	hash2 := chunk.ComputeContentHash()
	if hash != hash2 {
		t.Errorf("ComputeContentHash not deterministic: %q != %q", hash, hash2)
	}

	// Different content = different hash
	chunk2 := Chunk{Content: "func world() {}"}
	hash3 := chunk2.ComputeContentHash()
	if hash == hash3 {
		t.Errorf("Different content should produce different hash")
	}

	// Verify format: should be hex-encoded SHA256
	expected := fmt.Sprintf("%x", sha256.Sum256([]byte("func hello() {}")))
	if hash != expected {
		t.Errorf("Hash format mismatch: got %q, want %q", hash, expected)
	}
}

func TestComputeContentHash_EmptyContent(t *testing.T) {
	chunk := Chunk{Content: ""}
	hash := chunk.ComputeContentHash()
	if hash == "" {
		t.Fatal("ComputeContentHash should return hash even for empty content")
	}
}

// TestGetChunksByFilePath_SQLite tests the storage method for retrieving chunk summaries.
func TestGetChunksByFilePath_SQLite(t *testing.T) {
	storage, cleanup := createTestSQLiteStorage(t)
	defer cleanup()

	ctx := context.Background()

	// Store chunks with content hashes
	chunks := []ChunkWithEmbedding{
		{
			Chunk: Chunk{
				ID: "chunk1", FilePath: "file.go", Type: ChunkFunction,
				Name: "Func1", Content: "func Func1() {}", ContentHash: "hash1",
				StartLine: 1, EndLine: 3, Language: "go",
			},
			Embedding: []float32{0.1, 0.2, 0.3},
		},
		{
			Chunk: Chunk{
				ID: "chunk2", FilePath: "file.go", Type: ChunkFunction,
				Name: "Func2", Content: "func Func2() {}", ContentHash: "hash2",
				StartLine: 5, EndLine: 7, Language: "go",
			},
			Embedding: []float32{0.4, 0.5, 0.6},
		},
		{
			Chunk: Chunk{
				ID: "chunk3", FilePath: "other.go", Type: ChunkFunction,
				Name: "Other", Content: "func Other() {}", ContentHash: "hash3",
				StartLine: 1, EndLine: 3, Language: "go",
			},
			Embedding: []float32{0.7, 0.8, 0.9},
		},
	}

	if err := storage.CreateBatch(ctx, chunks); err != nil {
		t.Fatalf("CreateBatch failed: %v", err)
	}

	// Get chunks for file.go — should return 2
	summaries, err := storage.GetChunksByFilePath(ctx, "file.go")
	if err != nil {
		t.Fatalf("GetChunksByFilePath failed: %v", err)
	}
	if len(summaries) != 2 {
		t.Fatalf("Expected 2 summaries, got %d", len(summaries))
	}

	// Verify fields
	found := map[string]bool{}
	for _, s := range summaries {
		found[s.ID] = true
		if s.ID == "chunk1" {
			if s.ContentHash != "hash1" {
				t.Errorf("Expected content_hash 'hash1', got %q", s.ContentHash)
			}
			if s.Name != "Func1" {
				t.Errorf("Expected name 'Func1', got %q", s.Name)
			}
		}
	}
	if !found["chunk1"] || !found["chunk2"] {
		t.Errorf("Missing expected chunk IDs: %v", found)
	}

	// Get chunks for non-existent file — should return empty
	summaries, err = storage.GetChunksByFilePath(ctx, "nonexistent.go")
	if err != nil {
		t.Fatalf("GetChunksByFilePath for nonexistent file failed: %v", err)
	}
	if len(summaries) != 0 {
		t.Fatalf("Expected 0 summaries for nonexistent file, got %d", len(summaries))
	}
}

func TestReadEmbeddings_SQLite(t *testing.T) {
	storage, cleanup := createTestSQLiteStorage(t)
	defer cleanup()

	ctx := context.Background()

	chunks := []ChunkWithEmbedding{
		{
			Chunk: Chunk{
				ID: "c1", FilePath: "f.go", Type: ChunkFunction,
				Name: "A", Content: "a", StartLine: 1, EndLine: 1, Language: "go",
			},
			Embedding: []float32{1.0, 2.0, 3.0},
		},
		{
			Chunk: Chunk{
				ID: "c2", FilePath: "f.go", Type: ChunkFunction,
				Name: "B", Content: "b", StartLine: 2, EndLine: 2, Language: "go",
			},
			Embedding: []float32{4.0, 5.0, 6.0},
		},
	}

	if err := storage.CreateBatch(ctx, chunks); err != nil {
		t.Fatalf("CreateBatch failed: %v", err)
	}

	// Read embeddings for both IDs
	embeddings, err := storage.ReadEmbeddings(ctx, []string{"c1", "c2"})
	if err != nil {
		t.Fatalf("ReadEmbeddings failed: %v", err)
	}
	if len(embeddings) != 2 {
		t.Fatalf("Expected 2 embeddings, got %d", len(embeddings))
	}

	// Verify values
	if len(embeddings["c1"]) != 3 || embeddings["c1"][0] != 1.0 {
		t.Errorf("Unexpected embedding for c1: %v", embeddings["c1"])
	}
	if len(embeddings["c2"]) != 3 || embeddings["c2"][0] != 4.0 {
		t.Errorf("Unexpected embedding for c2: %v", embeddings["c2"])
	}

	// Read embedding for nonexistent ID — should return empty map
	embeddings, err = storage.ReadEmbeddings(ctx, []string{"nonexistent"})
	if err != nil {
		t.Fatalf("ReadEmbeddings for nonexistent ID failed: %v", err)
	}
	if len(embeddings) != 0 {
		t.Fatalf("Expected 0 embeddings for nonexistent ID, got %d", len(embeddings))
	}

	// Empty IDs — should return empty map
	embeddings, err = storage.ReadEmbeddings(ctx, []string{})
	if err != nil {
		t.Fatalf("ReadEmbeddings for empty IDs failed: %v", err)
	}
	if len(embeddings) != 0 {
		t.Fatalf("Expected 0 embeddings for empty IDs, got %d", len(embeddings))
	}
}

func TestContentHashMigration_SQLite(t *testing.T) {
	storage, cleanup := createTestSQLiteStorage(t)
	defer cleanup()

	ctx := context.Background()

	// Store a chunk without content_hash (simulating legacy data)
	chunks := []ChunkWithEmbedding{
		{
			Chunk: Chunk{
				ID: "legacy1", FilePath: "old.go", Type: ChunkFunction,
				Name: "Legacy", Content: "func Legacy() {}",
				StartLine: 1, EndLine: 1, Language: "go",
			},
			Embedding: []float32{0.1, 0.2},
		},
	}

	if err := storage.CreateBatch(ctx, chunks); err != nil {
		t.Fatalf("CreateBatch failed: %v", err)
	}

	// GetChunksByFilePath should return it with empty ContentHash
	summaries, err := storage.GetChunksByFilePath(ctx, "old.go")
	if err != nil {
		t.Fatalf("GetChunksByFilePath failed: %v", err)
	}
	if len(summaries) != 1 {
		t.Fatalf("Expected 1 summary, got %d", len(summaries))
	}
	// Legacy row has no content_hash — should be empty string
	if summaries[0].ContentHash != "" {
		t.Errorf("Expected empty content_hash for legacy row, got %q", summaries[0].ContentHash)
	}
}

// TestDiffChunks tests the core diff logic.
func TestDiffChunks(t *testing.T) {
	tests := []struct {
		name         string
		oldSummaries []ChunkSummary
		newChunks    []Chunk
		wantReuse    int // chunks with embeddings reused
		wantEmbed    int // chunks needing new embeddings
		wantDelete   int // old chunks to delete
	}{
		{
			name:         "no old chunks — all new",
			oldSummaries: nil,
			newChunks: []Chunk{
				{ID: "a", Content: "func A() {}", ContentHash: hashContent("func A() {}")},
			},
			wantReuse: 0, wantEmbed: 1, wantDelete: 0,
		},
		{
			name: "identical content — all reused",
			oldSummaries: []ChunkSummary{
				{ID: "a", ContentHash: hashContent("func A() {}")},
			},
			newChunks: []Chunk{
				{ID: "a", Content: "func A() {}", ContentHash: hashContent("func A() {}")},
			},
			wantReuse: 1, wantEmbed: 0, wantDelete: 0,
		},
		{
			name: "content changed — re-embed and delete old",
			oldSummaries: []ChunkSummary{
				{ID: "a", ContentHash: hashContent("func A() {}")},
			},
			newChunks: []Chunk{
				{ID: "a", Content: "func A() { return }", ContentHash: hashContent("func A() { return }")},
			},
			wantReuse: 0, wantEmbed: 1, wantDelete: 1,
		},
		{
			name: "function moved — same content, different ID",
			oldSummaries: []ChunkSummary{
				{ID: "old-a", ContentHash: hashContent("func A() {}")},
			},
			newChunks: []Chunk{
				{ID: "new-a", Content: "func A() {}", ContentHash: hashContent("func A() {}")},
			},
			wantReuse: 1, wantEmbed: 0, wantDelete: 0,
		},
		{
			name: "function removed",
			oldSummaries: []ChunkSummary{
				{ID: "a", ContentHash: hashContent("func A() {}")},
				{ID: "b", ContentHash: hashContent("func B() {}")},
			},
			newChunks: []Chunk{
				{ID: "a", Content: "func A() {}", ContentHash: hashContent("func A() {}")},
			},
			wantReuse: 1, wantEmbed: 0, wantDelete: 1,
		},
		{
			name: "function added",
			oldSummaries: []ChunkSummary{
				{ID: "a", ContentHash: hashContent("func A() {}")},
			},
			newChunks: []Chunk{
				{ID: "a", Content: "func A() {}", ContentHash: hashContent("func A() {}")},
				{ID: "b", Content: "func B() {}", ContentHash: hashContent("func B() {}")},
			},
			wantReuse: 1, wantEmbed: 1, wantDelete: 0,
		},
		{
			name: "legacy NULL content_hash — treat as needs re-embed and delete old",
			oldSummaries: []ChunkSummary{
				{ID: "a", ContentHash: ""},
			},
			newChunks: []Chunk{
				{ID: "a", Content: "func A() {}", ContentHash: hashContent("func A() {}")},
			},
			wantReuse: 0, wantEmbed: 1, wantDelete: 1,
		},
		{
			name: "complete rewrite",
			oldSummaries: []ChunkSummary{
				{ID: "old1", ContentHash: hashContent("old code 1")},
				{ID: "old2", ContentHash: hashContent("old code 2")},
			},
			newChunks: []Chunk{
				{ID: "new1", Content: "new code 1", ContentHash: hashContent("new code 1")},
				{ID: "new2", Content: "new code 2", ContentHash: hashContent("new code 2")},
			},
			wantReuse: 0, wantEmbed: 2, wantDelete: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DiffChunks(tt.oldSummaries, tt.newChunks)
			if len(result.Reuse) != tt.wantReuse {
				t.Errorf("Reuse: got %d, want %d", len(result.Reuse), tt.wantReuse)
			}
			if len(result.NeedEmbed) != tt.wantEmbed {
				t.Errorf("NeedEmbed: got %d, want %d", len(result.NeedEmbed), tt.wantEmbed)
			}
			if len(result.Delete) != tt.wantDelete {
				t.Errorf("Delete: got %d, want %d", len(result.Delete), tt.wantDelete)
			}
		})
	}
}

func hashContent(content string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(content)))
}

// createTestSQLiteStorage creates an in-memory SQLite storage for testing.
func createTestSQLiteStorage(t *testing.T) (*SQLiteStorage, func()) {
	t.Helper()
	storage, err := NewSQLiteStorage(":memory:", 0)
	if err != nil {
		t.Fatalf("Failed to create test storage: %v", err)
	}
	return storage, func() { storage.Close() }
}
