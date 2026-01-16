package semantic

import (
	"strings"
	"testing"
)

func TestMarkdownChunker_HeaderDetection(t *testing.T) {
	chunker := NewMarkdownChunker(4000)

	tests := []struct {
		name       string
		content    string
		wantChunks int
		wantNames  []string
		wantLevels []int // header levels for verification
	}{
		{
			name: "single h1 header",
			content: `# Title

Content under title.`,
			wantChunks: 1,
			wantNames:  []string{"test > Title"},
		},
		{
			name: "h1 and h2 headers",
			content: `# Main Title

Introduction content.

## Section One

Section one content.`,
			wantChunks: 2,
			wantNames:  []string{"test > Main Title", "test > Main Title > Section One"},
		},
		{
			name: "all header levels h1-h6",
			content: `# H1
Content 1
## H2
Content 2
### H3
Content 3
#### H4
Content 4
##### H5
Content 5
###### H6
Content 6`,
			wantChunks: 6,
			wantNames: []string{
				"test > H1",
				"test > H1 > H2",
				"test > H1 > H2 > H3",
				"test > H1 > H2 > H3 > H4",
				"test > H1 > H2 > H3 > H4 > H5",
				"test > H1 > H2 > H3 > H4 > H5 > H6",
			},
		},
		{
			name: "header resets hierarchy",
			content: `# First

Content.

## Subsection

Sub content.

# Second

Different content.`,
			wantChunks: 3,
			wantNames:  []string{"test > First", "test > First > Subsection", "test > Second"},
		},
		{
			name: "header with special characters",
			content: `# Hello World! (v2.0)

Content here.`,
			wantChunks: 1,
			wantNames:  []string{"test > Hello World! (v2.0)"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chunks, err := chunker.Chunk("test.md", []byte(tt.content))
			if err != nil {
				t.Fatalf("Chunk() error = %v", err)
			}

			if len(chunks) != tt.wantChunks {
				t.Errorf("got %d chunks, want %d", len(chunks), tt.wantChunks)
				for i, c := range chunks {
					t.Logf("  chunk[%d]: name=%q, content=%q", i, c.Name, truncate(c.Content, 50))
				}
			}

			for i, wantName := range tt.wantNames {
				if i >= len(chunks) {
					t.Errorf("missing chunk[%d] with name %q", i, wantName)
					continue
				}
				if chunks[i].Name != wantName {
					t.Errorf("chunk[%d].Name = %q, want %q", i, chunks[i].Name, wantName)
				}
			}
		})
	}
}

func TestMarkdownChunker_HeaderWithContent(t *testing.T) {
	chunker := NewMarkdownChunker(4000)

	content := `# Main Section

This is the main section content.
It spans multiple lines.

## Subsection

This is subsection content.
`

	chunks, err := chunker.Chunk("test.md", []byte(content))
	if err != nil {
		t.Fatalf("Chunk() error = %v", err)
	}

	if len(chunks) != 2 {
		t.Fatalf("got %d chunks, want 2", len(chunks))
	}

	// First chunk should contain header AND its content
	if !strings.Contains(chunks[0].Content, "# Main Section") {
		t.Error("chunk[0] should contain the header line")
	}
	if !strings.Contains(chunks[0].Content, "main section content") {
		t.Error("chunk[0] should contain the content under the header")
	}
	if !strings.Contains(chunks[0].Content, "spans multiple lines") {
		t.Error("chunk[0] should contain all content until next header")
	}

	// First chunk should NOT contain subsection content
	if strings.Contains(chunks[0].Content, "subsection content") {
		t.Error("chunk[0] should not contain subsection content")
	}

	// Second chunk should contain its header and content
	if !strings.Contains(chunks[1].Content, "## Subsection") {
		t.Error("chunk[1] should contain the subsection header")
	}
	if !strings.Contains(chunks[1].Content, "subsection content") {
		t.Error("chunk[1] should contain subsection content")
	}
}

func TestMarkdownChunker_ContentAccumulation(t *testing.T) {
	chunker := NewMarkdownChunker(4000)

	content := `# Header

Line 1
Line 2
Line 3

Paragraph 2.

- List item 1
- List item 2

Final paragraph.
`

	chunks, err := chunker.Chunk("test.md", []byte(content))
	if err != nil {
		t.Fatalf("Chunk() error = %v", err)
	}

	if len(chunks) != 1 {
		t.Fatalf("got %d chunks, want 1", len(chunks))
	}

	// All content should be in single chunk
	for _, expected := range []string{"Line 1", "Line 2", "Line 3", "Paragraph 2", "List item 1", "List item 2", "Final paragraph"} {
		if !strings.Contains(chunks[0].Content, expected) {
			t.Errorf("chunk should contain %q", expected)
		}
	}
}

func TestMarkdownChunker_SupportedExtensions(t *testing.T) {
	chunker := NewMarkdownChunker(4000)
	exts := chunker.SupportedExtensions()

	want := map[string]bool{"md": true, "markdown": true}
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

// truncate helper for debug output
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
