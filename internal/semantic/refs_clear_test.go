package semantic

import (
	"context"
	"testing"
)

// TestSQLiteStorage_ClearRemovesRefs proves a wiped index keeps no edges.
// Clearing is what a forced rebuild does first, so leaving references behind
// would mean "from scratch" was not really from scratch.
func TestSQLiteStorage_ClearRemovesRefs(t *testing.T) {
	storage, cleanup := createTestSQLiteStorage(t)
	defer cleanup()

	ctx := context.Background()

	if err := storage.CreateBatch(ctx, []ChunkWithEmbedding{
		{Chunk: Chunk{ID: "caller", FilePath: "a.go", Type: ChunkFunction, Name: "Caller", Content: "x", StartLine: 1, EndLine: 2, Language: "go"}, Embedding: []float32{0.1}},
		{Chunk: Chunk{ID: "target", FilePath: "b.go", Type: ChunkFunction, Name: "Target", Content: "x", StartLine: 1, EndLine: 2, Language: "go"}, Embedding: []float32{0.2}},
	}); err != nil {
		t.Fatalf("CreateBatch: %v", err)
	}

	rs := RefStorage(storage)
	if err := rs.StoreRefs(ctx, []ChunkRef{
		{ChunkID: "caller", RefType: RefCalls, RefName: "Target"},
	}); err != nil {
		t.Fatalf("StoreRefs: %v", err)
	}
	if err := rs.ResolveRefs(ctx); err != nil {
		t.Fatalf("ResolveRefs: %v", err)
	}

	if err := storage.Clear(ctx); err != nil {
		t.Fatalf("Clear: %v", err)
	}

	refs, err := rs.GetRefs(ctx, "caller")
	if err != nil {
		t.Fatalf("GetRefs after Clear: %v", err)
	}
	if len(refs) != 0 {
		t.Errorf("Clear left %d references behind: %+v", len(refs), refs)
	}
}

// TestLexicalIndex_ClearRemovesRefs is the same guarantee for the sidecar index
// a vector backend delegates to.
func TestLexicalIndex_ClearRemovesRefs(t *testing.T) {
	idx, err := NewLexicalIndex(":memory:", 4)
	if err != nil {
		t.Fatalf("NewLexicalIndex: %v", err)
	}
	defer idx.Close()

	ctx := context.Background()

	if err := idx.IndexBatch(ctx, []Chunk{
		{ID: "caller", FilePath: "a.go", Type: ChunkFunction, Name: "Caller", Content: "x", StartLine: 1, EndLine: 2, Language: "go"},
		{ID: "target", FilePath: "b.go", Type: ChunkFunction, Name: "Target", Content: "x", StartLine: 1, EndLine: 2, Language: "go"},
	}); err != nil {
		t.Fatalf("IndexBatch: %v", err)
	}

	rs := RefStorage(idx)
	if err := rs.StoreRefs(ctx, []ChunkRef{
		{ChunkID: "caller", RefType: RefCalls, RefName: "Target"},
	}); err != nil {
		t.Fatalf("StoreRefs: %v", err)
	}

	if err := idx.Clear(ctx); err != nil {
		t.Fatalf("Clear: %v", err)
	}

	refs, err := rs.GetRefs(ctx, "caller")
	if err != nil {
		t.Fatalf("GetRefs after Clear: %v", err)
	}
	if len(refs) != 0 {
		t.Errorf("Clear left %d references behind: %+v", len(refs), refs)
	}
}

// TestIndexManager_ForceReindexKeepsGraphExact rebuilds an index from scratch
// twice and expects the same answer both times, with no edges accumulated from
// the discarded run.
func TestIndexManager_ForceReindexKeepsGraphExact(t *testing.T) {
	storage, mgr, dir := newRefsHarness(t)
	ctx := context.Background()

	writeGoFile(t, dir, "caller.go", `package main

func Run() {
	w := &Worker{}
	w.Process()
}
`)
	writeGoFile(t, dir, "worker.go", `package main

type Worker struct{}

func (w *Worker) Process() {}
`)

	rs := RefStorage(storage)

	for attempt := 1; attempt <= 2; attempt++ {
		if _, err := mgr.Index(ctx, dir, IndexOptions{Force: true}); err != nil {
			t.Fatalf("Index attempt %d: %v", attempt, err)
		}

		edges, err := rs.GetCallersByName(ctx, "Process")
		if err != nil {
			t.Fatalf("GetCallersByName attempt %d: %v", attempt, err)
		}
		if len(edges) != 1 {
			t.Fatalf("attempt %d: GetCallersByName(Process) = %d edges, want 1: %+v", attempt, len(edges), edges)
		}
	}

	// A rebuilt index must not retain rows from the run it replaced.
	var orphans int
	err := storage.db.QueryRow(`
		SELECT COUNT(*) FROM chunk_refs r
		WHERE NOT EXISTS (SELECT 1 FROM chunks c WHERE c.id = r.chunk_id)
	`).Scan(&orphans)
	if err != nil {
		t.Fatalf("count orphaned refs: %v", err)
	}
	if orphans != 0 {
		t.Errorf("rebuild left %d references whose source chunk no longer exists", orphans)
	}
}
