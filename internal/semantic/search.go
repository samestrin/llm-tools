package semantic

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
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
	reranker RerankerInterface // Optional reranker for improved precision
}

// NewSearcher creates a new Searcher with the given storage and embedder
func NewSearcher(storage Storage, embedder EmbedderInterface) *Searcher {
	return &Searcher{
		storage:  storage,
		embedder: embedder,
	}
}

// NewSearcherWithReranker creates a Searcher with reranking support
func NewSearcherWithReranker(storage Storage, embedder EmbedderInterface, reranker RerankerInterface) *Searcher {
	return &Searcher{
		storage:  storage,
		embedder: embedder,
		reranker: reranker,
	}
}

// SetReranker sets or updates the reranker (can be nil to disable)
func (s *Searcher) SetReranker(reranker RerankerInterface) {
	s.reranker = reranker
}

// HasReranker returns true if a reranker is configured
func (s *Searcher) HasReranker() bool {
	return s.reranker != nil
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

	// Determine if reranking is enabled
	shouldRerank := opts.Rerank && s.reranker != nil

	// If reranking, over-fetch candidates
	searchOpts := opts
	if shouldRerank {
		candidates := opts.RerankCandidates
		if candidates <= 0 {
			// Default: fetch 5x the desired results, minimum 50
			candidates = opts.TopK * 5
			if candidates < 50 {
				candidates = 50
			}
		}
		searchOpts.TopK = candidates
		// Don't apply embedding threshold when reranking - let reranker decide
		searchOpts.Threshold = 0
		// If user provided threshold but no rerank threshold, use threshold as rerank threshold
		if opts.RerankThreshold == 0 && opts.Threshold > 0 {
			searchOpts.RerankThreshold = opts.Threshold
		}
	}

	// Search storage
	results, err := s.storage.Search(ctx, queryEmbedding, searchOpts)
	if err != nil {
		return nil, err
	}

	// Apply reranking if enabled
	if shouldRerank && len(results) > 0 {
		results, err = s.applyReranking(ctx, query, results, searchOpts)
		if err != nil {
			// Log reranking error but fall back to embedding-only results
			slog.Warn("reranking failed, using embedding scores", "error", err)
		}
	}

	// Apply relevance labels
	s.applyRelevanceLabels(ctx, results)

	return results, nil
}

// applyReranking reranks results using the cross-encoder and updates scores
func (s *Searcher) applyReranking(ctx context.Context, query string, results []SearchResult, opts SearchOptions) ([]SearchResult, error) {
	if len(results) == 0 {
		return results, nil
	}

	// Extract document texts for reranking
	documents := make([]string, len(results))
	for i, r := range results {
		documents[i] = r.Chunk.Content
	}

	// Get reranker scores
	scores, err := s.reranker.Rerank(ctx, query, documents)
	if err != nil {
		return results, err
	}

	// Update results with reranker scores
	for i := range results {
		if i < len(scores) {
			results[i].Score = scores[i]
		}
	}

	// Sort by new scores (descending)
	sortResultsByScore(results)

	// Apply reranker threshold
	if opts.RerankThreshold > 0 {
		filtered := results[:0]
		for _, r := range results {
			if r.Score >= opts.RerankThreshold {
				filtered = append(filtered, r)
			}
		}
		results = filtered
	}

	// Apply TopK limit
	if opts.TopK > 0 && len(results) > opts.TopK {
		results = results[:opts.TopK]
	}

	return results, nil
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
		// Log and fall back to dense-only search
		slog.Warn("lexical search failed, falling back to dense-only", "error", err)
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

	// Apply relevance labels
	s.applyRelevanceLabels(ctx, results)

	return results, nil
}

// PrefilterSearchOptions configures prefilter search behavior.
type PrefilterSearchOptions struct {
	SearchOptions     // Embedded search options
	PrefilterTopK int // Number of candidates from lexical search (default: TopK * 10)
}

