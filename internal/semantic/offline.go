package semantic

import (
	"context"
	"fmt"
	"math"
	"net"
	"strings"
	"time"
)

// OfflineMode represents the current offline mode state
type OfflineMode int

const (
	// OnlineMode indicates the embedder is working normally
	OnlineMode OfflineMode = iota
	// OfflineFallback indicates we're using keyword fallback
	OfflineFallback
)

// OfflineEmbedder wraps an embedder with offline mode support
type OfflineEmbedder struct {
	embedder      EmbedderInterface
	offlineMode   OfflineMode
	lastCheck     time.Time
	checkInterval time.Duration
	dimensions    int
}

// NewOfflineEmbedder creates an embedder with offline fallback support
func NewOfflineEmbedder(embedder EmbedderInterface, dimensions int) *OfflineEmbedder {
	return &OfflineEmbedder{
		embedder:      embedder,
		offlineMode:   OnlineMode,
		checkInterval: 30 * time.Second,
		dimensions:    dimensions,
	}
}

// IsOffline returns true if operating in offline mode
func (o *OfflineEmbedder) IsOffline() bool {
	return o.offlineMode == OfflineFallback
}

// Embed tries to use the embedder, falls back to keyword embedding if unavailable
func (o *OfflineEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	// Try the real embedder first
	if o.offlineMode == OnlineMode || time.Since(o.lastCheck) > o.checkInterval {
		embedding, err := o.embedder.Embed(ctx, text)
		if err == nil {
			o.offlineMode = OnlineMode
			return embedding, nil
		}

		// Check if it's a network error
		if isNetworkError(err) {
			o.offlineMode = OfflineFallback
			o.lastCheck = time.Now()
		} else {
			// For non-network errors, propagate the error
			return nil, err
		}
	}

	// Fall back to keyword-based embedding
	return o.keywordEmbedding(text), nil
}

// EmbedBatch tries to use the embedder batch, falls back if unavailable
func (o *OfflineEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	// Try the real embedder first
	if o.offlineMode == OnlineMode || time.Since(o.lastCheck) > o.checkInterval {
		embeddings, err := o.embedder.EmbedBatch(ctx, texts)
		if err == nil {
			o.offlineMode = OnlineMode
			return embeddings, nil
		}

		// Check if it's a network error
		if isNetworkError(err) {
			o.offlineMode = OfflineFallback
			o.lastCheck = time.Now()
		} else {
			// For non-network errors, propagate the error
			return nil, err
		}
	}

	// Fall back to keyword-based embedding
	result := make([][]float32, len(texts))
	for i, text := range texts {
		result[i] = o.keywordEmbedding(text)
	}
	return result, nil
}

// Dimensions returns the embedding dimensions
func (o *OfflineEmbedder) Dimensions() int {
	if dims := o.embedder.Dimensions(); dims > 0 {
		return dims
	}
	return o.dimensions
}

// keywordEmbedding creates a simple hash-based embedding from keywords
// This provides basic semantic similarity through keyword matching
func (o *OfflineEmbedder) keywordEmbedding(text string) []float32 {
	dims := o.dimensions
	if dims == 0 {
		dims = 1024
	}

	embedding := make([]float32, dims)

	// Tokenize and normalize
	text = strings.ToLower(text)
	words := strings.FieldsFunc(text, func(r rune) bool {
		return !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_')
	})

	// Create a bag-of-words style embedding with position awareness
	for i, word := range words {
		if len(word) < 2 {
			continue
		}

		// Hash the word to a dimension
		hash := hashString(word)

		// Primary position based on word hash
		pos := int(hash % uint32(dims))
		embedding[pos] += 1.0

		// Add positional information (words at start matter more)
		posWeight := 1.0 / float32(1+i/5)
		embedding[pos] *= 1.0 + posWeight*0.1

		// Add bigram features for adjacent words
		if i < len(words)-1 {
			bigram := word + "_" + words[i+1]
			bigramHash := hashString(bigram)
			bigramPos := int(bigramHash % uint32(dims))
			embedding[bigramPos] += 0.5
		}
	}

	// Normalize the embedding
	var norm float32
	for _, v := range embedding {
		norm += v * v
	}
	if norm > 0 {
		norm = float32(1.0 / math.Sqrt(float64(norm)))
		for i := range embedding {
			embedding[i] *= norm
		}
	}

	return embedding
}

// hashString creates a simple hash of a string
func hashString(s string) uint32 {
	hash := uint32(2166136261) // FNV-1a offset basis
	for _, b := range []byte(s) {
		hash ^= uint32(b)
		hash *= 16777619 // FNV-1a prime
	}
	return hash
}

// isNetworkError checks if an error is network-related
func isNetworkError(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()

	// Check for common network error patterns (all lowercase for case-insensitive matching)
	networkPatterns := []string{
		"connection refused",
		"connection reset",
		"no such host",
		"network is unreachable",
		"timeout",
		"dial tcp",
		"dial udp",
		"no route to host",
		"connection timed out",
		"eof",
	}

	errLower := strings.ToLower(errStr)
	for _, pattern := range networkPatterns {
		if strings.Contains(errLower, pattern) {
			return true
		}
	}

	// Check for net.Error interface
	if _, ok := err.(net.Error); ok {
		return true
	}

	return false
}

// OfflineSearcher provides search capabilities with offline fallback
type OfflineSearcher struct {
	storage  Storage
	embedder *OfflineEmbedder
}

// NewOfflineSearcher creates a searcher with offline support
func NewOfflineSearcher(storage Storage, embedder *OfflineEmbedder) *OfflineSearcher {
	return &OfflineSearcher{
		storage:  storage,
		embedder: embedder,
	}
}

// Search performs semantic or keyword-based search
func (s *OfflineSearcher) Search(ctx context.Context, query string, opts SearchOptions) ([]SearchResult, error) {
	// Get query embedding (may be keyword-based if offline)
	queryEmbedding, err := s.embedder.Embed(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to embed query: %w", err)
	}

	// Search the index
	results, err := s.storage.Search(ctx, queryEmbedding, opts)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	// If offline, boost results with keyword matches
	if s.embedder.IsOffline() {
		results = s.boostKeywordMatches(query, results)
	}

	return results, nil
}

// boostKeywordMatches improves ranking for results with keyword matches
func (s *OfflineSearcher) boostKeywordMatches(query string, results []SearchResult) []SearchResult {
	queryWords := strings.Fields(strings.ToLower(query))

	for i := range results {
		content := strings.ToLower(results[i].Chunk.Content)
		name := strings.ToLower(results[i].Chunk.Name)

		matchCount := 0
		for _, word := range queryWords {
			if len(word) < 2 {
				continue
			}
			if strings.Contains(content, word) || strings.Contains(name, word) {
				matchCount++
			}
		}

		// Boost score based on keyword matches
		if matchCount > 0 && len(queryWords) > 0 {
			matchRatio := float32(matchCount) / float32(len(queryWords))
			results[i].Score += matchRatio * 0.2 // Up to 20% boost
			if results[i].Score > 1.0 {
				results[i].Score = 1.0
			}
		}
	}

	return results
}

// IsOffline returns true if the searcher is operating in offline mode
func (s *OfflineSearcher) IsOffline() bool {
	return s.embedder.IsOffline()
}
