package semantic

import (
	"context"
	"errors"
)

var (
	// ErrNotFound indicates that a chunk was not found in storage
	ErrNotFound = errors.New("chunk not found")

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
	TopK       int     // Maximum number of results to return
	Threshold  float32 // Minimum similarity score (0.0 - 1.0)
	Type       string  // Filter by chunk type
	PathFilter string  // Filter by file path pattern (glob)
}

// Storage defines the interface for persisting and querying chunks with embeddings
type Storage interface {
	// Create stores a new chunk with its embedding
	Create(ctx context.Context, chunk Chunk, embedding []float32) error

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

	// Close releases any resources held by the storage
	Close() error
}

