package semantic

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
)

// TestLexicalIndex_RefStorage proves the sidecar index carries the call graph,
// which is what lets a vector backend answer reference queries it cannot
// express itself.
func TestLexicalIndex_RefStorage(t *testing.T) {
	idx, err := NewLexicalIndex(":memory:", 4)
	if err != nil {
		t.Fatalf("NewLexicalIndex: %v", err)
	}
	defer idx.Close()

	ctx := context.Background()

	rs, ok := interface{}(idx).(RefStorage)
	if !ok {
		t.Fatal("LexicalIndex does not implement RefStorage")
	}

	if err := idx.IndexBatch(ctx, []Chunk{
		{ID: "caller", FilePath: "a.go", Type: ChunkFunction, Name: "Caller", Content: "x", StartLine: 10, EndLine: 12, Language: "go"},
		{ID: "target", FilePath: "b.go", Type: ChunkMethod, Name: "Process", Content: "x", StartLine: 5, EndLine: 6, Language: "go"},
		{ID: "dup-a", FilePath: "c.go", Type: ChunkFunction, Name: "Dup", Content: "x", StartLine: 1, EndLine: 2, Language: "go"},
		{ID: "dup-b", FilePath: "d.go", Type: ChunkFunction, Name: "Dup", Content: "x", StartLine: 1, EndLine: 2, Language: "go"},
	}); err != nil {
		t.Fatalf("IndexBatch: %v", err)
	}

	if err := rs.StoreRefs(ctx, []ChunkRef{
		{ChunkID: "caller", RefType: RefCalls, RefName: "w.Process"},
		{ChunkID: "caller", RefType: RefCalls, RefName: "Dup"},
	}); err != nil {
		t.Fatalf("StoreRefs: %v", err)
	}
	if err := rs.ResolveRefs(ctx); err != nil {
		t.Fatalf("ResolveRefs: %v", err)
	}

	edges, err := rs.GetCallersByName(ctx, "Process")
	if err != nil {
		t.Fatalf("GetCallersByName: %v", err)
	}
	if len(edges) != 1 {
		t.Fatalf("GetCallersByName(Process) = %d edges, want 1: %+v", len(edges), edges)
	}
	if edges[0].ChunkID != "caller" || edges[0].RefTargetID != "target" {
		t.Errorf("edge = %+v, want caller -> target", edges[0])
	}
	if edges[0].FilePath != "a.go" || edges[0].StartLine != 10 {
		t.Errorf("caller location = %s:%d, want a.go:10", edges[0].FilePath, edges[0].StartLine)
	}

	// The no-guessing rule has to hold on this backend too.
	for _, id := range []string{"dup-a", "dup-b"} {
		callers, err := rs.GetCallers(ctx, id)
		if err != nil {
			t.Fatalf("GetCallers(%s): %v", id, err)
		}
		if len(callers) != 0 {
			t.Errorf("GetCallers(%s) = %+v, want none for an ambiguous name", id, callers)
		}
	}
}

