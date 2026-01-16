package semantic

import (
	"context"
	"math"
	"testing"
	"time"
)

func TestCalculateThresholds(t *testing.T) {
	tests := []struct {
		name         string
		perfectMatch float32
		baseline     float32
		wantHigh     float32
		wantMedium   float32
		wantLow      float32
	}{
		{
			name:         "standard range 0-1",
			perfectMatch: 1.0,
			baseline:     0.0,
			wantHigh:     0.70,
			wantMedium:   0.40,
			wantLow:      0.15,
		},
		{
			name:         "typical nomic range",
			perfectMatch: 0.85,
			baseline:     0.30,
			// range = 0.55
			// high = 0.30 + 0.70*0.55 = 0.30 + 0.385 = 0.685
			// medium = 0.30 + 0.40*0.55 = 0.30 + 0.22 = 0.52
			// low = 0.30 + 0.15*0.55 = 0.30 + 0.0825 = 0.3825
			wantHigh:   0.685,
			wantMedium: 0.52,
			wantLow:    0.3825,
		},
		{
			name:         "narrow qwen range",
			perfectMatch: 0.08,
			baseline:     0.02,
			// range = 0.06
			// high = 0.02 + 0.70*0.06 = 0.02 + 0.042 = 0.062
			// medium = 0.02 + 0.40*0.06 = 0.02 + 0.024 = 0.044
			// low = 0.02 + 0.15*0.06 = 0.02 + 0.009 = 0.029
			wantHigh:   0.062,
			wantMedium: 0.044,
			wantLow:    0.029,
		},
		{
			name:         "zero baseline",
			perfectMatch: 0.5,
			baseline:     0.0,
			wantHigh:     0.35,
			wantMedium:   0.20,
			wantLow:      0.075,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotHigh, gotMedium, gotLow := calculateThresholds(tt.perfectMatch, tt.baseline)

			// Use approximate comparison for float32
			const epsilon = 0.0001

			if math.Abs(float64(gotHigh-tt.wantHigh)) > epsilon {
				t.Errorf("high: got %v, want %v", gotHigh, tt.wantHigh)
			}
			if math.Abs(float64(gotMedium-tt.wantMedium)) > epsilon {
				t.Errorf("medium: got %v, want %v", gotMedium, tt.wantMedium)
			}
			if math.Abs(float64(gotLow-tt.wantLow)) > epsilon {
				t.Errorf("low: got %v, want %v", gotLow, tt.wantLow)
			}
		})
	}
}

func TestCalculateThresholds_EdgeCases(t *testing.T) {
	tests := []struct {
		name         string
		perfectMatch float32
		baseline     float32
	}{
		{
			name:         "equal perfect and baseline",
			perfectMatch: 0.5,
			baseline:     0.5,
		},
		{
			name:         "baseline higher than perfect (invalid)",
			perfectMatch: 0.3,
			baseline:     0.7,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			high, medium, low := calculateThresholds(tt.perfectMatch, tt.baseline)

			// Should not panic and should return valid floats
			if math.IsNaN(float64(high)) || math.IsInf(float64(high), 0) {
				t.Errorf("high is invalid: %v", high)
			}
			if math.IsNaN(float64(medium)) || math.IsInf(float64(medium), 0) {
				t.Errorf("medium is invalid: %v", medium)
			}
			if math.IsNaN(float64(low)) || math.IsInf(float64(low), 0) {
				t.Errorf("low is invalid: %v", low)
			}

			// Thresholds should be ordered: high > medium > low
			if high < medium {
				t.Errorf("high (%v) should be >= medium (%v)", high, medium)
			}
			if medium < low {
				t.Errorf("medium (%v) should be >= low (%v)", medium, low)
			}
		})
	}
}

