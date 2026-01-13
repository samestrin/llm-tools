package semantic

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"time"
)

// MemoryStatus represents the status of a memory entry
type MemoryStatus string

const (
	// MemoryStatusPending is the default status for new entries
	MemoryStatusPending MemoryStatus = "pending"
	// MemoryStatusPromoted indicates the entry has been graduated to CLAUDE.md
	MemoryStatusPromoted MemoryStatus = "promoted"
)

// MemoryEntry represents a stored Q&A memory for semantic search
type MemoryEntry struct {
	ID          string       `json:"id"`
	Question    string       `json:"question"`
	Answer      string       `json:"answer"`
	Tags        []string     `json:"tags,omitempty"`
	Source      string       `json:"source,omitempty"`
	Status      MemoryStatus `json:"status"`
	Occurrences int          `json:"occurrences"`
	CreatedAt   string       `json:"created_at"`
	UpdatedAt   string       `json:"updated_at"`
}

// NewMemoryEntry creates a new MemoryEntry with default values
func NewMemoryEntry(question, answer string) *MemoryEntry {
	now := time.Now().Format(time.RFC3339)
	entry := &MemoryEntry{
		Question:    question,
		Answer:      answer,
		Tags:        []string{},
		Source:      "manual",
		Status:      MemoryStatusPending,
		Occurrences: 1,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	entry.ID = entry.GenerateID()
	return entry
}

// GenerateID creates a deterministic ID for the memory entry based on its content
func (m *MemoryEntry) GenerateID() string {
	data := fmt.Sprintf("memory:%s:%s", m.Question, m.CreatedAt)
	hash := sha256.Sum256([]byte(data))
	return fmt.Sprintf("mem-%x", hash[:8])
}

// EmbeddingText returns the formatted text used for generating embeddings
// Uses structured format: "Question: {question}\nAnswer: {answer}"
func (m *MemoryEntry) EmbeddingText() string {
	return fmt.Sprintf("Question: %s\nAnswer: %s", m.Question, m.Answer)
}

// MemorySearchResult pairs a memory entry with its similarity score
type MemorySearchResult struct {
	Entry     MemoryEntry `json:"entry"`
	Score     float32     `json:"score"`
	Embedding []float32   `json:"-"` // Not included in JSON output
}

// MinimalJSON returns a compact JSON representation for --min output
func (mr MemorySearchResult) MinimalJSON() string {
	// Truncate question to 50 chars for minimal output
	question := mr.Entry.Question
	if len(question) > 50 {
		question = question[:47] + "..."
	}

	minimal := map[string]interface{}{
		"id": mr.Entry.ID,
		"q":  question,
		"s":  mr.Score,
	}
	data, _ := json.Marshal(minimal)
	return string(data)
}

// MemorySearchOptions configures memory search behavior
type MemorySearchOptions struct {
	TopK      int          // Maximum results to return (default: 10)
	Threshold float32      // Minimum similarity score (0.0-1.0)
	Tags      []string     // Filter by tags (any match)
	Source    string       // Filter by source
	Status    MemoryStatus // Filter by status
}

// MemoryWithEmbedding pairs a memory entry with its embedding for batch operations
type MemoryWithEmbedding struct {
	Entry     MemoryEntry
	Embedding []float32
}

// MemoryListOptions configures how memory entries are listed
type MemoryListOptions struct {
	Limit  int          // Maximum number of entries to return (0 = no limit)
	Offset int          // Number of entries to skip
	Tags   []string     // Filter by tags (any match)
	Source string       // Filter by source
	Status MemoryStatus // Filter by status
}
