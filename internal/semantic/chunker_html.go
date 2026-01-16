package semantic

import (
	"bytes"
	"io"
	"strings"

	"golang.org/x/net/html"
)

// HTMLChunker implements structure-aware chunking for HTML files.
// It chunks by semantic elements (section, article, main, aside) and
// falls back to div boundaries if no semantic elements are found.
type HTMLChunker struct {
	maxChunkSize int

	// Elements that define chunk boundaries
	chunkElements map[string]bool

	// Elements to strip entirely (including their content)
	stripElements map[string]bool

	// Elements to preserve exactly (with whitespace)
	preserveElements map[string]bool
}

// NewHTMLChunker creates a new HTML chunker with the specified max chunk size.
// If maxChunkSize is 0 or negative, defaults to 4000.
func NewHTMLChunker(maxChunkSize int) *HTMLChunker {
	if maxChunkSize <= 0 {
		maxChunkSize = 4000
	}
	return &HTMLChunker{
		maxChunkSize: maxChunkSize,
		chunkElements: map[string]bool{
			"section": true,
			"article": true,
			"main":    true,
			"aside":   true,
		},
		stripElements: map[string]bool{
			"script":   true,
			"style":    true,
			"nav":      true,
			"footer":   true,
			"header":   true,
			"noscript": true,
		},
		preserveElements: map[string]bool{
			"pre":        true,
			"code":       true,
			"table":      true,
			"blockquote": true,
		},
	}
}

// htmlSection represents a parsed section of HTML
type htmlSection struct {
	elementTag string // e.g., "section", "article", "div"
	content    strings.Builder
	startLine  int
	hierarchy  []string // Parent element hierarchy for naming
}

// Chunk breaks HTML content into semantic chunks based on element boundaries.
func (c *HTMLChunker) Chunk(path string, content []byte) ([]Chunk, error) {
	if len(content) == 0 {
		return nil, nil
	}

	filename := extractFilename(path)

	// Parse HTML
	doc, err := html.Parse(bytes.NewReader(content))
	if err != nil {
		// On parse error, fall back to treating as plain text
		return c.fallbackToText(path, filename, content)
	}

	var chunks []Chunk
	var hierarchy []string

	// Find and process chunk boundaries
	c.walkNode(doc, &chunks, path, filename, &hierarchy, false)

	// If no semantic chunks found, return entire body as one chunk
	if len(chunks) == 0 {
		text := c.extractText(doc, false)
		if text = strings.TrimSpace(text); text != "" {
			chunk := Chunk{
				FilePath:  path,
				Type:      ChunkFile,
				Name:      filename,
				Content:   text,
				StartLine: 1,
				EndLine:   1,
				Language:  "html",
			}
			chunk.ID = chunk.GenerateID()
			chunks = append(chunks, chunk)
		}
	}

	return chunks, nil
}

// walkNode recursively walks the HTML DOM, collecting chunks at semantic boundaries
func (c *HTMLChunker) walkNode(n *html.Node, chunks *[]Chunk, path, filename string, hierarchy *[]string, insidePreserve bool) {
	if n == nil {
		return
	}

	// Handle element nodes
	if n.Type == html.ElementNode {
		tag := strings.ToLower(n.Data)

		// Skip stripped elements entirely
		if c.stripElements[tag] {
			return
		}

		// Check if this is a chunk boundary
		if c.chunkElements[tag] {
			// Extract text from this element and its children
			text := c.extractText(n, false)
			if text = strings.TrimSpace(text); text != "" {
				// Build hierarchy name
				name := c.buildHTMLChunkName(filename, *hierarchy, tag)

				chunk := Chunk{
					FilePath:  path,
					Type:      ChunkFile,
					Name:      name,
					Content:   text,
					StartLine: 1, // HTML parsing doesn't give us line numbers easily
					EndLine:   1,
					Language:  "html",
				}
				chunk.ID = chunk.GenerateID()
				*chunks = append(*chunks, chunk)
			}
			return // Don't recurse into already-processed semantic elements
		}

		// Track hierarchy for naming
		if tag == "div" || tag == "body" {
			*hierarchy = append(*hierarchy, tag)
			defer func() { *hierarchy = (*hierarchy)[:len(*hierarchy)-1] }()
		}

		// Check if entering a preserve element
		if c.preserveElements[tag] {
			insidePreserve = true
		}
	}

	// Recurse to children
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		c.walkNode(child, chunks, path, filename, hierarchy, insidePreserve)
	}
}

