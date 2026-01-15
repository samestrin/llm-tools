package semantic

import (
	"math"
	"testing"
	"time"
)

// ===== RecencyConfig Struct Tests =====

func TestRecencyConfig_DefaultValues(t *testing.T) {
	// Test that DefaultRecencyConfig returns expected defaults
	cfg := DefaultRecencyConfig()

	if cfg.Factor != 0.5 {
		t.Errorf("DefaultRecencyConfig.Factor = %v, want 0.5", cfg.Factor)
	}
	if cfg.HalfLifeDays != 7.0 {
		t.Errorf("DefaultRecencyConfig.HalfLifeDays = %v, want 7.0", cfg.HalfLifeDays)
	}
}

func TestRecencyConfig_ZeroFactor(t *testing.T) {
	// Factor of zero should effectively disable recency boost
	cfg := RecencyConfig{Factor: 0.0, HalfLifeDays: 7.0}
	boost := CalculateRecencyBoost(10.0, cfg)

	if boost != 1.0 {
		t.Errorf("CalculateRecencyBoost with zero factor = %v, want 1.0", boost)
	}
}

// ===== ValidateRecencyConfig Tests =====

func TestValidateRecencyConfig_Valid(t *testing.T) {
	tests := []struct {
		name string
		cfg  RecencyConfig
	}{
		{"default", RecencyConfig{Factor: 0.5, HalfLifeDays: 7.0}},
		{"zero_factor", RecencyConfig{Factor: 0.0, HalfLifeDays: 7.0}},
		{"high_factor", RecencyConfig{Factor: 2.0, HalfLifeDays: 7.0}},
		{"short_half_life", RecencyConfig{Factor: 0.5, HalfLifeDays: 0.5}},
		{"long_half_life", RecencyConfig{Factor: 0.5, HalfLifeDays: 365.0}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateRecencyConfig(tt.cfg)
			if err != nil {
				t.Errorf("ValidateRecencyConfig(%+v) = %v, want nil", tt.cfg, err)
			}
		})
	}
}

func TestValidateRecencyConfig_NegativeFactor(t *testing.T) {
	cfg := RecencyConfig{Factor: -0.5, HalfLifeDays: 7.0}
	err := ValidateRecencyConfig(cfg)

	if err == nil {
		t.Error("ValidateRecencyConfig with negative factor should return error")
	}
	if err != nil && !containsString(err.Error(), "factor") {
		t.Errorf("Error message should mention 'factor': %v", err)
	}
}

func TestValidateRecencyConfig_ZeroHalfLife(t *testing.T) {
	cfg := RecencyConfig{Factor: 0.5, HalfLifeDays: 0.0}
	err := ValidateRecencyConfig(cfg)

	if err == nil {
		t.Error("ValidateRecencyConfig with zero half-life should return error")
	}
	if err != nil && !containsString(err.Error(), "half") {
		t.Errorf("Error message should mention 'half': %v", err)
	}
}

func TestValidateRecencyConfig_NegativeHalfLife(t *testing.T) {
	cfg := RecencyConfig{Factor: 0.5, HalfLifeDays: -7.0}
	err := ValidateRecencyConfig(cfg)

	if err == nil {
		t.Error("ValidateRecencyConfig with negative half-life should return error")
	}
}

// ===== CalculateRecencyBoost Formula Tests =====

