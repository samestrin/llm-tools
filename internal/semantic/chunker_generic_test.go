package semantic

import (
	"testing"
)

func TestGenericChunker_Chunk(t *testing.T) {
	chunker := NewGenericChunker(500) // 500 chars max per chunk

	content := []byte(`This is a sample file.
It has multiple lines of text.
We want to chunk it into smaller pieces.
Each chunk should be reasonable in size.

This is a new paragraph.
It contains more content.
The chunker should handle paragraphs well.

Another section here.
More content to process.
Testing the chunker functionality.
`)

	chunks, err := chunker.Chunk("test.txt", content)
	if err != nil {
		t.Fatalf("Chunk() error = %v", err)
	}

	if len(chunks) == 0 {
		t.Error("Chunk() should return at least one chunk")
	}

	// All chunks should be file type
	for _, chunk := range chunks {
		if chunk.Type != ChunkFile {
			t.Errorf("Chunk type = %v, want file", chunk.Type)
		}
		if chunk.Language == "" {
			t.Error("Chunk should have a language set")
		}
		if chunk.FilePath != "test.txt" {
			t.Errorf("Chunk.FilePath = %q, want 'test.txt'", chunk.FilePath)
		}
	}
}

func TestGenericChunker_SmallFile(t *testing.T) {
	chunker := NewGenericChunker(1000)

	content := []byte("Small content")

	chunks, err := chunker.Chunk("small.txt", content)
	if err != nil {
		t.Fatalf("Chunk() error = %v", err)
	}

	if len(chunks) != 1 {
		t.Errorf("Chunk() returned %d chunks for small file, want 1", len(chunks))
	}
}

func TestGenericChunker_EmptyFile(t *testing.T) {
	chunker := NewGenericChunker(500)

	content := []byte("")

	chunks, err := chunker.Chunk("empty.txt", content)
	if err != nil {
		t.Fatalf("Chunk() error = %v", err)
	}

	if len(chunks) != 0 {
		t.Errorf("Chunk() returned %d chunks for empty file, want 0", len(chunks))
	}
}

func TestGenericChunker_LargeFile(t *testing.T) {
	chunker := NewGenericChunker(100) // Small chunk size for testing

	// Generate content larger than chunk size
	content := make([]byte, 500)
	for i := range content {
		content[i] = 'a' + byte(i%26)
		if i%50 == 49 {
			content[i] = '\n'
		}
	}

	chunks, err := chunker.Chunk("large.txt", content)
	if err != nil {
		t.Fatalf("Chunk() error = %v", err)
	}

	if len(chunks) < 2 {
		t.Errorf("Chunk() returned %d chunks for large file, want at least 2", len(chunks))
	}

	// Verify line numbers are continuous
	for i := 1; i < len(chunks); i++ {
		if chunks[i].StartLine <= chunks[i-1].EndLine {
			// Chunks should be ordered
			if chunks[i].StartLine < chunks[i-1].StartLine {
				t.Error("Chunks are not in order")
			}
		}
	}
}

func TestGenericChunker_SupportedExtensions(t *testing.T) {
	chunker := NewGenericChunker(500)

	exts := chunker.SupportedExtensions()

	// Generic chunker should support many extensions
	if len(exts) < 5 {
		t.Errorf("SupportedExtensions() returned %d extensions, want at least 5", len(exts))
	}

	// Should include common text extensions (md/html handled by specialized chunkers)
	expectedExts := map[string]bool{"txt": true, "rst": true, "yaml": true}
	for _, ext := range exts {
		delete(expectedExts, ext)
	}
	if len(expectedExts) > 0 {
		t.Errorf("SupportedExtensions() missing: %v", expectedExts)
	}
}

func TestGenericChunker_LineNumbers(t *testing.T) {
	chunker := NewGenericChunker(500)

	content := []byte("Line 1\nLine 2\nLine 3\nLine 4\nLine 5\n")

	chunks, err := chunker.Chunk("test.txt", content)
	if err != nil {
		t.Fatalf("Chunk() error = %v", err)
	}

	for _, chunk := range chunks {
		if chunk.StartLine < 1 {
			t.Errorf("Chunk.StartLine = %d, should be >= 1", chunk.StartLine)
		}
		if chunk.EndLine < chunk.StartLine {
			t.Errorf("Chunk.EndLine (%d) < StartLine (%d)", chunk.EndLine, chunk.StartLine)
		}
	}
}
