package semantic

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
)

const (
	// MaxQueries is the maximum number of queries allowed in a single Multisearch call
	MaxQueries = 10
)

// OutputFormat specifies how multisearch results should be organized
type OutputFormat string

const (
	// OutputBlended returns all results in a flat array sorted by score (default)
	OutputBlended OutputFormat = "blended"
	// OutputByQuery groups results by which query they matched
	OutputByQuery OutputFormat = "by_query"
	// OutputByCollection groups results by which collection/profile they came from
	OutputByCollection OutputFormat = "by_collection"
)

// ValidOutputFormats returns all valid output format values
func ValidOutputFormats() []string {
	return []string{string(OutputBlended), string(OutputByQuery), string(OutputByCollection)}
}

// IsValidOutputFormat checks if the given format string is valid
func IsValidOutputFormat(format string) bool {
	switch OutputFormat(format) {
	case OutputBlended, OutputByQuery, OutputByCollection:
		return true
	case "": // empty means default (blended)
		return true
	}
	return false
}

// MultisearchOptions configures batch multisearch behavior
type MultisearchOptions struct {
	Queries         []string     `json:"queries"`                     // 1-10 search queries to execute
	TopK            int          `json:"top_k,omitempty"`             // Maximum total results to return (0 = unlimited)
	Threshold       float32      `json:"threshold,omitempty"`         // Minimum similarity score (0.0 - 1.0)
	Profiles        []string     `json:"profiles,omitempty"`          // Profiles to search across (nil = default)
	BoostMultiMatch *bool        `json:"boost_multi_match,omitempty"` // Boost scores for multi-match results (default: true)
	Output          OutputFormat `json:"output,omitempty"`            // Output format: blended (default), by_query, by_collection
}

// IsBoostEnabled returns whether multi-match boosting is enabled.
// Defaults to true if not explicitly set.
func (o *MultisearchOptions) IsBoostEnabled() bool {
	if o.BoostMultiMatch == nil {
		return true // default to enabled
	}
	return *o.BoostMultiMatch
}

const (
	// BoostFactor is the score boost per additional query match beyond the first
	BoostFactor = 0.05
	// MaxScore is the maximum allowed boosted score
	MaxScore = 1.0
)

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
	if o.TopK < 0 {
		return errors.New("top_k cannot be negative")
	}
	if o.Threshold < 0 || o.Threshold > 1 {
		return errors.New("threshold must be between 0.0 and 1.0")
	}
	if !IsValidOutputFormat(string(o.Output)) {
		return fmt.Errorf("invalid output format: %s", o.Output)
	}
	return nil
}

// EnhancedResult extends SearchResult with multi-match tracking and boosting.
type EnhancedResult struct {
	SearchResult            // Embedded base result
	MatchedQueries []string `json:"matched_queries"` // Which queries matched this result
	BoostedScore   float32  `json:"boosted_score"`   // Score after multi-match boosting
}

// MultisearchResult contains the results of a batch multisearch operation
type MultisearchResult struct {
	// Results is the deduplicated, sorted results with boosting (used for blended output)
	Results []EnhancedResult `json:"results,omitempty"`
	// ByQuery groups results by which query they matched (used for by_query output)
	ByQuery map[string][]EnhancedResult `json:"by_query,omitempty"`
	// ByCollection groups results by which collection/profile they came from (used for by_collection output)
	ByCollection map[string][]EnhancedResult `json:"by_collection,omitempty"`
	// Format indicates which output format was used
	Format         OutputFormat   `json:"format,omitempty"`
	TotalQueries   int            `json:"total_queries"`   // Number of queries executed
	TotalResults   int            `json:"total_results"`   // Number of results after deduplication
	QueriesMatched map[string]int `json:"queries_matched"` // How many results each query contributed
}

// CalculateBoostedScore computes the boosted score based on match count.
// Formula: BoostedScore = BaseScore + (0.05 * (MatchCount - 1))
// Score is capped at 1.0
func CalculateBoostedScore(baseScore float32, matchCount int) float32 {
	if matchCount <= 1 {
		return baseScore
	}
	boosted := baseScore + BoostFactor*float32(matchCount-1)
	if boosted > MaxScore {
		return MaxScore
	}
	return boosted
}

// enhancedResultEntry tracks a result with which queries matched it
type enhancedResultEntry struct {
	result         SearchResult
	matchedQueries map[string]bool // set of queries that matched this chunk
}

