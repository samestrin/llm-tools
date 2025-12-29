package semantic

import (
	"regexp"
	"strings"
)

// PHPChunker implements the Chunker interface for PHP
type PHPChunker struct {
	functionPattern  *regexp.Regexp
	classPattern     *regexp.Regexp
	interfacePattern *regexp.Regexp
	traitPattern     *regexp.Regexp
}

// NewPHPChunker creates a new PHP chunker
func NewPHPChunker() *PHPChunker {
	return &PHPChunker{
		// function name() or visibility function name()
		functionPattern: regexp.MustCompile(`(?m)^(?:(?:public|private|protected|static|final|abstract)\s+)*function\s+(\w+)\s*\([^)]*\)`),

		// class Name or abstract/final class Name extends/implements
		classPattern: regexp.MustCompile(`(?m)^(?:abstract\s+|final\s+)?class\s+(\w+)`),

		// interface Name
		interfacePattern: regexp.MustCompile(`(?m)^interface\s+(\w+)`),

		// trait Name
		traitPattern: regexp.MustCompile(`(?m)^trait\s+(\w+)`),
	}
}

// Chunk parses PHP source code and extracts semantic chunks
func (c *PHPChunker) Chunk(path string, content []byte) ([]Chunk, error) {
	if len(content) == 0 {
		return nil, nil
	}

	contentStr := string(content)
	lines := strings.Split(contentStr, "\n")
	lang := LanguageFromExtension(path)

	var chunks []Chunk
	processed := make(map[string]bool)

	// Find classes
	c.extractClasses(contentStr, lines, path, lang, &chunks, processed)

	// Find interfaces
	c.extractInterfaces(contentStr, lines, path, lang, &chunks, processed)

	// Find traits (as structs)
	c.extractTraits(contentStr, lines, path, lang, &chunks, processed)

	// Find functions
	c.extractFunctions(contentStr, lines, path, lang, &chunks, processed)

	return chunks, nil
}

// extractClasses finds class declarations
func (c *PHPChunker) extractClasses(content string, lines []string, path, lang string, chunks *[]Chunk, processed map[string]bool) {
	matches := c.classPattern.FindAllStringSubmatchIndex(content, -1)

	for _, match := range matches {
		if len(match) >= 4 {
			name := content[match[2]:match[3]]
			if processed["class:"+name] {
				continue
			}
			processed["class:"+name] = true

			startLine := c.lineNumber(content, match[0])
			endLine := c.findBlockEnd(lines, startLine-1)

			chunk := Chunk{
				FilePath:  path,
				Type:      ChunkStruct,
				Name:      name,
				StartLine: startLine,
				EndLine:   endLine,
				Language:  lang,
				Signature: "class " + name,
			}
			chunk.Content = c.extractContent(lines, startLine-1, endLine-1)
			chunk.ID = chunk.GenerateID()
			*chunks = append(*chunks, chunk)
		}
	}
}

// extractInterfaces finds interface declarations
func (c *PHPChunker) extractInterfaces(content string, lines []string, path, lang string, chunks *[]Chunk, processed map[string]bool) {
	matches := c.interfacePattern.FindAllStringSubmatchIndex(content, -1)

	for _, match := range matches {
		if len(match) >= 4 {
			name := content[match[2]:match[3]]
			if processed["iface:"+name] {
				continue
			}
			processed["iface:"+name] = true

			startLine := c.lineNumber(content, match[0])
			endLine := c.findBlockEnd(lines, startLine-1)

			chunk := Chunk{
				FilePath:  path,
				Type:      ChunkInterface,
				Name:      name,
				StartLine: startLine,
				EndLine:   endLine,
				Language:  lang,
				Signature: "interface " + name,
			}
			chunk.Content = c.extractContent(lines, startLine-1, endLine-1)
			chunk.ID = chunk.GenerateID()
			*chunks = append(*chunks, chunk)
		}
	}
}

// extractTraits finds trait declarations
func (c *PHPChunker) extractTraits(content string, lines []string, path, lang string, chunks *[]Chunk, processed map[string]bool) {
	matches := c.traitPattern.FindAllStringSubmatchIndex(content, -1)

	for _, match := range matches {
		if len(match) >= 4 {
			name := content[match[2]:match[3]]
			if processed["trait:"+name] {
				continue
			}
			processed["trait:"+name] = true

			startLine := c.lineNumber(content, match[0])
			endLine := c.findBlockEnd(lines, startLine-1)

			chunk := Chunk{
				FilePath:  path,
				Type:      ChunkStruct, // Treat traits as struct-like
				Name:      name,
				StartLine: startLine,
				EndLine:   endLine,
				Language:  lang,
				Signature: "trait " + name,
			}
			chunk.Content = c.extractContent(lines, startLine-1, endLine-1)
			chunk.ID = chunk.GenerateID()
			*chunks = append(*chunks, chunk)
		}
	}
}

// extractFunctions finds function definitions
func (c *PHPChunker) extractFunctions(content string, lines []string, path, lang string, chunks *[]Chunk, processed map[string]bool) {
	matches := c.functionPattern.FindAllStringSubmatchIndex(content, -1)

	for _, match := range matches {
		if len(match) >= 4 {
			name := content[match[2]:match[3]]
			if processed["func:"+name] {
				continue
			}
			processed["func:"+name] = true

			startLine := c.lineNumber(content, match[0])
			endLine := c.findBlockEnd(lines, startLine-1)

			chunk := Chunk{
				FilePath:  path,
				Type:      ChunkFunction,
				Name:      name,
				StartLine: startLine,
				EndLine:   endLine,
				Language:  lang,
				Signature: c.extractSignature(lines, startLine-1),
			}
			chunk.Content = c.extractContent(lines, startLine-1, endLine-1)
			chunk.ID = chunk.GenerateID()
			*chunks = append(*chunks, chunk)
		}
	}
}

// lineNumber returns the 1-based line number for a byte offset
func (c *PHPChunker) lineNumber(content string, offset int) int {
	return strings.Count(content[:offset], "\n") + 1
}

// findBlockEnd finds the closing brace of a PHP block
func (c *PHPChunker) findBlockEnd(lines []string, lineIdx int) int {
	braceCount := 0
	started := false

	for i := lineIdx; i < len(lines); i++ {
		line := lines[i]
		for _, ch := range line {
			if ch == '{' {
				braceCount++
				started = true
			} else if ch == '}' {
				braceCount--
				if started && braceCount == 0 {
					return i + 1
				}
			}
		}
	}

	if !started {
		return lineIdx + 1
	}

	return len(lines)
}

// extractSignature extracts the function/class definition line
func (c *PHPChunker) extractSignature(lines []string, lineIdx int) string {
	if lineIdx >= 0 && lineIdx < len(lines) {
		sig := strings.TrimSpace(lines[lineIdx])
		// Remove opening brace
		sig = strings.TrimSuffix(sig, "{")
		return strings.TrimSpace(sig)
	}
	return ""
}

// extractContent extracts lines from startIdx to endIdx
func (c *PHPChunker) extractContent(lines []string, startIdx, endIdx int) string {
	if startIdx < 0 {
		startIdx = 0
	}
	if endIdx >= len(lines) {
		endIdx = len(lines) - 1
	}
	if startIdx > endIdx {
		return ""
	}

	return strings.Join(lines[startIdx:endIdx+1], "\n")
}

// SupportedExtensions returns the file extensions this chunker handles
func (c *PHPChunker) SupportedExtensions() []string {
	return []string{"php", "phtml", "php5", "php7"}
}
