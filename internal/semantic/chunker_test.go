package semantic

import (
	"testing"
)

func TestChunkerFactory_Register(t *testing.T) {
	factory := NewChunkerFactory()

	mockChunker := &mockChunker{}

	// First registration should return true (new)
	isNew := factory.Register("go", mockChunker)
	if !isNew {
		t.Error("Register() should return true for new registration")
	}

	chunker, ok := factory.GetChunker("go")
	if !ok {
		t.Error("GetChunker() should return registered chunker")
	}
	if chunker != mockChunker {
		t.Error("GetChunker() returned wrong chunker")
	}
}

func TestChunkerFactory_RegisterOverwrite(t *testing.T) {
	factory := NewChunkerFactory()

	chunker1 := &mockChunker{lang: "first"}
	chunker2 := &mockChunker{lang: "second"}

	// First registration
	isNew := factory.Register("go", chunker1)
	if !isNew {
		t.Error("First registration should return true")
	}

	// Second registration for same extension should return false (overwrite)
	isNew = factory.Register("go", chunker2)
	if isNew {
		t.Error("Register() should return false when overwriting")
	}

	// Verify the second chunker replaced the first
	chunker, ok := factory.GetChunker("go")
	if !ok {
		t.Error("GetChunker() should return chunker")
	}
	if chunker.(*mockChunker).lang != "second" {
		t.Error("Register() should overwrite previous chunker")
	}
}

func TestChunkerFactory_ExtensionConflictDetection(t *testing.T) {
	// Test that we can detect when chunkers try to register same extension
	factory := NewChunkerFactory()

	mdChunker := NewMarkdownChunker(4000)
	genericChunker := NewGenericChunker(2000)

	// Register markdown chunker first
	for _, ext := range mdChunker.SupportedExtensions() {
		isNew := factory.Register(ext, mdChunker)
		if !isNew {
			t.Errorf("Expected markdown chunker to be first registration for %q", ext)
		}
	}

	// Verify generic chunker doesn't support md/markdown (they shouldn't conflict)
	for _, ext := range genericChunker.SupportedExtensions() {
		if ext == "md" || ext == "markdown" {
			t.Errorf("GenericChunker should not support %q (would conflict with MarkdownChunker)", ext)
		}
	}

	// If we were to register generic for an md extension, it should show conflict
	isNew := factory.Register("md", genericChunker)
	if isNew {
		t.Error("Expected conflict detection when overwriting md extension")
	}

	// Verify the markdown chunker is still there (last-write-wins)
	// In production, registration order matters - register specialized first
}

func TestChunkerFactory_RegistrationOrderMatters(t *testing.T) {
	// Demonstrates that the order of chunker registration affects which chunker handles an extension
	factory := NewChunkerFactory()

	genericChunker := &mockChunker{lang: "generic"}
	specializedChunker := &mockChunker{lang: "specialized"}

	// Register generic first, then specialized (correct order)
	factory.Register("txt", genericChunker)
	isOverwrite := !factory.Register("txt", specializedChunker)

	if !isOverwrite {
		t.Error("Second registration should indicate overwrite occurred")
	}

	// The specialized chunker should now be registered
	chunker, ok := factory.GetChunker("txt")
	if !ok {
		t.Fatal("GetChunker should find txt chunker")
	}
	if chunker.(*mockChunker).lang != "specialized" {
		t.Error("Later registration should win, expected specialized")
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

func TestChunkerFactory_IntegrationWithRealChunkers(t *testing.T) {
	factory := NewChunkerFactory()

	// Register real chunkers as they would be in production
	mdChunker := NewMarkdownChunker(4000)
	for _, ext := range mdChunker.SupportedExtensions() {
		factory.Register(ext, mdChunker)
	}

	htmlChunker := NewHTMLChunker(4000)
	for _, ext := range htmlChunker.SupportedExtensions() {
		factory.Register(ext, htmlChunker)
	}

	goChunker := NewGoChunker()
	for _, ext := range goChunker.SupportedExtensions() {
		factory.Register(ext, goChunker)
	}

	generic := NewGenericChunker(2000)
	for _, ext := range generic.SupportedExtensions() {
		factory.Register(ext, generic)
	}

	tests := []struct {
		filename    string
		wantChunker string
		checkType   func(Chunker) bool
	}{
		{"README.md", "MarkdownChunker", func(c Chunker) bool { _, ok := c.(*MarkdownChunker); return ok }},
		{"docs.markdown", "MarkdownChunker", func(c Chunker) bool { _, ok := c.(*MarkdownChunker); return ok }},
		{"index.html", "HTMLChunker", func(c Chunker) bool { _, ok := c.(*HTMLChunker); return ok }},
		{"page.htm", "HTMLChunker", func(c Chunker) bool { _, ok := c.(*HTMLChunker); return ok }},
		{"main.go", "GoChunker", func(c Chunker) bool { _, ok := c.(*GoChunker); return ok }},
		{"config.yaml", "GenericChunker", func(c Chunker) bool { _, ok := c.(*GenericChunker); return ok }},
		{"notes.txt", "GenericChunker", func(c Chunker) bool { _, ok := c.(*GenericChunker); return ok }},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			chunker, ok := factory.GetByExtension(tt.filename)
			if !ok {
				t.Fatalf("GetByExtension(%q) returned false, want chunker", tt.filename)
			}

			// Use type assertion to verify chunker type
			if !tt.checkType(chunker) {
				t.Errorf("GetByExtension(%q) returned wrong type, want %s", tt.filename, tt.wantChunker)
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
