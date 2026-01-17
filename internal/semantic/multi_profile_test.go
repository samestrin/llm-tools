package semantic

import (
	"context"
	"errors"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

// ===== Multi-Profile Search Tests (Sprint 8.13 Phase 2) =====

// mockDelayStorage wraps a storage with configurable delay for parallelism testing
type mockDelayStorage struct {
	Storage
	delay     time.Duration
	callCount int64
}

func (m *mockDelayStorage) Search(ctx context.Context, queryEmbedding []float32, opts SearchOptions) ([]SearchResult, error) {
	time.Sleep(m.delay)
	atomic.AddInt64(&m.callCount, 1)
	return m.Storage.Search(ctx, queryEmbedding, opts)
}

// TestMultiProfileSearcher_TwoProfiles verifies parallel search across two profiles.
func TestMultiProfileSearcher_TwoProfiles(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	// Create two separate storage instances
	storage1, err := NewSQLiteStorage(filepath.Join(tmpDir, "code.db"), 4)
	if err != nil {
		t.Fatalf("Failed to create code storage: %v", err)
	}
	defer storage1.Close()

	storage2, err := NewSQLiteStorage(filepath.Join(tmpDir, "docs.db"), 4)
	if err != nil {
		t.Fatalf("Failed to create docs storage: %v", err)
	}
	defer storage2.Close()

	// Add test data to each storage
	storage1.Create(ctx, Chunk{ID: "code-1", Name: "CodeFunc", Domain: "code"}, []float32{0.1, 0.2, 0.3, 0.4})
	storage2.Create(ctx, Chunk{ID: "docs-1", Name: "DocsFunc", Domain: "docs"}, []float32{0.1, 0.2, 0.3, 0.4})

	embedder := &mockEmbedder{embedding: []float32{0.1, 0.2, 0.3, 0.4}}

	// Create storage factory
	storageMap := map[string]Storage{
		"code": storage1,
		"docs": storage2,
	}

	mps := NewMultiProfileSearcher(embedder, storageMap)

	// Search across both profiles
	results, err := mps.Search(ctx, "test query", SearchOptions{
		TopK:     10,
		Profiles: []string{"code", "docs"},
	})
	if err != nil {
		t.Fatalf("MultiProfileSearcher.Search() error = %v", err)
	}

	// Should have results from both profiles
	if len(results) != 2 {
		t.Errorf("got %d results, want 2 (one from each profile)", len(results))
	}

	// Verify results have correct profile tags
	profileCounts := make(map[string]int)
	for _, r := range results {
		profileCounts[r.Chunk.Domain]++
	}
	if profileCounts["code"] != 1 {
		t.Errorf("code profile count = %d, want 1", profileCounts["code"])
	}
	if profileCounts["docs"] != 1 {
		t.Errorf("docs profile count = %d, want 1", profileCounts["docs"])
	}
}

// TestMultiProfileSearcher_ParallelExecution verifies queries run in parallel.
func TestMultiProfileSearcher_ParallelExecution(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	// Create two storages with delays
	baseStorage1, err := NewSQLiteStorage(filepath.Join(tmpDir, "code.db"), 4)
	if err != nil {
		t.Fatalf("Failed to create code storage: %v", err)
	}
	defer baseStorage1.Close()

	baseStorage2, err := NewSQLiteStorage(filepath.Join(tmpDir, "docs.db"), 4)
	if err != nil {
		t.Fatalf("Failed to create docs storage: %v", err)
	}
	defer baseStorage2.Close()

	// Add test data
	baseStorage1.Create(ctx, Chunk{ID: "code-1", Name: "CodeFunc"}, []float32{0.1, 0.2, 0.3, 0.4})
	baseStorage2.Create(ctx, Chunk{ID: "docs-1", Name: "DocsFunc"}, []float32{0.1, 0.2, 0.3, 0.4})

	// Wrap with 50ms delay each
	delay := 50 * time.Millisecond
	storage1 := &mockDelayStorage{Storage: baseStorage1, delay: delay}
	storage2 := &mockDelayStorage{Storage: baseStorage2, delay: delay}

	embedder := &mockEmbedder{embedding: []float32{0.1, 0.2, 0.3, 0.4}}

	storageMap := map[string]Storage{
		"code": storage1,
		"docs": storage2,
	}

	mps := NewMultiProfileSearcher(embedder, storageMap)

	// Time the search
	start := time.Now()
	_, err = mps.Search(ctx, "test query", SearchOptions{
		TopK:     10,
		Profiles: []string{"code", "docs"},
	})
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("MultiProfileSearcher.Search() error = %v", err)
	}

	// If parallel, should complete in ~50ms, not ~100ms
	// Allow some overhead (75ms max for parallel, 100ms+ would indicate sequential)
	maxParallelTime := 75 * time.Millisecond
	if elapsed > maxParallelTime {
		t.Errorf("Search took %v, expected < %v for parallel execution", elapsed, maxParallelTime)
	}

	// Both storages should have been called
	if storage1.callCount != 1 {
		t.Errorf("storage1 call count = %d, want 1", storage1.callCount)
	}
	if storage2.callCount != 1 {
		t.Errorf("storage2 call count = %d, want 1", storage2.callCount)
	}
}

