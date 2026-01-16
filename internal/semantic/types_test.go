package semantic

import (
	"encoding/json"
	"strings"
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

func TestChunk_Preview_PreferSignature(t *testing.T) {
	chunk := Chunk{
		Signature: "func ValidateToken(token string) (*Claims, error)",
		Content:   "func ValidateToken(token string) (*Claims, error) {\n\tif token == \"\" {\n\t\treturn nil, ErrEmptyToken\n\t}\n\t// ... more code ...\n}",
	}

	preview := chunk.Preview()
	if preview != "func ValidateToken(token string) (*Claims, error)" {
		t.Errorf("Preview() = %q, want signature", preview)
	}
}

func TestChunk_Preview_FallbackToContent(t *testing.T) {
	chunk := Chunk{
		Signature: "", // Empty signature
		Content:   "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(\"Hello\")\n}",
	}

	preview := chunk.Preview()
	// Should fall back to content, with newlines replaced by spaces
	if !strings.HasPrefix(preview, "package main") {
		t.Errorf("Preview() = %q, want content fallback starting with 'package main'", preview)
	}
	if strings.Contains(preview, "\n") {
		t.Errorf("Preview() should not contain newlines, got %q", preview)
	}
}

func TestChunk_Preview_TruncateAt150Chars(t *testing.T) {
	// Create content longer than 150 chars
	longContent := strings.Repeat("a", 200)
	chunk := Chunk{
		Signature: "",
		Content:   longContent,
	}

	preview := chunk.Preview()
	if len(preview) > 153 { // 150 + "..."
		t.Errorf("Preview() length = %d, want <= 153 (150 + ...)", len(preview))
	}
	if !strings.HasSuffix(preview, "...") {
		t.Errorf("Preview() should end with '...' when truncated, got %q", preview)
	}
}

func TestChunk_Preview_ExactlyAt150Chars(t *testing.T) {
	// Content exactly 150 chars should NOT have ellipsis
	exactContent := strings.Repeat("b", 150)
	chunk := Chunk{
		Signature: "",
		Content:   exactContent,
	}

	preview := chunk.Preview()
	if len(preview) != 150 {
		t.Errorf("Preview() length = %d, want 150", len(preview))
	}
	if strings.HasSuffix(preview, "...") {
		t.Errorf("Preview() should NOT end with '...' when exactly 150 chars")
	}
}

func TestChunk_Preview_EmptyContent(t *testing.T) {
	chunk := Chunk{
		Signature: "",
		Content:   "",
	}

	preview := chunk.Preview()
	if preview != "" {
		t.Errorf("Preview() = %q, want empty string for empty content", preview)
	}
}

func TestChunk_Preview_NewlinesReplacedWithSpaces(t *testing.T) {
	chunk := Chunk{
		Signature: "",
		Content:   "line1\nline2\nline3",
	}

	preview := chunk.Preview()
	expected := "line1 line2 line3"
	if preview != expected {
		t.Errorf("Preview() = %q, want %q", preview, expected)
	}
}

func TestChunk_Preview_SignatureWithNewlines(t *testing.T) {
	// Some signatures might have newlines (e.g., multi-line function signatures)
	chunk := Chunk{
		Signature: "func LongFunction(\n\tparam1 string,\n\tparam2 int,\n) error",
		Content:   "ignored",
	}

	preview := chunk.Preview()
	if strings.Contains(preview, "\n") {
		t.Errorf("Preview() should not contain newlines even in signature, got %q", preview)
	}
	if strings.Contains(preview, "\t") {
		t.Errorf("Preview() should not contain tabs even in signature, got %q", preview)
	}
	// \n\t becomes "  " (space+space) since both are replaced independently
	expected := "func LongFunction(  param1 string,  param2 int, ) error"
	if preview != expected {
		t.Errorf("Preview() = %q, want %q", preview, expected)
	}
}

func TestSearchResult_PreviewField(t *testing.T) {
	result := SearchResult{
		Chunk: Chunk{
			ID:       "test-1",
			FilePath: "/test/file.go",
		},
		Score:     0.75,
		Relevance: "high",
		Preview:   "func Test() {}",
	}

	if result.Preview != "func Test() {}" {
		t.Errorf("SearchResult.Preview = %q, want 'func Test() {}'", result.Preview)
	}
}

func TestChunk_DomainField(t *testing.T) {
	chunk := Chunk{
		ID:       "test-1",
		FilePath: "/test/file.go",
		Domain:   "code",
	}

	if chunk.Domain != "code" {
		t.Errorf("Chunk.Domain = %q, want 'code'", chunk.Domain)
	}

	// Test JSON serialization includes domain
	data, err := json.Marshal(chunk)
	if err != nil {
		t.Fatalf("Failed to marshal Chunk: %v", err)
	}
	if !strings.Contains(string(data), `"domain":"code"`) {
		t.Errorf("JSON should contain domain field, got: %s", string(data))
	}
}

func TestChunk_DomainOmitEmpty(t *testing.T) {
	chunk := Chunk{
		ID:       "test-2",
		FilePath: "/test/file.go",
		// Domain not set
	}

	data, err := json.Marshal(chunk)
	if err != nil {
		t.Fatalf("Failed to marshal Chunk: %v", err)
	}
	if strings.Contains(string(data), "domain") {
		t.Errorf("JSON should omit empty domain field, got: %s", string(data))
	}
}
