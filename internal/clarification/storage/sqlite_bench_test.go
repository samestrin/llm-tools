package storage

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/samestrin/llm-tools/internal/clarification/tracking"
)

func BenchmarkSQLiteStorage_Create(b *testing.B) {
	dir := b.TempDir()
	path := filepath.Join(dir, "bench.db")
	storage, err := NewSQLiteStorage(context.Background(), path)
	if err != nil {
		b.Fatalf("failed to create storage: %v", err)
	}
	defer storage.Close()

	ctx := context.Background()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		entry := &tracking.Entry{
			ID:                "bench-" + string(rune(i/65536)) + string(rune(i%65536)),
			CanonicalQuestion: "Benchmark question",
			CurrentAnswer:     "Benchmark answer",
			Occurrences:       1,
			FirstSeen:         "2025-01-01",
			LastSeen:          "2025-01-01",
			Status:            "active",
			Confidence:        "high",
			Variants:          []string{"v1", "v2"},
			ContextTags:       []string{"bench"},
			SprintsSeen:       []string{"sprint-1"},
		}
		if err := storage.Create(ctx, entry); err != nil {
			b.Fatalf("Create failed: %v", err)
		}
	}
}

func BenchmarkSQLiteStorage_Read(b *testing.B) {
	dir := b.TempDir()
	path := filepath.Join(dir, "bench.db")
	storage, err := NewSQLiteStorage(context.Background(), path)
	if err != nil {
		b.Fatalf("failed to create storage: %v", err)
	}
	defer storage.Close()

	ctx := context.Background()

	// Create test entry
	entry := &tracking.Entry{
		ID:                "bench-read",
		CanonicalQuestion: "Benchmark question",
		CurrentAnswer:     "Benchmark answer",
		Occurrences:       1,
		FirstSeen:         "2025-01-01",
		LastSeen:          "2025-01-01",
		Status:            "active",
		Confidence:        "high",
		Variants:          []string{"v1", "v2", "v3"},
		ContextTags:       []string{"bench", "test"},
		SprintsSeen:       []string{"sprint-1"},
	}
	storage.Create(ctx, entry)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		if _, err := storage.Read(ctx, "bench-read"); err != nil {
			b.Fatalf("Read failed: %v", err)
		}
	}
}

func BenchmarkSQLiteStorage_Update(b *testing.B) {
	dir := b.TempDir()
	path := filepath.Join(dir, "bench.db")
	storage, err := NewSQLiteStorage(context.Background(), path)
	if err != nil {
		b.Fatalf("failed to create storage: %v", err)
	}
	defer storage.Close()

	ctx := context.Background()

	// Create test entry
	entry := &tracking.Entry{
		ID:                "bench-update",
		CanonicalQuestion: "Benchmark question",
		CurrentAnswer:     "Benchmark answer",
		Occurrences:       1,
		FirstSeen:         "2025-01-01",
		LastSeen:          "2025-01-01",
		Status:            "active",
		Confidence:        "high",
		Variants:          []string{"v1"},
		ContextTags:       []string{"bench"},
		SprintsSeen:       []string{"sprint-1"},
	}
	storage.Create(ctx, entry)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		entry.Occurrences = i
		if err := storage.Update(ctx, entry); err != nil {
			b.Fatalf("Update failed: %v", err)
		}
	}
}

func BenchmarkSQLiteStorage_List(b *testing.B) {
	dir := b.TempDir()
	path := filepath.Join(dir, "bench.db")
	storage, err := NewSQLiteStorage(context.Background(), path)
	if err != nil {
		b.Fatalf("failed to create storage: %v", err)
	}
	defer storage.Close()

	ctx := context.Background()

	// Create 100 test entries
	for i := 0; i < 100; i++ {
		entry := &tracking.Entry{
			ID:                "bench-list-" + string(rune(i)),
			CanonicalQuestion: "Benchmark question",
			CurrentAnswer:     "Benchmark answer",
			Occurrences:       i,
			FirstSeen:         "2025-01-01",
			LastSeen:          "2025-01-01",
			Status:            "active",
			Confidence:        "high",
			Variants:          []string{"v1", "v2"},
			ContextTags:       []string{"bench", "test"},
			SprintsSeen:       []string{"sprint-1"},
		}
		storage.Create(ctx, entry)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		if _, err := storage.List(ctx, ListFilter{Status: "active"}); err != nil {
			b.Fatalf("List failed: %v", err)
		}
	}
}

func BenchmarkSQLiteStorage_FTSSearch(b *testing.B) {
	dir := b.TempDir()
	path := filepath.Join(dir, "bench.db")
	storage, err := NewSQLiteStorage(context.Background(), path)
	if err != nil {
		b.Fatalf("failed to create storage: %v", err)
	}
	defer storage.Close()

	ctx := context.Background()

	// Create 100 test entries with searchable content
	questions := []string{
		"How to configure authentication?",
		"Database connection pooling best practices",
		"OAuth2 token refresh mechanism",
		"API rate limiting implementation",
		"Caching strategies for microservices",
	}
	for i := 0; i < 100; i++ {
		entry := &tracking.Entry{
			ID:                "bench-fts-" + string(rune(i)),
			CanonicalQuestion: questions[i%len(questions)],
			CurrentAnswer:     "Detailed answer for benchmark testing purposes",
			Occurrences:       i,
			FirstSeen:         "2025-01-01",
			LastSeen:          "2025-01-01",
			Status:            "active",
			Confidence:        "high",
			Variants:          []string{"v1"},
			ContextTags:       []string{"bench"},
			SprintsSeen:       []string{"sprint-1"},
		}
		storage.Create(ctx, entry)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		if _, err := storage.List(ctx, ListFilter{Query: "authentication"}); err != nil {
			b.Fatalf("FTS search failed: %v", err)
		}
	}
}