// extractText extracts text content from an HTML node and its children
func (c *HTMLChunker) extractText(n *html.Node, insidePreserve bool) string {
	if n == nil {
		return ""
	}

	var result strings.Builder

	switch n.Type {
	case html.TextNode:
		text := n.Data
		if !insidePreserve {
			// Collapse whitespace for non-preserve elements
			text = collapseWhitespace(text)
		}
		result.WriteString(text)

	case html.ElementNode:
		tag := strings.ToLower(n.Data)

		// Skip stripped elements
		if c.stripElements[tag] {
			return ""
		}

		// Check if this is a preserve element
		preserveNow := insidePreserve || c.preserveElements[tag]

		// Add newlines before block elements
		if isBlockElement(tag) && result.Len() > 0 {
			result.WriteString("\n")
		}

		// Handle br tags
		if tag == "br" {
			result.WriteString("\n")
		}

		// Recurse into children
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			result.WriteString(c.extractText(child, preserveNow))
		}

		// Add newlines after block elements
		if isBlockElement(tag) {
			result.WriteString("\n")
		}

	default:
		// Recurse for other node types
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			result.WriteString(c.extractText(child, insidePreserve))
		}
	}

	return result.String()
}

// buildHTMLChunkName creates a descriptive name from the element hierarchy
func (c *HTMLChunker) buildHTMLChunkName(filename string, hierarchy []string, currentTag string) string {
	parts := []string{filename}
	// Filter out body from hierarchy
	for _, h := range hierarchy {
		if h != "body" {
			parts = append(parts, h)
		}
	}
	parts = append(parts, currentTag)
	return strings.Join(parts, " > ")
}

// fallbackToText handles parse errors by treating content as plain text
func (c *HTMLChunker) fallbackToText(path, filename string, content []byte) ([]Chunk, error) {
	text := string(content)
	if text = strings.TrimSpace(text); text == "" {
		return nil, nil
	}

	chunk := Chunk{
		FilePath:  path,
		Type:      ChunkFile,
		Name:      filename + ":text",
		Content:   text,
		StartLine: 1,
		EndLine:   1,
		Language:  "html",
	}
	chunk.ID = chunk.GenerateID()
	return []Chunk{chunk}, nil
}

// collapseWhitespace replaces multiple whitespace characters with a single space
func collapseWhitespace(s string) string {
	var result strings.Builder
	inWhitespace := false
	for _, r := range s {
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' {
			if !inWhitespace {
				result.WriteRune(' ')
				inWhitespace = true
			}
		} else {
			result.WriteRune(r)
			inWhitespace = false
		}
	}
	return result.String()
}

// isBlockElement returns true if the element is a block-level element
func isBlockElement(tag string) bool {
	blocks := map[string]bool{
		"p": true, "div": true, "section": true, "article": true,
		"header": true, "footer": true, "nav": true, "aside": true,
		"main": true, "h1": true, "h2": true, "h3": true,
		"h4": true, "h5": true, "h6": true, "ul": true, "ol": true,
		"li": true, "table": true, "tr": true, "td": true, "th": true,
		"blockquote": true, "pre": true, "figure": true, "figcaption": true,
	}
	return blocks[tag]
}

// SupportedExtensions returns the file extensions this chunker handles
func (c *HTMLChunker) SupportedExtensions() []string {
	return []string{"html", "htm"}
}

// Ensure HTMLChunker implements Chunker interface
var _ Chunker = (*HTMLChunker)(nil)

// Suppress unused import warning for io package (used by html.Parse internally)
var _ io.Reader
