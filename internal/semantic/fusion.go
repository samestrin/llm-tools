package semantic

import (
	"fmt"
	"sort"
)

// FuseRRF combines dense and lexical search results using Reciprocal Rank Fusion.
// The formula is: score = sum(1 / (k + rank)) for each result list.
// Parameter k controls the smoothing - larger k reduces the impact of rank differences.
// Default k=60 is commonly used in literature.
// Returns results sorted by fused score in descending order.
func FuseRRF(denseResults, lexicalResults []SearchResult, k int) []SearchResult {
	results, _ := FuseRRFWithError(denseResults, lexicalResults, k)
	return results
}

// FuseRRFWithError is like FuseRRF but returns an error for invalid parameters.
func FuseRRFWithError(denseResults, lexicalResults []SearchResult, k int) ([]SearchResult, error) {
	if k <= 0 {
		return nil, fmt.Errorf("RRF k parameter must be positive, got: %d", k)
	}

	// Handle nil inputs
	if denseResults == nil {
		denseResults = []SearchResult{}
	}
	if lexicalResults == nil {
		lexicalResults = []SearchResult{}
	}

	// Map to accumulate scores by chunk ID
	scoreMap := make(map[string]float32)
	chunkMap := make(map[string]Chunk) // Preserve chunk data

	// Process dense results
	for rank, result := range denseResults {
		id := result.Chunk.ID
		rrfScore := float32(1.0 / float64(k+rank+1)) // rank+1 because ranks are 1-indexed in formula
		scoreMap[id] += rrfScore
		if _, exists := chunkMap[id]; !exists {
			chunkMap[id] = result.Chunk
		}
	}

	// Process lexical results
	for rank, result := range lexicalResults {
		id := result.Chunk.ID
		rrfScore := float32(1.0 / float64(k+rank+1))
		scoreMap[id] += rrfScore
		if _, exists := chunkMap[id]; !exists {
			chunkMap[id] = result.Chunk
		}
	}

	// Build result slice
	results := make([]SearchResult, 0, len(scoreMap))
	for id, score := range scoreMap {
		results = append(results, SearchResult{
			Chunk: chunkMap[id],
			Score: score,
		})
	}

	// Sort by score descending, with chunk ID as tie-breaker for determinism
	sort.Slice(results, func(i, j int) bool {
		if results[i].Score != results[j].Score {
			return results[i].Score > results[j].Score
		}
		return results[i].Chunk.ID < results[j].Chunk.ID
	})

	return results, nil
}

// FuseRRFWithTopK combines results using RRF and limits to top K results.
// If topK is 0 or negative, all results are returned.
func FuseRRFWithTopK(denseResults, lexicalResults []SearchResult, k int, topK int) []SearchResult {
	results := FuseRRF(denseResults, lexicalResults, k)
	if topK > 0 && len(results) > topK {
		results = results[:topK]
	}
	return results
}

// FuseWeighted combines dense and lexical results using weighted score fusion.
// The formula is: score = alpha * dense_score + (1-alpha) * lexical_score
// Alpha controls the balance: alpha=1.0 means 100% dense, alpha=0.0 means 100% lexical.
// Note: Scores are combined directly without normalization. For best results,
// ensure both score sources use comparable scales.
func FuseWeighted(denseResults, lexicalResults []SearchResult, alpha float64) ([]SearchResult, error) {
	if alpha < 0.0 || alpha > 1.0 {
		return nil, fmt.Errorf("fusion alpha must be between 0.0 and 1.0, got: %f", alpha)
	}

	// Handle nil inputs
	if denseResults == nil {
		denseResults = []SearchResult{}
	}
	if lexicalResults == nil {
		lexicalResults = []SearchResult{}
	}

	// Map to accumulate scores by chunk ID
	scoreMap := make(map[string]float64)
	chunkMap := make(map[string]Chunk)

	// Process dense results with alpha weight
	for _, result := range denseResults {
		id := result.Chunk.ID
		scoreMap[id] += alpha * float64(result.Score)
		if _, exists := chunkMap[id]; !exists {
			chunkMap[id] = result.Chunk
		}
	}

	// Process lexical results with (1-alpha) weight
	for _, result := range lexicalResults {
		id := result.Chunk.ID
		scoreMap[id] += (1.0 - alpha) * float64(result.Score)
		if _, exists := chunkMap[id]; !exists {
			chunkMap[id] = result.Chunk
		}
	}

	// Build result slice
	results := make([]SearchResult, 0, len(scoreMap))
	for id, score := range scoreMap {
		results = append(results, SearchResult{
			Chunk: chunkMap[id],
			Score: float32(score),
		})
	}

	// Sort by score descending, with chunk ID as tie-breaker for determinism
	sort.Slice(results, func(i, j int) bool {
		if results[i].Score != results[j].Score {
			return results[i].Score > results[j].Score
		}
		return results[i].Chunk.ID < results[j].Chunk.ID
	})

	return results, nil
}
