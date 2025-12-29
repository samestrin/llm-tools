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
