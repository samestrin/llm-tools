package semantic

import (
	"testing"
)

func TestGoChunker_ExtractRefs(t *testing.T) {
	src := []byte(`package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Println("hello")
	result := process()
	os.Exit(0)
}

func process() string {
	return "done"
}
`)
	chunker := NewGoChunker()
	chunks, err := chunker.Chunk("main.go", src)
	if err != nil {
		t.Fatalf("Chunk: %v", err)
	}

	refs, err := chunker.ExtractRefs("main.go", src, chunks)
	if err != nil {
		t.Fatalf("ExtractRefs: %v", err)
	}

	if len(refs) == 0 {
		t.Fatal("Expected refs, got none")
	}

	// Check for calls
	foundCalls := map[string]bool{}
	foundImports := map[string]bool{}
	for _, r := range refs {
		switch r.RefType {
		case RefCalls:
			foundCalls[r.RefName] = true
		case RefImports:
			foundImports[r.RefName] = true
		}
	}

	if !foundCalls["process"] {
		t.Error("Expected call to 'process'")
	}
	if !foundCalls["fmt.Println"] {
		t.Error("Expected call to 'fmt.Println'")
	}
	if !foundImports["fmt"] {
		t.Error("Expected import 'fmt'")
	}
}

func TestGoChunker_ExtractRefs_Empty(t *testing.T) {
	src := []byte(`package main
`)
	chunker := NewGoChunker()
	chunks, _ := chunker.Chunk("empty.go", src)

	refs, err := chunker.ExtractRefs("empty.go", src, chunks)
	if err != nil {
		t.Fatalf("ExtractRefs: %v", err)
	}
	// No functions, so no refs (or just imports)
	_ = refs
}
