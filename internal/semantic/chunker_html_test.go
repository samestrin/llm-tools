package semantic

import (
	"strings"
	"testing"
)

func TestHTMLChunker_SemanticElements(t *testing.T) {
	chunker := NewHTMLChunker(4000)

	tests := []struct {
		name         string
		content      string
		wantChunks   int
		wantContains map[int][]string
	}{
		{
			name: "section elements",
			content: `<!DOCTYPE html>
<html>
<body>
<section>
  <h1>First Section</h1>
  <p>First section content.</p>
</section>
<section>
  <h2>Second Section</h2>
  <p>Second section content.</p>
</section>
</body>
</html>`,
			wantChunks: 2,
			wantContains: map[int][]string{
				0: {"First Section", "First section content"},
				1: {"Second Section", "Second section content"},
			},
		},
		{
			name: "article elements",
			content: `<html>
<body>
<article>
  <h1>Article Title</h1>
  <p>Article body.</p>
</article>
</body>
</html>`,
			wantChunks: 1,
			wantContains: map[int][]string{
				0: {"Article Title", "Article body"},
			},
		},
		{
			name: "main and aside",
			content: `<html>
<body>
<main>
  <p>Main content here.</p>
</main>
<aside>
  <p>Sidebar content.</p>
</aside>
</body>
</html>`,
			wantChunks: 2,
			wantContains: map[int][]string{
				0: {"Main content"},
				1: {"Sidebar content"},
			},
		},
		{
			name: "nested semantic elements",
			content: `<html>
<body>
<section>
  <article>
    <p>Nested article.</p>
  </article>
</section>
</body>
</html>`,
			wantChunks: 1, // Article inside section should be single chunk
			wantContains: map[int][]string{
				0: {"Nested article"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chunks, err := chunker.Chunk("test.html", []byte(tt.content))
			if err != nil {
				t.Fatalf("Chunk() error = %v", err)
			}

			if len(chunks) != tt.wantChunks {
				t.Errorf("got %d chunks, want %d", len(chunks), tt.wantChunks)
				for i, c := range chunks {
					t.Logf("  chunk[%d]: %q", i, truncate(c.Content, 80))
				}
			}

			for idx, contents := range tt.wantContains {
				if idx >= len(chunks) {
					continue
				}
				for _, want := range contents {
					if !strings.Contains(chunks[idx].Content, want) {
						t.Errorf("chunk[%d] should contain %q, got %q", idx, want, truncate(chunks[idx].Content, 100))
					}
				}
			}
		})
	}
}

func TestHTMLChunker_DivFallback(t *testing.T) {
	chunker := NewHTMLChunker(4000)

	// HTML without semantic elements - should fall back to divs
	content := `<html>
<body>
<div class="section1">
  <p>First div content.</p>
</div>
<div class="section2">
  <p>Second div content.</p>
</div>
</body>
</html>`

	chunks, err := chunker.Chunk("test.html", []byte(content))
	if err != nil {
		t.Fatalf("Chunk() error = %v", err)
	}

	// Should get at least one chunk with content
	if len(chunks) == 0 {
		t.Error("expected at least one chunk")
	}

	// Content should be extracted
	found := false
	for _, chunk := range chunks {
		if strings.Contains(chunk.Content, "First div content") ||
			strings.Contains(chunk.Content, "Second div content") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected div content to be extracted")
	}
}

func TestHTMLChunker_TextExtraction(t *testing.T) {
	chunker := NewHTMLChunker(4000)

	content := `<html>
<body>
<section>
  <h1>Title</h1>
  <p>Paragraph one.</p>
  <p>Paragraph two.</p>
  <ul>
    <li>Item 1</li>
    <li>Item 2</li>
  </ul>
</section>
</body>
</html>`

	chunks, err := chunker.Chunk("test.html", []byte(content))
	if err != nil {
		t.Fatalf("Chunk() error = %v", err)
	}

	if len(chunks) == 0 {
		t.Fatal("expected at least one chunk")
	}

	// All text should be extracted
	for _, want := range []string{"Title", "Paragraph one", "Paragraph two", "Item 1", "Item 2"} {
		if !strings.Contains(chunks[0].Content, want) {
			t.Errorf("chunk should contain %q", want)
		}
	}
}

func TestHTMLChunker_SupportedExtensions(t *testing.T) {
	chunker := NewHTMLChunker(4000)
	exts := chunker.SupportedExtensions()

	want := map[string]bool{"html": true, "htm": true}
	got := make(map[string]bool)
	for _, ext := range exts {
		got[ext] = true
	}

	for ext := range want {
		if !got[ext] {
			t.Errorf("missing extension %q", ext)
		}
	}
}

// Adversarial tests for edge cases
func TestHTMLChunker_EdgeCases(t *testing.T) {
	chunker := NewHTMLChunker(4000)

	tests := []struct {
		name        string
		content     string
		wantChunks  int
		description string
	}{
		{
			name:        "empty document",
			content:     "",
			wantChunks:  0,
			description: "empty input should return no chunks",
		},
		{
			name:        "whitespace only",
			content:     "   \n\t  \n  ",
			wantChunks:  0,
			description: "whitespace-only should return no chunks",
		},
		{
			name:        "script only document",
			content:     `<html><body><script>alert('xss');</script></body></html>`,
			wantChunks:  0,
			description: "document with only script should return no chunks",
		},
		{
			name:        "style only document",
			content:     `<html><body><style>body{color:red;}</style></body></html>`,
			wantChunks:  0,
			description: "document with only style should return no chunks",
		},
		{
			name:        "deeply nested structure",
			content:     `<html><body><div><div><div><div><div><div><div><div><p>Deep content</p></div></div></div></div></div></div></div></div></body></html>`,
			wantChunks:  1,
			description: "deeply nested HTML should still extract content",
		},
		{
			name:        "malformed unclosed tags",
			content:     `<html><body><section><p>Unclosed paragraph<p>Another paragraph</section></body></html>`,
			wantChunks:  1,
			description: "unclosed tags should be handled gracefully",
		},
		{
			name:        "invalid UTF-8 bytes",
			content:     "<html><body><section>Valid text \xff\xfe invalid bytes</section></body></html>",
			wantChunks:  1,
			description: "invalid UTF-8 should not crash",
		},
		{
			name:        "no body tag",
			content:     `<html><section><p>Content without body tag</p></section></html>`,
			wantChunks:  1,
			description: "document without body tag should still work",
		},
		{
			name:        "plain text only",
			content:     `Just some plain text without any HTML tags at all.`,
			wantChunks:  1,
			description: "plain text should be returned as single chunk",
		},
		{
			name:        "html entities",
			content:     `<html><body><section>&lt;script&gt;alert('xss')&lt;/script&gt;</section></body></html>`,
			wantChunks:  1,
			description: "HTML entities should be decoded in output",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chunks, err := chunker.Chunk("test.html", []byte(tt.content))
			if err != nil {
				t.Fatalf("Chunk() error = %v (%s)", err, tt.description)
			}

			if len(chunks) != tt.wantChunks {
				t.Errorf("got %d chunks, want %d (%s)", len(chunks), tt.wantChunks, tt.description)
				for i, c := range chunks {
					t.Logf("  chunk[%d]: %q", i, truncate(c.Content, 80))
				}
			}
		})
	}
}

