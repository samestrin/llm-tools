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
