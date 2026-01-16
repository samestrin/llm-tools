package semantic

import (
	"strconv"
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

func TestMarkdownChunker_FencedCodeBlockPreservation(t *testing.T) {
	chunker := NewMarkdownChunker(4000)

	tests := []struct {
		name         string
		content      string
		wantChunks   int
		wantContains []string // strings that should be in same chunk
		wantNotSplit string   // code block that must not be split
	}{
		{
			name:         "basic fenced code block",
			content:      "# Section\n\nSome text.\n\n```go\nfunc main() {\n    println(\"hello\")\n}\n```\n\nMore text.",
			wantChunks:   1,
			wantContains: []string{"```go", "func main()", "```"},
		},
		{
			name:         "code block with header-like content inside",
			content:      "# Real Header\n\n```markdown\n# This is NOT a header\n## Neither is this\n```\n\nAfter code.",
			wantChunks:   1, // Should NOT split on the fake headers inside code
			wantContains: []string{"# Real Header", "# This is NOT a header", "## Neither is this"},
		},
		{
			name:         "tilde fence",
			content:      "# Section\n\n~~~python\nprint('hello')\n~~~\n\nDone.",
			wantChunks:   1,
			wantContains: []string{"~~~python", "print('hello')", "~~~"},
		},
		{
			name:         "longer fence matches",
			content:      "# Section\n\n````\ncode with ``` inside\n````\n\nEnd.",
			wantChunks:   1,
			wantContains: []string{"````", "code with ``` inside"},
		},
		{
			name:         "multiple code blocks",
			content:      "# Section\n\n```\nblock1\n```\n\ntext\n\n```\nblock2\n```",
			wantChunks:   1,
			wantContains: []string{"block1", "block2"},
		},
		{
			name:       "code block then real header",
			content:    "# First\n\n```\ncode\n```\n\n# Second\n\nContent.",
			wantChunks: 2,
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
					t.Logf("  chunk[%d]: %q", i, truncate(c.Content, 80))
				}
			}

			// For single chunk tests, verify all expected content is together
			if tt.wantChunks == 1 && len(chunks) == 1 {
				for _, want := range tt.wantContains {
					if !strings.Contains(chunks[0].Content, want) {
						t.Errorf("chunk should contain %q", want)
					}
				}
			}
		})
	}
}

func TestMarkdownChunker_YAMLFrontmatter(t *testing.T) {
	chunker := NewMarkdownChunker(4000)

	tests := []struct {
		name         string
		content      string
		wantChunks   int
		wantNames    []string
		wantContains map[int][]string // chunk index -> content that should be there
	}{
		{
			name: "basic frontmatter",
			content: `---
title: My Document
date: 2026-01-16
---

# Introduction

Content here.`,
			wantChunks: 2,
			wantNames:  []string{"test:frontmatter", "test > Introduction"},
			wantContains: map[int][]string{
				0: {"title: My Document", "date: 2026-01-16"},
				1: {"# Introduction", "Content here"},
			},
		},
		{
			name: "frontmatter only",
			content: `---
key: value
---`,
			wantChunks: 1,
			wantNames:  []string{"test:frontmatter"},
			wantContains: map[int][]string{
				0: {"key: value"},
			},
		},
		{
			name: "no frontmatter",
			content: `# Header

Content without frontmatter.`,
			wantChunks: 1,
			wantNames:  []string{"test > Header"},
		},
		{
			name: "frontmatter-like content not at start",
			content: `# Header

---
not: frontmatter
---

More content.`,
			wantChunks: 1, // The --- in middle is NOT frontmatter
			wantNames:  []string{"test > Header"},
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
					t.Logf("  chunk[%d]: name=%q", i, c.Name)
				}
			}

			for i, wantName := range tt.wantNames {
				if i >= len(chunks) {
					continue
				}
				if chunks[i].Name != wantName {
					t.Errorf("chunk[%d].Name = %q, want %q", i, chunks[i].Name, wantName)
				}
			}

			for idx, contents := range tt.wantContains {
				if idx >= len(chunks) {
					continue
				}
				for _, want := range contents {
					if !strings.Contains(chunks[idx].Content, want) {
						t.Errorf("chunk[%d] should contain %q", idx, want)
					}
				}
			}
		})
	}
}

func TestMarkdownChunker_Preamble(t *testing.T) {
	chunker := NewMarkdownChunker(4000)

	content := `This is content before any header.

It has multiple paragraphs.

# First Header

Content after header.`

	chunks, err := chunker.Chunk("test.md", []byte(content))
	if err != nil {
		t.Fatalf("Chunk() error = %v", err)
	}

	if len(chunks) != 2 {
		t.Fatalf("got %d chunks, want 2", len(chunks))
	}

	// First chunk is preamble
	if !strings.HasPrefix(chunks[0].Name, "test:") {
		t.Errorf("preamble chunk should have name like 'test:1-N', got %q", chunks[0].Name)
	}
	if !strings.Contains(chunks[0].Content, "content before any header") {
		t.Error("preamble should contain content before header")
	}

	// Second chunk is header section
	if chunks[1].Name != "test > First Header" {
		t.Errorf("chunk[1].Name = %q, want %q", chunks[1].Name, "test > First Header")
	}
}

