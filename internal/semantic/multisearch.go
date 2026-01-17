package semantic

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

const (
	// MaxQueries is the maximum number of queries allowed in a single Multisearch call
	MaxQueries = 10
)

// MultisearchOptions configures batch multisearch behavior
type MultisearchOptions struct {
	Queries   []string `json:"queries"`             // 1-10 search queries to execute
	TopK      int      `json:"top_k,omitempty"`     // Maximum total results to return (0 = unlimited)
	Threshold float32  `json:"threshold,omitempty"` // Minimum similarity score (0.0 - 1.0)
	Profiles  []string `json:"profiles,omitempty"`  // Profiles to search across (nil = default)
}

// Validate validates MultisearchOptions and returns an error if invalid
func (o *MultisearchOptions) Validate() error {
	if len(o.Queries) == 0 {
		return errors.New("queries cannot be empty")
	}
	if len(o.Queries) > MaxQueries {
		return fmt.Errorf("query count exceeds maximum of %d", MaxQueries)
	}
	for i, q := range o.Queries {
		if q == "" {
			return fmt.Errorf("query at index %d cannot be empty", i)
		}
	}
	if o.Threshold < 0 || o.Threshold > 1 {
		return errors.New("threshold must be between 0.0 and 1.0")
	}
	return nil
}

// MultisearchResult contains the results of a batch multisearch operation
type MultisearchResult struct {
	Results        []SearchResult `json:"results"`         // Deduplicated, sorted results
	TotalQueries   int            `json:"total_queries"`   // Number of queries executed
	TotalResults   int            `json:"total_results"`   // Number of results after deduplication
	QueriesMatched map[string]int `json:"queries_matched"` // How many results each query contributed
}

// Multisearch performs batch search across multiple queries with deduplication.
// Results are deduplicated by Chunk.ID, keeping the highest score for each chunk.
// Final results are sorted by score descending.
func (s *Searcher) Multisearch(ctx context.Context, opts MultisearchOptions) (*MultisearchResult, error) {
	// Validate options
	if err := opts.Validate(); err != nil {
		return nil, err
	}

	// Generate embeddings for all queries in batch
	embeddings, err := s.embedder.EmbedBatch(ctx, opts.Queries)
	if err != nil {
		return nil, fmt.Errorf("failed to generate embeddings: %w", err)
	}

	// Prepare search options
	searchOpts := SearchOptions{
		TopK:      opts.TopK,
		Threshold: opts.Threshold,
		Profiles:  opts.Profiles,
	}

	// Track results per query and deduplicate
	resultMap := make(map[string]SearchResult)
	queriesMatched := make(map[string]int)
	var mu sync.Mutex
	var wg sync.WaitGroup
	errChan := make(chan error, len(embeddings))

	for i, embedding := range embeddings {
		wg.Add(1)
		go func(idx int, emb []float32, query string) {
			defer wg.Done()

			results, err := s.storage.Search(ctx, emb, searchOpts)
			if err != nil {
				errChan <- err
				return
			}

			// Merge results under lock, keeping highest score for each chunk
			mu.Lock()
			queriesMatched[query] = len(results)
			for _, result := range results {
				existing, ok := resultMap[result.Chunk.ID]
				if !ok || result.Score > existing.Score {
					resultMap[result.Chunk.ID] = result
				}
			}
			mu.Unlock()
		}(i, embedding, opts.Queries[i])
	}

	wg.Wait()
	close(errChan)

	// Collect any errors
	var errs []error
	for err := range errChan {
		if err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return nil, errors.Join(errs...)
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

	// Apply relevance labels
	s.applyRelevanceLabels(ctx, results)

	return &MultisearchResult{
		Results:        results,
		TotalQueries:   len(opts.Queries),
		TotalResults:   len(results),
		QueriesMatched: queriesMatched,
	}, nil
}
