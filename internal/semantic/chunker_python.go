package semantic

import (
	"regexp"
	"strings"
)

// PythonChunker implements the Chunker interface for Python
type PythonChunker struct {
	functionPattern *regexp.Regexp
	classPattern    *regexp.Regexp
	asyncFuncPat    *regexp.Regexp
}

// NewPythonChunker creates a new Python chunker
func NewPythonChunker() *PythonChunker {
	return &PythonChunker{
		// def function_name(params): or async def function_name(params):
		functionPattern: regexp.MustCompile(`(?m)^(?:@\w+.*\n)*\s*(?:async\s+)?def\s+(\w+)\s*\([^)]*\)`),

		// class ClassName: or class ClassName(Parent):
		classPattern: regexp.MustCompile(`(?m)^(?:@\w+.*\n)*\s*class\s+(\w+)\s*(?:\([^)]*\))?:`),

		// async def for specific matching
		asyncFuncPat: regexp.MustCompile(`(?m)^async\s+def\s+(\w+)`),
	}
}

// Chunk parses Python source code and extracts semantic chunks
func (c *PythonChunker) Chunk(path string, content []byte) ([]Chunk, error) {
	if len(content) == 0 {
		return nil, nil
	}

	contentStr := string(content)
	lines := strings.Split(contentStr, "\n")
	lang := LanguageFromExtension(path)

	var chunks []Chunk
	processed := make(map[string]bool)

	// Find classes first
	c.extractClasses(contentStr, lines, path, lang, &chunks, processed)

	// Find functions (top-level only)
	c.extractFunctions(contentStr, lines, path, lang, &chunks, processed)

	return chunks, nil
}

// extractClasses finds class declarations
func (c *PythonChunker) extractClasses(content string, lines []string, path, lang string, chunks *[]Chunk, processed map[string]bool) {
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

// extractFunctions finds function definitions
func (c *PythonChunker) extractFunctions(content string, lines []string, path, lang string, chunks *[]Chunk, processed map[string]bool) {
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
func (c *PythonChunker) lineNumber(content string, offset int) int {
	return strings.Count(content[:offset], "\n") + 1
}

// findBlockEnd finds the end of a Python block by indentation
func (c *PythonChunker) findBlockEnd(lines []string, startIdx int) int {
	if startIdx >= len(lines) {
		return len(lines)
	}

	// Get the base indentation of the block header
	startLine := lines[startIdx]
	baseIndent := c.getIndentation(startLine)

	// Look for the colon that starts the block
	hasColon := strings.Contains(startLine, ":")
	if !hasColon {
		// Check next line
		if startIdx+1 < len(lines) && strings.Contains(lines[startIdx+1], ":") {
			startIdx++
			startLine = lines[startIdx]
		}
	}

	// Find where the block ends (next line with same or less indentation)
	foundBody := false
	for i := startIdx + 1; i < len(lines); i++ {
		line := lines[i]

		// Skip empty lines and comments
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		lineIndent := c.getIndentation(line)

		// First non-empty line after header is the body
		if !foundBody {
			foundBody = true
			continue
		}

		// If we hit a line with same or less indentation, block ends
		if lineIndent <= baseIndent {
			return i
		}
	}

	return len(lines)
}

// getIndentation returns the number of leading spaces/tabs
func (c *PythonChunker) getIndentation(line string) int {
	count := 0
	for _, ch := range line {
		if ch == ' ' {
			count++
		} else if ch == '\t' {
			count += 4 // Count tab as 4 spaces
		} else {
			break
		}
	}
	return count
}

// extractSignature extracts the function/class definition line
func (c *PythonChunker) extractSignature(lines []string, lineIdx int) string {
	if lineIdx >= 0 && lineIdx < len(lines) {
		sig := strings.TrimSpace(lines[lineIdx])
		// Remove trailing colon
		sig = strings.TrimSuffix(sig, ":")
		return strings.TrimSpace(sig)
	}
	return ""
}

// extractContent extracts lines from startIdx to endIdx
func (c *PythonChunker) extractContent(lines []string, startIdx, endIdx int) string {
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
func (c *PythonChunker) SupportedExtensions() []string {
	return []string{"py", "pyw", "pyi"}
}
