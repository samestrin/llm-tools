package treesitter

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/samestrin/llm-tools/internal/semantic"
)

func readTestFile(t *testing.T, parts ...string) []byte {
	t.Helper()
	path := filepath.Join(append([]string{"testdata"}, parts...)...)
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read test file %s: %v", path, err)
	}
	return content
}

// --- Python Tests ---

func TestPythonChunker_BasicExtraction(t *testing.T) {
	chunker := NewPythonChunker()
	content := readTestFile(t, "python", "sample.py")
	chunks, err := chunker.Chunk("sample.py", content)
	if err != nil {
		t.Fatalf("Chunk failed: %v", err)
	}

	if len(chunks) == 0 {
		t.Fatal("Expected chunks, got none")
	}

	// Verify we found expected constructs
	found := map[string]semantic.ChunkType{}
	for _, c := range chunks {
		found[c.Name] = c.Type
	}

	// Should find the class
	if typ, ok := found["UserService"]; !ok {
		t.Error("Expected to find UserService class")
	} else if typ != semantic.ChunkStruct {
		t.Errorf("UserService should be ChunkStruct, got %v", typ)
	}

	// Should find methods
	if typ, ok := found["get_user"]; !ok {
		t.Error("Expected to find get_user method")
	} else if typ != semantic.ChunkMethod {
		t.Errorf("get_user should be ChunkMethod, got %v", typ)
	}

	// Should find standalone function
	if _, ok := found["standalone_function"]; !ok {
		t.Error("Expected to find standalone_function")
	}

	// Should find nested_example
	if _, ok := found["nested_example"]; !ok {
		t.Error("Expected to find nested_example")
	}
}

func TestPythonChunker_ChunkFields(t *testing.T) {
	chunker := NewPythonChunker()
	content := readTestFile(t, "python", "sample.py")
	chunks, _ := chunker.Chunk("sample.py", content)

	for _, c := range chunks {
		if c.ID == "" {
			t.Errorf("Chunk %q has empty ID", c.Name)
		}
		if c.Language != "py" {
			t.Errorf("Chunk %q has language %q, want 'py'", c.Name, c.Language)
		}
		if c.StartLine <= 0 {
			t.Errorf("Chunk %q has invalid StartLine: %d", c.Name, c.StartLine)
		}
		if c.EndLine < c.StartLine {
			t.Errorf("Chunk %q has EndLine %d < StartLine %d", c.Name, c.EndLine, c.StartLine)
		}
		if c.Content == "" {
			t.Errorf("Chunk %q has empty Content", c.Name)
		}
	}
}

func TestPythonChunker_Extensions(t *testing.T) {
	chunker := NewPythonChunker()
	exts := chunker.SupportedExtensions()
	if len(exts) != 3 {
		t.Errorf("Expected 3 extensions, got %d: %v", len(exts), exts)
	}
}

// --- JavaScript/TypeScript Tests ---

func TestJSChunker_TypeScriptExtraction(t *testing.T) {
	chunker := NewJSChunker()
	content := readTestFile(t, "javascript", "sample.ts")
	chunks, err := chunker.Chunk("sample.ts", content)
	if err != nil {
		t.Fatalf("Chunk failed: %v", err)
	}

	if len(chunks) == 0 {
		t.Fatal("Expected chunks, got none")
	}

	found := map[string]semantic.ChunkType{}
	for _, c := range chunks {
		found[c.Name] = c.Type
	}

	// Should find interface
	if typ, ok := found["UserDTO"]; !ok {
		t.Error("Expected to find UserDTO interface")
	} else if typ != semantic.ChunkInterface {
		t.Errorf("UserDTO should be ChunkInterface, got %v", typ)
	}

	// Should find class
	if typ, ok := found["UserController"]; !ok {
		t.Error("Expected to find UserController class")
	} else if typ != semantic.ChunkStruct {
		t.Errorf("UserController should be ChunkStruct, got %v", typ)
	}

	// Should find function
	if _, ok := found["createApp"]; !ok {
		t.Error("Expected to find createApp function")
	}

	// Should find type alias
	if _, ok := found["UserRole"]; !ok {
		t.Error("Expected to find UserRole type alias")
	}
}

