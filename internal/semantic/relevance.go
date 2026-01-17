package semantic

import (
	"sort"
)

// LabelRelevance returns a human-readable relevance label based on calibrated thresholds.
// Returns "high", "medium", or "low" based on score comparison to calibration thresholds.
// Returns empty string if calibration is nil (caller should use LabelByPercentile as fallback).
func LabelRelevance(score float32, cal *CalibrationMetadata) string {
	if cal == nil {
		return ""
	}

	// Clamp score to valid range [0,1]
	if score < 0 {
		score = 0
	} else if score > 1 {
		score = 1
	}

	switch {
	case score >= cal.HighThreshold:
		return "high"
	case score >= cal.MediumThreshold:
		return "medium"
	default:
		return "low"
	}
}

// LabelByPercentile assigns relevance labels based on the score's position within the result set.
// Used as fallback when no calibration data is available.
// Distribution: top 20% = "high", middle 50% = "medium", bottom 30% = "low"
func LabelByPercentile(score float32, allScores []float32) string {
	if len(allScores) == 0 {
		return "low"
	}

	// Clamp score to valid range [0,1]
	if score < 0 {
		score = 0
	} else if score > 1 {
		score = 1
	}

	// Single result is always "high" (it's the best we have)
	if len(allScores) == 1 {
		return "high"
	}

	// Sort scores descending to determine position
	sorted := make([]float32, len(allScores))
	copy(sorted, allScores)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i] > sorted[j]
	})

	// Find position of this score in sorted list
	position := 0
	for i, s := range sorted {
		if score >= s {
			position = i
			break
		}
		position = i + 1
	}

	// Calculate percentile thresholds
	n := len(allScores)
	highCutoff := int(float64(n) * 0.20) // Top 20%
	lowCutoff := int(float64(n) * 0.70)  // Bottom 30% starts at 70%

	// Ensure at least 1 result can be "high" for small sets
	if highCutoff == 0 {
		highCutoff = 1
	}

	// For small sets, ensure medium band exists (at least 2 results to have "low")
	if n <= 3 && lowCutoff <= highCutoff {
		lowCutoff = n // No "low" for very small sets
	}

	// Assign label based on position
	if position < highCutoff {
		return "high"
	}
	if position < lowCutoff {
		return "medium"
	}
	return "low"
}

// LabelAllByPercentile assigns relevance labels to all scores in a single pass.
// This is O(n log n) vs O(n² log n) when calling LabelByPercentile in a loop.
// Returns a slice of labels corresponding to each score in the input slice.
func LabelAllByPercentile(scores []float32) []string {
	n := len(scores)
	if n == 0 {
		return nil
	}

	labels := make([]string, n)

	// Single result is always "high"
	if n == 1 {
		labels[0] = "high"
		return labels
	}

	// Create index-score pairs for sorting while preserving original indices
	type indexedScore struct {
		index int
		score float32
	}
	indexed := make([]indexedScore, n)
	for i, s := range scores {
		// Clamp score to valid range [0,1]
		if s < 0 {
			s = 0
		} else if s > 1 {
			s = 1
		}
		indexed[i] = indexedScore{index: i, score: s}
	}

	// Sort by score descending (single sort - O(n log n))
	sort.Slice(indexed, func(i, j int) bool {
		return indexed[i].score > indexed[j].score
	})

	// Calculate percentile thresholds
	highCutoff := int(float64(n) * 0.20) // Top 20%
	lowCutoff := int(float64(n) * 0.70)  // Bottom 30% starts at 70%

	// Ensure at least 1 result can be "high" for small sets
	if highCutoff == 0 {
		highCutoff = 1
	}

	// For small sets, ensure medium band exists
	if n <= 3 && lowCutoff <= highCutoff {
		lowCutoff = n
	}

	// Assign labels based on sorted position (single pass - O(n))
	for position, item := range indexed {
		var label string
		if position < highCutoff {
			label = "high"
		} else if position < lowCutoff {
			label = "medium"
		} else {
			label = "low"
		}
		labels[item.index] = label
	}

	return labels
}
