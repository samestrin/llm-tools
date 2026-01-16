package semantic

import (
	"context"
	"errors"
	"fmt"
)

// EmbedderInterface defines the interface for embedding generation
// (separate from the concrete Embedder to allow mocking)
type EmbedderInterface interface {
	Embed(ctx context.Context, text string) ([]float32, error)
	EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)
	Dimensions() int
	Model() string
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
	if opts.Threshold < 0 || opts.Threshold > 1 {
		return nil, errors.New("threshold must be between 0.0 and 1.0")
	}
	if opts.TopK < 0 {
		return nil, errors.New("topK must be non-negative")
	}

	// Generate embedding for query
	queryEmbedding, err := s.embedder.Embed(ctx, query)
	if err != nil {
		return nil, err
	}

	// Search storage
	return s.storage.Search(ctx, queryEmbedding, opts)
}

// HybridSearchOptions configures hybrid search behavior.
type HybridSearchOptions struct {
	SearchOptions         // Embedded search options
	FusionK       int     // RRF k parameter (default: 60)
	FusionAlpha   float64 // Weighted fusion alpha (default: 0.7)
	UseWeighted   bool    // Use weighted fusion instead of RRF
}

// HybridSearch performs combined dense (vector) and lexical (FTS5) search.
// Results are fused using Reciprocal Rank Fusion (RRF) for improved recall.
// Returns an error if the storage doesn't support lexical search.
func (s *Searcher) HybridSearch(ctx context.Context, query string, opts HybridSearchOptions) ([]SearchResult, error) {
	if query == "" {
		return nil, errors.New("query cannot be empty")
	}
	if opts.Threshold < 0 || opts.Threshold > 1 {
		return nil, errors.New("threshold must be between 0.0 and 1.0")
	}
	if opts.TopK < 0 {
		return nil, errors.New("topK must be non-negative")
	}

	// Check if storage supports lexical search
	lexicalSearcher, ok := s.storage.(LexicalSearcher)
	if !ok {
		return nil, errors.New("hybrid search requires storage with lexical search support")
	}

	// Set defaults
	k := opts.FusionK
	if k <= 0 {
		k = 60
	}
	alpha := opts.FusionAlpha
	if alpha == 0 {
		alpha = 0.7 // Default when not specified; alpha=0 (lexical-only) is still achievable via explicit negative
	}

	// Perform dense (vector) search
	queryEmbedding, err := s.embedder.Embed(ctx, query)
	if err != nil {
		return nil, err
	}

	denseResults, err := s.storage.Search(ctx, queryEmbedding, opts.SearchOptions)
	if err != nil {
		return nil, err
	}

	// Perform lexical search
	lexicalOpts := LexicalSearchOptions{
		TopK:       opts.TopK,
		Type:       opts.Type,
		PathFilter: opts.PathFilter,
	}
	lexicalResults, err := lexicalSearcher.LexicalSearch(ctx, query, lexicalOpts)
	if err != nil {
		// Log but don't fail - fall back to dense-only
		// fmt.Fprintf(os.Stderr, "Warning: lexical search failed: %v\n", err)
		return denseResults, nil
	}

	// Fuse results
	var results []SearchResult
	if opts.UseWeighted {
		results, err = FuseWeighted(denseResults, lexicalResults, alpha)
		if err != nil {
			return nil, err
		}
	} else {
		results = FuseRRF(denseResults, lexicalResults, k)
	}

	// Apply TopK limit
	if opts.TopK > 0 && len(results) > opts.TopK {
		results = results[:opts.TopK]
	}

	return results, nil
}

// SearchMultiple performs search with multiple query terms and combines results
func (s *Searcher) SearchMultiple(ctx context.Context, queries []string, opts SearchOptions) ([]SearchResult, error) {
	if len(queries) == 0 {
		return nil, errors.New("queries cannot be empty")
	}
	for i, q := range queries {
		if q == "" {
			return nil, fmt.Errorf("query at index %d cannot be empty", i)
		}
	}
	if opts.Threshold < 0 || opts.Threshold > 1 {
		return nil, errors.New("threshold must be between 0.0 and 1.0")
	}
	if opts.TopK < 0 {
		return nil, errors.New("topK must be non-negative")
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
