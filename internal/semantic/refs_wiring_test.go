package semantic

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// newRefsHarness returns a real IndexManager writing into an empty temp
// directory, wired the same way the CLI wires it.
func newRefsHarness(t *testing.T) (*SQLiteStorage, *IndexManager, string) {
	t.Helper()

	storage, err := NewSQLiteStorage(":memory:", 4)
	if err != nil {
		t.Fatalf("NewSQLiteStorage: %v", err)
	}
	t.Cleanup(func() { storage.Close() })

	factory := NewChunkerFactory()
	factory.Register("go", NewGoChunker())

	mgr := NewIndexManager(storage, &mockEmbedder{embedding: []float32{0.1, 0.2, 0.3, 0.4}}, factory)
	return storage, mgr, t.TempDir()
}

func writeGoFile(t *testing.T, dir, name, src string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(src), 0644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

// chunkByName returns the single indexed chunk with this name, failing if it
// is missing or ambiguous.
func chunkByName(t *testing.T, storage *SQLiteStorage, name string) Chunk {
	t.Helper()
	chunks, err := storage.List(context.Background(), ListOptions{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	var found []Chunk
	for _, c := range chunks {
		if c.Name == name {
			found = append(found, c)
		}
	}
	if len(found) != 1 {
		t.Fatalf("expected exactly one chunk named %q, found %d", name, len(found))
	}
	return found[0]
}

// TestIndexManager_ResolvesRefsDuringIndex proves indexing alone produces a
// usable call graph. Nothing here calls ResolveRefs: if the index path stops
// resolving, this test fails.
func TestIndexManager_ResolvesRefsDuringIndex(t *testing.T) {
	storage, mgr, dir := newRefsHarness(t)
	ctx := context.Background()

	writeGoFile(t, dir, "caller.go", `package main

func Run() {
	w := &Worker{}
	w.Process()
	Helper()
}
`)
	writeGoFile(t, dir, "worker.go", `package main

type Worker struct{}

func (w *Worker) Process() {}

func Helper() {}
`)

	if _, err := mgr.Index(ctx, dir, IndexOptions{}); err != nil {
		t.Fatalf("Index: %v", err)
	}

	run := chunkByName(t, storage, "Run")
	process := chunkByName(t, storage, "Process")
	helper := chunkByName(t, storage, "Helper")

	rs := RefStorage(storage)

	// A method call reached through a receiver must land on the method chunk.
	callers, err := rs.GetCallersByName(ctx, "Process")
	if err != nil {
		t.Fatalf("GetCallersByName(Process): %v", err)
	}
	if len(callers) != 1 {
		t.Fatalf("GetCallersByName(Process) = %d edges, want 1: %+v", len(callers), callers)
	}
	if callers[0].ChunkID != run.ID {
		t.Errorf("caller chunk = %q, want Run's chunk %q", callers[0].ChunkID, run.ID)
	}
	if callers[0].RefTargetID != process.ID {
		t.Errorf("edge target = %q, want Process's chunk %q", callers[0].RefTargetID, process.ID)
	}
	if callers[0].FilePath != filepath.Join(dir, "caller.go") {
		t.Errorf("caller file = %q, want caller.go", callers[0].FilePath)
	}
	if callers[0].StartLine != run.StartLine {
		t.Errorf("caller line = %d, want Run's line %d", callers[0].StartLine, run.StartLine)
	}

	// A bare call in the same package must land on the function chunk.
	helperCallers, err := rs.GetCallersByName(ctx, "Helper")
	if err != nil {
		t.Fatalf("GetCallersByName(Helper): %v", err)
	}
	if len(helperCallers) != 1 || helperCallers[0].RefTargetID != helper.ID {
		t.Errorf("GetCallersByName(Helper) = %+v, want one edge targeting %q", helperCallers, helper.ID)
	}
}

// TestIndexManager_TwoStepIndexThenRename follows a call across three states:
// the callee does not exist yet, the callee appears, and the callee is renamed
// away. The edge must go unresolved, then resolved, then unresolved again
// rather than pointing at a chunk that no longer exists.
func TestIndexManager_TwoStepIndexThenRename(t *testing.T) {
	storage, mgr, dir := newRefsHarness(t)
	ctx := context.Background()
	rs := RefStorage(storage)

	// State 1: A calls B, but B is not indexed yet.
	writeGoFile(t, dir, "a.go", `package main

func A() {
	B()
}
`)
	if _, err := mgr.Index(ctx, dir, IndexOptions{}); err != nil {
		t.Fatalf("Index (state 1): %v", err)
	}

	if callers, err := rs.GetCallersByName(ctx, "B"); err != nil {
		t.Fatalf("GetCallersByName(B) state 1: %v", err)
	} else if len(callers) != 0 {
		t.Errorf("state 1: B has callers %+v, want none (B is not indexed yet)", callers)
	}

	// State 2: B appears. The edge recorded earlier must now resolve.
	writeGoFile(t, dir, "b.go", `package main

func B() {}
`)
	if _, err := mgr.Index(ctx, dir, IndexOptions{}); err != nil {
		t.Fatalf("Index (state 2): %v", err)
	}

	a := chunkByName(t, storage, "A")
	b := chunkByName(t, storage, "B")

	callers, err := rs.GetCallersByName(ctx, "B")
	if err != nil {
		t.Fatalf("GetCallersByName(B) state 2: %v", err)
	}
	if len(callers) != 1 {
		t.Fatalf("state 2: GetCallersByName(B) = %d edges, want 1: %+v", len(callers), callers)
	}
	if callers[0].ChunkID != a.ID {
		t.Errorf("state 2: caller = %q, want A's chunk %q", callers[0].ChunkID, a.ID)
	}
	if callers[0].RefTargetID != b.ID {
		t.Errorf("state 2: target = %q, want B's chunk %q", callers[0].RefTargetID, b.ID)
	}

	// State 3: B is renamed. Its chunk disappears, so the edge must let go of
	// it rather than keep pointing at a dead chunk.
	writeGoFile(t, dir, "b.go", `package main

func Renamed() {}
`)
	if _, err := mgr.Update(ctx, dir, UpdateOptions{}); err != nil {
		t.Fatalf("Update (state 3): %v", err)
	}

	if callers, err := rs.GetCallersByName(ctx, "B"); err != nil {
		t.Fatalf("GetCallersByName(B) state 3: %v", err)
	} else if len(callers) != 0 {
		t.Errorf("state 3: B still has callers %+v after being renamed away", callers)
	}

	edges, err := rs.GetRefsByName(ctx, "A")
	if err != nil {
		t.Fatalf("GetRefsByName(A) state 3: %v", err)
	}
	for _, e := range edges {
		if e.RefName != "B" {
			continue
		}
		if e.RefTargetID != "" {
			t.Errorf("state 3: A's call to B still targets %q, want cleared", e.RefTargetID)
		}
		if e.RefTargetID == b.ID {
			t.Errorf("state 3: A's call to B still points at the deleted chunk %q", b.ID)
		}
	}
}

// TestIndexManager_UpdatePicksUpNewCalls covers the path people actually use
// day to day. Editing a function to add a call and running an incremental
// update has to record that call, otherwise the graph quietly goes stale while
// still looking answerable.
func TestIndexManager_UpdatePicksUpNewCalls(t *testing.T) {
	storage, mgr, dir := newRefsHarness(t)
	ctx := context.Background()
	rs := RefStorage(storage)

	writeGoFile(t, dir, "a.go", `package main

func A() {
}
`)
	writeGoFile(t, dir, "b.go", `package main

func B() {}
`)
	if _, err := mgr.Index(ctx, dir, IndexOptions{}); err != nil {
		t.Fatalf("Index: %v", err)
	}

	if callers, err := rs.GetCallersByName(ctx, "B"); err != nil {
		t.Fatalf("GetCallersByName(B) before edit: %v", err)
	} else if len(callers) != 0 {
		t.Fatalf("precondition: B already has callers %+v", callers)
	}

	// A is edited to call B.
	writeGoFile(t, dir, "a.go", `package main

func A() {
	B()
}
`)
	if _, err := mgr.Update(ctx, dir, UpdateOptions{}); err != nil {
		t.Fatalf("Update: %v", err)
	}

	a := chunkByName(t, storage, "A")
	b := chunkByName(t, storage, "B")

	callers, err := rs.GetCallersByName(ctx, "B")
	if err != nil {
		t.Fatalf("GetCallersByName(B) after edit: %v", err)
	}
	if len(callers) != 1 {
		t.Fatalf("GetCallersByName(B) = %d edges, want 1 after A was edited to call it: %+v", len(callers), callers)
	}
	if callers[0].ChunkID != a.ID {
		t.Errorf("caller = %q, want A's chunk %q", callers[0].ChunkID, a.ID)
	}
	if callers[0].RefTargetID != b.ID {
		t.Errorf("target = %q, want B's chunk %q", callers[0].RefTargetID, b.ID)
	}
}