func TestHTMLChunker_ScriptStripping(t *testing.T) {
	chunker := NewHTMLChunker(4000)

	// Script content should never appear in output (security check)
	content := `<html>
<body>
<section>
  <p>Safe content</p>
  <script>alert('xss'); document.cookie;</script>
  <p>More safe content</p>
</section>
</body>
</html>`

	chunks, err := chunker.Chunk("test.html", []byte(content))
	if err != nil {
		t.Fatalf("Chunk() error = %v", err)
	}

	for _, chunk := range chunks {
		if strings.Contains(chunk.Content, "alert") ||
			strings.Contains(chunk.Content, "document.cookie") ||
			strings.Contains(chunk.Content, "script") {
			t.Errorf("script content leaked into chunk: %q", truncate(chunk.Content, 200))
		}
	}

	// Verify safe content is preserved
	if len(chunks) == 0 {
		t.Fatal("expected at least one chunk")
	}
	if !strings.Contains(chunks[0].Content, "Safe content") {
		t.Error("safe content should be preserved")
	}
}

func TestHTMLChunker_NavigationStripping(t *testing.T) {
	chunker := NewHTMLChunker(4000)

	content := `<html>
<body>
<nav><a href="/">Home</a><a href="/about">About</a></nav>
<header><h1>Site Header</h1></header>
<main>
  <p>Main content that matters.</p>
</main>
<footer>Copyright 2024</footer>
</body>
</html>`

	chunks, err := chunker.Chunk("test.html", []byte(content))
	if err != nil {
		t.Fatalf("Chunk() error = %v", err)
	}

	for _, chunk := range chunks {
		// nav, header, footer content should be stripped
		if strings.Contains(chunk.Content, "Home") ||
			strings.Contains(chunk.Content, "About") ||
			strings.Contains(chunk.Content, "Site Header") ||
			strings.Contains(chunk.Content, "Copyright") {
			t.Errorf("navigation/header/footer content leaked: %q", truncate(chunk.Content, 200))
		}
	}

	// Main content should be preserved
	found := false
	for _, chunk := range chunks {
		if strings.Contains(chunk.Content, "Main content that matters") {
			found = true
			break
		}
	}
	if !found {
		t.Error("main content should be preserved")
	}
}