func TestCalculateRecencyBoost_Formula(t *testing.T) {
	// Test the formula: boost = 1 + factor * exp(-age_days / half_life)
	tests := []struct {
		name      string
		ageDays   float64
		factor    float64
		halfLife  float64
		wantBoost float64
		tolerance float64
	}{
		// Standard scenarios
		{"today", 0, 0.5, 7, 1.5, 0.001},
		{"half_life", 7, 0.5, 7, 1.184, 0.001},        // 1 + 0.5 * exp(-1) ≈ 1.184
		{"double_half_life", 14, 0.5, 7, 1.068, 0.01}, // 1 + 0.5 * exp(-2) ≈ 1.068
		{"30_days", 30, 0.5, 7, 1.007, 0.01},          // Very small boost

		// Fractional days
		{"fractional_day", 0.5, 0.5, 7, 1.466, 0.01}, // 1 + 0.5 * exp(-0.5/7) ≈ 1.466
		{"12_hours", 0.5, 0.5, 7, 1.466, 0.01},

		// Different factors
		{"zero_factor", 7, 0.0, 7, 1.0, 0.001},
		{"high_factor", 0, 1.0, 7, 2.0, 0.001},
		{"low_factor", 0, 0.25, 7, 1.25, 0.001},

		// Different half-lives
		{"short_half_life", 1, 0.5, 0.5, 1.068, 0.01}, // 1 + 0.5 * exp(-2) ≈ 1.068
		{"long_half_life", 14, 0.5, 14, 1.184, 0.001}, // 1 + 0.5 * exp(-1) ≈ 1.184

		// Very old files
		{"very_old_30_days", 30, 0.5, 7, 1.007, 0.01},
		{"very_old_365_days", 365, 0.5, 7, 1.0, 0.001}, // Approaches 1.0
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := RecencyConfig{Factor: tt.factor, HalfLifeDays: tt.halfLife}
			got := CalculateRecencyBoost(tt.ageDays, cfg)

			if math.Abs(got-tt.wantBoost) > tt.tolerance {
				t.Errorf("CalculateRecencyBoost(%v, %+v) = %v, want %v (±%v)",
					tt.ageDays, cfg, got, tt.wantBoost, tt.tolerance)
			}
		})
	}
}

func TestCalculateRecencyBoost_MaxBoost(t *testing.T) {
	// Maximum boost should be 1 + factor (when age = 0)
	tests := []struct {
		factor  float64
		wantMax float64
	}{
		{0.5, 1.5},
		{1.0, 2.0},
		{0.25, 1.25},
		{0.0, 1.0},
	}

	for _, tt := range tests {
		t.Run("factor_"+formatFloat(tt.factor), func(t *testing.T) {
			cfg := RecencyConfig{Factor: tt.factor, HalfLifeDays: 7.0}
			got := CalculateRecencyBoost(0, cfg)

			if math.Abs(got-tt.wantMax) > 0.001 {
				t.Errorf("Max boost with factor %v = %v, want %v", tt.factor, got, tt.wantMax)
			}
		})
	}
}

func TestCalculateRecencyBoost_MinBoost(t *testing.T) {
	// Minimum boost should approach 1.0 as age approaches infinity
	cfg := RecencyConfig{Factor: 0.5, HalfLifeDays: 7.0}

	// Very old file (1000 days)
	boost := CalculateRecencyBoost(1000, cfg)

	if math.Abs(boost-1.0) > 0.001 {
		t.Errorf("Boost for 1000-day-old file = %v, should approach 1.0", boost)
	}
}

// ===== Edge Case Tests =====

func TestCalculateRecencyBoost_FutureDate(t *testing.T) {
	// Future dates (negative age) should be clamped to 0
	cfg := RecencyConfig{Factor: 0.5, HalfLifeDays: 7.0}

	// Negative age (1 day in the future)
	boost := CalculateRecencyBoost(-1.0, cfg)
	maxBoost := 1.0 + cfg.Factor // Maximum boost when age = 0

	if math.Abs(boost-maxBoost) > 0.001 {
		t.Errorf("Boost for future date (-1 day) = %v, want %v (max boost)", boost, maxBoost)
	}
}

func TestCalculateRecencyBoost_VeryLargeAge(t *testing.T) {
	// Very large ages should not cause overflow or return NaN/Inf
	cfg := RecencyConfig{Factor: 0.5, HalfLifeDays: 7.0}

	// 10,000 days old
	boost := CalculateRecencyBoost(10000, cfg)

	if math.IsNaN(boost) || math.IsInf(boost, 0) {
		t.Errorf("Boost for 10000-day-old file returned NaN/Inf: %v", boost)
	}
	if boost < 1.0 || boost > 1.001 {
		t.Errorf("Boost for 10000-day-old file = %v, want ~1.0", boost)
	}
}

