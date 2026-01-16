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
	if chunks[0].Name != "test:preamble" {
		t.Errorf("preamble chunk should have name 'test:preamble', got %q", chunks[0].Name)
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

	// All chunks should be within reasonable size
	// Buffer allows for: maxChunkSize + longest line (since lines aren't split mid-line)
	maxLineLen := len("This is line number 20 of content.") // ~35 chars
	maxAllowedSize := 200 + maxLineLen + 50                 // chunker size + line + margin
	for i, chunk := range chunks {
		if len(chunk.Content) > maxAllowedSize {
			t.Errorf("chunk[%d] too large: %d chars (max allowed: %d)", i, len(chunk.Content), maxAllowedSize)
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

func TestMarkdownChunker_FileWithoutExtension(t *testing.T) {
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

func TestMarkdownChunker_EmptyPath(t *testing.T) {
	chunker := NewMarkdownChunker(4000)

	content := `# Title

Content here.`
	_, err := chunker.Chunk("", []byte(content))
	if err == nil {
		t.Error("expected error for empty path, got nil")
	}
}

func TestMarkdownChunker_UnclosedFencedCodeBlock(t *testing.T) {
	chunker := NewMarkdownChunker(4000)

	// Fenced code block that never closes
	content := "# Title\n\n```go\nfunc main() {\n    fmt.Println(\"hello\")\n\n# This should NOT be detected as a header"

	chunks, err := chunker.Chunk("test.md", []byte(content))
	if err != nil {
		t.Fatalf("Chunk() error = %v", err)
	}

	// Should have only one chunk since we're inside an unclosed fence
	if len(chunks) != 1 {
		t.Errorf("expected 1 chunk for unclosed fence, got %d", len(chunks))
	}

	// The "# This should NOT..." line should be in the content, not a separate header chunk
	if !strings.Contains(chunks[0].Content, "# This should NOT") {
		t.Error("unclosed fence should contain the hash line as content, not treat it as header")
	}
}

func TestMarkdownChunker_UnclosedFrontmatter(t *testing.T) {
	chunker := NewMarkdownChunker(4000)

	// Frontmatter that never closes
	content := `---
title: Test
author: Someone

# Title

Content here.`

	chunks, err := chunker.Chunk("test.md", []byte(content))
	if err != nil {
		t.Fatalf("Chunk() error = %v", err)
	}

	// Unclosed frontmatter should not extract frontmatter chunk
	for _, chunk := range chunks {
		if strings.Contains(chunk.Name, "frontmatter") {
			t.Error("unclosed frontmatter should not create frontmatter chunk")
		}
	}
}

func TestMarkdownChunker_FrontmatterOnlyWithTrailingWhitespace(t *testing.T) {
	chunker := NewMarkdownChunker(4000)

	// File with only frontmatter followed by trailing whitespace
	content := `---
title: Test Doc
author: Someone
---



`
	chunks, err := chunker.Chunk("test.md", []byte(content))
	if err != nil {
		t.Fatalf("Chunk() error = %v", err)
	}

	// Should produce exactly one chunk (the frontmatter)
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk for frontmatter-only file with trailing whitespace, got %d", len(chunks))
	}

	// Chunk should be frontmatter
	if chunks[0].Name != "test:frontmatter" {
		t.Errorf("expected frontmatter chunk, got name %q", chunks[0].Name)
	}
	if chunks[0].Language != "yaml" {
		t.Errorf("expected yaml language, got %q", chunks[0].Language)
	}
	if !strings.Contains(chunks[0].Content, "title: Test Doc") {
		t.Error("frontmatter content should contain 'title: Test Doc'")
	}
}

func TestMarkdownChunker_SingleLineExceedsMaxChunkSize(t *testing.T) {
	// Very small max chunk size
	chunker := NewMarkdownChunker(50)

	// Single very long line that exceeds max chunk size (with spaces for word-boundary splitting)
	longLine := strings.Repeat("word ", 50) // 250 chars, spaces every 5 chars
	content := "# Title\n\n" + longLine

	chunks, err := chunker.Chunk("test.md", []byte(content))
	if err != nil {
		t.Fatalf("Chunk() error = %v", err)
	}

	// Should produce multiple chunks when splitting long line
	if len(chunks) < 2 {
		t.Errorf("expected multiple chunks for long line, got %d", len(chunks))
	}

	// Verify content is preserved (may be split across chunks)
	var allContent strings.Builder
	for _, chunk := range chunks {
		allContent.WriteString(chunk.Content)
		allContent.WriteString(" ") // Add space between chunks since we trim
	}
	// Count words - should have all 50 "word" occurrences
	wordCount := strings.Count(allContent.String(), "word")
	if wordCount != 50 {
		t.Errorf("expected 50 'word' occurrences across all chunks, got %d", wordCount)
	}

	// Verify no single chunk is massively oversized (allow some tolerance for edge cases)
	// Long lines without spaces may not split perfectly, so we use 2x as tolerance
	maxAllowed := 50 * 2
	for i, chunk := range chunks {
		if len(chunk.Content) > maxAllowed {
			t.Errorf("chunk %d exceeds max allowed size: %d > %d", i, len(chunk.Content), maxAllowed)
		}
	}
}

func TestMarkdownChunker_ChunkIDPopulated(t *testing.T) {
	chunker := NewMarkdownChunker(4000)

	content := `# Title

Content here.`

	chunks, err := chunker.Chunk("test.md", []byte(content))
	if err != nil {
		t.Fatalf("Chunk() error = %v", err)
	}

	if len(chunks) == 0 {
		t.Fatal("expected at least one chunk")
	}

	// Verify ID is populated
	if chunks[0].ID == "" {
		t.Error("chunk.ID should be populated")
	}

	// Verify ID is deterministic
	chunks2, _ := chunker.Chunk("test.md", []byte(content))
	if chunks[0].ID != chunks2[0].ID {
		t.Errorf("chunk.ID should be deterministic, got %q and %q", chunks[0].ID, chunks2[0].ID)
	}
}

func TestMarkdownChunker_PathsWithSpecialCharacters(t *testing.T) {
	chunker := NewMarkdownChunker(4000)

	content := `# Title

Content.`

	tests := []struct {
		name string
		path string
	}{
		{"spaces in path", "/path/to/my file.md"},
		{"unicode in path", "/path/to/文档.md"},
		{"special chars", "/path/to/test-file_v2.0.md"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chunks, err := chunker.Chunk(tt.path, []byte(content))
			if err != nil {
				t.Fatalf("Chunk(%q) error = %v", tt.path, err)
			}
			if len(chunks) == 0 {
				t.Error("expected at least one chunk")
			}
		})
	}
}

func TestMarkdownChunker_MixedFenceTypes(t *testing.T) {
	chunker := NewMarkdownChunker(4000)

	// Document with both ``` and ~~~ fences
	content := "# Title\n\n```go\ncode1\n```\n\n~~~python\ncode2\n~~~\n\n# Another Section\n\nMore content."

	chunks, err := chunker.Chunk("test.md", []byte(content))
	if err != nil {
		t.Fatalf("Chunk() error = %v", err)
	}

	// Should have 2 chunks: "Title" and "Another Section"
	if len(chunks) != 2 {
		t.Errorf("expected 2 chunks, got %d", len(chunks))
	}

	// Verify both code blocks are preserved in first chunk
	if !strings.Contains(chunks[0].Content, "code1") || !strings.Contains(chunks[0].Content, "code2") {
		t.Error("both code blocks should be in the first chunk")
	}
}

func TestMarkdownChunker_FrontmatterWithDotDotDot(t *testing.T) {
	chunker := NewMarkdownChunker(4000)

	// Frontmatter closed with ... per YAML spec
	content := `---
title: Test
author: Someone
...

# Title

Content here.`

	chunks, err := chunker.Chunk("test.md", []byte(content))
	if err != nil {
		t.Fatalf("Chunk() error = %v", err)
	}

	// Should have frontmatter chunk and title chunk
	hasFrontmatter := false
	for _, chunk := range chunks {
		if strings.Contains(chunk.Name, "frontmatter") {
			hasFrontmatter = true
			if !strings.Contains(chunk.Content, "title: Test") {
				t.Error("frontmatter content not preserved")
			}
		}
	}
	if !hasFrontmatter {
		t.Error("expected frontmatter chunk when closed with ...")
	}
}

func TestMarkdownChunker_TrailingHashInHeader(t *testing.T) {
	chunker := NewMarkdownChunker(4000)

	// Header with trailing hashes per CommonMark
	content := `# Title ##

Content here.

## Section ###

More content.`

	chunks, err := chunker.Chunk("test.md", []byte(content))
	if err != nil {
		t.Fatalf("Chunk() error = %v", err)
	}

	if len(chunks) < 2 {
		t.Fatal("expected at least 2 chunks")
	}

	// Verify trailing hashes are stripped from chunk names
	if strings.Contains(chunks[0].Name, "##") {
		t.Errorf("trailing hashes should be stripped from name, got %q", chunks[0].Name)
	}
	if strings.Contains(chunks[1].Name, "###") {
		t.Errorf("trailing hashes should be stripped from name, got %q", chunks[1].Name)
	}
}

// Benchmark tests

func BenchmarkMarkdownChunker_LargeDocument(b *testing.B) {
	chunker := NewMarkdownChunker(4000)

	// Build a large markdown document with many sections
	var builder strings.Builder
	builder.WriteString("# Large Document\n\n")
	for i := 0; i < 100; i++ {
		builder.WriteString("## Section ")
		builder.WriteString(strconv.Itoa(i + 1))
		builder.WriteString("\n\n")
		for j := 0; j < 5; j++ {
			builder.WriteString("Paragraph with some content. ")
			builder.WriteString(strings.Repeat("Lorem ipsum dolor sit amet. ", 10))
			builder.WriteString("\n\n")
		}
	}
	content := []byte(builder.String())

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = chunker.Chunk("test.md", content)
	}
}

func BenchmarkMarkdownChunker_DeepNesting(b *testing.B) {
	chunker := NewMarkdownChunker(4000)

	// Build document with deep header nesting
	var builder strings.Builder
	builder.WriteString("# Level 1\n\n")
	builder.WriteString("## Level 2\n\n")
	builder.WriteString("### Level 3\n\n")
	builder.WriteString("#### Level 4\n\n")
	builder.WriteString("##### Level 5\n\n")
	builder.WriteString("###### Level 6\n\n")

	// Add content at each level with many subsections
	for i := 0; i < 50; i++ {
		builder.WriteString("## Section ")
		builder.WriteString(strconv.Itoa(i + 1))
		builder.WriteString("\n\n")
		builder.WriteString("### Subsection A\n\nContent A\n\n")
		builder.WriteString("### Subsection B\n\nContent B\n\n")
		builder.WriteString("#### Sub-subsection\n\nDeeper content\n\n")
	}
	content := []byte(builder.String())

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = chunker.Chunk("test.md", content)
	}
}

func BenchmarkMarkdownChunker_CodeFences(b *testing.B) {
	chunker := NewMarkdownChunker(4000)

	// Build document with many fenced code blocks
	var builder strings.Builder
	builder.WriteString("# Code Examples\n\n")
	for i := 0; i < 50; i++ {
		builder.WriteString("## Example ")
		builder.WriteString(strconv.Itoa(i + 1))
		builder.WriteString("\n\n```go\n")
		builder.WriteString("func example")
		builder.WriteString(strconv.Itoa(i))
		builder.WriteString("() {\n")
		builder.WriteString("    // Some code here\n")
		builder.WriteString("    fmt.Println(\"Hello\")\n")
		builder.WriteString("}\n```\n\n")
	}
	content := []byte(builder.String())

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = chunker.Chunk("test.md", content)
	}
}
