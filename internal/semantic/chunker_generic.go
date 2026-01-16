package semantic

import (
	"bytes"
	"strings"
)

// GenericChunker implements a simple text-based chunker for unsupported file types
type GenericChunker struct {
	maxChunkSize int
}

// NewGenericChunker creates a new generic text chunker
func NewGenericChunker(maxChunkSize int) *GenericChunker {
	if maxChunkSize <= 0 {
		maxChunkSize = 2000 // Default to 2000 characters
	}
	return &GenericChunker{
		maxChunkSize: maxChunkSize,
	}
}

// Chunk splits content into chunks based on paragraphs/size
func (c *GenericChunker) Chunk(path string, content []byte) ([]Chunk, error) {
	if len(content) == 0 {
		return nil, nil
	}

	lang := LanguageFromExtension(path)
	if lang == "" {
		lang = "text"
	}

	// Split by double newlines (paragraphs) or by size
	var chunks []Chunk
	lines := bytes.Split(content, []byte("\n"))

	var currentContent strings.Builder
	startLine := 1
	currentLine := 1

	for _, line := range lines {
		lineStr := string(line) + "\n"

		// Check if adding this line would exceed chunk size
		if currentContent.Len()+len(lineStr) > c.maxChunkSize && currentContent.Len() > 0 {
			// Create chunk from accumulated content
			chunks = append(chunks, c.createChunk(path, lang, currentContent.String(), startLine, currentLine-1))

			// Reset for next chunk
			currentContent.Reset()
			startLine = currentLine
		}

		currentContent.WriteString(lineStr)
		currentLine++
	}

	// Handle remaining content
	if currentContent.Len() > 0 {
		// Trim trailing newline for final content
		finalContent := strings.TrimRight(currentContent.String(), "\n")
		if finalContent != "" {
			chunks = append(chunks, c.createChunk(path, lang, finalContent, startLine, currentLine-1))
		}
	}

	return chunks, nil
}

// createChunk creates a Chunk with the given parameters
func (c *GenericChunker) createChunk(path, lang, content string, startLine, endLine int) Chunk {
	chunk := Chunk{
		FilePath:  path,
		Type:      ChunkFile,
		Name:      chunkName(path),
		Content:   content,
		StartLine: startLine,
		EndLine:   endLine,
		Language:  lang,
	}
	chunk.ID = chunk.GenerateID()
	return chunk
}

// chunkName generates a descriptive name for a generic chunk
func chunkName(path string) string {
	// Extract filename without extension
	name := path
	if idx := strings.LastIndex(path, "/"); idx >= 0 {
		name = path[idx+1:]
	}
	if idx := strings.LastIndex(name, "."); idx >= 0 {
		name = name[:idx]
	}
	return name
}

// SupportedExtensions returns the file extensions this chunker handles.
// Extensions NOT listed here: md, markdown, html, htm (handled by specialized chunkers)
func (c *GenericChunker) SupportedExtensions() []string {
	return []string{
		// Text files
		"txt", "text",
		// Documentation (md/markdown handled by MarkdownChunker)
		"rst", "adoc",
		// Config files
		"yaml", "yml", "toml", "ini", "cfg", "conf",
		// Data files
		"json", "xml", "csv",
		// Shell scripts
		"sh", "bash", "zsh", "fish",
		// Other
		"log", "diff", "patch",
	}
}