func TestJSChunker_ChunkFields(t *testing.T) {
	chunker := NewJSChunker()
	content := readTestFile(t, "javascript", "sample.ts")
	chunks, _ := chunker.Chunk("sample.ts", content)

	for _, c := range chunks {
		if c.ID == "" {
			t.Errorf("Chunk %q has empty ID", c.Name)
		}
		if c.Language != "ts" {
			t.Errorf("Chunk %q has language %q, want 'ts'", c.Name, c.Language)
		}
		if c.StartLine <= 0 || c.EndLine < c.StartLine {
			t.Errorf("Chunk %q has invalid line range: %d-%d", c.Name, c.StartLine, c.EndLine)
		}
	}
}

func TestJSChunker_Extensions(t *testing.T) {
	chunker := NewJSChunker()
	exts := chunker.SupportedExtensions()
	if len(exts) != 6 {
		t.Errorf("Expected 6 extensions, got %d: %v", len(exts), exts)
	}
}

// --- Rust Tests ---

func TestRustChunker_BasicExtraction(t *testing.T) {
	chunker := NewRustChunker()
	content := readTestFile(t, "rust", "sample.rs")
	chunks, err := chunker.Chunk("sample.rs", content)
	if err != nil {
		t.Fatalf("Chunk failed: %v", err)
	}

	if len(chunks) == 0 {
		t.Fatal("Expected chunks, got none")
	}

	found := map[string]semantic.ChunkType{}
	for _, c := range chunks {
		found[c.Name] = c.Type
	}

	// Should find struct
	if typ, ok := found["Config"]; !ok {
		t.Error("Expected to find Config struct")
	} else if typ != semantic.ChunkStruct {
		t.Errorf("Config should be ChunkStruct, got %v", typ)
	}

	// Should find enum
	if _, ok := found["Status"]; !ok {
		t.Error("Expected to find Status enum")
	}

	// Should find trait
	if typ, ok := found["Repository"]; !ok {
		t.Error("Expected to find Repository trait")
	} else if typ != semantic.ChunkInterface {
		t.Errorf("Repository should be ChunkInterface, got %v", typ)
	}

	// Should find standalone function
	if _, ok := found["process_data"]; !ok {
		t.Error("Expected to find process_data function")
	}

	// Should find type alias
	if _, ok := found["ConfigMap"]; !ok {
		t.Error("Expected to find ConfigMap type alias")
	}
}

func TestRustChunker_ImplMethods(t *testing.T) {
	chunker := NewRustChunker()
	content := readTestFile(t, "rust", "sample.rs")
	chunks, _ := chunker.Chunk("sample.rs", content)

	// Should find methods from impl blocks
	methods := map[string]bool{}
	for _, c := range chunks {
		if c.Type == semantic.ChunkMethod {
			methods[c.Name] = true
		}
	}

	if !methods["new"] {
		t.Error("Expected to find 'new' method from impl Config")
	}
	if !methods["with_timeout"] {
		t.Error("Expected to find 'with_timeout' method from impl Config")
	}
}

func TestRustChunker_ChunkFields(t *testing.T) {
	chunker := NewRustChunker()
	content := readTestFile(t, "rust", "sample.rs")
	chunks, _ := chunker.Chunk("sample.rs", content)

	for _, c := range chunks {
		if c.Language != "rs" {
			t.Errorf("Chunk %q has language %q, want 'rs'", c.Name, c.Language)
		}
	}
}

// --- Shared Helpers Tests ---

func TestTruncateContent(t *testing.T) {
	short := "hello world"
	if got := TruncateContent(short, 100); got != short {
		t.Errorf("Short content should not be truncated")
	}

	long := "line1\nline2\nline3\nline4\nline5"
	truncated := TruncateContent(long, 15)
	if len(truncated) > 30 { // 15 + truncation marker
		t.Errorf("Truncated content too long: %d", len(truncated))
	}
}

func TestGetLangEntry(t *testing.T) {
	entry := GetLangEntry("py")
	if entry == nil {
		t.Fatal("Expected language entry for .py")
	}

	entry = GetLangEntry("nonexistent_extension_xyz")
	if entry != nil {
		t.Error("Expected nil for unknown extension")
	}
}