func TestHTMLChunker_PreserveElements(t *testing.T) {
	chunker := NewHTMLChunker(4000)

	tests := []struct {
		name         string
		content      string
		wantContains []string
		description  string
	}{
		{
			name: "pre element whitespace preserved",
			content: `<html><body><section>
<pre>
  function hello() {
    console.log("world");
  }
</pre>
</section></body></html>`,
			wantContains: []string{"function hello()", "console.log"},
			description:  "pre element content should be preserved with structure",
		},
		{
			name: "code element preserved",
			content: `<html><body><section>
<p>Use the <code>fmt.Println()</code> function to print.</p>
</section></body></html>`,
			wantContains: []string{"fmt.Println()"},
			description:  "inline code element content should be preserved",
		},
		{
			name: "table structure preserved",
			content: `<html><body><section>
<table>
  <tr><th>Name</th><th>Value</th></tr>
  <tr><td>foo</td><td>bar</td></tr>
</table>
</section></body></html>`,
			wantContains: []string{"Name", "Value", "foo", "bar"},
			description:  "table content should be extracted",
		},
		{
			name: "blockquote preserved",
			content: `<html><body><section>
<blockquote>
  To be or not to be, that is the question.
</blockquote>
</section></body></html>`,
			wantContains: []string{"To be or not to be"},
			description:  "blockquote content should be preserved",
		},
		{
			name: "nested pre in section",
			content: `<html><body><section>
<p>Introduction text.</p>
<pre><code>
package main

import "fmt"

func main() {
    fmt.Println("Hello")
}
</code></pre>
<p>Conclusion text.</p>
</section></body></html>`,
			wantContains: []string{"Introduction text", "package main", "fmt.Println", "Conclusion text"},
			description:  "nested pre/code should preserve code structure",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chunks, err := chunker.Chunk("test.html", []byte(tt.content))
			if err != nil {
				t.Fatalf("Chunk() error = %v (%s)", err, tt.description)
			}

			if len(chunks) == 0 {
				t.Fatalf("expected at least one chunk (%s)", tt.description)
			}

			// Check all expected content is present
			for _, want := range tt.wantContains {
				found := false
				for _, chunk := range chunks {
					if strings.Contains(chunk.Content, want) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("should contain %q (%s)", want, tt.description)
					for i, c := range chunks {
						t.Logf("  chunk[%d]: %q", i, truncate(c.Content, 200))
					}
				}
			}
		})
	}
}

