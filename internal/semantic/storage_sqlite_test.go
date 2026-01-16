package semantic

import (
	"testing"
)

func TestSQLiteStorage(t *testing.T) {
	StorageTestSuite(t, func() (Storage, func()) {
		storage, err := NewSQLiteStorage(":memory:", 4)
		if err != nil {
			t.Fatalf("Failed to create SQLite storage: %v", err)
		}
		return storage, func() { storage.Close() }
	})
}

func TestSQLiteStorageMemory(t *testing.T) {
	MemoryStorageTestSuite(t, func() (Storage, func()) {
		storage, err := NewSQLiteStorage(":memory:", 4)
		if err != nil {
			t.Fatalf("Failed to create SQLite storage: %v", err)
		}
		return storage, func() { storage.Close() }
	})
}

func TestSQLiteStorage_Close(t *testing.T) {
	storage, err := NewSQLiteStorage(":memory:", 4)
	if err != nil {
		t.Fatalf("Failed to create SQLite storage: %v", err)
	}

	// Close should not error
	if err := storage.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}

	// Second close should not error
	if err := storage.Close(); err != nil {
		t.Errorf("Second Close() error = %v", err)
	}
}

func TestCosineSimilarity(t *testing.T) {
	tests := []struct {
		name string
		a    []float32
		b    []float32
		want float32
	}{
		{
			name: "identical vectors",
			a:    []float32{1, 0, 0, 0},
			b:    []float32{1, 0, 0, 0},
			want: 1.0,
		},
		{
			name: "orthogonal vectors",
			a:    []float32{1, 0, 0, 0},
			b:    []float32{0, 1, 0, 0},
			want: 0.0,
		},
		{
			name: "opposite vectors",
			a:    []float32{1, 0, 0, 0},
			b:    []float32{-1, 0, 0, 0},
			want: -1.0,
		},
		{
			name: "similar vectors",
			a:    []float32{0.9, 0.1, 0, 0},
			b:    []float32{0.8, 0.2, 0, 0},
			want: 0.98, // approximately
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cosineSimilarity(tt.a, tt.b)
			// Allow small floating point differences
			diff := got - tt.want
			if diff < -0.05 || diff > 0.05 {
				t.Errorf("cosineSimilarity() = %v, want %v (±0.05)", got, tt.want)
			}
		})
	}
}

func TestEncodeDecodeEmbedding(t *testing.T) {
	original := []float32{0.1, 0.2, 0.3, 0.4, 0.5}

	encoded, err := encodeEmbedding(original)
	if err != nil {
		t.Fatalf("encodeEmbedding() error = %v", err)
	}

	decoded, err := decodeEmbedding(encoded)
	if err != nil {
		t.Fatalf("decodeEmbedding() error = %v", err)
	}

	if len(decoded) != len(original) {
		t.Fatalf("decoded length = %d, want %d", len(decoded), len(original))
	}

	for i := range original {
		if decoded[i] != original[i] {
			t.Errorf("decoded[%d] = %v, want %v", i, decoded[i], original[i])
		}
	}
}

func TestSQLiteStorage_CalibrationMetadata_Empty(t *testing.T) {
	storage, err := NewSQLiteStorage(":memory:", 4)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer storage.Close()

	ctx := t.Context()

	// Get on fresh database should return nil, nil
	meta, err := storage.GetCalibrationMetadata(ctx)
	if err != nil {
		t.Fatalf("GetCalibrationMetadata() error = %v", err)
	}
	if meta != nil {
		t.Errorf("GetCalibrationMetadata() = %v, want nil", meta)
	}
}

func TestSQLiteStorage_CalibrationMetadata_RoundTrip(t *testing.T) {
	storage, err := NewSQLiteStorage(":memory:", 4)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer storage.Close()

	ctx := t.Context()

	// Create test metadata
	original := &CalibrationMetadata{
		EmbeddingModel:    "test-model",
		PerfectMatchScore: 0.95,
		BaselineScore:     0.20,
		ScoreRange:        0.75,
		HighThreshold:     0.725,
		MediumThreshold:   0.50,
		LowThreshold:      0.3125,
	}

	// Set
	err = storage.SetCalibrationMetadata(ctx, original)
	if err != nil {
		t.Fatalf("SetCalibrationMetadata() error = %v", err)
	}

	// Get
	retrieved, err := storage.GetCalibrationMetadata(ctx)
	if err != nil {
		t.Fatalf("GetCalibrationMetadata() error = %v", err)
	}
	if retrieved == nil {
		t.Fatal("GetCalibrationMetadata() returned nil")
	}

	// Verify fields
	if retrieved.EmbeddingModel != original.EmbeddingModel {
		t.Errorf("EmbeddingModel = %v, want %v", retrieved.EmbeddingModel, original.EmbeddingModel)
	}
	if retrieved.PerfectMatchScore != original.PerfectMatchScore {
		t.Errorf("PerfectMatchScore = %v, want %v", retrieved.PerfectMatchScore, original.PerfectMatchScore)
	}
	if retrieved.BaselineScore != original.BaselineScore {
		t.Errorf("BaselineScore = %v, want %v", retrieved.BaselineScore, original.BaselineScore)
	}
	if retrieved.HighThreshold != original.HighThreshold {
		t.Errorf("HighThreshold = %v, want %v", retrieved.HighThreshold, original.HighThreshold)
	}
	if retrieved.MediumThreshold != original.MediumThreshold {
		t.Errorf("MediumThreshold = %v, want %v", retrieved.MediumThreshold, original.MediumThreshold)
	}
	if retrieved.LowThreshold != original.LowThreshold {
		t.Errorf("LowThreshold = %v, want %v", retrieved.LowThreshold, original.LowThreshold)
	}
}

func TestSQLiteStorage_CalibrationMetadata_Overwrite(t *testing.T) {
	storage, err := NewSQLiteStorage(":memory:", 4)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer storage.Close()

	ctx := t.Context()

	// Set initial
	initial := &CalibrationMetadata{
		EmbeddingModel:    "model-v1",
		PerfectMatchScore: 0.80,
		BaselineScore:     0.10,
	}
	err = storage.SetCalibrationMetadata(ctx, initial)
	if err != nil {
		t.Fatalf("SetCalibrationMetadata() error = %v", err)
	}

	// Set updated
	updated := &CalibrationMetadata{
		EmbeddingModel:    "model-v2",
		PerfectMatchScore: 0.95,
		BaselineScore:     0.15,
	}
	err = storage.SetCalibrationMetadata(ctx, updated)
	if err != nil {
		t.Fatalf("SetCalibrationMetadata() error = %v", err)
	}

	// Get should return updated
	retrieved, err := storage.GetCalibrationMetadata(ctx)
	if err != nil {
		t.Fatalf("GetCalibrationMetadata() error = %v", err)
	}

	if retrieved.EmbeddingModel != "model-v2" {
		t.Errorf("EmbeddingModel = %v, want model-v2", retrieved.EmbeddingModel)
	}
	if retrieved.PerfectMatchScore != 0.95 {
		t.Errorf("PerfectMatchScore = %v, want 0.95", retrieved.PerfectMatchScore)
	}
}
