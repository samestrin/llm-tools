package semantic

import (
	"regexp"
	"strings"
)

// RustChunker implements the Chunker interface for Rust
type RustChunker struct {
	functionPattern  *regexp.Regexp
	structPattern    *regexp.Regexp
	enumPattern      *regexp.Regexp
	traitPattern     *regexp.Regexp
	implPattern      *regexp.Regexp
	modPattern       *regexp.Regexp
	typeAliasPattern *regexp.Regexp
}

// NewRustChunker creates a new Rust chunker
func NewRustChunker() *RustChunker {
	return &RustChunker{
		// fn name(...) or pub fn name(...) or async fn name(...)
		functionPattern: regexp.MustCompile(`(?m)^(?:\s*)(?:pub(?:\s*\([^)]*\))?\s+)?(?:async\s+)?(?:unsafe\s+)?(?:extern\s+"[^"]*"\s+)?fn\s+(\w+)\s*(?:<[^>]*>)?\s*\([^)]*\)`),

		// struct Name or pub struct Name
		structPattern: regexp.MustCompile(`(?m)^(?:\s*)(?:pub(?:\s*\([^)]*\))?\s+)?struct\s+(\w+)`),

		// enum Name or pub enum Name
		enumPattern: regexp.MustCompile(`(?m)^(?:\s*)(?:pub(?:\s*\([^)]*\))?\s+)?enum\s+(\w+)`),

		// trait Name or pub trait Name
		traitPattern: regexp.MustCompile(`(?m)^(?:\s*)(?:pub(?:\s*\([^)]*\))?\s+)?(?:unsafe\s+)?trait\s+(\w+)`),

		// impl Name or impl Trait for Name
		implPattern: regexp.MustCompile(`(?m)^(?:\s*)(?:unsafe\s+)?impl(?:<[^>]*>)?\s+(?:(\w+)(?:<[^>]*>)?\s+for\s+)?(\w+)`),

		// mod name or pub mod name
		modPattern: regexp.MustCompile(`(?m)^(?:\s*)(?:pub(?:\s*\([^)]*\))?\s+)?mod\s+(\w+)`),

		// type Name = ... or pub type Name = ...
		typeAliasPattern: regexp.MustCompile(`(?m)^(?:\s*)(?:pub(?:\s*\([^)]*\))?\s+)?type\s+(\w+)`),
	}
}

// Chunk parses Rust source code and extracts semantic chunks
func (c *RustChunker) Chunk(path string, content []byte) ([]Chunk, error) {
	if len(content) == 0 {
		return nil, nil
	}

	contentStr := string(content)
	lines := strings.Split(contentStr, "\n")
	lang := LanguageFromExtension(path)

	var chunks []Chunk
	processed := make(map[string]bool)

	// Find structs
	c.extractStructs(contentStr, lines, path, lang, &chunks, processed)

	// Find enums
	c.extractEnums(contentStr, lines, path, lang, &chunks, processed)

	// Find traits
	c.extractTraits(contentStr, lines, path, lang, &chunks, processed)

	// Find impl blocks
	c.extractImpls(contentStr, lines, path, lang, &chunks, processed)

	// Find functions (standalone, not methods)
	c.extractFunctions(contentStr, lines, path, lang, &chunks, processed)

	// Find modules
	c.extractMods(contentStr, lines, path, lang, &chunks, processed)

	// Find type aliases
	c.extractTypeAliases(contentStr, lines, path, lang, &chunks, processed)

	return chunks, nil
}