func TestMarkdownChunker_SizeBasedFallback(t *testing.T) {
	// Use moderate max chunk size to trigger fallback with multiple short lines
	chunker := NewMarkdownChunker(200)

	// Create a section with many lines that together exceed maxChunkSize
	var lines []string
	lines = append(lines, "# Header", "")
	for i := 0; i < 20; i++ {
		lines = append(lines, "This is line number "+strconv.Itoa(i+1)+" of content.")
	}
	lines = append(lines, "", "Final line.")
	largeContent := strings.Join(lines, "\n")

	chunks, err := chunker.Chunk("test.md", []byte(largeContent))
	if err != nil {
		t.Fatalf("Chunk() error = %v", err)
	}

	// Should split the large section into multiple chunks
	if len(chunks) < 2 {
		t.Errorf("expected large section to be split, got %d chunks (total size: %d)", len(chunks), len(largeContent))
	}

	// All chunks should be within reasonable size (with some buffer for line boundaries)
	for i, chunk := range chunks {
		// Allow buffer for line that might exceed slightly
		if len(chunk.Content) > 400 {
			t.Errorf("chunk[%d] too large: %d chars", i, len(chunk.Content))
		}
	}

	// Verify all content is preserved
	var allContent strings.Builder
	for _, chunk := range chunks {
		allContent.WriteString(chunk.Content)
		allContent.WriteString("\n")
	}
	// Check that key content is present
	if !strings.Contains(allContent.String(), "line number 1") {
		t.Error("missing content from beginning")
	}
	if !strings.Contains(allContent.String(), "line number 20") {
		t.Error("missing content from end")
	}
}

func TestMarkdownChunker_EdgeCases(t *testing.T) {
	chunker := NewMarkdownChunker(4000)

	tests := []struct {
		name       string
		content    string
		wantChunks int
		wantErr    bool
	}{
		{
			name:       "empty file",
			content:    "",
			wantChunks: 0,
		},
		{
			name:       "whitespace only",
			content:    "   \n\n   \t\n",
			wantChunks: 1, // Creates a preamble chunk
		},
		{
			name:       "no headers at all",
			content:    "Just some content\nwithout any headers\nat all.",
			wantChunks: 1,
		},
		{
			name:       "single header no content",
			content:    "# Just a Header",
			wantChunks: 1,
		},
		{
			name:       "header immediately followed by header",
			content:    "# First\n## Second\n### Third",
			wantChunks: 3,
		},
		{
			name:       "deeply nested headers",
			content:    "# H1\n## H2\n### H3\n#### H4\n##### H5\n###### H6\ncontent",
			wantChunks: 6,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chunks, err := chunker.Chunk("test.md", []byte(tt.content))

			if (err != nil) != tt.wantErr {
				t.Fatalf("Chunk() error = %v, wantErr %v", err, tt.wantErr)
			}

			if len(chunks) != tt.wantChunks {
				t.Errorf("got %d chunks, want %d", len(chunks), tt.wantChunks)
				for i, c := range chunks {
					t.Logf("  chunk[%d]: name=%q, content=%q", i, c.Name, truncate(c.Content, 50))
				}
			}
		})
	}
}

func TestMarkdownChunker_IndentedCodeBlockPreservation(t *testing.T) {
	chunker := NewMarkdownChunker(4000)

	tests := []struct {
		name         string
		content      string
		wantChunks   int
		wantContains []string
	}{
		{
			name: "indented code block (4 spaces)",
			content: `# Section

Here is some code:

    func example() {
        return 42
    }

After the code.`,
			wantChunks:   1,
			wantContains: []string{"func example()", "return 42"},
		},
		{
			name: "indented code with header-like line",
			content: `# Real Header

    # This looks like a header but is code
    ## Also code

Not code.`,
			wantChunks:   1,
			wantContains: []string{"# Real Header", "# This looks like a header but is code"},
		},
		{
			name:         "tab-indented code",
			content:      "# Section\n\n\tcode line 1\n\tcode line 2\n\nNot code.",
			wantChunks:   1,
			wantContains: []string{"code line 1", "code line 2"},
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
					t.Logf("  chunk[%d]: %q", i, truncate(c.Content, 80))
				}
			}

			if len(chunks) > 0 {
				for _, want := range tt.wantContains {
					if !strings.Contains(chunks[0].Content, want) {
						t.Errorf("chunk should contain %q", want)
					}
				}
			}
		})
	}
}

func TestMarkdownChunker_DefaultMaxChunkSize(t *testing.T) {
	// Test that NewMarkdownChunker uses default when 0 or negative is passed
	chunker := NewMarkdownChunker(0)
	if chunker.maxChunkSize != 4000 {
		t.Errorf("default maxChunkSize should be 4000, got %d", chunker.maxChunkSize)
	}

	chunker2 := NewMarkdownChunker(-100)
	if chunker2.maxChunkSize != 4000 {
		t.Errorf("negative maxChunkSize should default to 4000, got %d", chunker2.maxChunkSize)
	}
}

func TestMarkdownChunker_NegativeLineNumber(t *testing.T) {
	// Test itoa with edge case
	chunker := NewMarkdownChunker(4000)

	// File without extension
	content := `# Title

Content here.`
	chunks, err := chunker.Chunk("README", []byte(content))
	if err != nil {
		t.Fatalf("Chunk() error = %v", err)
	}

	if len(chunks) == 0 {
		t.Fatal("expected at least one chunk")
	}

	// Verify file without extension works
	if !strings.Contains(chunks[0].Name, "README") {
		t.Errorf("Name should contain filename, got %q", chunks[0].Name)
	}
}