// PrefilterSearch performs a two-stage search: first lexical (FTS5) to narrow candidates,
// then vector search on those candidates. This improves performance on large indexes by
// reducing the vector search space while maintaining semantic relevance.
// Returns an error if the storage doesn't support lexical search.
func (s *Searcher) PrefilterSearch(ctx context.Context, query string, opts PrefilterSearchOptions) ([]SearchResult, error) {
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
		return nil, errors.New("prefilter search requires storage with lexical search support")
	}

	// Set defaults for prefilter candidates
	prefilterTopK := opts.PrefilterTopK
	if prefilterTopK <= 0 {
		// Default: fetch 10x the desired results for prefiltering
		prefilterTopK = opts.TopK * 10
		if prefilterTopK < 100 {
			prefilterTopK = 100
		}
	}

	// Stage 1: Lexical search to get candidate chunk IDs
	lexicalOpts := LexicalSearchOptions{
		TopK:       prefilterTopK,
		Type:       opts.Type,
		PathFilter: opts.PathFilter,
	}
	lexicalResults, err := lexicalSearcher.LexicalSearch(ctx, query, lexicalOpts)
	if err != nil {
		// Log and fall back to regular search
		slog.Warn("lexical prefilter failed, falling back to regular search", "error", err)
		return s.Search(ctx, query, opts.SearchOptions)
	}

	// If no lexical matches, fall back to regular search
	if len(lexicalResults) == 0 {
		slog.Debug("no lexical matches for prefilter, falling back to regular search")
		return s.Search(ctx, query, opts.SearchOptions)
	}

	// Extract chunk IDs from lexical results
	chunkIDs := make([]string, len(lexicalResults))
	for i, r := range lexicalResults {
		chunkIDs[i] = r.Chunk.ID
	}

	slog.Debug("prefilter narrowed search space", "lexical_candidates", len(chunkIDs), "original_topk", opts.TopK)

	// Stage 2: Vector search restricted to candidate chunks
	searchOpts := opts.SearchOptions
	searchOpts.ChunkIDs = chunkIDs

	// Generate embedding for query
	queryEmbedding, err := s.embedder.Embed(ctx, query)
	if err != nil {
		return nil, err
	}

	// Determine if reranking is enabled
	shouldRerank := opts.Rerank && s.reranker != nil

	// If reranking, adjust options
	if shouldRerank {
		candidates := opts.RerankCandidates
		if candidates <= 0 {
			candidates = opts.TopK * 5
			if candidates < 50 {
				candidates = 50
			}
		}
		searchOpts.TopK = candidates
		searchOpts.Threshold = 0
		if opts.RerankThreshold == 0 && opts.Threshold > 0 {
			searchOpts.RerankThreshold = opts.Threshold
		}
	}

	// Search storage with chunk ID filter
	results, err := s.storage.Search(ctx, queryEmbedding, searchOpts)
	if err != nil {
		return nil, err
	}

	// Apply reranking if enabled
	if shouldRerank && len(results) > 0 {
		results, err = s.applyReranking(ctx, query, results, searchOpts)
		if err != nil {
			slog.Warn("reranking failed, using embedding scores", "error", err)
		}
	}

	// Apply relevance labels
	s.applyRelevanceLabels(ctx, results)

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

	// Collect all results using parallel search
	resultMap := make(map[string]SearchResult)
	var mu sync.Mutex
	var wg sync.WaitGroup
	errChan := make(chan error, len(embeddings))

	for _, embedding := range embeddings {
		wg.Add(1)
		go func(emb []float32) {
			defer wg.Done()

			results, err := s.storage.Search(ctx, emb, opts)
			if err != nil {
				errChan <- err
				return
			}

			// Merge results under lock, keeping highest score for each chunk
			mu.Lock()
			for _, result := range results {
				existing, ok := resultMap[result.Chunk.ID]
				if !ok || result.Score > existing.Score {
					resultMap[result.Chunk.ID] = result
				}
			}
			mu.Unlock()
		}(embedding)
	}

	wg.Wait()
	close(errChan)

	// Collect all errors from goroutines
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

	return results, nil
}

// ApplyPercentileRelevanceLabels applies percentile-based relevance labels to results.
// This is useful for multi-profile search where calibration data is not available.
func ApplyPercentileRelevanceLabels(results []SearchResult) {
	if len(results) == 0 {
		return
	}
	allScores := make([]float32, len(results))
	for i, r := range results {
		allScores[i] = r.Score
	}
	labels := LabelAllByPercentile(allScores)
	for i := range results {
		results[i].Relevance = labels[i]
		results[i].Preview = results[i].Chunk.Preview()
	}
}

// applyRelevanceLabels applies relevance labels and previews to search results.
// Uses calibration thresholds if available, otherwise falls back to percentile-based labeling.
func (s *Searcher) applyRelevanceLabels(ctx context.Context, results []SearchResult) {
	if len(results) == 0 {
		return
	}

	// Try to get calibration metadata
	var cal *CalibrationMetadata
	cal, err := s.storage.GetCalibrationMetadata(ctx)
	if err != nil {
		slog.Debug("failed to get calibration metadata, using percentile fallback", "error", err)
		cal = nil
	}

	if cal != nil {
		// Use calibration-based labeling
		for i := range results {
			results[i].Relevance = LabelRelevance(results[i].Score, cal)
			results[i].Preview = results[i].Chunk.Preview()
		}
	} else {
		// Fallback to percentile-based labeling (O(n log n) batch processing)
		slog.Debug("no calibration data available, using percentile-based relevance labeling")
		allScores := make([]float32, len(results))
		for i, r := range results {
			allScores[i] = r.Score
		}
		labels := LabelAllByPercentile(allScores)
		for i := range results {
			results[i].Relevance = labels[i]
			results[i].Preview = results[i].Chunk.Preview()
		}
	}
}
