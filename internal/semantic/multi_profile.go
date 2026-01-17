package semantic

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"golang.org/x/sync/errgroup"
)

// MultiProfileSearcher orchestrates search across multiple storage profiles (collections).
// It executes queries in parallel across configured profiles and merges results.
type MultiProfileSearcher struct {
	embedder       EmbedderInterface
	storageMap     map[string]Storage
	defaultProfile string
	mu             sync.RWMutex
}

// NewMultiProfileSearcher creates a new MultiProfileSearcher with the given storage mapping.
// storageMap maps profile names (e.g., "code", "docs") to their Storage instances.
func NewMultiProfileSearcher(embedder EmbedderInterface, storageMap map[string]Storage) *MultiProfileSearcher {
	return &MultiProfileSearcher{
		embedder:       embedder,
		storageMap:     storageMap,
		defaultProfile: "code", // default fallback
	}
}

// SetDefaultProfile sets the profile to use when Profiles is nil or empty.
func (mps *MultiProfileSearcher) SetDefaultProfile(profile string) {
	mps.mu.Lock()
	defer mps.mu.Unlock()
	mps.defaultProfile = profile
}

// GetDefaultProfile returns the current default profile.
func (mps *MultiProfileSearcher) GetDefaultProfile() string {
	mps.mu.RLock()
	defer mps.mu.RUnlock()
	return mps.defaultProfile
}

// Search performs semantic search across specified profiles in parallel.
// If opts.Profiles is nil or empty, the default profile is used.
// Results are deduplicated by Chunk.ID (keeping highest score), merged, and sorted by score.
func (mps *MultiProfileSearcher) Search(ctx context.Context, query string, opts SearchOptions) ([]SearchResult, error) {
	if query == "" {
		return nil, fmt.Errorf("query cannot be empty")
	}

	// Resolve profiles to search
	profiles := opts.Profiles
	if len(profiles) == 0 {
		mps.mu.RLock()
		profiles = []string{mps.defaultProfile}
		mps.mu.RUnlock()
	}

	// Validate all profiles exist
	for _, profile := range profiles {
		if _, ok := mps.storageMap[profile]; !ok {
			return nil, fmt.Errorf("unknown profile: '%s'", profile)
		}
	}

	// Generate query embedding once
	queryEmbedding, err := mps.embedder.Embed(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to generate embedding: %w", err)
	}

	// Single profile - no parallel overhead
	if len(profiles) == 1 {
		storage := mps.storageMap[profiles[0]]
		results, err := storage.Search(ctx, queryEmbedding, opts)
		if err != nil {
			return nil, err
		}
		// Tag results with profile
		for i := range results {
			if results[i].Chunk.Domain == "" {
				results[i].Chunk.Domain = profiles[0]
			}
		}
		return results, nil
	}

	// Multiple profiles - parallel execution with errgroup
	type profileResult struct {
		profile string
		results []SearchResult
		err     error
	}

	resultChan := make(chan profileResult, len(profiles))
	g, gctx := errgroup.WithContext(ctx)

	for _, profile := range profiles {
		profile := profile // capture for goroutine
		storage := mps.storageMap[profile]

		g.Go(func() error {
			results, err := storage.Search(gctx, queryEmbedding, opts)
			if err != nil {
				// Log error but don't fail the group - allow partial results
				slog.Warn("profile query failed", "profile", profile, "error", err)
				resultChan <- profileResult{profile: profile, err: err}
				return nil // Don't propagate error to allow partial results
			}

			// Tag results with profile
			for i := range results {
				if results[i].Chunk.Domain == "" {
					results[i].Chunk.Domain = profile
				}
			}

			resultChan <- profileResult{profile: profile, results: results}
			return nil
		})
	}

	// Wait for all goroutines and then close channel
	go func() {
		g.Wait()
		close(resultChan)
	}()

	// Collect results and merge
	resultMap := make(map[string]SearchResult)
	var successCount int

	for pr := range resultChan {
		if pr.err != nil {
			continue // Skip failed profiles
		}
		successCount++

		// Merge results, keeping highest score for each chunk ID
		for _, result := range pr.results {
			existing, ok := resultMap[result.Chunk.ID]
			if !ok || result.Score > existing.Score {
				resultMap[result.Chunk.ID] = result
			}
		}
	}

	// Check if all queries failed
	if successCount == 0 {
		return nil, fmt.Errorf("all profile queries failed")
	}

	// Convert map to slice
	merged := make([]SearchResult, 0, len(resultMap))
	for _, result := range resultMap {
		merged = append(merged, result)
	}

	// Sort by score descending
	sortResultsByScore(merged)

	// Apply TopK limit
	if opts.TopK > 0 && len(merged) > opts.TopK {
		merged = merged[:opts.TopK]
	}

	return merged, nil
}

// Multisearch performs batch search across multiple queries and profiles.
// Uses the underlying Multisearch implementation with multi-profile support.
func (mps *MultiProfileSearcher) Multisearch(ctx context.Context, opts MultisearchOptions) (*MultisearchResult, error) {
	// Validate options
	if err := opts.Validate(); err != nil {
		return nil, err
	}

	// Resolve profiles
	profiles := opts.Profiles
	if len(profiles) == 0 {
		mps.mu.RLock()
		profiles = []string{mps.defaultProfile}
		mps.mu.RUnlock()
	}

	// Validate all profiles exist
	for _, profile := range profiles {
		if _, ok := mps.storageMap[profile]; !ok {
			return nil, fmt.Errorf("unknown profile: '%s'", profile)
		}
	}

	// Generate embeddings for all queries in batch
	embeddings, err := mps.embedder.EmbedBatch(ctx, opts.Queries)
	if err != nil {
		return nil, fmt.Errorf("failed to generate embeddings: %w", err)
	}

	// Create search options for storage queries
	searchOpts := SearchOptions{
		TopK:      0, // Don't limit per-query, limit after merge
		Threshold: opts.Threshold,
	}

	// Track results per query and deduplicate
	resultMap := make(map[string]SearchResult)
	queriesMatched := make(map[string]int)
	var mu sync.Mutex

	g, gctx := errgroup.WithContext(ctx)

	// Execute all queries across all profiles in parallel
	for i, embedding := range embeddings {
		queryIdx := i
		emb := embedding
		query := opts.Queries[queryIdx]

		for _, profile := range profiles {
			profile := profile
			storage := mps.storageMap[profile]

			g.Go(func() error {
				results, err := storage.Search(gctx, emb, searchOpts)
				if err != nil {
					slog.Warn("query failed", "query", query, "profile", profile, "error", err)
					return nil // Allow partial results
				}

				mu.Lock()
				queriesMatched[query] += len(results)
				for _, result := range results {
					if result.Chunk.Domain == "" {
						result.Chunk.Domain = profile
					}
					existing, ok := resultMap[result.Chunk.ID]
					if !ok || result.Score > existing.Score {
						resultMap[result.Chunk.ID] = result
					}
				}
				mu.Unlock()

				return nil
			})
		}
	}

	if err := g.Wait(); err != nil {
		return nil, err
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

	return &MultisearchResult{
		Results:        results,
		TotalQueries:   len(opts.Queries),
		TotalResults:   len(results),
		QueriesMatched: queriesMatched,
	}, nil
}
