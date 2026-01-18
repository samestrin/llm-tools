package semantic

import (
	"bytes"
	"fmt"
	"log/slog"
	"path/filepath"
	"regexp"
	"strconv"
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

// listMarkerRegex matches markdown list markers (-, *, +, or number.)
var listMarkerRegex = regexp.MustCompile(`^(\s*)([-*+]|\d+\.)\s+(.+)$`)

// markdownSection represents a parsed section of markdown
type markdownSection struct {
	level         int    // Header level (1-6), 0 for preamble/frontmatter
	title         string // Header text
	content       strings.Builder
	startLine     int
	endLine       int
	hierarchy     []string // Parent headers for naming
	listItem      string   // Current list item text for code blocks in lists
	listHierarchy []string // Full list hierarchy (parent list items)
}

// Chunk breaks markdown content into semantic chunks based on header boundaries.
// It preserves code blocks intact and tracks header hierarchy for chunk naming.
func (c *MarkdownChunker) Chunk(path string, content []byte) ([]Chunk, error) {
	if path == "" {
		return nil, fmt.Errorf("path cannot be empty")
	}
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

	// State for list tracking
	listStack := make(map[int]string) //indent -> list item text
	var listHierarchy []string        // ordered list items from parent to child

	// Check for YAML frontmatter at start of file
	startLine := 0
	if len(lines) > 0 && strings.TrimSpace(string(lines[0])) == "---" {
		frontmatter, endLine := extractFrontmatter(lines)
		if frontmatter != "" {
			chunk := &Chunk{
				FilePath:  path,
				Type:      ChunkFile,
				Name:      filename + ":frontmatter",
				Content:   frontmatter,
				StartLine: 1,
				EndLine:   endLine,
				Language:  "yaml",
			}
			chunk.ID = chunk.GenerateID()
			chunks = append(chunks, *chunk)
			startLine = endLine // Skip frontmatter lines
		}
	}

	for i := startLine; i < len(lines); i++ {
		lineBytes := lines[i]
		line := string(lineBytes)
		lineNum := i + 1

		// Check for list markers (only if not inside a fence)
		// This must happen before fence detection to catch list items correctly
		if !insideFence {
			if match := listMarkerRegex.FindStringSubmatch(line); match != nil {
				indent := len(match[1])
				listText := strings.TrimSpace(match[3])

				// Strip trailing colon from list item text for cleaner chunk names
				listText = strings.TrimSuffix(listText, ":")

				// Clear deeper levels when a new list item is found
				// Also rebuild listHierarchy to maintain parent-child order
				var newHierarchy []string
				for k := range listStack {
					if k < indent {
						newHierarchy = append(newHierarchy, listStack[k])
					} else if k > indent {
						delete(listStack, k)
					}
				}
				listHierarchy = newHierarchy
				// Add current item to hierarchy
				listHierarchy = append(listHierarchy, listText)
				// Add/update this indent level
				listStack[indent] = listText
			} else if !strings.HasPrefix(strings.TrimLeft(line, " \t"), "#") {
				// Not a header, not a list marker - clear list stack at deeper levels
				// Only keep list items at or above current indent (minus indentation step)
				// This handles paragraphs that continue a list's context
			}
		}

		// Check for fence boundaries
		if isFenceStart(line) && !insideFence {
			insideFence = true
			fenceChar, fenceLength = getFenceInfo(line)

			// Capture list context when entering a code fence
			// Look for the list item at current indent or the closest parent
			if currentSection != nil {
				// Calculate indent of the fence line
				indent := len(line) - len(strings.TrimLeft(line, " \t"))
				// Find the closest list item (deepest indent <= current indent)
				bestList := ""
				bestLevel := -1
				for level, item := range listStack {
					if level <= indent && level > bestLevel {
						bestList = item
						bestLevel = level
					}
				}

				// Capture full list hierarchy up to the matched level
				if bestList != "" {
					hierarchy := make([]string, len(listHierarchy))
					copy(hierarchy, listHierarchy)
					// Trim to items at or below the matched level
					currentSection.listHierarchy = hierarchy
					currentSection.listItem = bestList
				}
			}
		} else if insideFence && isFenceEnd(line, fenceChar, fenceLength) {
			insideFence = false
			// Don't clear listItem or listHierarchy - let them persist for section naming
		}

		// Only detect headers if not inside a code fence
		// Pre-check: headers must start with # (after leading whitespace)
		if !insideFence && strings.HasPrefix(strings.TrimLeft(line, " \t"), "#") {
			if match := headerRegex.FindStringSubmatch(line); match != nil {
				// Found a header - finalize current section and start new one
				if currentSection != nil {
					currentSection.endLine = lineNum - 1
					sectionChunks := c.sectionToChunks(path, filename, currentSection)
					chunks = append(chunks, sectionChunks...)
				}

				level := len(match[1])
				title := stripTrailingHashes(strings.TrimSpace(match[2]))

				// Update header stack for hierarchy
				headerStack = updateHeaderStack(headerStack, level, title)

				// Clear list stack for new section (list items are section-local)
				listStack = make(map[int]string)
				listHierarchy = nil

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
		sectionChunks := c.sectionToChunks(path, filename, currentSection)
		chunks = append(chunks, sectionChunks...)
	}

	return chunks, nil
}

// sectionToChunks converts a markdownSection to one or more Chunks.
// If the section exceeds maxChunkSize, it splits by line boundaries.
// Very long lines are split at word boundaries when possible.
func (c *MarkdownChunker) sectionToChunks(path, filename string, section *markdownSection) []Chunk {
	content := strings.TrimRight(section.content.String(), "\n")
	if content == "" {
		return nil
	}

	// If content fits in one chunk, return single chunk
	if len(content) <= c.maxChunkSize {
		name := c.buildChunkName(filename, section)
		chunk := Chunk{
			FilePath:  path,
			Type:      ChunkFile,
			Name:      name,
			Content:   content,
			StartLine: section.startLine,
			EndLine:   section.endLine,
			Language:  "markdown",
		}
		chunk.ID = chunk.GenerateID()
		return []Chunk{chunk}
	}

	// Split large section by line boundaries
	var chunks []Chunk
	lines := strings.Split(content, "\n")
	var currentContent strings.Builder
	currentStartLine := section.startLine
	partNum := 1

	for i, line := range lines {
		// Handle very long lines that exceed maxChunkSize by themselves
		if len(line) > c.maxChunkSize {
			slog.Warn("oversized line detected, splitting at word boundaries",
				"file", path,
				"line", section.startLine+i,
				"lineLength", len(line),
				"maxChunkSize", c.maxChunkSize)
			// First, emit any accumulated content
			if currentContent.Len() > 0 {
				name := c.buildChunkNameWithPart(filename, section, partNum)
				chunk := Chunk{
					FilePath:  path,
					Type:      ChunkFile,
					Name:      name,
					Content:   strings.TrimRight(currentContent.String(), "\n"),
					StartLine: currentStartLine,
					EndLine:   section.startLine + i - 1,
					Language:  "markdown",
				}
				chunk.ID = chunk.GenerateID()
				chunks = append(chunks, chunk)
				currentContent.Reset()
				partNum++
			}

			// Split the long line at word boundaries
			lineParts := c.splitLongLine(line)
			for _, part := range lineParts {
				name := c.buildChunkNameWithPart(filename, section, partNum)
				chunk := Chunk{
					FilePath:  path,
					Type:      ChunkFile,
					Name:      name,
					Content:   part,
					StartLine: section.startLine + i,
					EndLine:   section.startLine + i,
					Language:  "markdown",
				}
				chunk.ID = chunk.GenerateID()
				chunks = append(chunks, chunk)
				partNum++
			}
			currentStartLine = section.startLine + i + 1
			continue
		}

		lineWithNewline := line + "\n"

		// Check if adding this line would exceed max size
		if currentContent.Len()+len(lineWithNewline) > c.maxChunkSize && currentContent.Len() > 0 {
			// Emit current chunk
			name := c.buildChunkNameWithPart(filename, section, partNum)
			chunk := Chunk{
				FilePath:  path,
				Type:      ChunkFile,
				Name:      name,
				Content:   strings.TrimRight(currentContent.String(), "\n"),
				StartLine: currentStartLine,
				EndLine:   section.startLine + i - 1,
				Language:  "markdown",
			}
			chunk.ID = chunk.GenerateID()
			chunks = append(chunks, chunk)

			// Reset for next chunk
			currentContent.Reset()
			currentStartLine = section.startLine + i
			partNum++
		}

		currentContent.WriteString(lineWithNewline)
	}

	// Emit final chunk
	if currentContent.Len() > 0 {
		name := c.buildChunkNameWithPart(filename, section, partNum)
		chunk := Chunk{
			FilePath:  path,
			Type:      ChunkFile,
			Name:      name,
			Content:   strings.TrimRight(currentContent.String(), "\n"),
			StartLine: currentStartLine,
			EndLine:   section.endLine,
			Language:  "markdown",
		}
		chunk.ID = chunk.GenerateID()
		chunks = append(chunks, chunk)
	}

	return chunks
}

// buildChunkName creates a descriptive name from the header hierarchy
func (c *MarkdownChunker) buildChunkName(filename string, section *markdownSection) string {
	if len(section.hierarchy) == 0 {
		// Preamble or content before first header - use :preamble suffix for consistency with :frontmatter
		return filename + ":preamble"
	}

	// Build hierarchical name: "filename > H1 > H2 > H3 > ListItem(s)"
	parts := []string{filename}
	parts = append(parts, section.hierarchy...)
	// Add list hierarchy if present (for code blocks inside lists)
	// This includes all parent list items for nested contexts
	if len(section.listHierarchy) > 0 {
		parts = append(parts, section.listHierarchy...)
	}
	return strings.Join(parts, " > ")
}

// buildChunkNameWithPart creates a chunk name with a part number suffix
func (c *MarkdownChunker) buildChunkNameWithPart(filename string, section *markdownSection, partNum int) string {
	baseName := c.buildChunkName(filename, section)
	if partNum == 1 {
		return baseName
	}
	return baseName + " (part " + strconv.Itoa(partNum) + ")"
}

// splitLongLine breaks a line that exceeds maxChunkSize into smaller parts.
// It splits at word boundaries (spaces) when possible, preserving content integrity.
// If no space is found within maxChunkSize (e.g., a long URL), it falls back to
// hard splitting at maxChunkSize.
func (c *MarkdownChunker) splitLongLine(line string) []string {
	if len(line) <= c.maxChunkSize {
		return []string{line}
	}

	var parts []string
	remaining := line

	for len(remaining) > c.maxChunkSize {
		// Try to find a space within the maxChunkSize limit
		splitPoint := c.maxChunkSize
		for i := c.maxChunkSize - 1; i > c.maxChunkSize/2; i-- {
			if remaining[i] == ' ' {
				splitPoint = i
				break
			}
		}

		// Add the part (trimming trailing space if we split at a space)
		part := strings.TrimRight(remaining[:splitPoint], " ")
		parts = append(parts, part)

		// Move to the remaining content (trimming leading space)
		remaining = strings.TrimLeft(remaining[splitPoint:], " ")
	}

	// Add any remaining content
	if len(remaining) > 0 {
		parts = append(parts, remaining)
	}

	return parts
}

// extractFrontmatter extracts YAML frontmatter from the beginning of a file.
// Returns the frontmatter content (excluding delimiters) and the line number after the closing delimiter.
// Supports both --- and ... as closing delimiters per YAML spec.
// Returns empty string and 0 if no valid frontmatter found.
func extractFrontmatter(lines [][]byte) (string, int) {
	if len(lines) < 2 {
		return "", 0
	}

	// First line must be ---
	if strings.TrimSpace(string(lines[0])) != "---" {
		return "", 0
	}

	// Find closing delimiter (--- or ... per YAML spec)
	var content strings.Builder
	for i := 1; i < len(lines); i++ {
		line := string(lines[i])
		trimmed := strings.TrimSpace(line)
		if trimmed == "---" || trimmed == "..." {
			// Found closing delimiter
			return strings.TrimRight(content.String(), "\n"), i + 1
		}
		content.WriteString(line)
		content.WriteString("\n")
	}

	// No closing delimiter found
	return "", 0
}

// stripTrailingHashes removes optional closing hash sequence per CommonMark spec
// e.g., "Title ##" → "Title", "Title" → "Title"
func stripTrailingHashes(title string) string {
	// Strip trailing whitespace first
	title = strings.TrimRight(title, " \t")
	// Strip trailing hash characters
	for len(title) > 0 && title[len(title)-1] == '#' {
		title = title[:len(title)-1]
	}
	// Strip any whitespace between content and the hashes
	return strings.TrimRight(title, " \t")
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

// isFenceStart checks if a line starts a fenced code block.
// Supports both ``` and ~~~ fence markers.
// Allows any leading whitespace (more permissive than CommonMark's 0-3 spaces).
func isFenceStart(line string) bool {
	trimmed := strings.TrimLeft(line, " \t")
	return strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~")
}

// getFenceInfo returns the fence character and minimum length for matching
func getFenceInfo(line string) (byte, int) {
	trimmed := strings.TrimLeft(line, " \t")
	if strings.HasPrefix(trimmed, "```") {
		count := 0
		for i := 0; i < len(trimmed) && trimmed[i] == '`'; i++ {
			count++
		}
		return '`', count
	}
	if strings.HasPrefix(trimmed, "~~~") {
		count := 0
		for i := 0; i < len(trimmed) && trimmed[i] == '~'; i++ {
			count++
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

	// Count matching fence characters using byte iteration
	count := 0
	for i := 0; i < len(trimmed) && trimmed[i] == fenceChar; i++ {
		count++
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
// Handles hidden files (e.g., .gitignore) and files with multiple dots
func extractFilename(path string) string {
	base := filepath.Base(path)
	// Handle hidden files (start with .) - don't strip the leading dot as extension
	if strings.HasPrefix(base, ".") && !strings.Contains(base[1:], ".") {
		// Hidden file with no extension (e.g., .gitignore)
		return base
	}
	ext := filepath.Ext(base)
	if ext != "" {
		return base[:len(base)-len(ext)]
	}
	return base
}

// SupportedExtensions returns the file extensions this chunker handles
func (c *MarkdownChunker) SupportedExtensions() []string {
	return []string{"md", "markdown"}
}
