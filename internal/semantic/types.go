package semantic

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"
)

// ChunkType represents the type of code chunk
type ChunkType int

const (
	ChunkFunction ChunkType = iota
	ChunkMethod
	ChunkStruct
	ChunkInterface
	ChunkFile
)

// String returns the string representation of ChunkType
func (ct ChunkType) String() string {
	switch ct {
	case ChunkFunction:
		return "function"
	case ChunkMethod:
		return "method"
	case ChunkStruct:
		return "struct"
	case ChunkInterface:
		return "interface"
	case ChunkFile:
		return "file"
	default:
		return "unknown"
	}
}

// ParseChunkType parses a string into a ChunkType
func ParseChunkType(s string) (ChunkType, error) {
	switch strings.ToLower(s) {
	case "function":
		return ChunkFunction, nil
	case "method":
		return ChunkMethod, nil
	case "struct":
		return ChunkStruct, nil
	case "interface":
		return ChunkInterface, nil
	case "file":
		return ChunkFile, nil
	default:
		return ChunkFunction, fmt.Errorf("unknown chunk type: %q", s)
	}
}

// MarshalJSON implements json.Marshaler for ChunkType
func (ct ChunkType) MarshalJSON() ([]byte, error) {
	return json.Marshal(ct.String())
}

// UnmarshalJSON implements json.Unmarshaler for ChunkType
func (ct *ChunkType) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	parsed, err := ParseChunkType(s)
	if err != nil {
		return err
	}
	*ct = parsed
	return nil
}

// Chunk represents a semantic unit of code (function, struct, etc.)
type Chunk struct {
	ID        string    `json:"id"`
	FilePath  string    `json:"file_path"`
	Type      ChunkType `json:"type"`
	Name      string    `json:"name"`
	Signature string    `json:"signature,omitempty"`
	Content   string    `json:"content"`
	StartLine int       `json:"start_line"`
	EndLine   int       `json:"end_line"`
	Language  string    `json:"language"`
	Domain    string    `json:"domain,omitempty"`     // Source profile/collection (e.g., "code", "docs") - set during indexing based on profile
	FileMtime int64     `json:"file_mtime,omitempty"` // Unix timestamp of file modification time
}

// GenerateID creates a deterministic ID for the chunk based on its content
func (c *Chunk) GenerateID() string {
	data := fmt.Sprintf("%s:%s:%d", c.FilePath, c.Name, c.StartLine)
	hash := sha256.Sum256([]byte(data))
	return fmt.Sprintf("%x", hash[:8])
}

// Preview returns a short preview of the chunk content for display purposes.
// Prefers Signature if available, otherwise uses truncated Content.
// Replaces newlines with spaces and truncates to 150 characters with "..." suffix.
func (c *Chunk) Preview() string {
	const maxLen = 150

	// Prefer signature if available
	text := c.Signature
	if text == "" {
		text = c.Content
	}

	if text == "" {
		return ""
	}

	// Replace newlines with spaces
	text = strings.ReplaceAll(text, "\n", " ")
	text = strings.ReplaceAll(text, "\t", " ")

	// Truncate if necessary (count runes, not bytes, for proper UTF-8 handling)
	if utf8.RuneCountInString(text) > maxLen {
		runes := []rune(text)
		return string(runes[:maxLen]) + "..."
	}

	return text
}

// SearchResult represents a chunk with its similarity score
type SearchResult struct {
	Chunk     Chunk     `json:"chunk"`
	Score     float32   `json:"score"`
	Relevance string    `json:"relevance,omitempty"` // "high", "medium", or "low"
	Preview   string    `json:"preview,omitempty"`   // Short preview of chunk content
	Embedding []float32 `json:"-"`                   // Not included in JSON output
}

// MinimalJSON returns a compact JSON representation for --min output
func (sr SearchResult) MinimalJSON() string {
	minimal := map[string]interface{}{
		"f": sr.Chunk.FilePath,
		"n": sr.Chunk.Name,
		"l": sr.Chunk.StartLine,
		"s": sr.Score,
	}
	// Include relevance if set
	if sr.Relevance != "" {
		minimal["r"] = sr.Relevance
	}
	// Include preview if set
	if sr.Preview != "" {
		minimal["pr"] = sr.Preview
	}
	data, _ := json.Marshal(minimal)
	return string(data)
}

// IndexStats holds statistics about the semantic index
type IndexStats struct {
	FilesIndexed   int    `json:"files_indexed"`
	ChunksTotal    int    `json:"chunks_total"`
	EmbeddingModel string `json:"embedding_model"`
	IndexSizeBytes int64  `json:"index_size_bytes"`
	LastUpdated    string `json:"last_updated"`
}

// IndexHealth represents the health status of the index
type IndexHealth struct {
	Status        string     `json:"status"` // "healthy", "stale", "missing"
	Stats         IndexStats `json:"stats"`
	StaleFiles    int        `json:"stale_files"`
	NewFiles      int        `json:"new_files"`
	ModifiedFiles int        `json:"modified_files"`
}

// ===== Memory Retrieval Stats Types =====

// RetrievalStats holds retrieval statistics for a memory entry.
type RetrievalStats struct {
	MemoryID       string  `json:"memory_id"`
	RetrievalCount int     `json:"retrieval_count"`
	LastRetrieved  string  `json:"last_retrieved,omitempty"`
	Status         string  `json:"status"`
	AvgScore       float32 `json:"avg_score,omitempty"`
	Question       string  `json:"question"`   // Memory question content
	CreatedAt      string  `json:"created_at"` // Memory creation timestamp
}

// MemoryRetrieval represents a single retrieval event for batch tracking.
type MemoryRetrieval struct {
	MemoryID string
	Score    float32
}

// RetrievalLogEntry represents a single retrieval event log.
type RetrievalLogEntry struct {
	ID        int64   `json:"id"`
	MemoryID  string  `json:"memory_id"`
	Query     string  `json:"query"`
	Score     float32 `json:"score"`
	Timestamp string  `json:"timestamp"`
}
