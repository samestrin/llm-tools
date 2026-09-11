package semantic

import (
	"context"
	"testing"
)

// seedCallGraph stores a small resolved call graph: Caller (a.go:10) calls
// Method (b.go:5) and the out-of-index fmt.Println.
func seedCallGraph(t *testing.T, storage *SQLiteStorage) RefStorage {
	t.Helper()
	ctx := context.Background()

	chunks := []ChunkWithEmbedding{
		{Chunk: Chunk{ID: "caller", FilePath: "a.go", Type: ChunkFunction, Name: "Caller", Content: "x", StartLine: 10, EndLine: 14, Language: "go"}, Embedding: []float32{0.1}},
		{Chunk: Chunk{ID: "target", FilePath: "b.go", Type: ChunkMethod, Name: "Method", Content: "x", StartLine: 5, EndLine: 7, Language: "go"}, Embedding: []float32{0.2}},
		{Chunk: Chunk{ID: "lonely", FilePath: "c.go", Type: ChunkFunction, Name: "Lonely", Content: "x", StartLine: 1, EndLine: 2, Language: "go"}, Embedding: []float32{0.3}},
	}
	if err := storage.CreateBatch(ctx, chunks); err != nil {
		t.Fatalf("CreateBatch: %v", err)
	}

	rs := RefStorage(storage)
	if err := rs.StoreRefs(ctx, []ChunkRef{
		{ChunkID: "caller", RefType: RefCalls, RefName: "recv.Method"},
		{ChunkID: "caller", RefType: RefCalls, RefName: "fmt.Println"},
	}); err != nil {
		t.Fatalf("StoreRefs: %v", err)
	}
	if err := rs.ResolveRefs(ctx); err != nil {
		t.Fatalf("ResolveRefs: %v", err)
	}
	return rs
}

func TestGetCallersByName(t *testing.T) {
	storage, cleanup := createTestSQLiteStorage(t)
	defer cleanup()

	rs := seedCallGraph(t, storage)
	ctx := context.Background()

	edges, err := rs.GetCallersByName(ctx, "Method")
	if err != nil {
		t.Fatalf("GetCallersByName: %v", err)
	}
	if len(edges) != 1 {
		t.Fatalf("GetCallersByName(Method) returned %d edges, want 1: %+v", len(edges), edges)
	}

	// The edge must carry where the *caller* lives, so a command can print it
	// without a second lookup.
	got := edges[0]
	if got.ChunkID != "caller" {
		t.Errorf("caller chunk = %q, want %q", got.ChunkID, "caller")
	}
	if got.FilePath != "a.go" || got.StartLine != 10 {
		t.Errorf("caller location = %s:%d, want a.go:10", got.FilePath, got.StartLine)
	}
	if got.Name != "Caller" {
		t.Errorf("caller name = %q, want %q", got.Name, "Caller")
	}
	if got.RefType != RefCalls {
		t.Errorf("ref type = %q, want %q", got.RefType, RefCalls)
	}
}

func TestGetCallersByName_NoCallersAndUnknownSymbol(t *testing.T) {
	storage, cleanup := createTestSQLiteStorage(t)
	defer cleanup()

	rs := seedCallGraph(t, storage)
	ctx := context.Background()

	// A symbol that exists but nobody calls, and a symbol that does not exist
	// at all, both mean "no callers" and neither is an error.
	for _, name := range []string{"Lonely", "NoSuchSymbol"} {
		edges, err := rs.GetCallersByName(ctx, name)
		if err != nil {
			t.Fatalf("GetCallersByName(%s): %v", name, err)
		}
		if len(edges) != 0 {
			t.Errorf("GetCallersByName(%s) = %+v, want no edges", name, edges)
		}
	}
}

func TestGetRefsByName(t *testing.T) {
	storage, cleanup := createTestSQLiteStorage(t)
	defer cleanup()

	rs := seedCallGraph(t, storage)
	ctx := context.Background()

	edges, err := rs.GetRefsByName(ctx, "Caller")
	if err != nil {
		t.Fatalf("GetRefsByName: %v", err)
	}
	if len(edges) != 2 {
		t.Fatalf("GetRefsByName(Caller) returned %d edges, want 2: %+v", len(edges), edges)
	}

	byName := map[string]RefEdge{}
	for _, e := range edges {
		byName[e.RefName] = e
	}

	// A reference into the index carries the callee's location.
	resolved, ok := byName["recv.Method"]
	if !ok {
		t.Fatalf("missing edge for recv.Method: %+v", edges)
	}
	if !resolved.Resolved() {
		t.Errorf("recv.Method reported unresolved, want resolved")
	}
	if resolved.FilePath != "b.go" || resolved.StartLine != 5 {
		t.Errorf("callee location = %s:%d, want b.go:5", resolved.FilePath, resolved.StartLine)
	}

	// A reference outside the index is still listed, but carries no location
	// so a command can mark it external rather than drop it.
	external, ok := byName["fmt.Println"]
	if !ok {
		t.Fatalf("missing edge for fmt.Println: %+v", edges)
	}
	if external.Resolved() {
		t.Errorf("fmt.Println reported resolved, want unresolved")
	}
	if external.FilePath != "" {
		t.Errorf("external edge location = %q, want empty", external.FilePath)
	}
}

func TestGetRefsByName_UnknownSymbol(t *testing.T) {
	storage, cleanup := createTestSQLiteStorage(t)
	defer cleanup()

	rs := seedCallGraph(t, storage)

	edges, err := rs.GetRefsByName(context.Background(), "NoSuchSymbol")
	if err != nil {
		t.Fatalf("GetRefsByName: %v", err)
	}
	if len(edges) != 0 {
		t.Errorf("GetRefsByName(NoSuchSymbol) = %+v, want no edges", edges)
	}
}
