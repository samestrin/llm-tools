package semantic

import (
	"testing"
)

func TestChunkerFactory_Register(t *testing.T) {
	factory := NewChunkerFactory()

	mockChunker := &mockChunker{}

	factory.Register("go", mockChunker)

	chunker, ok := factory.GetChunker("go")
	if !ok {
		t.Error("GetChunker() should return registered chunker")
	}
	if chunker != mockChunker {
		t.Error("GetChunker() returned wrong chunker")
	}
}

func TestChunkerFactory_GetByExtension(t *testing.T) {
	factory := NewChunkerFactory()

	goChunker := &mockChunker{lang: "go"}
	jsChunker := &mockChunker{lang: "js"}

	factory.Register("go", goChunker)
	factory.Register("js", jsChunker)
	factory.Register("ts", jsChunker) // TypeScript uses same chunker

	tests := []struct {
		filename string
		wantLang string
		wantOk   bool
	}{
		{"main.go", "go", true},
		{"test_file.go", "go", true},
		{"app.js", "js", true},
		{"component.ts", "js", true},
		{"README.md", "", false},
		{"unknown.xyz", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			chunker, ok := factory.GetByExtension(tt.filename)
			if ok != tt.wantOk {
				t.Errorf("GetByExtension(%q) ok = %v, want %v", tt.filename, ok, tt.wantOk)
			}
			if ok && chunker.(*mockChunker).lang != tt.wantLang {
				t.Errorf("GetByExtension(%q) lang = %v, want %v", tt.filename, chunker.(*mockChunker).lang, tt.wantLang)
			}
		})
	}
}

func TestChunkerFactory_SupportedExtensions(t *testing.T) {
	factory := NewChunkerFactory()

	factory.Register("go", &mockChunker{})
	factory.Register("js", &mockChunker{})

	exts := factory.SupportedExtensions()

	if len(exts) != 2 {
		t.Errorf("SupportedExtensions() returned %d, want 2", len(exts))
	}

	// Check both extensions are present
	found := make(map[string]bool)
	for _, ext := range exts {
		found[ext] = true
	}

	if !found["go"] {
		t.Error("SupportedExtensions() missing 'go'")
	}
	if !found["js"] {
		t.Error("SupportedExtensions() missing 'js'")
	}
}

func TestLanguageFromExtension(t *testing.T) {
	tests := []struct {
		filename string
		want     string
	}{
		{"main.go", "go"},
		{"test.py", "py"},
		{"app.js", "js"},
		{"component.ts", "ts"},
		{"index.tsx", "tsx"},
		{"script.php", "php"},
		{"styles.css", "css"},
		{"data.json", "json"},
		{"config.yaml", "yaml"},
		{"config.yml", "yml"},
		{"README.md", "md"},
		{"Makefile", ""},
		{"noextension", ""},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			got := LanguageFromExtension(tt.filename)
			if got != tt.want {
				t.Errorf("LanguageFromExtension(%q) = %q, want %q", tt.filename, got, tt.want)
			}
		})
	}
}

// mockChunker is a test double for Chunker
type mockChunker struct {
	lang   string
	chunks []Chunk
	err    error
}

func (m *mockChunker) Chunk(path string, content []byte) ([]Chunk, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.chunks, nil
}

func (m *mockChunker) SupportedExtensions() []string {
	return []string{m.lang}
}
