package semantic

import (
	"strings"
	"testing"
)

func TestApplyOverlap_Disabled(t *testing.T) {
	chunks := []Chunk{
		{Name: "A", StartLine: 1, EndLine: 5, Content: "line1\nline2\nline3\nline4\nline5"},
		{Name: "B", StartLine: 6, EndLine: 10, Content: "line6\nline7\nline8\nline9\nline10"},
	}
	cfg := OverlapConfig{OverlapLines: 0, IncludeParentContext: false}
	result := ApplyOverlap(chunks, []byte("line1\nline2\nline3\nline4\nline5\nline6\nline7\nline8\nline9\nline10"), cfg)

	for _, c := range result {
		if c.EmbedText != "" {
			t.Errorf("Expected empty EmbedText when overlap disabled, got %q for chunk %s", c.EmbedText, c.Name)
		}
	}
}

func TestApplyOverlap_WithOverlap(t *testing.T) {
	fileContent := "line1\nline2\nline3\nline4\nline5\nline6\nline7\nline8\nline9\nline10"
	chunks := []Chunk{
		{Name: "A", StartLine: 1, EndLine: 5, Content: "line1\nline2\nline3\nline4\nline5"},
		{Name: "B", StartLine: 6, EndLine: 10, Content: "line6\nline7\nline8\nline9\nline10"},
	}
	cfg := OverlapConfig{OverlapLines: 2, IncludeParentContext: false}
	result := ApplyOverlap(chunks, []byte(fileContent), cfg)

	// First chunk: no preceding overlap, but should have trailing overlap (lines 6-7)
	if result[0].EmbedText == "" {
		t.Fatal("Expected EmbedText for first chunk with overlap")
	}
	if !strings.Contains(result[0].EmbedText, "line6") {
		t.Errorf("First chunk EmbedText should include overlap line6, got: %q", result[0].EmbedText)
	}

	// Second chunk: should have leading overlap (lines 4-5)
	if result[1].EmbedText == "" {
		t.Fatal("Expected EmbedText for second chunk with overlap")
	}
	if !strings.Contains(result[1].EmbedText, "line4") || !strings.Contains(result[1].EmbedText, "line5") {
		t.Errorf("Second chunk EmbedText should include overlap lines 4-5, got: %q", result[1].EmbedText)
	}
}

func TestApplyOverlap_SingleChunk(t *testing.T) {
	fileContent := "line1\nline2\nline3"
	chunks := []Chunk{
		{Name: "A", StartLine: 1, EndLine: 3, Content: "line1\nline2\nline3"},
	}
	cfg := OverlapConfig{OverlapLines: 3, IncludeParentContext: false}
	result := ApplyOverlap(chunks, []byte(fileContent), cfg)

	// Single chunk — no overlap possible, EmbedText should be empty (no enrichment needed)
	if result[0].EmbedText != "" {
		t.Errorf("Single chunk should not have overlap EmbedText, got: %q", result[0].EmbedText)
	}
}

func TestApplyOverlap_WithParentContext_Go(t *testing.T) {
	fileContent := "package main\n\nimport \"fmt\"\n\nfunc Hello() {\n\tfmt.Println(\"hello\")\n}"
	chunks := []Chunk{
		{Name: "Hello", StartLine: 5, EndLine: 7, Content: "func Hello() {\n\tfmt.Println(\"hello\")\n}", Language: "go"},
	}
	cfg := OverlapConfig{OverlapLines: 0, IncludeParentContext: true}
	result := ApplyOverlap(chunks, []byte(fileContent), cfg)

	if result[0].EmbedText == "" {
		t.Fatal("Expected EmbedText with parent context")
	}
	if !strings.HasPrefix(result[0].EmbedText, "package main\n\n") {
		t.Errorf("Go chunk should have package prefix, got: %q", result[0].EmbedText[:min(50, len(result[0].EmbedText))])
	}
}

func TestApplyOverlap_WithParentContext_Python(t *testing.T) {
	fileContent := "class MyService:\n    def process(self, data):\n        return data\n\n    def validate(self, data):\n        return True"
	chunks := []Chunk{
		{Name: "process", StartLine: 2, EndLine: 3, Content: "    def process(self, data):\n        return data", Language: "py", Type: ChunkMethod},
		{Name: "validate", StartLine: 5, EndLine: 6, Content: "    def validate(self, data):\n        return True", Language: "py", Type: ChunkMethod},
	}
	cfg := OverlapConfig{OverlapLines: 0, IncludeParentContext: true}
	result := ApplyOverlap(chunks, []byte(fileContent), cfg)

	// Methods should get class context prepended
	if !strings.HasPrefix(result[0].EmbedText, "class MyService:\n\n") {
		t.Errorf("Python method should have class prefix, got: %q", result[0].EmbedText[:min(50, len(result[0].EmbedText))])
	}
}

