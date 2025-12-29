package semantic

import (
	"encoding/binary"
	"math"
	"sync"
)

// Buffer pool for reducing memory allocations
var (
	// embeddingBufferPool holds reusable byte slices for embedding operations
	embeddingBufferPool = sync.Pool{
		New: func() interface{} {
			// Default size for 1024-dimensional embeddings (4 bytes per float32)
			buf := make([]byte, 4096)
			return &buf
		},
	}

	// chunkBufferPool holds reusable string builders for chunk operations
	chunkBufferPool = sync.Pool{
		New: func() interface{} {
			return new(chunkBuffer)
		},
	}
)

// chunkBuffer is a reusable buffer for building chunk content
type chunkBuffer struct {
	data []byte
}

func (b *chunkBuffer) Reset() {
	b.data = b.data[:0]
}

func (b *chunkBuffer) Write(p []byte) {
	b.data = append(b.data, p...)
}

func (b *chunkBuffer) String() string {
	return string(b.data)
}

// GetEmbeddingBuffer gets a buffer from the pool, sized appropriately
func GetEmbeddingBuffer(size int) *[]byte {
	buf := embeddingBufferPool.Get().(*[]byte)
	if cap(*buf) < size {
		*buf = make([]byte, size)
	} else {
		*buf = (*buf)[:size]
	}
	return buf
}

// PutEmbeddingBuffer returns a buffer to the pool
func PutEmbeddingBuffer(buf *[]byte) {
	embeddingBufferPool.Put(buf)
}

// GetChunkBuffer gets a chunk buffer from the pool
func GetChunkBuffer() *chunkBuffer {
	buf := chunkBufferPool.Get().(*chunkBuffer)
	buf.Reset()
	return buf
}

// PutChunkBuffer returns a chunk buffer to the pool
func PutChunkBuffer(buf *chunkBuffer) {
	chunkBufferPool.Put(buf)
}

// encodeEmbeddingBinary converts a float32 slice to binary bytes (more efficient)
func encodeEmbeddingBinary(embedding []float32) []byte {
	size := len(embedding) * 4
	buf := GetEmbeddingBuffer(size)
	defer PutEmbeddingBuffer(buf)

	result := make([]byte, size)
	for i, v := range embedding {
		binary.LittleEndian.PutUint32(result[i*4:], math.Float32bits(v))
	}
	return result
}

// decodeEmbeddingBinary converts binary bytes back to a float32 slice
func decodeEmbeddingBinary(data []byte) []float32 {
	if len(data) == 0 || len(data)%4 != 0 {
		return nil
	}

	embedding := make([]float32, len(data)/4)
	for i := 0; i < len(embedding); i++ {
		bits := binary.LittleEndian.Uint32(data[i*4:])
		embedding[i] = math.Float32frombits(bits)
	}
	return embedding
}

// StreamingChunkProcessor processes chunks without loading all into memory
type StreamingChunkProcessor struct {
	batchSize int
	processor func([]Chunk) error
}

// NewStreamingChunkProcessor creates a new streaming processor
func NewStreamingChunkProcessor(batchSize int, processor func([]Chunk) error) *StreamingChunkProcessor {
	if batchSize <= 0 {
		batchSize = 100
	}
	return &StreamingChunkProcessor{
		batchSize: batchSize,
		processor: processor,
	}
}

// Process processes chunks in batches
func (p *StreamingChunkProcessor) Process(chunks <-chan Chunk) error {
	batch := make([]Chunk, 0, p.batchSize)

	for chunk := range chunks {
		batch = append(batch, chunk)

		if len(batch) >= p.batchSize {
			if err := p.processor(batch); err != nil {
				return err
			}
			batch = batch[:0]
		}
	}

	// Process remaining chunks
	if len(batch) > 0 {
		if err := p.processor(batch); err != nil {
			return err
		}
	}

	return nil
}

// CompactEmbedding reduces precision for storage (optional memory savings)
// Converts float32 embeddings to int8 quantized values (-127 to 127)
// This reduces memory by 75% but loses some precision
type CompactEmbedding struct {
	Data   []int8
	Scale  float32
	Offset float32
}

// Quantize converts a float32 embedding to a quantized form
func Quantize(embedding []float32) *CompactEmbedding {
	if len(embedding) == 0 {
		return nil
	}

	// Find min and max
	minVal := embedding[0]
	maxVal := embedding[0]
	for _, v := range embedding[1:] {
		if v < minVal {
			minVal = v
		}
		if v > maxVal {
			maxVal = v
		}
	}

	// Calculate scale and offset for mapping to 0-254 range
	valueRange := maxVal - minVal
	var scale float32
	if valueRange == 0 {
		scale = 1.0
	} else {
		scale = valueRange / 254.0
	}
	offset := minVal

	// Quantize values to 0-254 range, stored as int8 (shifted by -127)
	data := make([]int8, len(embedding))
	for i, v := range embedding {
		// Map to 0-254 range
		normalized := (v - offset) / scale
		// Shift to -127 to 127 for int8 storage
		shifted := normalized - 127.0
		if shifted < -127 {
			shifted = -127
		}
		if shifted > 127 {
			shifted = 127
		}
		data[i] = int8(shifted)
	}

	return &CompactEmbedding{
		Data:   data,
		Scale:  scale,
		Offset: offset,
	}
}

// Dequantize converts a quantized embedding back to float32
func (c *CompactEmbedding) Dequantize() []float32 {
	if c == nil || len(c.Data) == 0 {
		return nil
	}

	result := make([]float32, len(c.Data))
	for i, v := range c.Data {
		// Reverse: shift back from -127..127 to 0..254, then scale and offset
		normalized := float32(v) + 127.0
		result[i] = normalized*c.Scale + c.Offset
	}
	return result
}

// MemoryStats holds memory usage statistics
type MemoryStats struct {
	ChunksInMemory   int
	EmbeddingBytes   int64
	EstimatedTotalMB float64
}

// EstimateMemory estimates memory usage for a set of chunks
func EstimateMemory(chunks int, embeddingDim int) MemoryStats {
	// Each chunk: ~200 bytes (name, path, etc) + content (~500 avg) + embedding
	chunkOverhead := 700
	embeddingBytes := embeddingDim * 4

	totalBytes := int64(chunks) * int64(chunkOverhead+embeddingBytes)

	return MemoryStats{
		ChunksInMemory:   chunks,
		EmbeddingBytes:   int64(embeddingBytes),
		EstimatedTotalMB: float64(totalBytes) / (1024 * 1024),
	}
}