func BenchmarkSQLiteStorage_BulkInsert_100(b *testing.B) {
	ctx := context.Background()

	for i := 0; i < b.N; i++ {
		b.StopTimer()
		dir := b.TempDir()
		path := filepath.Join(dir, "bench.db")
		storage, _ := NewSQLiteStorage(ctx, path)

		entries := make([]tracking.Entry, 100)
		for j := range entries {
			entries[j] = tracking.Entry{
				ID:                "bulk-" + string(rune(j)),
				CanonicalQuestion: "Benchmark question",
				CurrentAnswer:     "Benchmark answer",
				Occurrences:       1,
				FirstSeen:         "2025-01-01",
				LastSeen:          "2025-01-01",
				Status:            "active",
				Confidence:        "high",
				Variants:          []string{"v1", "v2"},
				ContextTags:       []string{"bench", "test"},
				SprintsSeen:       []string{"sprint-1"},
			}
		}

		b.StartTimer()
		if _, err := storage.BulkInsert(ctx, entries); err != nil {
			b.Fatalf("BulkInsert failed: %v", err)
		}
		b.StopTimer()

		storage.Close()
	}
}

func BenchmarkSQLiteStorage_BulkInsert_1000(b *testing.B) {
	ctx := context.Background()

	for i := 0; i < b.N; i++ {
		b.StopTimer()
		dir := b.TempDir()
		path := filepath.Join(dir, "bench.db")
		storage, _ := NewSQLiteStorage(ctx, path)

		entries := make([]tracking.Entry, 1000)
		for j := range entries {
			entries[j] = tracking.Entry{
				ID:                "bulk-" + string(rune(j/256)) + string(rune(j%256)),
				CanonicalQuestion: "Benchmark question",
				CurrentAnswer:     "Benchmark answer",
				Occurrences:       1,
				FirstSeen:         "2025-01-01",
				LastSeen:          "2025-01-01",
				Status:            "active",
				Confidence:        "high",
				Variants:          []string{"v1", "v2", "v3"},
				ContextTags:       []string{"bench", "test"},
				SprintsSeen:       []string{"sprint-1"},
			}
		}

		b.StartTimer()
		if _, err := storage.BulkInsert(ctx, entries); err != nil {
			b.Fatalf("BulkInsert failed: %v", err)
		}
		b.StopTimer()

		storage.Close()
	}
}

func BenchmarkSQLiteStorage_ConcurrentReads(b *testing.B) {
	dir := b.TempDir()
	path := filepath.Join(dir, "bench.db")
	storage, err := NewSQLiteStorage(context.Background(), path)
	if err != nil {
		b.Fatalf("failed to create storage: %v", err)
	}
	defer storage.Close()

	ctx := context.Background()

	// Create test entry
	entry := &tracking.Entry{
		ID:                "bench-concurrent",
		CanonicalQuestion: "Benchmark question",
		CurrentAnswer:     "Benchmark answer",
		Occurrences:       1,
		FirstSeen:         "2025-01-01",
		LastSeen:          "2025-01-01",
		Status:            "active",
		Confidence:        "high",
		Variants:          []string{"v1", "v2", "v3"},
		ContextTags:       []string{"bench", "test", "concurrent"},
		SprintsSeen:       []string{"sprint-1", "sprint-2"},
	}
	storage.Create(ctx, entry)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if _, err := storage.Read(ctx, "bench-concurrent"); err != nil {
				b.Errorf("Read failed: %v", err)
			}
		}
	})
}

func BenchmarkSQLiteStorage_Stats(b *testing.B) {
	dir := b.TempDir()
	path := filepath.Join(dir, "bench.db")
	storage, err := NewSQLiteStorage(context.Background(), path)
	if err != nil {
		b.Fatalf("failed to create storage: %v", err)
	}
	defer storage.Close()

	ctx := context.Background()

	// Create 100 test entries
	for i := 0; i < 100; i++ {
		entry := &tracking.Entry{
			ID:                "bench-stats-" + string(rune(i)),
			CanonicalQuestion: "Benchmark question",
			CurrentAnswer:     "Benchmark answer",
			Occurrences:       i,
			FirstSeen:         "2025-01-01",
			LastSeen:          "2025-01-01",
			Status:            []string{"active", "resolved", "pending"}[i%3],
			Confidence:        "high",
			Variants:          []string{"v1", "v2"},
			ContextTags:       []string{"tag1", "tag2", "tag3"},
			SprintsSeen:       []string{"sprint-1", "sprint-2"},
		}
		storage.Create(ctx, entry)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		if _, err := storage.Stats(ctx); err != nil {
			b.Fatalf("Stats failed: %v", err)
		}
	}
}