// Multisearch performs batch search across multiple queries with deduplication and boosting.
// Results are deduplicated by Chunk.ID, keeping the highest score for each chunk.
// Multi-match boosting adds 0.05 to the score for each additional query match.
// Final results are sorted by boosted score descending.
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
		TopK:      0, // Don't limit per-query, limit after merge
		Threshold: opts.Threshold,
		Profiles:  opts.Profiles,
	}

	// Track results per query with matched query tracking
	resultMap := make(map[string]*enhancedResultEntry)
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

			// Merge results under lock, keeping highest score and tracking matched queries
			mu.Lock()
			queriesMatched[query] = len(results)
			for _, result := range results {
				entry, ok := resultMap[result.Chunk.ID]
				if !ok {
					// New entry
					resultMap[result.Chunk.ID] = &enhancedResultEntry{
						result:         result,
						matchedQueries: map[string]bool{query: true},
					}
				} else {
					// Existing entry - update score if higher and track query
					if result.Score > entry.result.Score {
						entry.result = result
					}
					entry.matchedQueries[query] = true
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

	// Convert map to enhanced results with boosting
	boostEnabled := opts.IsBoostEnabled()
	results := make([]EnhancedResult, 0, len(resultMap))
	for _, entry := range resultMap {
		// Convert matched queries set to slice (preserve query order)
		matchedList := make([]string, 0, len(entry.matchedQueries))
		for _, q := range opts.Queries {
			if entry.matchedQueries[q] {
				matchedList = append(matchedList, q)
			}
		}

		// Calculate boosted score
		boostedScore := entry.result.Score
		if boostEnabled {
			boostedScore = CalculateBoostedScore(entry.result.Score, len(matchedList))
		}

		results = append(results, EnhancedResult{
			SearchResult:   entry.result,
			MatchedQueries: matchedList,
			BoostedScore:   boostedScore,
		})
	}

	// Sort by boosted score descending
	sortEnhancedResultsByScore(results)

	// Apply TopK limit
	if opts.TopK > 0 && len(results) > opts.TopK {
		results = results[:opts.TopK]
	}

	// Apply relevance labels to the embedded SearchResults
	searchResults := make([]SearchResult, len(results))
	for i := range results {
		searchResults[i] = results[i].SearchResult
	}
	s.applyRelevanceLabels(ctx, searchResults)
	for i := range results {
		results[i].SearchResult = searchResults[i]
	}

	return &MultisearchResult{
		Results:        results,
		TotalQueries:   len(opts.Queries),
		TotalResults:   len(results),
		QueriesMatched: queriesMatched,
	}, nil
}

// sortEnhancedResultsByScore sorts results by BoostedScore descending using O(n log n) sort
func sortEnhancedResultsByScore(results []EnhancedResult) {
	sort.Slice(results, func(i, j int) bool {
		return results[i].BoostedScore > results[j].BoostedScore
	})
}

// FormatByQuery converts a blended MultisearchResult to by_query format.
// Results are grouped by which queries they matched, and sorted by score within each group.
// A result that matches multiple queries will appear in multiple groups.
func (r *MultisearchResult) FormatByQuery() *MultisearchResult {
	byQuery := make(map[string][]EnhancedResult)

	// Group results by matched queries
	for _, result := range r.Results {
		for _, query := range result.MatchedQueries {
			byQuery[query] = append(byQuery[query], result)
		}
	}

	// Sort results within each group by boosted score
	for query := range byQuery {
		sortEnhancedResultsByScore(byQuery[query])
	}

	// Ensure all original queries have an entry (even if empty)
	for query := range r.QueriesMatched {
		if _, ok := byQuery[query]; !ok {
			byQuery[query] = []EnhancedResult{}
		}
	}

	return &MultisearchResult{
		ByQuery:        byQuery,
		Format:         OutputByQuery,
		TotalQueries:   r.TotalQueries,
		TotalResults:   r.TotalResults,
		QueriesMatched: r.QueriesMatched,
	}
}

// FormatByCollection converts a blended MultisearchResult to by_collection format.
// Results are grouped by their source collection/profile (using Chunk.Domain).
// Results are sorted by score within each group.
func (r *MultisearchResult) FormatByCollection() *MultisearchResult {
	byCollection := make(map[string][]EnhancedResult)

	// Group results by collection (Domain field)
	for _, result := range r.Results {
		domain := result.Chunk.Domain
		if domain == "" {
			domain = "default" // fallback for results without domain set
		}
		byCollection[domain] = append(byCollection[domain], result)
	}

	// Sort results within each group by boosted score
	for collection := range byCollection {
		sortEnhancedResultsByScore(byCollection[collection])
	}

	return &MultisearchResult{
		ByCollection:   byCollection,
		Format:         OutputByCollection,
		TotalQueries:   r.TotalQueries,
		TotalResults:   r.TotalResults,
		QueriesMatched: r.QueriesMatched,
	}
}

// FormatBlended returns the result in blended format (already the default).
// This is primarily for consistency when explicitly requesting blended output.
func (r *MultisearchResult) FormatBlended() *MultisearchResult {
	return &MultisearchResult{
		Results:        r.Results,
		Format:         OutputBlended,
		TotalQueries:   r.TotalQueries,
		TotalResults:   r.TotalResults,
		QueriesMatched: r.QueriesMatched,
	}
}

// FormatAs formats the result according to the specified output format.
func (r *MultisearchResult) FormatAs(format OutputFormat) *MultisearchResult {
	switch format {
	case OutputByQuery:
		return r.FormatByQuery()
	case OutputByCollection:
		return r.FormatByCollection()
	case OutputBlended, "":
		return r.FormatBlended()
	default:
		// Invalid format, default to blended
		return r.FormatBlended()
	}
}