func TestCalculateRecencyBoost_VerySmallHalfLife(t *testing.T) {
	// Very small half-life should not cause overflow
	cfg := RecencyConfig{Factor: 0.5, HalfLifeDays: 0.001} // ~1.4 minutes

	boost := CalculateRecencyBoost(1, cfg)

	if math.IsNaN(boost) || math.IsInf(boost, 0) {
		t.Errorf("Boost with very small half-life returned NaN/Inf: %v", boost)
	}
	// Should be very close to 1.0 (decay is extremely rapid)
	if math.Abs(boost-1.0) > 0.001 {
		t.Errorf("Boost with very small half-life = %v, want ~1.0", boost)
	}
}

// ===== CalculateAgeDays Tests =====

func TestCalculateAgeDays_Now(t *testing.T) {
	// File modified now should have age 0
	now := time.Now()
	age := CalculateAgeDays(now, now)

	if math.Abs(age) > 0.001 {
		t.Errorf("Age of now vs now = %v, want 0", age)
	}
}

func TestCalculateAgeDays_OneDay(t *testing.T) {
	now := time.Now()
	oneDayAgo := now.Add(-24 * time.Hour)
	age := CalculateAgeDays(oneDayAgo, now)

	if math.Abs(age-1.0) > 0.001 {
		t.Errorf("Age of 1 day ago = %v, want 1.0", age)
	}
}

func TestCalculateAgeDays_FractionalDay(t *testing.T) {
	now := time.Now()
	halfDayAgo := now.Add(-12 * time.Hour)
	age := CalculateAgeDays(halfDayAgo, now)

	if math.Abs(age-0.5) > 0.001 {
		t.Errorf("Age of 12 hours ago = %v, want 0.5", age)
	}
}

func TestCalculateAgeDays_Future(t *testing.T) {
	now := time.Now()
	future := now.Add(24 * time.Hour)
	age := CalculateAgeDays(future, now)

	// Future dates should be clamped to 0
	if age != 0 {
		t.Errorf("Age of future date = %v, want 0 (clamped)", age)
	}
}

func TestCalculateAgeDays_ZeroTime(t *testing.T) {
	// Zero time should return a very large age or be handled gracefully
	now := time.Now()
	zero := time.Time{}
	age := CalculateAgeDays(zero, now)

	// Zero time should be treated as "no mtime" - handled specially
	// The implementation might return 0 (neutral) or a large number
	if math.IsNaN(age) || math.IsInf(age, 0) {
		t.Errorf("Age with zero time returned NaN/Inf: %v", age)
	}
}

// ===== ApplyRecencyBoost Tests =====

func TestApplyRecencyBoost_ToSearchResults(t *testing.T) {
	// Test applying recency boost to search results
	now := time.Now()
	cfg := RecencyConfig{Factor: 0.5, HalfLifeDays: 7.0}

	results := []SearchResult{
		{Chunk: Chunk{ID: "1", FilePath: "recent.go"}, Score: 0.8},
		{Chunk: Chunk{ID: "2", FilePath: "old.go"}, Score: 0.8},
	}

	// Recent file (today) and old file (30 days)
	mtimes := map[string]time.Time{
		"recent.go": now,
		"old.go":    now.Add(-30 * 24 * time.Hour),
	}

	boosted := ApplyRecencyBoost(results, mtimes, cfg, now)

	if len(boosted) != 2 {
		t.Fatalf("ApplyRecencyBoost returned %d results, want 2", len(boosted))
	}

	// Recent file should have higher score after boost
	if boosted[0].Score <= boosted[1].Score {
		t.Errorf("Recent file score %v should be higher than old file score %v",
			boosted[0].Score, boosted[1].Score)
	}
}