// extractStructs finds struct declarations
func (c *RustChunker) extractStructs(content string, lines []string, path, lang string, chunks *[]Chunk, processed map[string]bool) {
	matches := c.structPattern.FindAllStringSubmatchIndex(content, -1)

	for _, match := range matches {
		if len(match) >= 4 {
			name := content[match[2]:match[3]]
			if processed["struct:"+name] {
				continue
			}
			processed["struct:"+name] = true

			startLine := c.lineNumber(content, match[0])
			endLine := c.findBlockEnd(lines, startLine-1)

			chunk := Chunk{
				FilePath:  path,
				Type:      ChunkStruct,
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

// extractEnums finds enum declarations
func (c *RustChunker) extractEnums(content string, lines []string, path, lang string, chunks *[]Chunk, processed map[string]bool) {
	matches := c.enumPattern.FindAllStringSubmatchIndex(content, -1)

	for _, match := range matches {
		if len(match) >= 4 {
			name := content[match[2]:match[3]]
			if processed["enum:"+name] {
				continue
			}
			processed["enum:"+name] = true

			startLine := c.lineNumber(content, match[0])
			endLine := c.findBlockEnd(lines, startLine-1)

			chunk := Chunk{
				FilePath:  path,
				Type:      ChunkStruct, // Treat enums as struct-like for categorization
				Name:      name,
				StartLine: startLine,
				EndLine:   endLine,
				Language:  lang,
				Signature: "enum " + name,
			}
			chunk.Content = c.extractContent(lines, startLine-1, endLine-1)
			chunk.ID = chunk.GenerateID()
			*chunks = append(*chunks, chunk)
		}
	}
}

// extractTraits finds trait declarations
func (c *RustChunker) extractTraits(content string, lines []string, path, lang string, chunks *[]Chunk, processed map[string]bool) {
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
				Type:      ChunkInterface, // Traits are like interfaces
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

// extractImpls finds impl blocks
func (c *RustChunker) extractImpls(content string, lines []string, path, lang string, chunks *[]Chunk, processed map[string]bool) {
	matches := c.implPattern.FindAllStringSubmatchIndex(content, -1)

	for _, match := range matches {
		if len(match) >= 6 {
			// Group 1: trait name (if impl Trait for Type)
			// Group 2: type name
			var name string
			var signature string

			// Check if this is "impl Trait for Type" or just "impl Type"
			if match[2] != -1 && match[3] != -1 {
				// impl Trait for Type
				traitName := content[match[2]:match[3]]
				typeName := content[match[4]:match[5]]
				name = typeName + "_" + traitName
				signature = "impl " + traitName + " for " + typeName
			} else {
				// impl Type
				typeName := content[match[4]:match[5]]
				name = typeName
				signature = "impl " + typeName
			}

			if processed["impl:"+name] {
				continue
			}
			processed["impl:"+name] = true

			startLine := c.lineNumber(content, match[0])
			endLine := c.findBlockEnd(lines, startLine-1)

			chunk := Chunk{
				FilePath:  path,
				Type:      ChunkMethod, // impl blocks contain methods
				Name:      name,
				StartLine: startLine,
				EndLine:   endLine,
				Language:  lang,
				Signature: signature,
			}
			chunk.Content = c.extractContent(lines, startLine-1, endLine-1)
			chunk.ID = chunk.GenerateID()
			*chunks = append(*chunks, chunk)
		}
	}
}

// extractFunctions finds standalone function definitions
func (c *RustChunker) extractFunctions(content string, lines []string, path, lang string, chunks *[]Chunk, processed map[string]bool) {
	matches := c.functionPattern.FindAllStringSubmatchIndex(content, -1)

	for _, match := range matches {
		if len(match) >= 4 {
			name := content[match[2]:match[3]]

			// Skip if already processed (might be inside an impl block)
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

// extractMods finds module declarations
func (c *RustChunker) extractMods(content string, lines []string, path, lang string, chunks *[]Chunk, processed map[string]bool) {
	matches := c.modPattern.FindAllStringSubmatchIndex(content, -1)

	for _, match := range matches {
		if len(match) >= 4 {
			name := content[match[2]:match[3]]
			if processed["mod:"+name] {
				continue
			}
			processed["mod:"+name] = true

			startLine := c.lineNumber(content, match[0])

			// Check if this is a module with a block or just a declaration
			line := lines[startLine-1]
			var endLine int
			if strings.Contains(line, "{") || (startLine < len(lines) && strings.Contains(lines[startLine], "{")) {
				endLine = c.findBlockEnd(lines, startLine-1)
			} else {
				// mod foo; - single line declaration
				endLine = startLine
			}

			chunk := Chunk{
				FilePath:  path,
				Type:      ChunkFile, // Modules are file-level constructs
				Name:      name,
				StartLine: startLine,
				EndLine:   endLine,
				Language:  lang,
				Signature: "mod " + name,
			}
			chunk.Content = c.extractContent(lines, startLine-1, endLine-1)
			chunk.ID = chunk.GenerateID()
			*chunks = append(*chunks, chunk)
		}
	}
}

// extractTypeAliases finds type alias declarations
func (c *RustChunker) extractTypeAliases(content string, lines []string, path, lang string, chunks *[]Chunk, processed map[string]bool) {
	matches := c.typeAliasPattern.FindAllStringSubmatchIndex(content, -1)

	for _, match := range matches {
		if len(match) >= 4 {
			name := content[match[2]:match[3]]
			if processed["type:"+name] {
				continue
			}
			processed["type:"+name] = true

			startLine := c.lineNumber(content, match[0])
			// Type aliases are typically single line
			endLine := startLine

			chunk := Chunk{
				FilePath:  path,
				Type:      ChunkStruct, // Type aliases define types
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
func (c *RustChunker) lineNumber(content string, offset int) int {
	return strings.Count(content[:offset], "\n") + 1
}

// findBlockEnd finds the closing brace of a Rust block
func (c *RustChunker) findBlockEnd(lines []string, lineIdx int) int {
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

// extractSignature extracts the function/struct definition line
func (c *RustChunker) extractSignature(lines []string, lineIdx int) string {
	if lineIdx >= 0 && lineIdx < len(lines) {
		sig := strings.TrimSpace(lines[lineIdx])
		// Remove opening brace
		sig = strings.TrimSuffix(sig, "{")
		return strings.TrimSpace(sig)
	}
	return ""
}

// extractContent extracts lines from startIdx to endIdx
func (c *RustChunker) extractContent(lines []string, startIdx, endIdx int) string {
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
func (c *RustChunker) SupportedExtensions() []string {
	return []string{"rs"}
}
