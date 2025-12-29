package semantic

import (
	"math"
	"testing"
)

func TestBinaryEmbeddingEncoding(t *testing.T) {
	original := []float32{1.0, 2.5, -3.7, 0.001, 100.5}

	encoded := encodeEmbeddingBinary(original)
	decoded := decodeEmbeddingBinary(encoded)

	if len(decoded) != len(original) {
		t.Fatalf("Length mismatch: got %d, want %d", len(decoded), len(original))
	}

	for i, v := range original {
		if decoded[i] != v {
			t.Errorf("Value mismatch at %d: got %f, want %f", i, decoded[i], v)
		}
	}
}

func TestBinaryEncoding_Empty(t *testing.T) {
	var empty []float32
	encoded := encodeEmbeddingBinary(empty)
	decoded := decodeEmbeddingBinary(encoded)

	if len(decoded) != 0 {
		t.Errorf("Expected empty result, got %d elements", len(decoded))
	}
}

func TestBinaryDecoding_InvalidData(t *testing.T) {
	// Invalid length (not multiple of 4)
	invalid := []byte{1, 2, 3}
	decoded := decodeEmbeddingBinary(invalid)

	if decoded != nil {
		t.Errorf("Expected nil for invalid data, got %v", decoded)
	}
}

func TestBufferPool(t *testing.T) {
	// Get and put buffers multiple times
	for i := 0; i < 100; i++ {
		buf := GetEmbeddingBuffer(1024)
		if len(*buf) != 1024 {
			t.Errorf("Buffer size = %d, want 1024", len(*buf))
		}
		PutEmbeddingBuffer(buf)
	}

	// Get larger buffer
	buf := GetEmbeddingBuffer(8192)
	if len(*buf) != 8192 {
		t.Errorf("Large buffer size = %d, want 8192", len(*buf))
	}
	PutEmbeddingBuffer(buf)
}

func TestChunkBufferPool(t *testing.T) {
	buf := GetChunkBuffer()
	buf.Write([]byte("test"))
	buf.Write([]byte(" data"))

	if buf.String() != "test data" {
		t.Errorf("Buffer content = %q, want 'test data'", buf.String())
	}

	PutChunkBuffer(buf)

	// Get again and verify reset
	buf2 := GetChunkBuffer()
	if len(buf2.data) != 0 {
		t.Errorf("Buffer should be reset, has %d bytes", len(buf2.data))
	}
	PutChunkBuffer(buf2)
}

func TestQuantize(t *testing.T) {
	original := []float32{0.0, 0.5, 1.0, -0.5, -1.0}

	compact := Quantize(original)
	if compact == nil {
		t.Fatal("Quantize returned nil")
	}

	restored := compact.Dequantize()
	if len(restored) != len(original) {
		t.Fatalf("Length mismatch: got %d, want %d", len(restored), len(original))
	}

	// Check values are approximately equal (some precision loss expected)
	for i, v := range original {
		diff := float32(math.Abs(float64(restored[i] - v)))
		if diff > 0.02 { // Allow 2% error from quantization
			t.Errorf("Value %d: diff %f too large (original %f, restored %f)", i, diff, v, restored[i])
		}
	}
}

func TestQuantize_Empty(t *testing.T) {
	var empty []float32
	compact := Quantize(empty)

	if compact != nil {
		t.Error("Expected nil for empty embedding")
	}
}

func TestQuantize_SingleValue(t *testing.T) {
	single := []float32{0.5}
	compact := Quantize(single)

	if compact == nil {
		t.Fatal("Quantize returned nil")
	}

	restored := compact.Dequantize()
	if len(restored) != 1 {
		t.Fatalf("Length = %d, want 1", len(restored))
	}
}

func TestDequantize_Nil(t *testing.T) {
	var compact *CompactEmbedding
	result := compact.Dequantize()

	if result != nil {
		t.Error("Expected nil for nil input")
	}
}

func TestStreamingChunkProcessor(t *testing.T) {
	processed := 0
	batches := 0

	processor := NewStreamingChunkProcessor(3, func(chunks []Chunk) error {
		processed += len(chunks)
		batches++
		return nil
	})

	// Create a channel and send chunks
	ch := make(chan Chunk, 10)
	for i := 0; i < 7; i++ {
		ch <- Chunk{Name: "chunk"}
	}
	close(ch)

	err := processor.Process(ch)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	if processed != 7 {
		t.Errorf("Processed = %d, want 7", processed)
	}

	// Should be 3 batches: 3, 3, 1
	if batches != 3 {
		t.Errorf("Batches = %d, want 3", batches)
	}
}

func TestEstimateMemory(t *testing.T) {
	stats := EstimateMemory(1000, 1024)

	if stats.ChunksInMemory != 1000 {
		t.Errorf("ChunksInMemory = %d, want 1000", stats.ChunksInMemory)
	}

	if stats.EmbeddingBytes != 4096 {
		t.Errorf("EmbeddingBytes = %d, want 4096", stats.EmbeddingBytes)
	}

	// 1000 chunks * (700 overhead + 4096 embedding) = ~4.5MB
	if stats.EstimatedTotalMB < 4.0 || stats.EstimatedTotalMB > 5.0 {
		t.Errorf("EstimatedTotalMB = %f, expected ~4.5MB", stats.EstimatedTotalMB)
	}
}

// BenchmarkBinaryVsJSONEncoding compares encoding performance
func BenchmarkBinaryEncoding(b *testing.B) {
	embedding := make([]float32, 1024)
	for i := range embedding {
		embedding[i] = float32(i) / 1024.0
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		data := encodeEmbeddingBinary(embedding)
		_ = decodeEmbeddingBinary(data)
	}
}

func BenchmarkJSONEncoding(b *testing.B) {
	embedding := make([]float32, 1024)
	for i := range embedding {
		embedding[i] = float32(i) / 1024.0
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		data, _ := encodeEmbedding(embedding)
		_, _ = decodeEmbedding(data)
	}
}

func BenchmarkQuantization(b *testing.B) {
	embedding := make([]float32, 1024)
	for i := range embedding {
		embedding[i] = float32(i) / 1024.0
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		compact := Quantize(embedding)
		_ = compact.Dequantize()
	}
}
