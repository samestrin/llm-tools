package semantic

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
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

// TestSQLiteStorage_ConcurrentReads verifies thread-safe concurrent reads
func TestSQLiteStorage_ConcurrentReads(t *testing.T) {
	// Use temp file database for reliable concurrent access testing
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "concurrent_reads_test.db")

	storage, err := NewSQLiteStorage(dbPath, 4)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer storage.Close()

	ctx := context.Background()

	// Create test data
	chunk := Chunk{
		ID:      "test-1",
		Content: "test content",
		Name:    "test.go:TestFunc",
		Type:    ChunkFunction,
	}
	embedding := []float32{0.1, 0.2, 0.3, 0.4}
	if err := storage.Create(ctx, chunk, embedding); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Concurrent reads should not panic or error
	const goroutines = 5
	const iterations = 20

	var wg sync.WaitGroup
	errChan := make(chan error, goroutines*iterations*3)

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				_, err := storage.Read(ctx, "test-1")
				if err != nil {
					errChan <- err
				}

				_, err = storage.List(ctx, ListOptions{Limit: 10})
				if err != nil {
					errChan <- err
				}

				_, err = storage.Search(ctx, embedding, SearchOptions{TopK: 5})
				if err != nil {
					errChan <- err
				}
			}
		}()
	}

	wg.Wait()
	close(errChan)

	for err := range errChan {
		t.Errorf("Concurrent read error: %v", err)
	}
}

// TestSQLiteStorage_ConcurrentWrites verifies thread-safe concurrent writes
func TestSQLiteStorage_ConcurrentWrites(t *testing.T) {
	// Use temp file database for reliable concurrent access testing
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "concurrent_writes_test.db")

	storage, err := NewSQLiteStorage(dbPath, 4)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer storage.Close()

	ctx := context.Background()

	const goroutines = 3
	const iterations = 10

	var wg sync.WaitGroup
	errChan := make(chan error, goroutines*iterations)

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				chunk := Chunk{
					ID:      idForWorker(workerID, j),
					Content: "test content",
					Name:    "test.go:TestFunc",
					Type:    ChunkFunction,
				}
				embedding := []float32{0.1, 0.2, 0.3, 0.4}
				if err := storage.Create(ctx, chunk, embedding); err != nil {
					errChan <- err
				}
			}
		}(i)
	}

	wg.Wait()
	close(errChan)

	for err := range errChan {
		t.Errorf("Concurrent write error: %v", err)
	}

	// Verify all chunks were created
	stats, err := storage.Stats(ctx)
	if err != nil {
		t.Fatalf("Stats() error = %v", err)
	}
	expected := goroutines * iterations
	if stats.ChunksTotal != expected {
		t.Errorf("ChunksTotal = %d, want %d", stats.ChunksTotal, expected)
	}
}

// TestSQLiteStorage_ConcurrentReadWrite verifies thread-safe mixed reads and writes
func TestSQLiteStorage_ConcurrentReadWrite(t *testing.T) {
	// Use temp file database for reliable concurrent access testing
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "concurrent_readwrite_test.db")

	storage, err := NewSQLiteStorage(dbPath, 4)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer storage.Close()

	ctx := context.Background()

	// Pre-populate with some data
	for i := 0; i < 5; i++ {
		chunk := Chunk{
			ID:      idForWorker(0, i),
			Content: "initial content",
			Name:    "test.go:TestFunc",
			Type:    ChunkFunction,
		}
		embedding := []float32{0.1, 0.2, 0.3, 0.4}
		if err := storage.Create(ctx, chunk, embedding); err != nil {
			t.Fatalf("Create() error = %v", err)
		}
	}

	const goroutines = 4
	const iterations = 10

	var wg sync.WaitGroup
	errChan := make(chan error, goroutines*iterations*3)

	// Writers
	for i := 0; i < goroutines/2; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				chunk := Chunk{
					ID:      idForWorker(workerID+100, j),
					Content: "new content",
					Name:    "new.go:NewFunc",
					Type:    ChunkFunction,
				}
				embedding := []float32{0.5, 0.6, 0.7, 0.8}
				if err := storage.Create(ctx, chunk, embedding); err != nil {
					errChan <- err
				}
			}
		}(i)
	}

	// Readers
	for i := 0; i < goroutines/2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			embedding := []float32{0.1, 0.2, 0.3, 0.4}
			for j := 0; j < iterations; j++ {
				_, err := storage.List(ctx, ListOptions{Limit: 10})
				if err != nil {
					errChan <- err
				}

				_, err = storage.Search(ctx, embedding, SearchOptions{TopK: 5})
				if err != nil {
					errChan <- err
				}

				_, err = storage.Stats(ctx)
				if err != nil {
					errChan <- err
				}
			}
		}()
	}

	wg.Wait()
	close(errChan)

	for err := range errChan {
		t.Errorf("Concurrent read/write error: %v", err)
	}
}

// idForWorker generates a unique ID for a worker and iteration
func idForWorker(workerID, iteration int) string {
	return fmt.Sprintf("worker-%d-%d", workerID, iteration)
}
