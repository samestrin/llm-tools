package semantic

import (
	"context"
	"errors"
)

// EmbedderInterface defines the interface for embedding generation
// (separate from the concrete Embedder to allow mocking)
type EmbedderInterface interface {
	Embed(ctx context.Context, text string) ([]float32, error)
	EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)
	Dimensions() int
}

// Searcher orchestrates semantic search across the index
type Searcher struct {
	storage  Storage
	embedder EmbedderInterface
}

// NewSearcher creates a new Searcher with the given storage and embedder
func NewSearcher(storage Storage, embedder EmbedderInterface) *Searcher {
	return &Searcher{
		storage:  storage,
		embedder: embedder,
	}
}

// Search performs semantic search using the query text
func (s *Searcher) Search(ctx context.Context, query string, opts SearchOptions) ([]SearchResult, error) {
	if query == "" {
		return nil, errors.New("query cannot be empty")
	}

	// Generate embedding for query
	queryEmbedding, err := s.embedder.Embed(ctx, query)
	if err != nil {
		return nil, err
	}

	// Search storage
	return s.storage.Search(ctx, queryEmbedding, opts)
}

// SearchMultiple performs search with multiple query terms and combines results
func (s *Searcher) SearchMultiple(ctx context.Context, queries []string, opts SearchOptions) ([]SearchResult, error) {
	if len(queries) == 0 {
		return nil, errors.New("queries cannot be empty")
	}

	// Generate embeddings for all queries
	embeddings, err := s.embedder.EmbedBatch(ctx, queries)
	if err != nil {
		return nil, err
	}

	// Collect all results
	resultMap := make(map[string]SearchResult)

	for _, embedding := range embeddings {
		results, err := s.storage.Search(ctx, embedding, opts)
		if err != nil {
			return nil, err
		}

		// Merge results, keeping highest score for each chunk
		for _, result := range results {
			existing, ok := resultMap[result.Chunk.ID]
			if !ok || result.Score > existing.Score {
				resultMap[result.Chunk.ID] = result
			}
		}
	}

	// Convert map to slice
	results := make([]SearchResult, 0, len(resultMap))
	for _, result := range resultMap {
		results = append(results, result)
	}

	// Sort by score descending
	sortResultsByScore(results)

	// Apply TopK limit
	if opts.TopK > 0 && len(results) > opts.TopK {
		results = results[:opts.TopK]
	}

	return results, nil
}
