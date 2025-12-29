package semantic

import (
	"encoding/json"
	"testing"
)

func TestChunkType_String(t *testing.T) {
	tests := []struct {
		ct   ChunkType
		want string
	}{
		{ChunkFunction, "function"},
		{ChunkMethod, "method"},
		{ChunkStruct, "struct"},
		{ChunkInterface, "interface"},
		{ChunkFile, "file"},
		{ChunkType(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.ct.String(); got != tt.want {
				t.Errorf("ChunkType.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestChunk_JSON(t *testing.T) {
	chunk := Chunk{
		ID:        "test-123",
		FilePath:  "/path/to/file.go",
		Type:      ChunkFunction,
		Name:      "TestFunction",
		Signature: "func TestFunction(t *testing.T)",
		Content:   "func TestFunction(t *testing.T) { ... }",
		StartLine: 10,
		EndLine:   20,
		Language:  "go",
	}

	// Test marshaling
	data, err := json.Marshal(chunk)
	if err != nil {
		t.Fatalf("Failed to marshal Chunk: %v", err)
	}

	// Test unmarshaling
	var decoded Chunk
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal Chunk: %v", err)
	}

	// Verify fields
	if decoded.ID != chunk.ID {
		t.Errorf("ID mismatch: got %v, want %v", decoded.ID, chunk.ID)
	}
	if decoded.FilePath != chunk.FilePath {
		t.Errorf("FilePath mismatch: got %v, want %v", decoded.FilePath, chunk.FilePath)
	}
	if decoded.Type != chunk.Type {
		t.Errorf("Type mismatch: got %v, want %v", decoded.Type, chunk.Type)
	}
	if decoded.Name != chunk.Name {
		t.Errorf("Name mismatch: got %v, want %v", decoded.Name, chunk.Name)
	}
	if decoded.StartLine != chunk.StartLine {
		t.Errorf("StartLine mismatch: got %v, want %v", decoded.StartLine, chunk.StartLine)
	}
	if decoded.EndLine != chunk.EndLine {
		t.Errorf("EndLine mismatch: got %v, want %v", decoded.EndLine, chunk.EndLine)
	}
	if decoded.Language != chunk.Language {
		t.Errorf("Language mismatch: got %v, want %v", decoded.Language, chunk.Language)
	}
}

func TestChunk_GenerateID(t *testing.T) {
	chunk := Chunk{
		FilePath:  "/path/to/file.go",
		Name:      "TestFunction",
		StartLine: 10,
	}

	id := chunk.GenerateID()
	if id == "" {
		t.Error("GenerateID() returned empty string")
	}

	// Same inputs should produce same ID
	id2 := chunk.GenerateID()
	if id != id2 {
		t.Errorf("GenerateID() not deterministic: %v != %v", id, id2)
	}

	// Different inputs should produce different ID
	chunk.StartLine = 20
	id3 := chunk.GenerateID()
	if id == id3 {
		t.Error("GenerateID() should produce different IDs for different inputs")
	}
}

func TestSearchResult_JSON(t *testing.T) {
	result := SearchResult{
		Chunk: Chunk{
			ID:       "test-123",
			FilePath: "/path/to/file.go",
			Name:     "TestFunction",
			Type:     ChunkFunction,
		},
		Score: 0.95,
	}

	// Test marshaling
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("Failed to marshal SearchResult: %v", err)
	}

	// Test unmarshaling
	var decoded SearchResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal SearchResult: %v", err)
	}

	if decoded.Score != result.Score {
		t.Errorf("Score mismatch: got %v, want %v", decoded.Score, result.Score)
	}
	if decoded.Chunk.ID != result.Chunk.ID {
		t.Errorf("Chunk.ID mismatch: got %v, want %v", decoded.Chunk.ID, result.Chunk.ID)
	}
}

func TestSearchResult_MinimalJSON(t *testing.T) {
	result := SearchResult{
		Chunk: Chunk{
			FilePath:  "/path/to/file.go",
			Name:      "TestFunction",
			StartLine: 10,
		},
		Score: 0.92,
	}

	// Test minimal JSON output format
	minimal := result.MinimalJSON()
	if minimal == "" {
		t.Error("MinimalJSON() returned empty string")
	}

	// Should contain essential fields only
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(minimal), &parsed); err != nil {
		t.Fatalf("MinimalJSON() produced invalid JSON: %v", err)
	}

	// Check required fields in minimal format
	if _, ok := parsed["f"]; !ok {
		t.Error("MinimalJSON() missing 'f' (file path)")
	}
	if _, ok := parsed["n"]; !ok {
		t.Error("MinimalJSON() missing 'n' (name)")
	}
	if _, ok := parsed["l"]; !ok {
		t.Error("MinimalJSON() missing 'l' (line)")
	}
	if _, ok := parsed["s"]; !ok {
		t.Error("MinimalJSON() missing 's' (score)")
	}
}

func TestChunkType_Parse(t *testing.T) {
	tests := []struct {
		input string
		want  ChunkType
		err   bool
	}{
		{"function", ChunkFunction, false},
		{"method", ChunkMethod, false},
		{"struct", ChunkStruct, false},
		{"interface", ChunkInterface, false},
		{"file", ChunkFile, false},
		{"invalid", ChunkFunction, true},
		{"", ChunkFunction, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseChunkType(tt.input)
			if (err != nil) != tt.err {
				t.Errorf("ParseChunkType(%q) error = %v, wantErr %v", tt.input, err, tt.err)
				return
			}
			if !tt.err && got != tt.want {
				t.Errorf("ParseChunkType(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