func TestMedian(t *testing.T) {
	tests := []struct {
		name   string
		scores []float32
		want   float32
	}{
		{
			name:   "odd count",
			scores: []float32{0.1, 0.5, 0.9},
			want:   0.5,
		},
		{
			name:   "even count",
			scores: []float32{0.1, 0.4, 0.6, 0.9},
			want:   0.5, // (0.4 + 0.6) / 2
		},
		{
			name:   "single value",
			scores: []float32{0.42},
			want:   0.42,
		},
		{
			name:   "two values",
			scores: []float32{0.2, 0.8},
			want:   0.5, // (0.2 + 0.8) / 2
		},
		{
			name:   "empty",
			scores: []float32{},
			want:   0,
		},
		{
			name:   "unsorted input",
			scores: []float32{0.9, 0.1, 0.5},
			want:   0.5,
		},
		{
			name:   "duplicate values",
			scores: []float32{0.3, 0.3, 0.3},
			want:   0.3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := median(tt.scores)
			if math.Abs(float64(got-tt.want)) > 0.0001 {
				t.Errorf("median() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMedian_DoesNotModifyInput(t *testing.T) {
	original := []float32{0.9, 0.1, 0.5}
	input := make([]float32, len(original))
	copy(input, original)

	median(input)

	for i, v := range input {
		if v != original[i] {
			t.Errorf("median() modified input at index %d: got %v, want %v", i, v, original[i])
		}
	}
}

// mockStorage implements Storage interface for testing calibration
type mockCalibrationStorage struct {
	chunks []Chunk
	stats  *IndexStats
}

func (m *mockCalibrationStorage) Create(ctx context.Context, chunk Chunk, embedding []float32) error {
	return nil
}
func (m *mockCalibrationStorage) CreateBatch(ctx context.Context, chunks []ChunkWithEmbedding) error {
	return nil
}
func (m *mockCalibrationStorage) Read(ctx context.Context, id string) (*Chunk, error) {
	return nil, nil
}
func (m *mockCalibrationStorage) Update(ctx context.Context, chunk Chunk, embedding []float32) error {
	return nil
}
func (m *mockCalibrationStorage) Delete(ctx context.Context, id string) error { return nil }
func (m *mockCalibrationStorage) DeleteByFilePath(ctx context.Context, fp string) (int, error) {
	return 0, nil
}
func (m *mockCalibrationStorage) List(ctx context.Context, opts ListOptions) ([]Chunk, error) {
	if opts.Limit > 0 && opts.Limit < len(m.chunks) {
		return m.chunks[:opts.Limit], nil
	}
	return m.chunks, nil
}
func (m *mockCalibrationStorage) Search(ctx context.Context, queryEmbedding []float32, opts SearchOptions) ([]SearchResult, error) {
	// Return a mock result with a score based on embedding similarity
	// For testing, we return a fixed score
	if len(m.chunks) == 0 {
		return nil, nil
	}
	return []SearchResult{{Chunk: m.chunks[0], Score: 0.85}}, nil
}
func (m *mockCalibrationStorage) Stats(ctx context.Context) (*IndexStats, error) {
	if m.stats != nil {
		return m.stats, nil
	}
	return &IndexStats{ChunksTotal: len(m.chunks)}, nil
}
func (m *mockCalibrationStorage) Clear(ctx context.Context) error { return nil }
func (m *mockCalibrationStorage) GetFileHash(ctx context.Context, filePath string) (string, error) {
	return "", nil
}
func (m *mockCalibrationStorage) SetFileHash(ctx context.Context, filePath string, hash string) error {
	return nil
}
func (m *mockCalibrationStorage) Close() error { return nil }
func (m *mockCalibrationStorage) StoreMemory(ctx context.Context, entry MemoryEntry, embedding []float32) error {
	return nil
}
func (m *mockCalibrationStorage) StoreMemoryBatch(ctx context.Context, entries []MemoryWithEmbedding) error {
	return nil
}
func (m *mockCalibrationStorage) SearchMemory(ctx context.Context, queryEmbedding []float32, opts MemorySearchOptions) ([]MemorySearchResult, error) {
	return nil, nil
}
func (m *mockCalibrationStorage) GetMemory(ctx context.Context, id string) (*MemoryEntry, error) {
	return nil, nil
}
func (m *mockCalibrationStorage) DeleteMemory(ctx context.Context, id string) error { return nil }
func (m *mockCalibrationStorage) ListMemory(ctx context.Context, opts MemoryListOptions) ([]MemoryEntry, error) {
	return nil, nil
}
func (m *mockCalibrationStorage) GetCalibrationMetadata(ctx context.Context) (*CalibrationMetadata, error) {
	return nil, nil
}
func (m *mockCalibrationStorage) SetCalibrationMetadata(ctx context.Context, meta *CalibrationMetadata) error {
	return nil
}

func TestRunCalibration_EmptyIndex(t *testing.T) {
	ctx := context.Background()
	storage := &mockCalibrationStorage{
		chunks: []Chunk{},
		stats:  &IndexStats{ChunksTotal: 0},
	}

	// Create a mock embedder (we won't actually call it)
	embedder := &Embedder{}

	_, err := RunCalibration(ctx, storage, embedder, "test-model")

	if err != ErrEmptyIndex {
		t.Errorf("expected ErrEmptyIndex, got %v", err)
	}
}

func TestCalibrationMetadata_Fields(t *testing.T) {
	meta := &CalibrationMetadata{
		EmbeddingModel:    "test-model",
		CalibrationDate:   time.Now(),
		PerfectMatchScore: 0.95,
		BaselineScore:     0.20,
		ScoreRange:        0.75,
		HighThreshold:     0.725,
		MediumThreshold:   0.50,
		LowThreshold:      0.3125,
	}

	if meta.EmbeddingModel != "test-model" {
		t.Errorf("EmbeddingModel = %v, want test-model", meta.EmbeddingModel)
	}
	if meta.PerfectMatchScore != 0.95 {
		t.Errorf("PerfectMatchScore = %v, want 0.95", meta.PerfectMatchScore)
	}
	if meta.BaselineScore != 0.20 {
		t.Errorf("BaselineScore = %v, want 0.20", meta.BaselineScore)
	}
	if meta.ScoreRange != 0.75 {
		t.Errorf("ScoreRange = %v, want 0.75", meta.ScoreRange)
	}
}