func TestHTMLChunker_BrTagHandling(t *testing.T) {
	chunker := NewHTMLChunker(4000)

	content := `<html><body><section>
<p>First line<br>Second line<br/>Third line</p>
</section></body></html>`

	chunks, err := chunker.Chunk("test.html", []byte(content))
	if err != nil {
		t.Fatalf("Chunk() error = %v", err)
	}

	if len(chunks) == 0 {
		t.Fatal("expected at least one chunk")
	}

	// br tags should create line breaks
	if !strings.Contains(chunks[0].Content, "First line") ||
		!strings.Contains(chunks[0].Content, "Second line") ||
		!strings.Contains(chunks[0].Content, "Third line") {
		t.Errorf("br tags should preserve line content, got: %q", chunks[0].Content)
	}
}

func TestHTMLChunker_NoscriptStripping(t *testing.T) {
	chunker := NewHTMLChunker(4000)

	content := `<html><body>
<section>
  <p>Visible content.</p>
  <noscript>JavaScript is disabled. Please enable it.</noscript>
  <p>More visible content.</p>
</section>
</body></html>`

	chunks, err := chunker.Chunk("test.html", []byte(content))
	if err != nil {
		t.Fatalf("Chunk() error = %v", err)
	}

	for _, chunk := range chunks {
		if strings.Contains(chunk.Content, "JavaScript is disabled") {
			t.Errorf("noscript content should be stripped, got: %q", truncate(chunk.Content, 200))
		}
	}

	// Visible content should be preserved
	found := false
	for _, chunk := range chunks {
		if strings.Contains(chunk.Content, "Visible content") {
			found = true
			break
		}
	}
	if !found {
		t.Error("visible content should be preserved")
	}
}

