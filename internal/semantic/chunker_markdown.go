package semantic

import (
	"bytes"
	"path/filepath"
	"regexp"
	"strings"
)

// MarkdownChunker implements structure-aware chunking for markdown files.
// It chunks by header boundaries, preserves code blocks, and maintains
// header hierarchy for meaningful chunk names.
type MarkdownChunker struct {
	maxChunkSize int
}

// NewMarkdownChunker creates a new markdown chunker with the specified max chunk size.
// If maxChunkSize is 0 or negative, defaults to 4000.
func NewMarkdownChunker(maxChunkSize int) *MarkdownChunker {
	if maxChunkSize <= 0 {
		maxChunkSize = 4000
	}
	return &MarkdownChunker{
		maxChunkSize: maxChunkSize,
	}
}

// headerRegex matches markdown headers (h1-h6)
var headerRegex = regexp.MustCompile(`^(#{1,6})\s+(.+)$`)

// markdownSection represents a parsed section of markdown
type markdownSection struct {
	level     int    // Header level (1-6), 0 for preamble/frontmatter
	title     string // Header text
	content   strings.Builder
	startLine int
	endLine   int
	hierarchy []string // Parent headers for naming
}

// Chunk breaks markdown content into semantic chunks based on header boundaries.
// It preserves code blocks intact and tracks header hierarchy for chunk naming.
func (c *MarkdownChunker) Chunk(path string, content []byte) ([]Chunk, error) {
	if len(content) == 0 {
		return nil, nil
	}

	lines := bytes.Split(content, []byte("\n"))
	filename := extractFilename(path)

	var chunks []Chunk
	var currentSection *markdownSection
	var headerStack []string // Stack of header titles for hierarchy

	// State for code block detection
	var insideFence bool
	var fenceChar byte
	var fenceLength int

	for i, lineBytes := range lines {
		line := string(lineBytes)
		lineNum := i + 1

		// Check for fence boundaries
		if isFenceStart(line) && !insideFence {
			insideFence = true
			fenceChar, fenceLength = getFenceInfo(line)
		} else if insideFence && isFenceEnd(line, fenceChar, fenceLength) {
			insideFence = false
		}

		// Only detect headers if not inside a code fence
		if !insideFence {
			if match := headerRegex.FindStringSubmatch(line); match != nil {
				// Found a header - finalize current section and start new one
				if currentSection != nil {
					currentSection.endLine = lineNum - 1
					chunk := c.sectionToChunk(path, filename, currentSection)
					if chunk != nil {
						chunks = append(chunks, *chunk)
					}
				}

				level := len(match[1])
				title := strings.TrimSpace(match[2])

				// Update header stack for hierarchy
				headerStack = updateHeaderStack(headerStack, level, title)

				// Start new section
				currentSection = &markdownSection{
					level:     level,
					title:     title,
					startLine: lineNum,
					hierarchy: make([]string, len(headerStack)),
				}
				copy(currentSection.hierarchy, headerStack)
				currentSection.content.WriteString(line)
				currentSection.content.WriteString("\n")
				continue
			}
		}

		// Accumulate content in current section
		if currentSection == nil {
			// Content before first header (preamble)
			currentSection = &markdownSection{
				level:     0,
				title:     "",
				startLine: lineNum,
				hierarchy: []string{},
			}
		}
		currentSection.content.WriteString(line)
		currentSection.content.WriteString("\n")
	}

	// Finalize last section
	if currentSection != nil {
		currentSection.endLine = len(lines)
		chunk := c.sectionToChunk(path, filename, currentSection)
		if chunk != nil {
			chunks = append(chunks, *chunk)
		}
	}

	return chunks, nil
}

// sectionToChunk converts a markdownSection to a Chunk
func (c *MarkdownChunker) sectionToChunk(path, filename string, section *markdownSection) *Chunk {
	content := strings.TrimRight(section.content.String(), "\n")
	if content == "" {
		return nil
	}

	// Build chunk name from hierarchy
	name := c.buildChunkName(filename, section)

	chunk := &Chunk{
		FilePath:  path,
		Type:      ChunkFile,
		Name:      name,
		Content:   content,
		StartLine: section.startLine,
		EndLine:   section.endLine,
		Language:  "markdown",
	}
	chunk.ID = chunk.GenerateID()
	return chunk
}

// buildChunkName creates a descriptive name from the header hierarchy
func (c *MarkdownChunker) buildChunkName(filename string, section *markdownSection) string {
	if len(section.hierarchy) == 0 {
		// Preamble or content before first header
		return filename + ":" + itoa(section.startLine) + "-" + itoa(section.endLine)
	}

	// Build hierarchical name: "filename > H1 > H2 > H3"
	parts := []string{filename}
	parts = append(parts, section.hierarchy...)
	return strings.Join(parts, " > ")
}

// updateHeaderStack maintains the header hierarchy stack
// When a new header is encountered, it pops headers of equal or lower level
// and pushes the new header
func updateHeaderStack(stack []string, level int, title string) []string {
	// Remove headers at same level or deeper
	for len(stack) >= level {
		stack = stack[:len(stack)-1]
	}
	// Add the new header
	return append(stack, title)
}

// isFenceStart checks if a line starts a fenced code block
func isFenceStart(line string) bool {
	trimmed := strings.TrimLeft(line, " \t")
	return strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~")
}

// getFenceInfo returns the fence character and minimum length for matching
func getFenceInfo(line string) (byte, int) {
	trimmed := strings.TrimLeft(line, " \t")
	if strings.HasPrefix(trimmed, "```") {
		count := 0
		for _, ch := range trimmed {
			if ch == '`' {
				count++
			} else {
				break
			}
		}
		return '`', count
	}
	if strings.HasPrefix(trimmed, "~~~") {
		count := 0
		for _, ch := range trimmed {
			if ch == '~' {
				count++
			} else {
				break
			}
		}
		return '~', count
	}
	return 0, 0
}

// isFenceEnd checks if a line ends a fenced code block
func isFenceEnd(line string, fenceChar byte, fenceLength int) bool {
	if fenceChar == 0 {
		return false
	}
	trimmed := strings.TrimLeft(line, " \t")
	if len(trimmed) < fenceLength {
		return false
	}

	// Count matching fence characters
	count := 0
	for _, ch := range trimmed {
		if byte(ch) == fenceChar {
			count++
		} else {
			break
		}
	}

	// Closing fence must have at least as many chars as opening
	// and nothing else on the line (except whitespace)
	if count >= fenceLength {
		rest := strings.TrimLeft(trimmed[count:], " \t")
		return rest == "" || rest == "\n" || rest == "\r\n"
	}
	return false
}

// extractFilename gets the base filename without extension
func extractFilename(path string) string {
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	if ext != "" {
		return base[:len(base)-len(ext)]
	}
	return base
}

// itoa converts int to string without importing strconv
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	negative := n < 0
	if negative {
		n = -n
	}
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if negative {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// SupportedExtensions returns the file extensions this chunker handles
func (c *MarkdownChunker) SupportedExtensions() []string {
	return []string{"md", "markdown"}
}
