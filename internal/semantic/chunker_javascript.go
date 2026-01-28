package semantic

import (
	"regexp"
	"strings"
)

// JSChunker implements the Chunker interface for JavaScript/TypeScript
type JSChunker struct {
	// Patterns to match various JS/TS constructs
	functionPattern    *regexp.Regexp
	arrowFuncPattern   *regexp.Regexp
	classPattern       *regexp.Regexp
	interfacePattern   *regexp.Regexp
	typeAliasPattern   *regexp.Regexp
	asyncFuncPattern   *regexp.Regexp
	exportFuncPattern  *regexp.Regexp
	exportClassPattern *regexp.Regexp
	// maxChunkSize limits chunk content to avoid exceeding embedding model context limits
	// Code is token-dense (~2 chars/token), so 1500 chars ≈ 750 tokens, safe for 2048 context
	maxChunkSize int
}

// NewJSChunker creates a new JavaScript/TypeScript chunker
func NewJSChunker() *JSChunker {
	return &JSChunker{
		// maxChunkSize: 1500 chars ≈ 750 tokens for code, safe for 2048 token context limits
		maxChunkSize: 1500,

		// function name(params) { or function name(params): type {
		functionPattern: regexp.MustCompile(`(?m)^(?:export\s+(?:default\s+)?)?(?:async\s+)?function\s+(\w+)\s*\([^)]*\)`),

		// const/let/var name = (params) => or const/let/var name = function
		arrowFuncPattern: regexp.MustCompile(`(?m)^(?:export\s+)?(?:const|let|var)\s+(\w+)\s*=\s*(?:async\s+)?(?:\([^)]*\)|[^=])\s*(?:=>|function)`),

		// class ClassName or class ClassName extends Parent
		classPattern: regexp.MustCompile(`(?m)^(?:export\s+(?:default\s+)?)?(?:abstract\s+)?class\s+(\w+)`),

		// interface Name { (TypeScript)
		interfacePattern: regexp.MustCompile(`(?m)^(?:export\s+)?interface\s+(\w+)`),

		// type Name = (TypeScript)
		typeAliasPattern: regexp.MustCompile(`(?m)^(?:export\s+)?type\s+(\w+)\s*=`),

		// async function name(params)
		asyncFuncPattern: regexp.MustCompile(`(?m)^(?:export\s+)?async\s+function\s+(\w+)`),

		// export function name or export default function name
		exportFuncPattern: regexp.MustCompile(`(?m)^export\s+(?:default\s+)?function\s+(\w+)`),

		// export class Name
		exportClassPattern: regexp.MustCompile(`(?m)^export\s+(?:default\s+)?class\s+(\w+)`),
	}
}

// Chunk parses JavaScript/TypeScript source code and extracts semantic chunks
func (c *JSChunker) Chunk(path string, content []byte) ([]Chunk, error) {
	if len(content) == 0 {
		return nil, nil
	}

	contentStr := string(content)
	lines := strings.Split(contentStr, "\n")
	lang := LanguageFromExtension(path)

	var chunks []Chunk

	// Track what we've already processed to avoid duplicates
	processed := make(map[string]bool)

	// Find all functions (including async, export)
	c.extractFunctions(contentStr, lines, path, lang, &chunks, processed)

	// Find arrow functions and function expressions
	c.extractArrowFunctions(contentStr, lines, path, lang, &chunks, processed)

	// Find classes
	c.extractClasses(contentStr, lines, path, lang, &chunks, processed)

	// Find TypeScript interfaces
	c.extractInterfaces(contentStr, lines, path, lang, &chunks, processed)

	// Find TypeScript type aliases
	c.extractTypeAliases(contentStr, lines, path, lang, &chunks, processed)

	return chunks, nil
}