func TestHTMLChunker_MalformedHTML(t *testing.T) {
	chunker := NewHTMLChunker(4000)

	tests := []struct {
		name         string
		content      string
		wantErr      bool
		wantChunks   int
		wantContains []string
		description  string
	}{
		{
			name:         "unclosed paragraph tags",
			content:      `<html><body><section><p>First paragraph<p>Second paragraph<p>Third paragraph</section></body></html>`,
			wantErr:      false,
			wantChunks:   1,
			wantContains: []string{"First paragraph", "Second paragraph", "Third paragraph"},
			description:  "unclosed <p> tags should still extract all text",
		},
		{
			name:         "mismatched tags",
			content:      `<html><body><section><div><p>Content</div></p></section></body></html>`,
			wantErr:      false,
			wantChunks:   1,
			wantContains: []string{"Content"},
			description:  "mismatched closing tags should still extract text",
		},
		{
			name:         "missing closing tags",
			content:      `<html><body><section><div><span>Nested content`,
			wantErr:      false,
			wantChunks:   1,
			wantContains: []string{"Nested content"},
			description:  "missing closing tags should still extract text",
		},
		{
			name:         "partial HTML fragment",
			content:      `<section><p>Just a section</p></section>`,
			wantErr:      false,
			wantChunks:   1,
			wantContains: []string{"Just a section"},
			description:  "HTML fragments should be parsed successfully",
		},
		{
			name:         "multiple root elements",
			content:      `<section>First</section><section>Second</section>`,
			wantErr:      false,
			wantChunks:   2,
			wantContains: []string{"First", "Second"},
			description:  "multiple root elements should create separate chunks",
		},
		{
			name:         "missing attribute quotes",
			content:      `<html><body><section><a href=broken>Link text</a><p>Paragraph</p></section></body></html>`,
			wantErr:      false,
			wantChunks:   1,
			wantContains: []string{"Link text", "Paragraph"},
			description:  "missing attribute quotes should still parse",
		},
		{
			name:         "completely invalid markup",
			content:      `<<<>>>not html at all>>><<<`,
			wantErr:      false,
			wantChunks:   1,
			wantContains: []string{"not html at all"},
			description:  "invalid markup should fall back to text extraction",
		},
		{
			name:         "CDATA sections",
			content:      `<html><body><section>Before<![CDATA[raw <content> here]]>After</section></body></html>`,
			wantErr:      false,
			wantChunks:   1,
			wantContains: []string{"Before", "After"},
			description:  "CDATA sections should be handled",
		},
		{
			name:         "doctype variations",
			content:      `<!DOCTYPE html PUBLIC "-//W3C//DTD XHTML 1.0 Transitional//EN"><html><body><section>Content</section></body></html>`,
			wantErr:      false,
			wantChunks:   1,
			wantContains: []string{"Content"},
			description:  "various doctypes should be handled",
		},
		{
			name:         "self-closing tags",
			content:      `<html><body><section><img src="test.png"/><br/><hr/><p>Text content</p></section></body></html>`,
			wantErr:      false,
			wantChunks:   1,
			wantContains: []string{"Text content"},
			description:  "self-closing tags should be handled",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chunks, err := chunker.Chunk("test.html", []byte(tt.content))

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error (%s)", tt.description)
				}
				return
			}

			if err != nil {
				t.Fatalf("Chunk() error = %v (%s)", err, tt.description)
			}

			if len(chunks) != tt.wantChunks {
				t.Errorf("got %d chunks, want %d (%s)", len(chunks), tt.wantChunks, tt.description)
				for i, c := range chunks {
					t.Logf("  chunk[%d]: %q", i, truncate(c.Content, 100))
				}
			}

			// Check expected content is present
			for _, want := range tt.wantContains {
				found := false
				for _, chunk := range chunks {
					if strings.Contains(chunk.Content, want) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("should contain %q (%s)", want, tt.description)
					for i, c := range chunks {
						t.Logf("  chunk[%d]: %q", i, truncate(c.Content, 100))
					}
				}
			}
		})
	}
}

func TestHTMLChunker_ChunkMetadata(t *testing.T) {
	chunker := NewHTMLChunker(4000)

	content := `<html>
<body>
<section>
  <h1>Main Section</h1>
  <p>Section content here.</p>
</section>
</body>
</html>`

	chunks, err := chunker.Chunk("example.html", []byte(content))
	if err != nil {
		t.Fatalf("Chunk() error = %v", err)
	}

	if len(chunks) == 0 {
		t.Fatal("expected at least one chunk")
	}

	chunk := chunks[0]

	// Verify metadata
	if chunk.FilePath != "example.html" {
		t.Errorf("FilePath = %q, want %q", chunk.FilePath, "example.html")
	}

	if chunk.Type != ChunkFile {
		t.Errorf("Type = %v, want ChunkFile", chunk.Type)
	}

	if chunk.Language != "html" {
		t.Errorf("Language = %q, want %q", chunk.Language, "html")
	}

	if chunk.ID == "" {
		t.Error("ID should not be empty")
	}

	if !strings.Contains(chunk.Name, "example") {
		t.Errorf("Name should contain filename, got %q", chunk.Name)
	}
}

func TestHTMLChunker_DefaultMaxChunkSize(t *testing.T) {
	// Test that NewHTMLChunker uses default when 0 or negative is passed
	chunker := NewHTMLChunker(0)
	if chunker.maxChunkSize != 4000 {
		t.Errorf("default maxChunkSize should be 4000, got %d", chunker.maxChunkSize)
	}

	chunker2 := NewHTMLChunker(-100)
	if chunker2.maxChunkSize != 4000 {
		t.Errorf("negative maxChunkSize should default to 4000, got %d", chunker2.maxChunkSize)
	}
}