func TestApplyRecencyBoost_MissingMtime(t *testing.T) {
	// Files without mtime should receive neutral boost (1.0)
	now := time.Now()
	cfg := RecencyConfig{Factor: 0.5, HalfLifeDays: 7.0}

	results := []SearchResult{
		{Chunk: Chunk{ID: "1", FilePath: "has_mtime.go"}, Score: 0.8},
		{Chunk: Chunk{ID: "2", FilePath: "no_mtime.go"}, Score: 0.8},
	}

	mtimes := map[string]time.Time{
		"has_mtime.go": now, // Only has_mtime.go has an entry
	}

	boosted := ApplyRecencyBoost(results, mtimes, cfg, now)

	// File without mtime should have original score (neutral boost)
	for _, r := range boosted {
		if r.Chunk.FilePath == "no_mtime.go" && r.Score != 0.8 {
			t.Errorf("File without mtime score = %v, want 0.8 (unchanged)", r.Score)
		}
	}
}

func TestApplyRecencyBoost_EmptyResults(t *testing.T) {
	now := time.Now()
	cfg := RecencyConfig{Factor: 0.5, HalfLifeDays: 7.0}

	results := []SearchResult{}
	mtimes := map[string]time.Time{}

	boosted := ApplyRecencyBoost(results, mtimes, cfg, now)

	if len(boosted) != 0 {
		t.Errorf("ApplyRecencyBoost on empty results returned %d, want 0", len(boosted))
	}
}

func TestApplyRecencyBoost_PreservesRanking(t *testing.T) {
	// When recency is similar, base score should still determine ranking
	now := time.Now()
	cfg := RecencyConfig{Factor: 0.5, HalfLifeDays: 7.0}

	results := []SearchResult{
		{Chunk: Chunk{ID: "1", FilePath: "high.go"}, Score: 0.9},
		{Chunk: Chunk{ID: "2", FilePath: "low.go"}, Score: 0.7},
	}

	// Same mtime for both
	mtimes := map[string]time.Time{
		"high.go": now,
		"low.go":  now,
	}

	boosted := ApplyRecencyBoost(results, mtimes, cfg, now)

	// Higher base score should still be higher after equal boost
	var highScore, lowScore float32
	for _, r := range boosted {
		if r.Chunk.FilePath == "high.go" {
			highScore = r.Score
		}
		if r.Chunk.FilePath == "low.go" {
			lowScore = r.Score
		}
	}

	if highScore <= lowScore {
		t.Errorf("Higher base score should remain higher: high=%v, low=%v", highScore, lowScore)
	}
}

func TestApplyRecencyBoost_DisabledWithZeroFactor(t *testing.T) {
	// Zero factor should not modify scores
	now := time.Now()
	cfg := RecencyConfig{Factor: 0.0, HalfLifeDays: 7.0}

	results := []SearchResult{
		{Chunk: Chunk{ID: "1", FilePath: "test.go"}, Score: 0.8},
	}

	mtimes := map[string]time.Time{
		"test.go": now.Add(-7 * 24 * time.Hour), // 7 days ago
	}

	boosted := ApplyRecencyBoost(results, mtimes, cfg, now)

	if boosted[0].Score != 0.8 {
		t.Errorf("Score with zero factor = %v, want 0.8 (unchanged)", boosted[0].Score)
	}
}

// ===== Benchmark Tests =====

func BenchmarkCalculateRecencyBoost(b *testing.B) {
	cfg := RecencyConfig{Factor: 0.5, HalfLifeDays: 7.0}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		CalculateRecencyBoost(float64(i%100), cfg)
	}
}

func BenchmarkApplyRecencyBoost(b *testing.B) {
	now := time.Now()
	cfg := RecencyConfig{Factor: 0.5, HalfLifeDays: 7.0}

	// Create 100 results
	results := make([]SearchResult, 100)
	mtimes := make(map[string]time.Time)
	for i := 0; i < 100; i++ {
		path := "file" + formatFloat(float64(i)) + ".go"
		results[i] = SearchResult{
			Chunk: Chunk{ID: formatFloat(float64(i)), FilePath: path},
			Score: float32(0.5 + float64(i)/200),
		}
		mtimes[path] = now.Add(-time.Duration(i) * 24 * time.Hour)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ApplyRecencyBoost(results, mtimes, cfg, now)
	}
}

// ===== Helper Functions =====

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstr(s, substr))
}

func containsSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func formatFloat(f float64) string {
	return time.Duration(f * float64(time.Second)).String()
}
