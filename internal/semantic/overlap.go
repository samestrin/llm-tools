package semantic

import (
	"regexp"
	"strings"
)

const maxEmbedTextLength = 8000

// OverlapConfig configures chunk overlap and parent context enrichment.
type OverlapConfig struct {
	OverlapLines         int  // Number of lines to overlap between adjacent chunks (0 = disabled)
	IncludeParentContext bool // Prepend enclosing scope (package/class) to EmbedText
}

// ApplyOverlap post-processes chunks to add overlap between adjacent chunks
// and optionally prepend parent context. The enrichment is written to each
// chunk's EmbedText field — the Content field is not modified.
func ApplyOverlap(chunks []Chunk, fileContent []byte, cfg OverlapConfig) []Chunk {
	if len(chunks) == 0 {
		return nil
	}

	// If nothing to do, return as-is
	if cfg.OverlapLines <= 0 && !cfg.IncludeParentContext {
		return chunks
	}

	fileLines := strings.Split(string(fileContent), "\n")
	parentCtx := ""
	if cfg.IncludeParentContext {
		parentCtx = extractParentContext(fileLines, chunks)
	}

	for i := range chunks {
		var parts []string

		// Add parent context if available
		if parentCtx != "" {
			parts = append(parts, parentCtx)
		}

		// Build overlapped content
		overlappedContent := buildOverlappedContent(chunks, i, fileLines, cfg.OverlapLines)
		if overlappedContent != "" {
			parts = append(parts, overlappedContent)
		} else if parentCtx != "" {
			// Parent context but no overlap change — use original content
			parts = append(parts, chunks[i].Content)
		}

		// Only set EmbedText if we actually enriched something
		if len(parts) > 0 && (parentCtx != "" || overlappedContent != "") {
			embedText := strings.Join(parts, "\n\n")
			if len(embedText) > maxEmbedTextLength {
				embedText = embedText[:maxEmbedTextLength]
			}
			chunks[i].EmbedText = embedText
		}
	}

	return chunks
}

// buildOverlappedContent creates content with leading/trailing overlap from adjacent chunks.
// Returns empty string if no overlap was added (single chunk or no adjacent content).
func buildOverlappedContent(chunks []Chunk, idx int, fileLines []string, overlapLines int) string {
	if overlapLines <= 0 || len(chunks) <= 1 {
		return ""
	}

	var leadingOverlap, trailingOverlap string

	// Leading overlap: lines before this chunk's start (from file content)
	if idx > 0 {
		startLine := chunks[idx].StartLine - 1 // convert to 0-based
		overlapStart := startLine - overlapLines
		if overlapStart < 0 {
			overlapStart = 0
		}
		if overlapStart < startLine && startLine <= len(fileLines) {
			leadingLines := fileLines[overlapStart:startLine]
			leadingOverlap = strings.Join(leadingLines, "\n")
		}
	}

	// Trailing overlap: lines after this chunk's end (from file content)
	if idx < len(chunks)-1 {
		endLine := chunks[idx].EndLine // 1-based, already inclusive
		overlapEnd := endLine + overlapLines
		if overlapEnd > len(fileLines) {
			overlapEnd = len(fileLines)
		}
		if endLine < overlapEnd && endLine <= len(fileLines) {
			trailingLines := fileLines[endLine:overlapEnd]
			trailingOverlap = strings.Join(trailingLines, "\n")
		}
	}

	if leadingOverlap == "" && trailingOverlap == "" {
		return ""
	}

	// Build: leading + original content + trailing
	var parts []string
	if leadingOverlap != "" {
		parts = append(parts, leadingOverlap)
	}
	parts = append(parts, chunks[idx].Content)
	if trailingOverlap != "" {
		parts = append(parts, trailingOverlap)
	}

	return strings.Join(parts, "\n")
}

var (
	goPackageRe   = regexp.MustCompile(`^package\s+(\w+)`)
	pythonClassRe = regexp.MustCompile(`^class\s+(\w+)`)
	jsClassRe     = regexp.MustCompile(`(?:export\s+)?class\s+(\w+)`)
)

// extractParentContext determines the enclosing scope context from file content.
// For Go: returns "package <name>"
// For Python/JS/TS: returns enclosing "class <name>:" if chunks are methods inside a class
func extractParentContext(fileLines []string, chunks []Chunk) string {
	if len(chunks) == 0 {
		return ""
	}

	lang := chunks[0].Language

	switch lang {
	case "go":
		return extractGoPackageContext(fileLines)
	case "py", "pyw", "pyi":
		return extractPythonClassContext(fileLines, chunks)
	case "js", "jsx", "ts", "tsx", "mjs", "cjs":
		return extractJSClassContext(fileLines, chunks)
	default:
		return ""
	}
}

func extractGoPackageContext(fileLines []string) string {
	for _, line := range fileLines {
		if m := goPackageRe.FindStringSubmatch(line); m != nil {
			return "package " + m[1]
		}
	}
	return ""
}

func extractPythonClassContext(fileLines []string, chunks []Chunk) string {
	// Find if any chunk is a method — if so, find the enclosing class
	hasMethod := false
	for _, c := range chunks {
		if c.Type == ChunkMethod {
			hasMethod = true
			break
		}
	}
	if !hasMethod {
		return ""
	}

	// Find the class definition line that precedes the first method
	for _, line := range fileLines {
		trimmed := strings.TrimSpace(line)
		if m := pythonClassRe.FindStringSubmatch(trimmed); m != nil {
			return "class " + m[1] + ":"
		}
	}
	return ""
}

func extractJSClassContext(fileLines []string, chunks []Chunk) string {
	hasMethod := false
	for _, c := range chunks {
		if c.Type == ChunkMethod {
			hasMethod = true
			break
		}
	}
	if !hasMethod {
		return ""
	}

	for _, line := range fileLines {
		trimmed := strings.TrimSpace(line)
		if m := jsClassRe.FindStringSubmatch(trimmed); m != nil {
			return "class " + m[1]
		}
	}
	return ""
}

// DeduplicateOverlapping removes search results that significantly overlap
// with a higher-scored result from the same file. Results should be sorted
// by score descending before calling this function.
func DeduplicateOverlapping(results []SearchResult, overlapThreshold float64) []SearchResult {
	if len(results) == 0 {
		return nil
	}

	var deduped []SearchResult
	for _, r := range results {
		overlaps := false
		for _, kept := range deduped {
			if r.Chunk.FilePath != kept.Chunk.FilePath {
				continue
			}
			overlapPct := lineOverlapPercent(
				r.Chunk.StartLine, r.Chunk.EndLine,
				kept.Chunk.StartLine, kept.Chunk.EndLine,
			)
			if overlapPct >= overlapThreshold {
				overlaps = true
				break
			}
		}
		if !overlaps {
			deduped = append(deduped, r)
		}
	}

	return deduped
}

// lineOverlapPercent computes the fraction of lines in chunk A that overlap with chunk B.
func lineOverlapPercent(aStart, aEnd, bStart, bEnd int) float64 {
	aLen := aEnd - aStart + 1
	if aLen <= 0 {
		return 0
	}

	// Compute overlap range
	overlapStart := aStart
	if bStart > overlapStart {
		overlapStart = bStart
	}
	overlapEnd := aEnd
	if bEnd < overlapEnd {
		overlapEnd = bEnd
	}

	overlapLen := overlapEnd - overlapStart + 1
	if overlapLen <= 0 {
		return 0
	}

	return float64(overlapLen) / float64(aLen)
}
