package semantic

import (
	"context"
	"testing"
	"time"
)

func TestHybridSearchMemory_BasicFusion(t *testing.T) {
	storage, err := NewSQLiteStorage(":memory:", 4)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer storage.Close()
	ctx := context.Background()

	// Store memories with distinct text
	entries := []struct {
		question  string
		answer    string
		embedding []float32
	}{
		{"JWT token rotation policy", "Rotate tokens every 24 hours", []float32{0.9, 0.1, 0.0, 0.0}},
		{"Database connection pooling", "Use pgbouncer with 50 max connections", []float32{0.0, 0.9, 0.1, 0.0}},
		{"API rate limiting approach", "Use token bucket with 100 req/sec", []float32{0.0, 0.0, 0.9, 0.1}},
	}
	for _, e := range entries {
		entry := NewMemoryEntry(e.question, e.answer)
		if err := storage.StoreMemory(ctx, *entry, e.embedding); err != nil {
			t.Fatalf("StoreMemory failed: %v", err)
		}
	}

	// Mock embedder returns a vector close to the first entry
	embedder := &mockEmbedder{embedding: []float32{0.8, 0.2, 0.0, 0.0}}

	results, err := HybridSearchMemory(ctx, storage, embedder, "JWT rotation", HybridMemorySearchOptions{
		MemorySearchOptions: MemorySearchOptions{TopK: 10},
	})
	if err != nil {
		t.Fatalf("HybridSearchMemory failed: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected non-empty results")
	}
	// JWT entry should be in results (matched by both dense and lexical)
	found := false
	for _, r := range results {
		if r.Entry.Question == "JWT token rotation policy" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected JWT rotation entry in results")
	}
}

func TestHybridSearchMemory_KeywordBoost(t *testing.T) {
	storage, err := NewSQLiteStorage(":memory:", 4)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer storage.Close()
	ctx := context.Background()

	// Both entries have similar embeddings, but only one has keyword match
	entry1 := NewMemoryEntry("Authentication middleware design", "Use bearer tokens")
	if err := storage.StoreMemory(ctx, *entry1, []float32{0.5, 0.5, 0.0, 0.0}); err != nil {
		t.Fatalf("StoreMemory failed: %v", err)
	}
	entry2 := NewMemoryEntry("Security review process", "Review code weekly")
	if err := storage.StoreMemory(ctx, *entry2, []float32{0.5, 0.5, 0.0, 0.0}); err != nil {
		t.Fatalf("StoreMemory failed: %v", err)
	}

	embedder := &mockEmbedder{embedding: []float32{0.5, 0.5, 0.0, 0.0}}

	results, err := HybridSearchMemory(ctx, storage, embedder, "authentication", HybridMemorySearchOptions{
		MemorySearchOptions: MemorySearchOptions{TopK: 10},
	})
	if err != nil {
		t.Fatalf("HybridSearchMemory failed: %v", err)
	}
	if len(results) < 2 {
		t.Fatalf("expected at least 2 results, got %d", len(results))
	}
	// Authentication entry should rank first (keyword match + dense match)
	if results[0].Entry.Question != "Authentication middleware design" {
		t.Errorf("expected authentication entry first, got %q", results[0].Entry.Question)
	}
}

