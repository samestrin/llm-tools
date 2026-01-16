package semantic

import (
	"testing"
)

func TestLabelRelevance_WithCalibration(t *testing.T) {
	cal := &CalibrationMetadata{
		HighThreshold:   0.70,
		MediumThreshold: 0.40,
		LowThreshold:    0.15,
	}

	tests := []struct {
		name     string
		score    float32
		expected string
	}{
		// High relevance cases
		{"score above high threshold", 0.85, "high"},
		{"score at high threshold", 0.70, "high"},
		{"score just above high threshold", 0.701, "high"},

		// Medium relevance cases
		{"score between thresholds", 0.55, "medium"},
		{"score at medium threshold", 0.40, "medium"},
		{"score just above medium threshold", 0.401, "medium"},
		{"score just below high threshold", 0.699, "medium"},

		// Low relevance cases
		{"score below medium threshold", 0.30, "low"},
		{"score at low threshold", 0.15, "low"},
		{"score below low threshold", 0.10, "low"},
		{"score just below medium threshold", 0.399, "low"},
		{"zero score", 0.0, "low"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := LabelRelevance(tt.score, cal)
			if got != tt.expected {
				t.Errorf("LabelRelevance(%v, cal) = %q, want %q", tt.score, got, tt.expected)
			}
		})
	}
}

func TestLabelRelevance_NilCalibration(t *testing.T) {
	// When calibration is nil, should return empty string (caller handles fallback)
	got := LabelRelevance(0.5, nil)
	if got != "" {
		t.Errorf("LabelRelevance(0.5, nil) = %q, want empty string", got)
	}
}

func TestLabelByPercentile(t *testing.T) {
	tests := []struct {
		name      string
		score     float32
		allScores []float32
		expected  string
	}{
		// Top 20% = high
		{
			name:      "top result in 10 results",
			score:     0.95,
			allScores: []float32{0.95, 0.85, 0.75, 0.65, 0.55, 0.45, 0.35, 0.25, 0.15, 0.05},
			expected:  "high",
		},
		{
			name:      "second result in 10 results (top 20%)",
			score:     0.85,
			allScores: []float32{0.95, 0.85, 0.75, 0.65, 0.55, 0.45, 0.35, 0.25, 0.15, 0.05},
			expected:  "high",
		},

		// Middle 50% = medium
		{
			name:      "third result in 10 results (medium)",
			score:     0.75,
			allScores: []float32{0.95, 0.85, 0.75, 0.65, 0.55, 0.45, 0.35, 0.25, 0.15, 0.05},
			expected:  "medium",
		},
		{
			name:      "middle result in 10 results",
			score:     0.55,
			allScores: []float32{0.95, 0.85, 0.75, 0.65, 0.55, 0.45, 0.35, 0.25, 0.15, 0.05},
			expected:  "medium",
		},
		{
			name:      "7th result in 10 results (bottom of medium)",
			score:     0.35,
			allScores: []float32{0.95, 0.85, 0.75, 0.65, 0.55, 0.45, 0.35, 0.25, 0.15, 0.05},
			expected:  "medium",
		},

		// Bottom 30% = low
		{
			name:      "8th result in 10 results (low)",
			score:     0.25,
			allScores: []float32{0.95, 0.85, 0.75, 0.65, 0.55, 0.45, 0.35, 0.25, 0.15, 0.05},
			expected:  "low",
		},
		{
			name:      "last result in 10 results",
			score:     0.05,
			allScores: []float32{0.95, 0.85, 0.75, 0.65, 0.55, 0.45, 0.35, 0.25, 0.15, 0.05},
			expected:  "low",
		},

		// Edge cases
		{
			name:      "single result always high",
			score:     0.5,
			allScores: []float32{0.5},
			expected:  "high",
		},
		{
			name:      "two results - first is high",
			score:     0.8,
			allScores: []float32{0.8, 0.3},
			expected:  "high",
		},
		{
			name:      "two results - second is medium/low boundary",
			score:     0.3,
			allScores: []float32{0.8, 0.3},
			expected:  "medium",
		},
		{
			name:      "five results - top 1 is high (20%)",
			score:     0.9,
			allScores: []float32{0.9, 0.7, 0.5, 0.3, 0.1},
			expected:  "high",
		},
		{
			name:      "five results - bottom 1-2 are low (30%)",
			score:     0.1,
			allScores: []float32{0.9, 0.7, 0.5, 0.3, 0.1},
			expected:  "low",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := LabelByPercentile(tt.score, tt.allScores)
			if got != tt.expected {
				t.Errorf("LabelByPercentile(%v, %v) = %q, want %q", tt.score, tt.allScores, got, tt.expected)
			}
		})
	}
}

func TestLabelByPercentile_EdgeCases(t *testing.T) {
	// Empty scores
	got := LabelByPercentile(0.5, []float32{})
	if got != "low" {
		t.Errorf("LabelByPercentile with empty scores = %q, want 'low'", got)
	}

	// Score not in list (should still work based on position)
	got = LabelByPercentile(0.6, []float32{0.9, 0.7, 0.5, 0.3, 0.1})
	// 0.6 would rank between 0.7 and 0.5, so position ~2 in 5 results = medium
	if got != "medium" {
		t.Errorf("LabelByPercentile with score not in list = %q, want 'medium'", got)
	}
}

func TestSearchResult_RelevanceField(t *testing.T) {
	// Verify SearchResult struct has Relevance field
	result := SearchResult{
		Chunk: Chunk{
			ID:       "test-1",
			FilePath: "/test/file.go",
		},
		Score:     0.75,
		Relevance: "high",
	}

	if result.Relevance != "high" {
		t.Errorf("SearchResult.Relevance = %q, want 'high'", result.Relevance)
	}
}