// TestMultiProfileSearcher_SingleProfile verifies single profile doesn't use parallel overhead.
func TestMultiProfileSearcher_SingleProfile(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	storage1, err := NewSQLiteStorage(filepath.Join(tmpDir, "code.db"), 4)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer storage1.Close()

	storage1.Create(ctx, Chunk{ID: "code-1", Name: "CodeFunc"}, []float32{0.1, 0.2, 0.3, 0.4})

	embedder := &mockEmbedder{embedding: []float32{0.1, 0.2, 0.3, 0.4}}

	storageMap := map[string]Storage{
		"code": storage1,
	}

	mps := NewMultiProfileSearcher(embedder, storageMap)

	results, err := mps.Search(ctx, "test query", SearchOptions{
		TopK:     10,
		Profiles: []string{"code"},
	})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}

	if len(results) != 1 {
		t.Errorf("got %d results, want 1", len(results))
	}
}

// TestMultiProfileSearcher_EmptyProfilesUsesDefault verifies nil/empty profiles use default.
func TestMultiProfileSearcher_EmptyProfilesUsesDefault(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	storage1, err := NewSQLiteStorage(filepath.Join(tmpDir, "code.db"), 4)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer storage1.Close()

	storage1.Create(ctx, Chunk{ID: "code-1", Name: "CodeFunc"}, []float32{0.1, 0.2, 0.3, 0.4})

	embedder := &mockEmbedder{embedding: []float32{0.1, 0.2, 0.3, 0.4}}

	storageMap := map[string]Storage{
		"code": storage1,
	}

	mps := NewMultiProfileSearcher(embedder, storageMap)
	mps.SetDefaultProfile("code")

	// Search with nil profiles - should use default
	results, err := mps.Search(ctx, "test query", SearchOptions{
		TopK:     10,
		Profiles: nil,
	})
	if err != nil {
		t.Fatalf("Search() with nil profiles error = %v", err)
	}

	if len(results) != 1 {
		t.Errorf("got %d results, want 1", len(results))
	}
}

// TestMultiProfileSearcher_UnknownProfile verifies error for unknown profile.
func TestMultiProfileSearcher_UnknownProfile(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	storage1, err := NewSQLiteStorage(filepath.Join(tmpDir, "code.db"), 4)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer storage1.Close()

	embedder := &mockEmbedder{embedding: []float32{0.1, 0.2, 0.3, 0.4}}

	storageMap := map[string]Storage{
		"code": storage1,
	}

	mps := NewMultiProfileSearcher(embedder, storageMap)

	_, err = mps.Search(ctx, "test query", SearchOptions{
		TopK:     10,
		Profiles: []string{"unknown"},
	})
	if err == nil {
		t.Fatal("Search() with unknown profile should return error")
	}
	if err.Error() != "unknown profile: 'unknown'" {
		t.Errorf("error = %q, want %q", err.Error(), "unknown profile: 'unknown'")
	}
}

// TestMultiProfileSearcher_PartialFailure verifies partial results on partial failure.
func TestMultiProfileSearcher_PartialFailure(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	// Create one working storage
	storage1, err := NewSQLiteStorage(filepath.Join(tmpDir, "code.db"), 4)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer storage1.Close()
	storage1.Create(ctx, Chunk{ID: "code-1", Name: "CodeFunc"}, []float32{0.1, 0.2, 0.3, 0.4})

	// Create failing storage
	failingStorage := &mockFailingStorage{err: errors.New("database error")}

	embedder := &mockEmbedder{embedding: []float32{0.1, 0.2, 0.3, 0.4}}

	storageMap := map[string]Storage{
		"code": storage1,
		"docs": failingStorage,
	}

	mps := NewMultiProfileSearcher(embedder, storageMap)

	// Should return partial results from successful profile
	results, err := mps.Search(ctx, "test query", SearchOptions{
		TopK:     10,
		Profiles: []string{"code", "docs"},
	})

	// Partial failure should not return error, just log warning
	if err != nil {
		t.Fatalf("Search() with partial failure error = %v, want nil", err)
	}

	// Should have results from successful profile
	if len(results) != 1 {
		t.Errorf("got %d results, want 1 (from successful profile)", len(results))
	}
}