func TestHybridSearchMemory_WithDecay(t *testing.T) {
	storage, err := NewSQLiteStorage(":memory:", 4)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer storage.Close()
	ctx := context.Background()

	// Old memory
	oldEntry := NewMemoryEntry("Old caching strategy", "Use memcached")
	oldEntry.CreatedAt = "2024-01-01T00:00:00Z"
	if err := storage.StoreMemory(ctx, *oldEntry, []float32{0.5, 0.5, 0.0, 0.0}); err != nil {
		t.Fatalf("StoreMemory failed: %v", err)
	}

	// New memory
	newEntry := NewMemoryEntry("New caching strategy", "Use Redis with TTL")
	newEntry.CreatedAt = time.Now().Format(time.RFC3339)
	if err := storage.StoreMemory(ctx, *newEntry, []float32{0.5, 0.5, 0.0, 0.0}); err != nil {
		t.Fatalf("StoreMemory failed: %v", err)
	}

	embedder := &mockEmbedder{embedding: []float32{0.5, 0.5, 0.0, 0.0}}
	decay := &TemporalDecayConfig{HalfLifeDays: 90, Enabled: true}

	results, err := HybridSearchMemory(ctx, storage, embedder, "caching", HybridMemorySearchOptions{
		MemorySearchOptions: MemorySearchOptions{TopK: 10},
		Decay:               decay,
	})
	if err != nil {
		t.Fatalf("HybridSearchMemory failed: %v", err)
	}
	if len(results) < 2 {
		t.Fatalf("expected at least 2 results, got %d", len(results))
	}
	// Newer entry should rank first due to decay
	if results[0].Entry.Question != "New caching strategy" {
		t.Errorf("expected new caching entry first with decay, got %q", results[0].Entry.Question)
	}
}

func TestHybridSearchMemory_WithoutDecay(t *testing.T) {
	storage, err := NewSQLiteStorage(":memory:", 4)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer storage.Close()
	ctx := context.Background()

	entry := NewMemoryEntry("Test memory", "Test answer")
	entry.CreatedAt = "2020-01-01T00:00:00Z"
	if err := storage.StoreMemory(ctx, *entry, []float32{0.5, 0.5, 0.0, 0.0}); err != nil {
		t.Fatalf("StoreMemory failed: %v", err)
	}

	embedder := &mockEmbedder{embedding: []float32{0.5, 0.5, 0.0, 0.0}}

	// No decay
	results, err := HybridSearchMemory(ctx, storage, embedder, "test", HybridMemorySearchOptions{
		MemorySearchOptions: MemorySearchOptions{TopK: 10},
	})
	if err != nil {
		t.Fatalf("HybridSearchMemory failed: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected results")
	}
	// Score should not be modified by decay
	if results[0].Score <= 0 {
		t.Errorf("expected positive score without decay, got %v", results[0].Score)
	}
}

func TestHybridSearchMemory_TopK(t *testing.T) {
	storage, err := NewSQLiteStorage(":memory:", 4)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer storage.Close()
	ctx := context.Background()

	for i := 0; i < 10; i++ {
		entry := NewMemoryEntry("Memory about testing approach", "Use unit tests")
		if err := storage.StoreMemory(ctx, *entry, []float32{0.5, 0.5, 0.0, 0.0}); err != nil {
			t.Fatalf("StoreMemory failed: %v", err)
		}
	}

	embedder := &mockEmbedder{embedding: []float32{0.5, 0.5, 0.0, 0.0}}

	results, err := HybridSearchMemory(ctx, storage, embedder, "testing", HybridMemorySearchOptions{
		MemorySearchOptions: MemorySearchOptions{TopK: 3},
	})
	if err != nil {
		t.Fatalf("HybridSearchMemory failed: %v", err)
	}
	if len(results) > 3 {
		t.Errorf("expected at most 3 results with TopK=3, got %d", len(results))
	}
}

func TestHybridSearchMemory_EmptyQuery(t *testing.T) {
	storage, err := NewSQLiteStorage(":memory:", 4)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer storage.Close()

	embedder := &mockEmbedder{embedding: []float32{0.5, 0.5, 0.0, 0.0}}

	_, err = HybridSearchMemory(context.Background(), storage, embedder, "", HybridMemorySearchOptions{})
	if err == nil {
		t.Error("expected error for empty query")
	}
}

type nonLexicalStorage struct {
	Storage
}

func TestHybridSearchMemory_UnsupportedStorage(t *testing.T) {
	embedder := &mockEmbedder{embedding: []float32{0.5, 0.5, 0.0, 0.0}}

	// Use a storage that doesn't implement MemoryLexicalSearcher
	_, err := HybridSearchMemory(context.Background(), &nonLexicalStorage{}, embedder, "test", HybridMemorySearchOptions{
		MemorySearchOptions: MemorySearchOptions{TopK: 10},
	})
	if err == nil {
		t.Error("expected error for storage without MemoryLexicalSearcher")
	}
}
