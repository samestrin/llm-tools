package semantic

import (
	"context"
	"errors"
	"sort"
	"time"
)

// HybridMemorySearchOptions configures hybrid memory search.
type HybridMemorySearchOptions struct {
	MemorySearchOptions                      // Embedded dense search options
	FusionK             int                  // RRF k parameter (default: 60)
	FusionAlpha         float64              // Weighted fusion alpha (default: 0.7)
	UseWeighted         bool                 // Use weighted fusion instead of RRF
	Decay               *TemporalDecayConfig // nil = no decay
}

// HybridSearchMemory performs hybrid search (dense + lexical) on memory entries.
// Requires storage to implement MemoryLexicalSearcher.
func HybridSearchMemory(
	ctx context.Context,
	storage Storage,
	embedder EmbedderInterface,
	query string,
	opts HybridMemorySearchOptions,
) ([]MemorySearchResult, error) {
	if query == "" {
		return nil, errors.New("query cannot be empty")
	}

	// Verify storage supports lexical search
	lexicalSearcher, ok := storage.(MemoryLexicalSearcher)
	if !ok {
		return nil, errors.New("storage does not support lexical memory search")
	}

	// Set defaults
	fusionK := opts.FusionK
	if fusionK <= 0 {
		fusionK = 60
	}
	topK := opts.TopK
	if topK <= 0 {
		topK = 10
	}

	// Over-fetch for better fusion results
	overFetchK := topK * 3
	if overFetchK < 50 {
		overFetchK = 50
	}

	// Generate query embedding
	queryEmbedding, err := embedder.Embed(ctx, query)
	if err != nil {
		return nil, err
	}

	// Dense search
	denseOpts := opts.MemorySearchOptions
	denseOpts.TopK = overFetchK
	denseResults, err := storage.SearchMemory(ctx, queryEmbedding, denseOpts)
	if err != nil {
		return nil, err
	}

	// Lexical search
	lexicalOpts := MemoryLexicalSearchOptions{
		TopK:   overFetchK,
		Tags:   opts.Tags,
		Source: opts.Source,
		Status: opts.Status,
	}
	lexicalResults, err := lexicalSearcher.LexicalSearchMemory(ctx, query, lexicalOpts)
	if err != nil {
		// Degrade gracefully: use dense-only results
		lexicalResults = []MemorySearchResult{}
	}

	// Fuse results
	var results []MemorySearchResult
	if opts.UseWeighted {
		alpha := opts.FusionAlpha
		if alpha == 0 {
			alpha = 0.7
		}
		results, err = FuseWeightedMemory(denseResults, lexicalResults, alpha)
		if err != nil {
			return nil, err
		}
	} else {
		results = FuseRRFMemory(denseResults, lexicalResults, fusionK)
	}

	// Apply topK limit
	if topK > 0 && len(results) > topK {
		results = results[:topK]
	}

	// Apply temporal decay if configured
	if opts.Decay != nil && opts.Decay.Enabled {
		ApplyTemporalDecay(results, *opts.Decay, time.Now())
		// Re-sort after decay since it may change relative ordering
		sort.Slice(results, func(i, j int) bool {
			return results[i].Score > results[j].Score
		})
	}

	return results, nil
}
