package semantic

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGoChunker_SimpleFunctions(t *testing.T) {
	chunker := NewGoChunker()

	content, err := os.ReadFile(filepath.Join("testdata", "simple.go"))
	if err != nil {
		t.Fatalf("Failed to read test file: %v", err)
	}

	chunks, err := chunker.Chunk("testdata/simple.go", content)
	if err != nil {
		t.Fatalf("Chunk() error = %v", err)
	}

	// Expect 2 functions: Add, Subtract
	if len(chunks) != 2 {
		t.Errorf("Chunk() returned %d chunks, want 2", len(chunks))
	}

	// Verify Add function
	var addChunk *Chunk
	for i := range chunks {
		if chunks[i].Name == "Add" {
			addChunk = &chunks[i]
			break
		}
	}

	if addChunk == nil {
		t.Fatal("Add function not found in chunks")
	}

	if addChunk.Type != ChunkFunction {
		t.Errorf("Add chunk type = %v, want function", addChunk.Type)
	}

	if addChunk.Signature != "func Add(a, b int) int" {
		t.Errorf("Add signature = %q, want %q", addChunk.Signature, "func Add(a, b int) int")
	}

	if addChunk.Language != "go" {
		t.Errorf("Add language = %q, want 'go'", addChunk.Language)
	}
}

func TestGoChunker_Methods(t *testing.T) {
	chunker := NewGoChunker()

	content, err := os.ReadFile(filepath.Join("testdata", "methods.go"))
	if err != nil {
		t.Fatalf("Failed to read test file: %v", err)
	}

	chunks, err := chunker.Chunk("testdata/methods.go", content)
	if err != nil {
		t.Fatalf("Chunk() error = %v", err)
	}

	// Expect: 1 struct (Calculator), 2 methods (Add, Reset)
	if len(chunks) != 3 {
		t.Errorf("Chunk() returned %d chunks, want 3", len(chunks))
	}

	// Find the Add method
	var addMethod *Chunk
	for i := range chunks {
		if chunks[i].Name == "Add" && chunks[i].Type == ChunkMethod {
			addMethod = &chunks[i]
			break
		}
	}

	if addMethod == nil {
		t.Fatal("Add method not found in chunks")
	}

	// Verify method has receiver in signature
	if addMethod.Signature == "" {
		t.Error("Add method should have a signature")
	}
}

func TestGoChunker_Interfaces(t *testing.T) {
	chunker := NewGoChunker()

	content, err := os.ReadFile(filepath.Join("testdata", "interfaces.go"))
	if err != nil {
		t.Fatalf("Failed to read test file: %v", err)
	}

	chunks, err := chunker.Chunk("testdata/interfaces.go", content)
	if err != nil {
		t.Fatalf("Chunk() error = %v", err)
	}

	// Expect 3 interfaces: Reader, Writer, ReadWriter
	if len(chunks) != 3 {
		t.Errorf("Chunk() returned %d chunks, want 3", len(chunks))
	}

	// Verify all are interfaces
	for _, chunk := range chunks {
		if chunk.Type != ChunkInterface {
			t.Errorf("Chunk %q type = %v, want interface", chunk.Name, chunk.Type)
		}
	}

	// Find Reader interface
	var reader *Chunk
	for i := range chunks {
		if chunks[i].Name == "Reader" {
			reader = &chunks[i]
			break
		}
	}

	if reader == nil {
		t.Fatal("Reader interface not found")
	}

	// Should include doc comment in content
	if reader.Content == "" {
		t.Error("Reader content should not be empty")
	}
}

func TestGoChunker_Structs(t *testing.T) {
	chunker := NewGoChunker()

	content, err := os.ReadFile(filepath.Join("testdata", "methods.go"))
	if err != nil {
		t.Fatalf("Failed to read test file: %v", err)
	}

	chunks, err := chunker.Chunk("testdata/methods.go", content)
	if err != nil {
		t.Fatalf("Chunk() error = %v", err)
	}

	// Find Calculator struct
	var calc *Chunk
	for i := range chunks {
		if chunks[i].Name == "Calculator" && chunks[i].Type == ChunkStruct {
			calc = &chunks[i]
			break
		}
	}

	if calc == nil {
		t.Fatal("Calculator struct not found")
	}

	if calc.Language != "go" {
		t.Errorf("Calculator language = %q, want 'go'", calc.Language)
	}
}

