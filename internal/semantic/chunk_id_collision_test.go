package semantic

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// TestGoChunker_ChunkIDsDependOnFilePath proves two identical declarations in
// different files get different identifiers. The id is derived from file path,
// name and start line, so leaving the path unset collapses every same-named
// symbol on the same line into one id.
func TestGoChunker_ChunkIDsDependOnFilePath(t *testing.T) {
	src := []byte(`package pkg

func Shared() {}
`)

	chunker := NewGoChunker()

	alpha, err := chunker.Chunk("alpha/util.go", src)
	if err != nil {
		t.Fatalf("Chunk(alpha): %v", err)
	}
	beta, err := chunker.Chunk("beta/util.go", src)
	if err != nil {
		t.Fatalf("Chunk(beta): %v", err)
	}

	if len(alpha) != 1 || len(beta) != 1 {
		t.Fatalf("expected one chunk per file, got %d and %d", len(alpha), len(beta))
	}

	if alpha[0].FilePath != "alpha/util.go" || beta[0].FilePath != "beta/util.go" {
		t.Fatalf("chunks carry the wrong path: %q and %q", alpha[0].FilePath, beta[0].FilePath)
	}

	if alpha[0].ID == beta[0].ID {
		t.Errorf("chunks from different files share id %q; the path is missing from the id", alpha[0].ID)
	}
}

// TestGoChunker_TypeChunkIDsDependOnFilePath is the same guarantee for type
// declarations, which are built by a separate path in the chunker.
func TestGoChunker_TypeChunkIDsDependOnFilePath(t *testing.T) {
	src := []byte(`package pkg

type Config struct{}
`)

	chunker := NewGoChunker()

	alpha, err := chunker.Chunk("alpha/types.go", src)
	if err != nil {
		t.Fatalf("Chunk(alpha): %v", err)
	}
	beta, err := chunker.Chunk("beta/types.go", src)
	if err != nil {
		t.Fatalf("Chunk(beta): %v", err)
	}

	if len(alpha) != 1 || len(beta) != 1 {
		t.Fatalf("expected one chunk per file, got %d and %d", len(alpha), len(beta))
	}

	if alpha[0].ID == beta[0].ID {
		t.Errorf("type chunks from different files share id %q; the path is missing from the id", alpha[0].ID)
	}
}

// TestIndexManager_TwinsInDifferentPackagesBothIndex is the failure this causes
// in practice. Two packages each declare Shared on the same line. If their ids
// collide, one silently fails to store, the surviving twin looks like the only
// Shared in the index, and a call to it gets confidently attributed to the
// wrong symbol — exactly what the call graph refuses to do elsewhere.
func TestIndexManager_TwinsInDifferentPackagesBothIndex(t *testing.T) {
	storage, mgr, dir := newRefsHarness(t)
	ctx := context.Background()

	for _, pkg := range []string{"alpha", "beta"} {
		if err := os.MkdirAll(filepath.Join(dir, pkg), 0755); err != nil {
			t.Fatalf("mkdir %s: %v", pkg, err)
		}
		src := "package " + pkg + `

func Shared() {}
`
		if err := os.WriteFile(filepath.Join(dir, pkg, "util.go"), []byte(src), 0644); err != nil {
			t.Fatalf("write %s: %v", pkg, err)
		}
	}

	writeGoFile(t, dir, "main.go", `package main

func Run() {
	alpha.Shared()
}
`)

	result, err := mgr.Index(ctx, dir, IndexOptions{})
	if err != nil {
		t.Fatalf("Index: %v", err)
	}
	if result.FilesSkipped != 0 {
		t.Errorf("Index skipped %d files; both twins must store: %v", result.FilesSkipped, result.Errors)
	}

	chunks, err := storage.List(ctx, ListOptions{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	var shared int
	for _, c := range chunks {
		if c.Name == "Shared" {
			shared++
		}
	}
	if shared != 2 {
		t.Fatalf("index holds %d chunks named Shared, want 2 (one per package)", shared)
	}

	// With both twins present the name is ambiguous, so the call must not be
	// attributed to either one.
	callers, err := RefStorage(storage).GetCallersByName(ctx, "Shared")
	if err != nil {
		t.Fatalf("GetCallersByName: %v", err)
	}
	if len(callers) != 0 {
		t.Errorf("ambiguous Shared was attributed to %+v, want no callers", callers)
	}
}