// TestLexicalIndex_UpgradesLegacyDatabase opens a sidecar written before the
// reference table existed. It must gain the table without error and without
// losing the chunks already stored in it.
func TestLexicalIndex_UpgradesLegacyDatabase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.db")

	// Recreate the sidecar an older build left behind: chunks, no references,
	// and no file_mtime column.
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open legacy db: %v", err)
	}
	if _, err := db.Exec(`
		CREATE TABLE chunks (
			rowid INTEGER PRIMARY KEY AUTOINCREMENT,
			id TEXT UNIQUE NOT NULL,
			file_path TEXT NOT NULL,
			type TEXT NOT NULL,
			name TEXT,
			signature TEXT,
			content TEXT,
			start_line INTEGER,
			end_line INTEGER,
			language TEXT
		);
		INSERT INTO chunks (id, file_path, type, name, content, start_line, end_line, language)
		VALUES ('kept', 'a.go', 'function', 'Kept', 'body', 3, 4, 'go');
	`); err != nil {
		t.Fatalf("seed legacy db: %v", err)
	}
	db.Close()

	idx, err := NewLexicalIndex(path, 4)
	if err != nil {
		t.Fatalf("opening a legacy sidecar must succeed: %v", err)
	}
	defer idx.Close()

	ctx := context.Background()

	// The chunk written by the old build survives the upgrade.
	var name string
	var startLine int
	if err := idx.db.QueryRow(`SELECT name, start_line FROM chunks WHERE id = 'kept'`).Scan(&name, &startLine); err != nil {
		t.Fatalf("pre-existing chunk lost during upgrade: %v", err)
	}
	if name != "Kept" || startLine != 3 {
		t.Errorf("pre-existing chunk = %s:%d, want Kept:3", name, startLine)
	}

	// The reference table is present and usable.
	rs := interface{}(idx).(RefStorage)
	if err := rs.StoreRefs(ctx, []ChunkRef{
		{ChunkID: "kept", RefType: RefCalls, RefName: "pkg.Kept"},
	}); err != nil {
		t.Fatalf("StoreRefs on upgraded sidecar: %v", err)
	}
	if err := rs.ResolveRefs(ctx); err != nil {
		t.Fatalf("ResolveRefs on upgraded sidecar: %v", err)
	}
	edges, err := rs.GetCallersByName(ctx, "Kept")
	if err != nil {
		t.Fatalf("GetCallersByName on upgraded sidecar: %v", err)
	}
	if len(edges) != 1 {
		t.Fatalf("upgraded sidecar returned %d edges, want 1: %+v", len(edges), edges)
	}
}

// TestSQLiteStorage_UpgradesLegacyChunkRefs opens a database whose reference
// table predates the column resolution matches on. The column must be added
// and backfilled so edges recorded by the old build still resolve.
func TestSQLiteStorage_UpgradesLegacyChunkRefs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy-refs.db")

	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open legacy db: %v", err)
	}
	if _, err := db.Exec(`
		CREATE TABLE chunks (
			id TEXT PRIMARY KEY,
			file_path TEXT NOT NULL,
			type TEXT NOT NULL,
			name TEXT NOT NULL,
			signature TEXT,
			content TEXT,
			start_line INTEGER,
			end_line INTEGER,
			language TEXT,
			domain TEXT DEFAULT 'code',
			embedding BLOB,
			file_mtime INTEGER,
			content_hash TEXT
		);
		CREATE TABLE chunk_refs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			chunk_id TEXT NOT NULL,
			ref_type TEXT NOT NULL,
			ref_name TEXT NOT NULL,
			ref_target_id TEXT
		);
		INSERT INTO chunks (id, file_path, type, name, content, start_line, end_line, language)
		VALUES ('caller', 'a.go', 'function', 'Caller', 'x', 1, 2, 'go'),
		       ('target', 'b.go', 'function', 'Target', 'x', 7, 8, 'go');
		INSERT INTO chunk_refs (chunk_id, ref_type, ref_name, ref_target_id)
		VALUES ('caller', 'calls', 'pkg.Target', NULL);
	`); err != nil {
		t.Fatalf("seed legacy db: %v", err)
	}
	db.Close()

	storage, err := NewSQLiteStorage(path, 0)
	if err != nil {
		t.Fatalf("opening a legacy database must succeed: %v", err)
	}
	defer storage.Close()

	ctx := context.Background()
	rs := RefStorage(storage)

	if err := rs.ResolveRefs(ctx); err != nil {
		t.Fatalf("ResolveRefs: %v", err)
	}

	edges, err := rs.GetCallersByName(ctx, "Target")
	if err != nil {
		t.Fatalf("GetCallersByName: %v", err)
	}
	if len(edges) != 1 {
		t.Fatalf("edge stored by the old build did not resolve after upgrade: got %d edges", len(edges))
	}
	if edges[0].ChunkID != "caller" {
		t.Errorf("resolved edge starts at %q, want %q", edges[0].ChunkID, "caller")
	}
}