func TestApplyOverlap_BothOverlapAndContext(t *testing.T) {
	fileContent := "package util\n\nfunc A() {\n\t// a\n}\n\nfunc B() {\n\t// b\n}"
	chunks := []Chunk{
		{Name: "A", StartLine: 3, EndLine: 5, Content: "func A() {\n\t// a\n}", Language: "go"},
		{Name: "B", StartLine: 7, EndLine: 9, Content: "func B() {\n\t// b\n}", Language: "go"},
	}
	cfg := OverlapConfig{OverlapLines: 2, IncludeParentContext: true}
	result := ApplyOverlap(chunks, []byte(fileContent), cfg)

	// Second chunk should have both package context and overlap
	if !strings.Contains(result[1].EmbedText, "package util") {
		t.Errorf("Should contain parent context, got: %q", result[1].EmbedText)
	}
}

func TestApplyOverlap_MaxLength(t *testing.T) {
	// Create a chunk with very long content
	longContent := strings.Repeat("x", 9000)
	fileContent := longContent
	chunks := []Chunk{
		{Name: "long", StartLine: 1, EndLine: 1, Content: longContent},
	}
	cfg := OverlapConfig{OverlapLines: 5, IncludeParentContext: true}
	result := ApplyOverlap(chunks, []byte(fileContent), cfg)

	// EmbedText should not exceed maxEmbedTextLength
	if result[0].EmbedText != "" && len(result[0].EmbedText) > 8000 {
		t.Errorf("EmbedText exceeds max length: %d", len(result[0].EmbedText))
	}
}

func TestApplyOverlap_EmptyChunks(t *testing.T) {
	result := ApplyOverlap(nil, []byte("content"), OverlapConfig{OverlapLines: 3})
	if result != nil {
		t.Errorf("Expected nil for empty chunks, got %v", result)
	}
}

func TestDeduplicateOverlapping(t *testing.T) {
	results := []SearchResult{
		{Chunk: Chunk{FilePath: "a.go", StartLine: 1, EndLine: 10}, Score: 0.9},
		{Chunk: Chunk{FilePath: "a.go", StartLine: 8, EndLine: 15}, Score: 0.7},  // overlaps with first (lines 8-10)
		{Chunk: Chunk{FilePath: "b.go", StartLine: 1, EndLine: 10}, Score: 0.8},  // different file, no overlap
		{Chunk: Chunk{FilePath: "a.go", StartLine: 20, EndLine: 30}, Score: 0.6}, // same file, no overlap
	}

	deduped := DeduplicateOverlapping(results, 0.5)

	// Should remove the overlapping second result (3 lines overlap out of 8 = 37.5% — below 50% threshold)
	// Actually: lines 8-10 overlap = 3 lines, result[1] spans 8 lines. 3/8 = 37.5%, below 50%. So it stays.
	// Let's test with higher overlap:
	results2 := []SearchResult{
		{Chunk: Chunk{FilePath: "a.go", StartLine: 1, EndLine: 10}, Score: 0.9},
		{Chunk: Chunk{FilePath: "a.go", StartLine: 5, EndLine: 12}, Score: 0.7}, // overlaps 6 lines out of 8 = 75%
	}
	deduped2 := DeduplicateOverlapping(results2, 0.5)
	if len(deduped2) != 1 {
		t.Errorf("Expected 1 result after dedup (75%% overlap), got %d", len(deduped2))
	}

	// Verify the higher-scored one survives
	if deduped2[0].Score != 0.9 {
		t.Errorf("Expected highest score to survive, got %.2f", deduped2[0].Score)
	}

	// Verify different files don't get deduped
	if len(deduped) != 4 {
		t.Errorf("Expected 4 results (different files + below threshold), got %d", len(deduped))
	}
}

func TestDeduplicateOverlapping_EmptyInput(t *testing.T) {
	result := DeduplicateOverlapping(nil, 0.5)
	if result != nil {
		t.Errorf("Expected nil for nil input, got %v", result)
	}
}
