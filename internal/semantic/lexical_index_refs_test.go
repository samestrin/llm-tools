package semantic

import (
	"context"
	"testing"
)

func seedLexicalRefs(t *testing.T) (*LexicalIndex, RefStorage) {
	t.Helper()

	idx, err := NewLexicalIndex(":memory:", 4)
	if err != nil {
		t.Fatalf("NewLexicalIndex: %v", err)
	}
	t.Cleanup(func() { idx.Close() })

	ctx := context.Background()
	if err := idx.IndexBatch(ctx, []Chunk{
		{ID: "caller", FilePath: "a.go", Type: ChunkFunction, Name: "Caller", Content: "x", StartLine: 10, EndLine: 12, Language: "go"},
		{ID: "target", FilePath: "b.go", Type: ChunkFunction, Name: "Target", Content: "x", StartLine: 5, EndLine: 6, Language: "go"},
	}); err != nil {
		t.Fatalf("IndexBatch: %v", err)
	}

	rs := RefStorage(idx)
	if err := rs.StoreRefs(ctx, []ChunkRef{
		{ChunkID: "caller", RefType: RefCalls, RefName: "pkg.Target"},
		{ChunkID: "caller", RefType: RefImports, RefName: "fmt"},
	}); err != nil {
		t.Fatalf("StoreRefs: %v", err)
	}
	if err := rs.ResolveRefs(ctx); err != nil {
		t.Fatalf("ResolveRefs: %v", err)
	}
	return idx, rs
}

func TestLexicalIndex_GetRefsByName(t *testing.T) {
	_, rs := seedLexicalRefs(t)

	edges, err := rs.GetRefsByName(context.Background(), "Caller")
	if err != nil {
		t.Fatalf("GetRefsByName: %v", err)
	}
	if len(edges) != 2 {
		t.Fatalf("GetRefsByName(Caller) = %d edges, want 2: %+v", len(edges), edges)
	}

	byName := map[string]RefEdge{}
	for _, e := range edges {
		byName[e.RefName] = e
	}

	target, ok := byName["pkg.Target"]
	if !ok {
		t.Fatalf("missing edge for pkg.Target: %+v", edges)
	}
	if !target.Resolved() || target.FilePath != "b.go" || target.StartLine != 5 {
		t.Errorf("pkg.Target edge = %+v, want resolved at b.go:5", target)
	}

	// An import points outside the index and stays unresolved.
	imp, ok := byName["fmt"]
	if !ok {
		t.Fatalf("missing edge for fmt: %+v", edges)
	}
	if imp.Resolved() {
		t.Errorf("fmt should be unresolved, got %+v", imp)
	}
}

func TestLexicalIndex_DeleteRefsByChunk(t *testing.T) {
	_, rs := seedLexicalRefs(t)
	ctx := context.Background()

	if err := rs.DeleteRefsByChunk(ctx, "caller"); err != nil {
		t.Fatalf("DeleteRefsByChunk: %v", err)
	}

	refs, err := rs.GetRefs(ctx, "caller")
	if err != nil {
		t.Fatalf("GetRefs: %v", err)
	}
	if len(refs) != 0 {
		t.Errorf("GetRefs after delete = %+v, want none", refs)
	}
}

func TestLexicalIndex_RefsRejectedAfterClose(t *testing.T) {
	idx, rs := seedLexicalRefs(t)
	ctx := context.Background()

	if err := idx.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	if err := rs.StoreRefs(ctx, []ChunkRef{{ChunkID: "caller", RefType: RefCalls, RefName: "X"}}); err == nil {
		t.Error("StoreRefs on a closed index should fail")
	}
	if _, err := rs.GetCallersByName(ctx, "Target"); err == nil {
		t.Error("GetCallersByName on a closed index should fail")
	}
	if err := rs.ResolveRefs(ctx); err == nil {
		t.Error("ResolveRefs on a closed index should fail")
	}
}
