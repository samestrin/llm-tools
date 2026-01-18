package semantic

import (
	"context"
	"math"
	"path/filepath"
	"testing"
)

// BenchmarkSearchMemory_LargeIndex benchmarks SearchMemory with a large index
// to verify constant memory usage across batch processing
func BenchmarkSearchMemory_LargeIndex(b *testing.B) {
	// Use temp file for better benchmark accuracy
	tmpDir := b.TempDir()
	dbPath := filepath.Join(tmpDir, "bench.db")
	storage, err := NewSQLiteStorage(dbPath, 384)
	if err != nil {
		b.Fatalf("Failed to create storage: %v", err)
	}
	defer storage.Close()

	// Create a large number of memory entries (10,000)
	const numEntries = 10000
	embeddingDim := 384

	// Pre-generate embeddings to avoid allocation in store loop (not realistic but necessary for benchmark purity)
	// In real benchmarks, embeddings come from an API, but here we generate them
	// This is just creating the dataset, not what we're benchmarking
	queryEmbedding := makeEmbedding(embeddingDim, 0.0)

	// Store entries in batches
	const batchSize = 100
	var entries []MemoryWithEmbedding
	for i := 0; i < numEntries; i++ {
		embedding := makeEmbedding(embeddingDim, float32(i)*0.01)
		entry := MemoryWithEmbedding{
			Entry: MemoryEntry{
				Question:    "Test Question " + itoa(i),
				Answer:      "Test Answer " + itoa(i),
				Tags:        []string{"bench", "test", itoa(i % 10)},
				Source:      "benchmark",
				Status:      "active",
				Occurrences: i + 1,
			},
			Embedding: embedding,
		}
		entries = append(entries, entry)
		if len(entries) >= batchSize {
			if err := storage.StoreMemoryBatch(context.Background(), entries); err != nil {
				b.Fatalf("Failed to store batch: %v", err)
			}
			entries = entries[:0]
		}
	}
	if len(entries) > 0 {
		if err := storage.StoreMemoryBatch(context.Background(), entries); err != nil {
			b.Fatalf("Failed to store final batch: %v", err)
		}
	}

	// Reset timer to measure only search performance
	b.ResetTimer()

	// Benchmark the search
	for i := 0; i < b.N; i++ {
		_, err := storage.SearchMemory(context.Background(), queryEmbedding, MemorySearchOptions{
			TopK:      10,
			Threshold: 0.0,
		})
		if err != nil {
			b.Fatalf("SearchMemory failed: %v", err)
		}
	}
}

// BenchmarkSearchMemory_BatchBoundaries tests behavior at batch size boundaries
func BenchmarkSearchMemory_BatchBoundaries(b *testing.B) {
	tests := []struct {
		name        string
		numEntries  int
		batchSize   int
		description string
	}{
		{"BelowBatchSize", 999, 1000, "Just below batch size threshold"},
		{"ExactBatchSize", 1000, 1000, "Exactly at batch size threshold"},
		{"AboveBatchSize", 1001, 1000, "Just above batch size threshold"},
		{"MultipleBatches", 5000, 1000, "Multiple full batches"},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			tmpDir := b.TempDir()
			dbPath := filepath.Join(tmpDir, "bench.db")
			storage, err := NewSQLiteStorage(dbPath, 384)
			if err != nil {
				b.Fatalf("Failed to create storage: %v", err)
			}
			defer storage.Close()

			// Create entries for this test
			entries := make([]MemoryWithEmbedding, tt.numEntries)
			for i := 0; i < tt.numEntries; i++ {
				embedding := makeEmbedding(384, float32(i)*0.01)
				entries[i] = MemoryWithEmbedding{
					Entry: MemoryEntry{
						Question:    "Test Question " + itoa(i),
						Answer:      "Test Answer " + itoa(i),
						Tags:        []string{"bench", "test"},
						Source:      "benchmark",
						Status:      "active",
						Occurrences: i + 1,
					},
					Embedding: embedding,
				}
			}

			if err := storage.StoreMemoryBatch(context.Background(), entries); err != nil {
				b.Fatalf("Failed to store entries: %v", err)
			}

			queryEmbedding := makeEmbedding(384, 0.0)

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, err := storage.SearchMemory(context.Background(), queryEmbedding, MemorySearchOptions{
					TopK:      10,
					Threshold: 0.0,
				})
				if err != nil {
					b.Fatalf("SearchMemory failed: %v", err)
				}
			}
		})
	}
}

// Helper function to create embeddings with a specific base pattern
func makeEmbedding(dim int, base float32) []float32 {
	emb := make([]float32, dim)
	for i := range emb {
		// Create a pattern that varies based on index
		if base != 0 {
			emb[i] = base + float32(i)*0.01
		} else {
			emb[i] = float32(math.Sin(float64(i) * 0.1))
		}
	}
	return emb
}

// Helper function to convert int to string (avoiding strconv to keep benchmark simple)
func itoa(i int) string {
	const digit = "0123456789"
	switch {
	case i == 0:
		return "0"
	case i < 0:
		return "-" + itoa(-i)
	default:
		var buf [20]byte
		var pos int
		for i > 0 {
			buf[pos] = digit[i%10]
			pos++
			i /= 10
		}
		// Reverse buf[0:pos]
		i = 0
		j := pos - 1
		for i < j {
			buf[i], buf[j] = buf[j], buf[i]
			i++
			j--
		}
		return string(buf[:pos])
	}
}
