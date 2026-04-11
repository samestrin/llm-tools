package semantic

import (
	"context"
	"testing"
)

func TestRefStorage_StoreAndGetRefs(t *testing.T) {
	storage, cleanup := createTestSQLiteStorage(t)
	defer cleanup()

	ctx := context.Background()

	// First store some chunks so foreign keys work
	chunks := []ChunkWithEmbedding{
		{Chunk: Chunk{ID: "c1", FilePath: "a.go", Type: ChunkFunction, Name: "A", Content: "func A() { B() }", StartLine: 1, EndLine: 1, Language: "go"}, Embedding: []float32{0.1}},
		{Chunk: Chunk{ID: "c2", FilePath: "a.go", Type: ChunkFunction, Name: "B", Content: "func B() {}", StartLine: 3, EndLine: 3, Language: "go"}, Embedding: []float32{0.2}},
	}
	if err := storage.CreateBatch(ctx, chunks); err != nil {
		t.Fatalf("CreateBatch: %v", err)
	}

	// Check if storage supports refs
	rs, ok := interface{}(storage).(RefStorage)
	if !ok {
		t.Fatal("SQLiteStorage does not implement RefStorage")
	}

	// Store refs
	refs := []ChunkRef{
		{ChunkID: "c1", RefType: RefCalls, RefName: "B"},
		{ChunkID: "c1", RefType: RefImports, RefName: "fmt"},
	}
	if err := rs.StoreRefs(ctx, refs); err != nil {
		t.Fatalf("StoreRefs: %v", err)
	}

	// Get refs FROM c1
	got, err := rs.GetRefs(ctx, "c1")
	if err != nil {
		t.Fatalf("GetRefs: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("Expected 2 refs, got %d", len(got))
	}

	// Get callers TO B (nothing yet since not resolved)
	callers, err := rs.GetCallers(ctx, "c2")
	if err != nil {
		t.Fatalf("GetCallers: %v", err)
	}
	// Before resolution, callers won't find c2 by target ID
	_ = callers

	// Resolve refs — should link "B" to chunk "c2"
	if err := rs.ResolveRefs(ctx); err != nil {
		t.Fatalf("ResolveRefs: %v", err)
	}

	// Now get callers TO c2 — should find c1
	callers, err = rs.GetCallers(ctx, "c2")
	if err != nil {
		t.Fatalf("GetCallers after resolve: %v", err)
	}
	if len(callers) != 1 {
		t.Fatalf("Expected 1 caller after resolve, got %d", len(callers))
	}
	if callers[0].ChunkID != "c1" {
		t.Errorf("Expected caller c1, got %s", callers[0].ChunkID)
	}
}

func TestRefStorage_DeleteRefsByChunk(t *testing.T) {
	storage, cleanup := createTestSQLiteStorage(t)
	defer cleanup()

	ctx := context.Background()

	chunks := []ChunkWithEmbedding{
		{Chunk: Chunk{ID: "c1", FilePath: "a.go", Type: ChunkFunction, Name: "A", Content: "a", StartLine: 1, EndLine: 1, Language: "go"}, Embedding: []float32{0.1}},
	}
	storage.CreateBatch(ctx, chunks)

	rs := interface{}(storage).(RefStorage)

	refs := []ChunkRef{
		{ChunkID: "c1", RefType: RefCalls, RefName: "B"},
		{ChunkID: "c1", RefType: RefCalls, RefName: "C"},
	}
	rs.StoreRefs(ctx, refs)

	// Delete
	if err := rs.DeleteRefsByChunk(ctx, "c1"); err != nil {
		t.Fatalf("DeleteRefsByChunk: %v", err)
	}

	got, _ := rs.GetRefs(ctx, "c1")
	if len(got) != 0 {
		t.Errorf("Expected 0 refs after delete, got %d", len(got))
	}
}

func TestRefStorage_EmptyRefs(t *testing.T) {
	storage, cleanup := createTestSQLiteStorage(t)
	defer cleanup()

	ctx := context.Background()
	rs := interface{}(storage).(RefStorage)

	// Store empty — should be no-op
	if err := rs.StoreRefs(ctx, nil); err != nil {
		t.Fatalf("StoreRefs nil: %v", err)
	}

	// Get refs for nonexistent chunk — should return empty
	got, err := rs.GetRefs(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("GetRefs nonexistent: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("Expected 0 refs, got %d", len(got))
	}
}