// TestQdrantStorage_RefStorage proves the call graph works on the backend the
// real configurations name. It runs against a live Qdrant when one is
// configured.
func TestQdrantStorage_RefStorage(t *testing.T) {
	apiURL := os.Getenv("QDRANT_API_URL")
	if apiURL == "" {
		t.Skip("Skipping Qdrant ref tests: QDRANT_API_URL not set")
	}

	storage, err := NewQdrantStorage(QdrantConfig{
		APIKey:         os.Getenv("QDRANT_API_KEY"),
		URL:            apiURL,
		CollectionName: "llm_tools_refs_test",
		EmbeddingDim:   4,
		InMemoryFTS:    true,
	})
	if err != nil {
		t.Fatalf("NewQdrantStorage: %v", err)
	}
	defer func() {
		storage.DeleteCollection()
		storage.Close()
	}()

	ctx := context.Background()

	rs, ok := interface{}(storage).(RefStorage)
	if !ok {
		t.Fatal("QdrantStorage does not implement RefStorage")
	}

	if err := storage.Clear(ctx); err != nil {
		t.Fatalf("Clear: %v", err)
	}

	if err := storage.CreateBatch(ctx, []ChunkWithEmbedding{
		{Chunk: Chunk{ID: "caller", FilePath: "a.go", Type: ChunkFunction, Name: "Caller", Content: "x", StartLine: 10, EndLine: 12, Language: "go"}, Embedding: []float32{0.1, 0.2, 0.3, 0.4}},
		{Chunk: Chunk{ID: "target", FilePath: "b.go", Type: ChunkMethod, Name: "Process", Content: "x", StartLine: 5, EndLine: 6, Language: "go"}, Embedding: []float32{0.2, 0.3, 0.4, 0.5}},
	}); err != nil {
		t.Fatalf("CreateBatch: %v", err)
	}

	if err := rs.StoreRefs(ctx, []ChunkRef{
		{ChunkID: "caller", RefType: RefCalls, RefName: "w.Process"},
	}); err != nil {
		t.Fatalf("StoreRefs: %v", err)
	}
	if err := rs.ResolveRefs(ctx); err != nil {
		t.Fatalf("ResolveRefs: %v", err)
	}

	edges, err := rs.GetCallersByName(ctx, "Process")
	if err != nil {
		t.Fatalf("GetCallersByName: %v", err)
	}
	if len(edges) != 1 {
		t.Fatalf("GetCallersByName(Process) = %d edges, want 1: %+v", len(edges), edges)
	}
	if edges[0].ChunkID != "caller" || edges[0].RefTargetID != "target" {
		t.Errorf("edge = %+v, want caller -> target", edges[0])
	}
	if edges[0].FilePath != "a.go" || edges[0].StartLine != 10 {
		t.Errorf("caller location = %s:%d, want a.go:10", edges[0].FilePath, edges[0].StartLine)
	}

	// Exercise the rest of the hand-off, so a delegation wired to the wrong
	// place cannot pass on the strength of one method alone.
	outgoing, err := rs.GetRefsByName(ctx, "Caller")
	if err != nil {
		t.Fatalf("GetRefsByName: %v", err)
	}
	if len(outgoing) != 1 || outgoing[0].RefName != "w.Process" {
		t.Errorf("GetRefsByName(Caller) = %+v, want the single w.Process edge", outgoing)
	}

	byID, err := rs.GetRefs(ctx, "caller")
	if err != nil {
		t.Fatalf("GetRefs: %v", err)
	}
	if len(byID) != 1 {
		t.Errorf("GetRefs(caller) = %+v, want 1 edge", byID)
	}

	incoming, err := rs.GetCallers(ctx, "target")
	if err != nil {
		t.Fatalf("GetCallers: %v", err)
	}
	if len(incoming) != 1 {
		t.Errorf("GetCallers(target) = %+v, want 1 edge", incoming)
	}

	if err := rs.DeleteRefsByChunk(ctx, "caller"); err != nil {
		t.Fatalf("DeleteRefsByChunk: %v", err)
	}
	remaining, err := rs.GetRefs(ctx, "caller")
	if err != nil {
		t.Fatalf("GetRefs after delete: %v", err)
	}
	if len(remaining) != 0 {
		t.Errorf("GetRefs after delete = %+v, want none", remaining)
	}
}
