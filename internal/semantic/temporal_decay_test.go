package semantic

import (
	"math"
	"testing"
	"time"
)

func TestDefaultTemporalDecayConfig(t *testing.T) {
	cfg := DefaultTemporalDecayConfig()
	if cfg.HalfLifeDays != 90.0 {
		t.Errorf("default HalfLifeDays = %v, want 90.0", cfg.HalfLifeDays)
	}
	if cfg.Enabled {
		t.Error("default Enabled should be false")
	}
}

func TestValidateTemporalDecayConfig(t *testing.T) {
	tests := []struct {
		name    string
		cfg     TemporalDecayConfig
		wantErr bool
	}{
		{name: "valid default", cfg: TemporalDecayConfig{HalfLifeDays: 90, Enabled: true}, wantErr: false},
		{name: "valid small", cfg: TemporalDecayConfig{HalfLifeDays: 0.001, Enabled: true}, wantErr: false},
		{name: "zero halflife", cfg: TemporalDecayConfig{HalfLifeDays: 0, Enabled: true}, wantErr: true},
		{name: "negative halflife", cfg: TemporalDecayConfig{HalfLifeDays: -1, Enabled: true}, wantErr: true},
		{name: "disabled with zero halflife", cfg: TemporalDecayConfig{HalfLifeDays: 0, Enabled: false}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateTemporalDecayConfig(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateTemporalDecayConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCalculateTemporalDecay(t *testing.T) {
	cfg := TemporalDecayConfig{HalfLifeDays: 90, Enabled: true}

	tests := []struct {
		name    string
		ageDays float64
		cfg     TemporalDecayConfig
		want    float64
		tol     float64
	}{
		{name: "age=0 -> 1.0", ageDays: 0, cfg: cfg, want: 1.0, tol: 1e-9},
		{name: "age=halflife -> 0.5", ageDays: 90, cfg: cfg, want: 0.5, tol: 1e-9},
		{name: "age=2*halflife -> 0.25", ageDays: 180, cfg: cfg, want: 0.25, tol: 1e-9},
		{name: "age=3*halflife -> 0.125", ageDays: 270, cfg: cfg, want: 0.125, tol: 1e-9},
		{name: "negative age clamped to 1.0", ageDays: -10, cfg: cfg, want: 1.0, tol: 1e-9},
		{name: "very large age -> small positive", ageDays: 10000, cfg: cfg, want: 0, tol: 0.01},
		{name: "disabled -> 1.0", ageDays: 90, cfg: TemporalDecayConfig{HalfLifeDays: 90, Enabled: false}, want: 1.0, tol: 1e-9},
		{name: "short halflife", ageDays: 1, cfg: TemporalDecayConfig{HalfLifeDays: 1, Enabled: true}, want: 0.5, tol: 1e-9},
		{name: "very short halflife no NaN", ageDays: 100, cfg: TemporalDecayConfig{HalfLifeDays: 0.001, Enabled: true}, want: 0, tol: 0.01},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalculateTemporalDecay(tt.ageDays, tt.cfg)
			if math.IsNaN(got) || math.IsInf(got, 0) {
				t.Fatalf("CalculateTemporalDecay() returned NaN or Inf")
			}
			if got < 0 {
				t.Fatalf("CalculateTemporalDecay() returned negative: %v", got)
			}
			if math.Abs(got-tt.want) > tt.tol {
				t.Errorf("CalculateTemporalDecay(%v) = %v, want %v (tol %v)", tt.ageDays, got, tt.want, tt.tol)
			}
		})
	}
}

func TestApplyTemporalDecay(t *testing.T) {
	now := time.Date(2026, 4, 17, 0, 0, 0, 0, time.UTC)
	cfg := TemporalDecayConfig{HalfLifeDays: 90, Enabled: true}

	t.Run("newer entries score higher", func(t *testing.T) {
		results := []MemorySearchResult{
			{Entry: MemoryEntry{ID: "old", CreatedAt: "2025-01-01T00:00:00Z"}, Score: 0.9},
			{Entry: MemoryEntry{ID: "new", CreatedAt: "2026-04-16T00:00:00Z"}, Score: 0.9},
		}
		ApplyTemporalDecay(results, cfg, now)
		if results[1].Score <= results[0].Score {
			t.Errorf("newer entry (score=%v) should score higher than old (score=%v)", results[1].Score, results[0].Score)
		}
	})

	t.Run("preserves relative ordering for same age", func(t *testing.T) {
		results := []MemorySearchResult{
			{Entry: MemoryEntry{ID: "a", CreatedAt: "2026-01-01T00:00:00Z"}, Score: 0.9},
			{Entry: MemoryEntry{ID: "b", CreatedAt: "2026-01-01T00:00:00Z"}, Score: 0.7},
		}
		ApplyTemporalDecay(results, cfg, now)
		if results[0].Score <= results[1].Score {
			t.Errorf("relative ordering should be preserved: a=%v > b=%v", results[0].Score, results[1].Score)
		}
	})

	t.Run("empty results no panic", func(t *testing.T) {
		results := ApplyTemporalDecay(nil, cfg, now)
		if results != nil {
			t.Errorf("expected nil, got %v", results)
		}
		results = ApplyTemporalDecay([]MemorySearchResult{}, cfg, now)
		if len(results) != 0 {
			t.Errorf("expected empty, got %v", results)
		}
	})

	t.Run("empty CreatedAt gets neutral decay", func(t *testing.T) {
		results := []MemorySearchResult{
			{Entry: MemoryEntry{ID: "a", CreatedAt: ""}, Score: 0.8},
		}
		ApplyTemporalDecay(results, cfg, now)
		if results[0].Score != 0.8 {
			t.Errorf("empty CreatedAt should get neutral decay, got score=%v", results[0].Score)
		}
	})

	t.Run("unparseable CreatedAt gets neutral decay", func(t *testing.T) {
		results := []MemorySearchResult{
			{Entry: MemoryEntry{ID: "a", CreatedAt: "not-a-date"}, Score: 0.8},
		}
		ApplyTemporalDecay(results, cfg, now)
		if results[0].Score != 0.8 {
			t.Errorf("unparseable CreatedAt should get neutral decay, got score=%v", results[0].Score)
		}
	})

	t.Run("disabled config no modification", func(t *testing.T) {
		disabledCfg := TemporalDecayConfig{HalfLifeDays: 90, Enabled: false}
		results := []MemorySearchResult{
			{Entry: MemoryEntry{ID: "old", CreatedAt: "2020-01-01T00:00:00Z"}, Score: 0.9},
		}
		ApplyTemporalDecay(results, disabledCfg, now)
		if results[0].Score != 0.9 {
			t.Errorf("disabled config should not modify scores, got %v", results[0].Score)
		}
	})
}

func BenchmarkCalculateTemporalDecay(b *testing.B) {
	cfg := TemporalDecayConfig{HalfLifeDays: 90, Enabled: true}
	for i := 0; i < b.N; i++ {
		CalculateTemporalDecay(float64(i%365), cfg)
	}
}