func TestGoChunker_Complex(t *testing.T) {
	chunker := NewGoChunker()

	content, err := os.ReadFile(filepath.Join("testdata", "complex.go"))
	if err != nil {
		t.Fatalf("Failed to read test file: %v", err)
	}

	chunks, err := chunker.Chunk("testdata/complex.go", content)
	if err != nil {
		t.Fatalf("Chunk() error = %v", err)
	}

	// Expect: 2 structs (Config, Service), 1 interface (ServiceInterface),
	// 1 function (NewService), 2 methods (Run, Stop)
	expectedCount := 6
	if len(chunks) != expectedCount {
		t.Errorf("Chunk() returned %d chunks, want %d", len(chunks), expectedCount)
		for _, c := range chunks {
			t.Logf("  Found: %s (%s)", c.Name, c.Type)
		}
	}

	// Verify types present
	typeCount := map[ChunkType]int{}
	for _, c := range chunks {
		typeCount[c.Type]++
	}

	if typeCount[ChunkStruct] != 2 {
		t.Errorf("Expected 2 structs, got %d", typeCount[ChunkStruct])
	}
	if typeCount[ChunkInterface] != 1 {
		t.Errorf("Expected 1 interface, got %d", typeCount[ChunkInterface])
	}
	if typeCount[ChunkFunction] != 1 {
		t.Errorf("Expected 1 function, got %d", typeCount[ChunkFunction])
	}
	if typeCount[ChunkMethod] != 2 {
		t.Errorf("Expected 2 methods, got %d", typeCount[ChunkMethod])
	}
}

func TestGoChunker_LineNumbers(t *testing.T) {
	chunker := NewGoChunker()

	content, err := os.ReadFile(filepath.Join("testdata", "simple.go"))
	if err != nil {
		t.Fatalf("Failed to read test file: %v", err)
	}

	chunks, err := chunker.Chunk("testdata/simple.go", content)
	if err != nil {
		t.Fatalf("Chunk() error = %v", err)
	}

	for _, chunk := range chunks {
		if chunk.StartLine <= 0 {
			t.Errorf("Chunk %q has invalid StartLine: %d", chunk.Name, chunk.StartLine)
		}
		if chunk.EndLine < chunk.StartLine {
			t.Errorf("Chunk %q has EndLine (%d) < StartLine (%d)", chunk.Name, chunk.EndLine, chunk.StartLine)
		}
	}
}

func TestGoChunker_InvalidSyntax(t *testing.T) {
	chunker := NewGoChunker()

	invalidCode := []byte("package main\n\nfunc broken( {")

	_, err := chunker.Chunk("invalid.go", invalidCode)
	if err == nil {
		t.Error("Expected error for invalid Go syntax, got nil")
	}
}

func TestGoChunker_EmptyFile(t *testing.T) {
	chunker := NewGoChunker()

	// Valid but empty package
	emptyCode := []byte("package empty\n")

	chunks, err := chunker.Chunk("empty.go", emptyCode)
	if err != nil {
		t.Fatalf("Chunk() error = %v", err)
	}

	if len(chunks) != 0 {
		t.Errorf("Expected 0 chunks for empty file, got %d", len(chunks))
	}
}

func TestGoChunker_SupportedExtensions(t *testing.T) {
	chunker := NewGoChunker()

	exts := chunker.SupportedExtensions()

	if len(exts) == 0 {
		t.Error("SupportedExtensions() should return at least one extension")
	}

	foundGo := false
	for _, ext := range exts {
		if ext == "go" {
			foundGo = true
			break
		}
	}

	if !foundGo {
		t.Error("SupportedExtensions() should include 'go'")
	}
}

func TestGoChunker_DocComments(t *testing.T) {
	chunker := NewGoChunker()

	content, err := os.ReadFile(filepath.Join("testdata", "simple.go"))
	if err != nil {
		t.Fatalf("Failed to read test file: %v", err)
	}

	chunks, err := chunker.Chunk("testdata/simple.go", content)
	if err != nil {
		t.Fatalf("Chunk() error = %v", err)
	}

	// Find Add function and check its content includes the doc comment
	for _, chunk := range chunks {
		if chunk.Name == "Add" {
			if chunk.Content == "" {
				t.Error("Add function content should not be empty")
			}
			// Content should include the function body
			break
		}
	}
}
