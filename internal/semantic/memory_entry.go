package semantic

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"strings"
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
	FilePath    string       `json:"file_path,omitempty"`
	Sprints     []string     `json:"sprints,omitempty"`
	Files       []string     `json:"files,omitempty"`
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

// GenerateID creates a deterministic ID for the memory entry based on its content.
// Format: mem-{YYYY-MM-DD}-{first 6 hex chars of md5(question)}
func (m *MemoryEntry) GenerateID() string {
	hash := md5.Sum([]byte(m.Question))
	date := m.CreatedAt
	// Extract date portion if CreatedAt is RFC3339
	if len(date) >= 10 {
		date = date[:10]
	}
	return fmt.Sprintf("mem-%s-%x", date, hash[:3])
}

// EmbeddingText returns the formatted text used for generating embeddings
// Uses structured format: "Question: {question}\nAnswer: {answer}"
func (m *MemoryEntry) EmbeddingText() string {
	return fmt.Sprintf("Question: %s\nAnswer: %s", m.Question, m.Answer)
}

// MemoryFileContent generates structured markdown with YAML frontmatter for file-backed memory.
func (m *MemoryEntry) MemoryFileContent() string {
	// Derive type from tags or source
	memType := "project"
	if m.Source != "" && m.Source != "manual" {
		memType = m.Source
	} else if len(m.Tags) > 0 {
		memType = m.Tags[0]
	}

	// Extract date from CreatedAt
	created := m.CreatedAt
	if len(created) >= 10 {
		created = created[:10]
	}

	// Derive brief title from question (first 60 chars, no newlines)
	title := m.Question
	if len(title) > 60 {
		title = title[:60]
	}
	title = strings.ReplaceAll(title, "\n", " ")

	// Format arrays for YAML
	formatArray := func(items []string) string {
		if len(items) == 0 {
			return "[]"
		}
		formatted := make([]string, len(items))
		for i, item := range items {
			formatted[i] = strings.TrimSpace(item)
		}
		return "[" + strings.Join(formatted, ", ") + "]"
	}

	var buf strings.Builder

	// Frontmatter
	buf.WriteString("---\n")
	buf.WriteString(fmt.Sprintf("id: %s\n", m.ID))
	buf.WriteString(fmt.Sprintf("question: %q\n", m.Question))
	buf.WriteString(fmt.Sprintf("created: %s\n", created))
	buf.WriteString("last_retrieved: \"\"\n")
	buf.WriteString(fmt.Sprintf("sprints: %s\n", formatArray(m.Sprints)))
	buf.WriteString(fmt.Sprintf("files: %s\n", formatArray(m.Files)))
	buf.WriteString(fmt.Sprintf("tags: %s\n", formatArray(m.Tags)))
	buf.WriteString("retrievals: 0\n")
	buf.WriteString("status: active\n")
	buf.WriteString(fmt.Sprintf("type: %s\n", memType))
	buf.WriteString("---\n\n")

	// Body
	buf.WriteString(fmt.Sprintf("# %s\n\n", title))
	buf.WriteString("## Decision\n\n")
	buf.WriteString(m.Answer)
	buf.WriteString("\n\n")
	buf.WriteString("## Rationale\n\n")
	buf.WriteString("- [from context]\n\n")
	buf.WriteString("## Applies When\n\n")
	buf.WriteString("- [conditions]\n\n")
	buf.WriteString("## Code Reference\n\n")

	if len(m.Files) > 0 {
		for _, f := range m.Files {
			buf.WriteString(fmt.Sprintf("- %s\n", strings.TrimSpace(f)))
		}
	} else {
		buf.WriteString("N/A\n")
	}

	return buf.String()
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
	TopK      int          // Maximum results to return (0 = unlimited/all results, default: 10)
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
