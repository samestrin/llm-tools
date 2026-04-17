package semantic

import (
	"fmt"
	"math"
	"time"
)

// TemporalDecayConfig configures temporal decay for memory search.
// Formula: multiplier = exp(-ln(2) / HalfLifeDays * ageDays)
// At age=0, multiplier=1.0. At age=HalfLifeDays, multiplier=0.5.
type TemporalDecayConfig struct {
	// HalfLifeDays controls decay rate. After this many days, score is halved.
	// Default: 90.0
	HalfLifeDays float64

	// Enabled controls whether decay is applied.
	Enabled bool
}

// DefaultTemporalDecayConfig returns the default configuration.
// HalfLife: 90 days, Enabled: false.
func DefaultTemporalDecayConfig() TemporalDecayConfig {
	return TemporalDecayConfig{
		HalfLifeDays: 90.0,
		Enabled:      false,
	}
}

// ValidateTemporalDecayConfig validates the configuration.
func ValidateTemporalDecayConfig(cfg TemporalDecayConfig) error {
	if cfg.HalfLifeDays <= 0 {
		return fmt.Errorf("half_life_days must be > 0, got %v", cfg.HalfLifeDays)
	}
	return nil
}

// CalculateTemporalDecay returns the decay multiplier for a given age.
// Returns a value in (0, 1.0] where 1.0 means no decay.
// Negative ages are clamped to 0 (no decay).
// Returns 1.0 if config is disabled.
func CalculateTemporalDecay(ageDays float64, cfg TemporalDecayConfig) float64 {
	if !cfg.Enabled {
		return 1.0
	}

	if ageDays < 0 {
		ageDays = 0
	}

	// Formula: exp(-ln(2) / halflife * age)
	// This gives exactly 0.5 when age == halflife
	multiplier := math.Exp(-math.Ln2 / cfg.HalfLifeDays * ageDays)

	// Guard against NaN/Inf
	if math.IsNaN(multiplier) || math.IsInf(multiplier, 0) {
		return 1.0
	}

	return multiplier
}

// ApplyTemporalDecay applies temporal decay to memory search results in-place.
// Entries with empty or unparseable CreatedAt receive neutral decay (1.0).
// Returns the modified slice for chaining.
func ApplyTemporalDecay(results []MemorySearchResult, cfg TemporalDecayConfig, now time.Time) []MemorySearchResult {
	if len(results) == 0 || !cfg.Enabled {
		return results
	}

	for i := range results {
		createdAt := results[i].Entry.CreatedAt
		if createdAt == "" {
			continue // neutral decay
		}

		t, err := time.Parse(time.RFC3339, createdAt)
		if err != nil {
			// Try fallback formats
			t, err = time.Parse("2006-01-02T15:04:05", createdAt)
			if err != nil {
				t, err = time.Parse("2006-01-02", createdAt)
				if err != nil {
					continue // neutral decay for unparseable dates
				}
			}
		}

		ageDays := now.Sub(t).Hours() / 24.0
		multiplier := CalculateTemporalDecay(ageDays, cfg)
		results[i].Score = float32(float64(results[i].Score) * multiplier)
	}

	return results
}
