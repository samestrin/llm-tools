package semantic

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand"
	"sort"
	"time"
)

// ErrEmptyIndex indicates calibration cannot proceed with an empty index
var ErrEmptyIndex = errors.New("cannot calibrate: index is empty")

const (
	// defaultProbeCount is the number of probes for each calibration type
	defaultProbeCount = 3

	// unrelatedProbeText is a nonsense string that should match nothing well
	unrelatedProbeText = "xyzzy quantum banana 7392 lorem ipsum"
)

// CalibrationMetadata stores calibration results for score normalization.
// Different embedding models produce vastly different score distributions.
// This metadata allows normalizing scores to comparable thresholds.
type CalibrationMetadata struct {
	EmbeddingModel    string    `json:"embedding_model"`
	CalibrationDate   time.Time `json:"calibration_date"`
	PerfectMatchScore float32   `json:"perfect_match_score"`
	BaselineScore     float32   `json:"baseline_score"`
	ScoreRange        float32   `json:"score_range"`
	HighThreshold     float32   `json:"high_threshold"`
	MediumThreshold   float32   `json:"medium_threshold"`
	LowThreshold      float32   `json:"low_threshold"`
}

// RunCalibration executes the calibration workflow to determine score thresholds.
// It runs self-match probes to find the "perfect match" score and unrelated probes
// to find the baseline, then calculates normalized thresholds.
//
// Returns ErrEmptyIndex if the index has no chunks to probe.
func RunCalibration(ctx context.Context, storage Storage, embedder EmbedderInterface, modelName string) (*CalibrationMetadata, error) {
	// Check if index has any chunks
	stats, err := storage.Stats(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get index stats: %w", err)
	}
	if stats.ChunksTotal == 0 {
		return nil, ErrEmptyIndex
	}

	// Run self-match probes to find perfect match score
	perfectMatch, err := runSelfMatchProbes(ctx, storage, embedder, defaultProbeCount)
	if err != nil {
		return nil, fmt.Errorf("self-match probes failed: %w", err)
	}

	// Run unrelated probes to find baseline score
	baseline, err := runUnrelatedProbes(ctx, storage, embedder, defaultProbeCount)
	if err != nil {
		return nil, fmt.Errorf("unrelated probes failed: %w", err)
	}

	// Calculate thresholds
	high, medium, low := calculateThresholds(perfectMatch, baseline)
	scoreRange := perfectMatch - baseline

	return &CalibrationMetadata{
		EmbeddingModel:    modelName,
		CalibrationDate:   time.Now(),
		PerfectMatchScore: perfectMatch,
		BaselineScore:     baseline,
		ScoreRange:        scoreRange,
		HighThreshold:     high,
		MediumThreshold:   medium,
		LowThreshold:      low,
	}, nil
}

// runSelfMatchProbes selects random chunks and queries them with their own content.
// Returns the median score of all probes.
func runSelfMatchProbes(ctx context.Context, storage Storage, embedder EmbedderInterface, count int) (float32, error) {
	if count <= 0 {
		return 0, fmt.Errorf("probe count must be positive, got %d", count)
	}

	// Get chunks to probe
	chunks, err := storage.List(ctx, ListOptions{Limit: count * 3}) // Get more than we need for random selection
	if err != nil {
		return 0, fmt.Errorf("failed to list chunks: %w", err)
	}
	if len(chunks) == 0 {
		return 0, ErrEmptyIndex
	}

	// Shuffle and select probes
	// Note: Since Go 1.20, the global rand is auto-seeded, making this safe
	rand.Shuffle(len(chunks), func(i, j int) {
		chunks[i], chunks[j] = chunks[j], chunks[i]
	})

	// Limit to requested count
	probeCount := count
	if len(chunks) < count {
		probeCount = len(chunks)
	}

	scores := make([]float32, 0, probeCount)

	for i := 0; i < probeCount; i++ {
		chunk := chunks[i]

		// Embed the chunk's content
		embedding, err := embedder.Embed(ctx, chunk.Content)
		if err != nil {
			return 0, fmt.Errorf("failed to embed chunk content: %w", err)
		}

		// Search for the chunk - it should find itself with highest score
		results, err := storage.Search(ctx, embedding, SearchOptions{TopK: 1})
		if err != nil {
			return 0, fmt.Errorf("failed to search: %w", err)
		}

		if len(results) > 0 {
			scores = append(scores, results[0].Score)
		} else {
			slog.Debug("calibration self-match probe returned no results", "chunk_id", chunk.ID)
		}
	}

	if len(scores) == 0 {
		return 0, fmt.Errorf("no self-match scores collected")
	}

	return median(scores), nil
}

// runUnrelatedProbes queries the index with unrelated text to establish baseline.
// Returns the median score of all probes.
func runUnrelatedProbes(ctx context.Context, storage Storage, embedder EmbedderInterface, count int) (float32, error) {
	if count <= 0 {
		return 0, fmt.Errorf("probe count must be positive, got %d", count)
	}

	// Embed the unrelated probe text
	embedding, err := embedder.Embed(ctx, unrelatedProbeText)
	if err != nil {
		return 0, fmt.Errorf("failed to embed unrelated text: %w", err)
	}

	scores := make([]float32, 0, count)

	// Run multiple probes (same query, but captures variance in results)
	for i := 0; i < count; i++ {
		results, err := storage.Search(ctx, embedding, SearchOptions{TopK: 1})
		if err != nil {
			return 0, fmt.Errorf("failed to search: %w", err)
		}

		if len(results) > 0 {
			scores = append(scores, results[0].Score)
		} else {
			slog.Debug("calibration unrelated probe returned no results", "probe_num", i)
		}
	}

	if len(scores) == 0 {
		return 0, fmt.Errorf("no unrelated scores collected")
	}

	return median(scores), nil
}

// calculateThresholds derives high/medium/low thresholds from calibration scores.
//
// Threshold calculation:
//   - range := perfectMatch - baseline
//   - high := baseline + (0.70 * range)  // Top 30% of range
//   - medium := baseline + (0.40 * range) // Top 60% of range
//   - low := baseline + (0.15 * range)    // Above noise floor
func calculateThresholds(perfectMatch, baseline float32) (high, medium, low float32) {
	scoreRange := perfectMatch - baseline

	// Handle edge case where perfectMatch is invalid or range is non-positive
	if perfectMatch <= 0 || scoreRange <= 0 {
		// Return sensible defaults when calibration data is unusable
		high = 0.70
		medium = 0.40
		low = 0.15
		return
	}

	high = baseline + (0.70 * scoreRange)
	medium = baseline + (0.40 * scoreRange)
	low = baseline + (0.15 * scoreRange)
	return
}

// median calculates the median value of a float32 slice.
// Returns 0 for empty slices.
func median(scores []float32) float32 {
	if len(scores) == 0 {
		return 0
	}

	// Sort a copy to avoid modifying the original
	sorted := make([]float32, len(scores))
	copy(sorted, scores)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i] < sorted[j]
	})

	n := len(sorted)
	if n%2 == 0 {
		// Even count: average of two middle values
		return (sorted[n/2-1] + sorted[n/2]) / 2
	}
	// Odd count: middle value
	return sorted[n/2]
}
