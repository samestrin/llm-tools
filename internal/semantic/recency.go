package semantic

import (
	"errors"
	"fmt"
	"math"
	"time"
)

// RecencyConfig configures recency boost calculation.
// The recency boost formula is: boost = 1 + factor * exp(-age_days / half_life_days)
// This gives a multiplicative boost to more recently modified files.
type RecencyConfig struct {
	// Factor controls the maximum boost magnitude.
	// When age=0, boost = 1 + Factor.
	// Default: 0.5 (max 50% boost for newest files)
	Factor float64

	// HalfLifeDays controls how quickly the boost decays.
	// After HalfLifeDays, the boost is reduced by ~63% (1-1/e).
	// Default: 7.0 (one week)
	HalfLifeDays float64
}

// DefaultRecencyConfig returns the default recency configuration.
// Factor: 0.5 (max 50% boost), HalfLife: 7 days.
func DefaultRecencyConfig() RecencyConfig {
	return RecencyConfig{
		Factor:       0.5,
		HalfLifeDays: 7.0,
	}
}

// ValidateRecencyConfig validates the recency configuration parameters.
// Returns an error if Factor < 0 or HalfLifeDays <= 0.
func ValidateRecencyConfig(cfg RecencyConfig) error {
	if cfg.Factor < 0 {
		return fmt.Errorf("factor must be >= 0, got %v", cfg.Factor)
	}
	if cfg.HalfLifeDays <= 0 {
		return fmt.Errorf("half_life must be > 0, got %v", cfg.HalfLifeDays)
	}
	return nil
}

// ErrInvalidRecencyConfig is returned when recency configuration is invalid.
var ErrInvalidRecencyConfig = errors.New("invalid recency configuration")

// CalculateRecencyBoost calculates the recency boost multiplier for a file.
// Formula: boost = 1 + factor * exp(-age_days / half_life_days)
//
// The boost is always >= 1.0 (no penalty for old files).
// Maximum boost is 1 + factor (when age_days = 0).
// Boost approaches 1.0 as age_days approaches infinity.
//
// Negative age_days (future dates) are clamped to 0 (max boost).
func CalculateRecencyBoost(ageDays float64, cfg RecencyConfig) float64 {
	// Handle edge cases
	if cfg.Factor == 0 {
		return 1.0
	}

	// Clamp negative age (future dates) to 0
	if ageDays < 0 {
		ageDays = 0
	}

	// Calculate decay exponent
	// Use negative exponent so that larger age = smaller boost
	exponent := -ageDays / cfg.HalfLifeDays

	// Calculate boost using exponential decay
	// exp(exponent) ranges from 1 (when age=0) to ~0 (when age is large)
	decay := math.Exp(exponent)

	// Calculate final boost
	boost := 1.0 + cfg.Factor*decay

	// Protect against NaN/Inf (shouldn't happen with valid inputs)
	if math.IsNaN(boost) || math.IsInf(boost, 0) {
		return 1.0
	}

	return boost
}

// CalculateAgeDays calculates the age in days between a file's modification time and now.
// Returns 0 if mtime is zero (unset) or in the future (clamped).
func CalculateAgeDays(mtime, now time.Time) float64 {
	// Handle zero time (no mtime available)
	if mtime.IsZero() {
		return 0 // Neutral boost for files without mtime
	}

	// Calculate duration since modification
	duration := now.Sub(mtime)

	// Convert to days (fractional)
	ageDays := duration.Hours() / 24.0

	// Clamp negative age (future dates) to 0
	if ageDays < 0 {
		return 0
	}

	return ageDays
}

// ApplyRecencyBoost applies recency boost to search results.
// Results are modified in place and returned (for chaining).
// Files without mtime entries receive neutral boost (1.0).
func ApplyRecencyBoost(results []SearchResult, mtimes map[string]time.Time, cfg RecencyConfig, now time.Time) []SearchResult {
	if len(results) == 0 {
		return results
	}

	// Early return if factor is zero (disabled)
	if cfg.Factor == 0 {
		return results
	}

	// Apply boost to each result
	for i := range results {
		filePath := results[i].Chunk.FilePath
		mtime, ok := mtimes[filePath]

		var boost float64
		if !ok || mtime.IsZero() {
			// No mtime - neutral boost
			boost = 1.0
		} else {
			ageDays := CalculateAgeDays(mtime, now)
			boost = CalculateRecencyBoost(ageDays, cfg)
		}

		// Apply multiplicative boost to score
		results[i].Score = float32(float64(results[i].Score) * boost)
	}

	return results
}