// TestMultiProfileSearcher_ResultDeduplication verifies dedup across profiles.
func TestMultiProfileSearcher_ResultDeduplication(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	// Create two storages with the same chunk ID (simulating indexed in both)
	storage1, err := NewSQLiteStorage(filepath.Join(tmpDir, "code.db"), 4)
	if err != nil {
		t.Fatalf("Failed to create code storage: %v", err)
	}
	defer storage1.Close()

	storage2, err := NewSQLiteStorage(filepath.Join(tmpDir, "docs.db"), 4)
	if err != nil {
		t.Fatalf("Failed to create docs storage: %v", err)
	}
	defer storage2.Close()

	// Same chunk ID in both storages with different scores
	storage1.Create(ctx, Chunk{ID: "shared-chunk", Name: "SharedFunc"}, []float32{0.9, 0.1, 0, 0}) // higher score
	storage2.Create(ctx, Chunk{ID: "shared-chunk", Name: "SharedFunc"}, []float32{0.5, 0.5, 0, 0}) // lower score

	// Query embedding closer to storage1's embedding
	embedder := &mockEmbedder{embedding: []float32{1, 0, 0, 0}}

	storageMap := map[string]Storage{
		"code": storage1,
		"docs": storage2,
	}

	mps := NewMultiProfileSearcher(embedder, storageMap)

	results, err := mps.Search(ctx, "test query", SearchOptions{
		TopK:     10,
		Profiles: []string{"code", "docs"},
	})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}

	// Should have only 1 result (deduplicated)
	if len(results) != 1 {
		t.Errorf("got %d results, want 1 (deduplicated)", len(results))
	}

	// Should keep higher score
	if len(results) > 0 && results[0].Score < 0.8 {
		t.Errorf("kept score = %f, want higher score (>= 0.8)", results[0].Score)
	}
}

// TestMultiProfileSearcher_ResultsSortedByScore verifies merged results are sorted.
func TestMultiProfileSearcher_ResultsSortedByScore(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	storage1, err := NewSQLiteStorage(filepath.Join(tmpDir, "code.db"), 4)
	if err != nil {
		t.Fatalf("Failed to create code storage: %v", err)
	}
	defer storage1.Close()

	storage2, err := NewSQLiteStorage(filepath.Join(tmpDir, "docs.db"), 4)
	if err != nil {
		t.Fatalf("Failed to create docs storage: %v", err)
	}
	defer storage2.Close()

	// Add chunks with different scores
	storage1.Create(ctx, Chunk{ID: "code-high", Name: "High"}, []float32{0.99, 0.01, 0, 0})
	storage1.Create(ctx, Chunk{ID: "code-low", Name: "Low"}, []float32{0.1, 0.9, 0, 0})
	storage2.Create(ctx, Chunk{ID: "docs-mid", Name: "Mid"}, []float32{0.7, 0.3, 0, 0})

	embedder := &mockEmbedder{embedding: []float32{1, 0, 0, 0}}

	storageMap := map[string]Storage{
		"code": storage1,
		"docs": storage2,
	}

	mps := NewMultiProfileSearcher(embedder, storageMap)

	results, err := mps.Search(ctx, "test query", SearchOptions{
		TopK:     10,
		Profiles: []string{"code", "docs"},
	})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}

	// Verify sorted by score descending
	for i := 1; i < len(results); i++ {
		if results[i].Score > results[i-1].Score {
			t.Errorf("results not sorted: score[%d]=%f > score[%d]=%f",
				i, results[i].Score, i-1, results[i-1].Score)
		}
	}
}

