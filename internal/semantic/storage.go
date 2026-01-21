package semantic

import (
	"context"
	"errors"
)

var (
	// ErrNotFound indicates that a chunk was not found in storage
	ErrNotFound = errors.New("chunk not found")

	// ErrMemoryNotFound indicates that a memory entry was not found in storage
	ErrMemoryNotFound = errors.New("memory entry not found")

	// ErrStorageClosed indicates that the storage has been closed
	ErrStorageClosed = errors.New("storage is closed")
)

// ListOptions configures how chunks are listed
type ListOptions struct {
	Limit    int    // Maximum number of chunks to return (0 = no limit)
	Offset   int    // Number of chunks to skip
	FilePath string // Filter by file path
	Type     string // Filter by chunk type
	Language string // Filter by language
}

// SearchOptions configures how vector search is performed
type SearchOptions struct {
	TopK       int     `json:"top_k,omitempty"`       // Maximum number of results to return (0 = unlimited/all results)
	Threshold  float32 `json:"threshold,omitempty"`   // Minimum similarity score (0.0 - 1.0)
	Type       string  `json:"type,omitempty"`        // Filter by chunk type
	PathFilter string  `json:"path_filter,omitempty"` // Filter by file path pattern (glob)
	// Profiles specifies which profiles (collections) to search across.
	// NOTE: This field is NOT used by Storage.Search directly. It is used by
	// higher-level APIs (Searcher, MultiProfileSearcher) to coordinate multi-profile
	// search and is included here to allow unified option passing through the stack.
	Profiles []string `json:"profiles,omitempty"`

	// Reranking options - applied by Searcher, not Storage
	Rerank           bool    `json:"rerank,omitempty"`            // Enable reranking with cross-encoder
	RerankCandidates int     `json:"rerank_candidates,omitempty"` // Number of candidates to fetch for reranking (default: TopK * 5)
	RerankThreshold  float32 `json:"rerank_threshold,omitempty"`  // Minimum reranker score (0.0 - 1.0)
}

// ChunkWithEmbedding pairs a chunk with its embedding for batch operations
type ChunkWithEmbedding struct {
	Chunk     Chunk
	Embedding []float32
}

// LexicalSearcher is an optional interface for storage backends that support
// full-text search. Backends implementing this interface can be used with
// hybrid search (dense + lexical fusion).
type LexicalSearcher interface {
	// LexicalSearch performs full-text search using FTS5.
	// Returns results ranked by BM25 relevance score.
	LexicalSearch(ctx context.Context, query string, opts LexicalSearchOptions) ([]SearchResult, error)
}

// LexicalSearchOptions configures lexical search parameters.
// This is defined here to avoid circular imports with fts_sqlite.go.
type LexicalSearchOptions struct {
	TopK       int     // Maximum results to return (0 = unlimited/all results, default: 10)
	Type       string  // Filter by chunk type
	PathFilter string  // Filter by file path prefix
	Threshold  float64 // Minimum BM25 score (more negative = more relevant)
}

// Storage defines the interface for persisting and querying chunks with embeddings
type Storage interface {
	// Create stores a new chunk with its embedding
	Create(ctx context.Context, chunk Chunk, embedding []float32) error

	// CreateBatch stores multiple chunks with their embeddings in a single operation
	// This is more efficient than multiple Create calls, especially for network-based storage
	CreateBatch(ctx context.Context, chunks []ChunkWithEmbedding) error

	// Read retrieves a chunk by its ID
	Read(ctx context.Context, id string) (*Chunk, error)

	// Update replaces an existing chunk and its embedding
	Update(ctx context.Context, chunk Chunk, embedding []float32) error

	// Delete removes a chunk by its ID
	Delete(ctx context.Context, id string) error

	// DeleteByFilePath removes all chunks for a given file path
	DeleteByFilePath(ctx context.Context, filePath string) (int, error)

	// List retrieves chunks based on filter options
	List(ctx context.Context, opts ListOptions) ([]Chunk, error)

	// Search finds chunks similar to the query embedding
	Search(ctx context.Context, queryEmbedding []float32, opts SearchOptions) ([]SearchResult, error)

	// Stats returns statistics about the stored index
	Stats(ctx context.Context) (*IndexStats, error)

	// Clear removes all chunks from storage (for force re-index)
	Clear(ctx context.Context) error

	// GetFileHash retrieves the stored content hash for a file path
	// Returns empty string if file is not indexed
	GetFileHash(ctx context.Context, filePath string) (string, error)

	// SetFileHash stores the content hash for a file path
	SetFileHash(ctx context.Context, filePath string, hash string) error

	// Close releases any resources held by the storage
	Close() error

	// ===== Memory Entry Methods =====

	// StoreMemory stores a memory entry with its embedding
	StoreMemory(ctx context.Context, entry MemoryEntry, embedding []float32) error

	// StoreMemoryBatch stores multiple memory entries with their embeddings in a single operation
	StoreMemoryBatch(ctx context.Context, entries []MemoryWithEmbedding) error

	// SearchMemory finds memory entries similar to the query embedding
	SearchMemory(ctx context.Context, queryEmbedding []float32, opts MemorySearchOptions) ([]MemorySearchResult, error)

	// GetMemory retrieves a memory entry by ID
	GetMemory(ctx context.Context, id string) (*MemoryEntry, error)

	// DeleteMemory removes a memory entry by ID
	DeleteMemory(ctx context.Context, id string) error

	// ListMemory retrieves memory entries based on filter options
	ListMemory(ctx context.Context, opts MemoryListOptions) ([]MemoryEntry, error)

	// ===== Calibration Metadata Methods =====

	// GetCalibrationMetadata retrieves stored calibration data.
	// Returns (nil, nil) if no calibration has been performed yet.
	// Returns (nil, error) on storage errors.
	GetCalibrationMetadata(ctx context.Context) (*CalibrationMetadata, error)

	// SetCalibrationMetadata stores calibration data.
	// Overwrites any existing calibration data.
	SetCalibrationMetadata(ctx context.Context, meta *CalibrationMetadata) error
}

// MemoryStatsTracker is an optional interface for tracking memory retrieval statistics.
// Storage implementations that support memory stats should implement this interface.
type MemoryStatsTracker interface {
	// TrackMemoryRetrieval records a single memory retrieval event.
	TrackMemoryRetrieval(ctx context.Context, memoryID string, query string, score float32) error

	// TrackMemoryRetrievalBatch records multiple memory retrieval events in a single transaction.
	TrackMemoryRetrievalBatch(ctx context.Context, retrievals []MemoryRetrieval, query string) error

	// GetMemoryStats returns stats for a specific memory entry.
	GetMemoryStats(ctx context.Context, memoryID string) (*RetrievalStats, error)

	// GetAllMemoryStats returns stats for all tracked memories.
	GetAllMemoryStats(ctx context.Context) ([]RetrievalStats, error)

	// GetMemoryRetrievalHistory returns recent retrieval log entries for a memory.
	GetMemoryRetrievalHistory(ctx context.Context, memoryID string, limit int) ([]RetrievalLogEntry, error)

	// PruneMemoryRetrievalLog removes retrieval log entries older than the specified duration.
	PruneMemoryRetrievalLog(ctx context.Context, olderThanDays int) (int64, error)

	// UpdateMemoryStatsStatus updates the status of a memory entry.
	UpdateMemoryStatsStatus(ctx context.Context, memoryID string, status string) error
}