// extractFunctions finds function declarations
func (c *JSChunker) extractFunctions(content string, lines []string, path, lang string, chunks *[]Chunk, processed map[string]bool) {
	matches := c.functionPattern.FindAllStringSubmatchIndex(content, -1)

	for _, match := range matches {
		if len(match) >= 4 {
			name := content[match[2]:match[3]]
			if processed[name] {
				continue
			}
			processed[name] = true

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

// extractArrowFunctions finds arrow functions and function expressions
func (c *JSChunker) extractArrowFunctions(content string, lines []string, path, lang string, chunks *[]Chunk, processed map[string]bool) {
	matches := c.arrowFuncPattern.FindAllStringSubmatchIndex(content, -1)

	for _, match := range matches {
		if len(match) >= 4 {
			name := content[match[2]:match[3]]
			if processed[name] {
				continue
			}
			processed[name] = true

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

// extractClasses finds class declarations
func (c *JSChunker) extractClasses(content string, lines []string, path, lang string, chunks *[]Chunk, processed map[string]bool) {
	matches := c.classPattern.FindAllStringSubmatchIndex(content, -1)

	for _, match := range matches {
		if len(match) >= 4 {
			name := content[match[2]:match[3]]
			if processed[name] {
				continue
			}
			processed[name] = true

			startLine := c.lineNumber(content, match[0])
			endLine := c.findBlockEnd(lines, startLine-1)

			chunk := Chunk{
				FilePath:  path,
				Type:      ChunkStruct, // Use Struct for class
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

// extractInterfaces finds TypeScript interface declarations
func (c *JSChunker) extractInterfaces(content string, lines []string, path, lang string, chunks *[]Chunk, processed map[string]bool) {
	matches := c.interfacePattern.FindAllStringSubmatchIndex(content, -1)

	for _, match := range matches {
		if len(match) >= 4 {
			name := content[match[2]:match[3]]
			if processed[name] {
				continue
			}
			processed[name] = true

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

// extractTypeAliases finds TypeScript type alias declarations
func (c *JSChunker) extractTypeAliases(content string, lines []string, path, lang string, chunks *[]Chunk, processed map[string]bool) {
	matches := c.typeAliasPattern.FindAllStringSubmatchIndex(content, -1)

	for _, match := range matches {
		if len(match) >= 4 {
			name := content[match[2]:match[3]]
			if processed[name] {
				continue
			}
			processed[name] = true

			startLine := c.lineNumber(content, match[0])
			// Type aliases are typically single line or until semicolon
			endLine := c.findStatementEnd(lines, startLine-1)

			chunk := Chunk{
				FilePath:  path,
				Type:      ChunkFunction, // Use Function for type alias
				Name:      name,
				StartLine: startLine,
				EndLine:   endLine,
				Language:  lang,
				Signature: "type " + name,
			}
			chunk.Content = c.extractContent(lines, startLine-1, endLine-1)
			chunk.ID = chunk.GenerateID()
			*chunks = append(*chunks, chunk)
		}
	}
}

// lineNumber returns the 1-based line number for a byte offset in content
func (c *JSChunker) lineNumber(content string, offset int) int {
	return strings.Count(content[:offset], "\n") + 1
}

// findBlockEnd finds the closing brace of a block starting at lineIdx
func (c *JSChunker) findBlockEnd(lines []string, lineIdx int) int {
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
					return i + 1 // 1-based line number
				}
			}
		}
	}

	// If no braces found, return same line (single-line function)
	if !started {
		return lineIdx + 1
	}

	return len(lines)
}

// findStatementEnd finds the end of a statement (semicolon or next declaration)
func (c *JSChunker) findStatementEnd(lines []string, lineIdx int) int {
	for i := lineIdx; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if strings.Contains(line, ";") {
			return i + 1
		}
		// Check if next line starts a new declaration
		if i > lineIdx && (strings.HasPrefix(line, "type ") ||
			strings.HasPrefix(line, "interface ") ||
			strings.HasPrefix(line, "class ") ||
			strings.HasPrefix(line, "function ") ||
			strings.HasPrefix(line, "const ") ||
			strings.HasPrefix(line, "let ") ||
			strings.HasPrefix(line, "var ") ||
			strings.HasPrefix(line, "export ")) {
			return i
		}
	}
	return len(lines)
}

// extractSignature extracts the first line as a signature
func (c *JSChunker) extractSignature(lines []string, lineIdx int) string {
	if lineIdx >= 0 && lineIdx < len(lines) {
		sig := strings.TrimSpace(lines[lineIdx])
		// Remove opening brace from signature
		sig = strings.TrimSuffix(sig, "{")
		return strings.TrimSpace(sig)
	}
	return ""
}

// extractContent extracts lines from startIdx to endIdx (inclusive)
// Content is truncated to maxChunkSize to avoid exceeding embedding model context limits
func (c *JSChunker) extractContent(lines []string, startIdx, endIdx int) string {
	if startIdx < 0 {
		startIdx = 0
	}
	if endIdx >= len(lines) {
		endIdx = len(lines) - 1
	}
	if startIdx > endIdx {
		return ""
	}

	content := strings.Join(lines[startIdx:endIdx+1], "\n")

	// Truncate if content exceeds maxChunkSize to stay within embedding model limits
	if c.maxChunkSize > 0 && len(content) > c.maxChunkSize {
		// Try to truncate at a line boundary
		truncated := content[:c.maxChunkSize]
		if lastNewline := strings.LastIndex(truncated, "\n"); lastNewline > c.maxChunkSize/2 {
			truncated = truncated[:lastNewline]
		}
		return truncated + "\n// ... truncated"
	}

	return content
}

// SupportedExtensions returns the file extensions this chunker handles
func (c *JSChunker) SupportedExtensions() []string {
	return []string{"js", "jsx", "ts", "tsx", "mjs", "cjs"}
}