// TestMultiProfileSearcher_TopKAppliedAfterMerge verifies TopK limits merged results.
func TestMultiProfileSearcher_TopKAppliedAfterMerge(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	storage1, err := NewSQLiteStorage(filepath.Join(tmpDir, "code.db"), 4)
	if err != nil {
		t.Fatalf("Failed to create code storage: %v", err)
	}
	defer storage1.Close()

	storage2, err := NewSQLiteStorage(filepath.Join(tmpDir, "docs.db"), 4)
	if err != nil {
		t.Fatalf("Failed to create docs storage: %v", err)
	}
	defer storage2.Close()

	// Add multiple chunks to each
	for i := 0; i < 5; i++ {
		storage1.Create(ctx, Chunk{ID: string(rune('a' + i)), Name: "Func"}, []float32{0.1, 0.2, 0.3, 0.4})
		storage2.Create(ctx, Chunk{ID: string(rune('f' + i)), Name: "Func"}, []float32{0.1, 0.2, 0.3, 0.4})
	}

	embedder := &mockEmbedder{embedding: []float32{0.1, 0.2, 0.3, 0.4}}

	storageMap := map[string]Storage{
		"code": storage1,
		"docs": storage2,
	}

	mps := NewMultiProfileSearcher(embedder, storageMap)

	results, err := mps.Search(ctx, "test query", SearchOptions{
		TopK:     3,
		Profiles: []string{"code", "docs"},
	})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}

	// Should return only 3 results even though 10 total exist
	if len(results) != 3 {
		t.Errorf("got %d results, want 3 (TopK limit)", len(results))
	}
}

// TestMultiProfileSearcher_ContextCancellation verifies context cancellation is respected.
func TestMultiProfileSearcher_ContextCancellation(t *testing.T) {
	tmpDir := t.TempDir()

	// Create storages with delay
	baseStorage1, err := NewSQLiteStorage(filepath.Join(tmpDir, "code.db"), 4)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer baseStorage1.Close()
	baseStorage1.Create(context.Background(), Chunk{ID: "code-1", Name: "Func"}, []float32{0.1, 0.2, 0.3, 0.4})

	// Wrap with long delay
	storage1 := &mockDelayStorage{Storage: baseStorage1, delay: 500 * time.Millisecond}

	embedder := &mockEmbedder{embedding: []float32{0.1, 0.2, 0.3, 0.4}}

	storageMap := map[string]Storage{
		"code": storage1,
	}

	mps := NewMultiProfileSearcher(embedder, storageMap)

	// Create cancellable context
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err = mps.Search(ctx, "test query", SearchOptions{
		TopK:     10,
		Profiles: []string{"code"},
	})

	// Should return context error
	if err == nil {
		t.Fatal("Search() should return error on context cancellation")
	}
	if !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled) {
		t.Errorf("error = %v, want context.DeadlineExceeded or context.Canceled", err)
	}
}

// mockFailingStorage always returns an error on Search
type mockFailingStorage struct {
	Storage
	err error
}

func (m *mockFailingStorage) Search(ctx context.Context, queryEmbedding []float32, opts SearchOptions) ([]SearchResult, error) {
	return nil, m.err
}

// Implement required Storage interface methods for mockFailingStorage
func (m *mockFailingStorage) Create(ctx context.Context, chunk Chunk, embedding []float32) error {
	return nil
}
func (m *mockFailingStorage) CreateBatch(ctx context.Context, chunks []ChunkWithEmbedding) error {
	return nil
}
func (m *mockFailingStorage) Read(ctx context.Context, id string) (*Chunk, error) {
	return nil, ErrNotFound
}
func (m *mockFailingStorage) Update(ctx context.Context, chunk Chunk, embedding []float32) error {
	return nil
}
func (m *mockFailingStorage) Delete(ctx context.Context, id string) error {
	return nil
}
func (m *mockFailingStorage) DeleteByFilePath(ctx context.Context, filePath string) (int, error) {
	return 0, nil
}
func (m *mockFailingStorage) List(ctx context.Context, opts ListOptions) ([]Chunk, error) {
	return nil, nil
}
func (m *mockFailingStorage) Stats(ctx context.Context) (*IndexStats, error) {
	return nil, nil
}
func (m *mockFailingStorage) Clear(ctx context.Context) error {
	return nil
}
func (m *mockFailingStorage) GetFileHash(ctx context.Context, filePath string) (string, error) {
	return "", nil
}
func (m *mockFailingStorage) SetFileHash(ctx context.Context, filePath string, hash string) error {
	return nil
}
func (m *mockFailingStorage) Close() error {
	return nil
}
