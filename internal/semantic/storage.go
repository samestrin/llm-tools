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
	TopK       int     // Maximum number of results to return (0 = unlimited/all results)
	Threshold  float32 // Minimum similarity score (0.0 - 1.0)
	Type       string  // Filter by chunk type
	PathFilter string  // Filter by file path pattern (glob)
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
