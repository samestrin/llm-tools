package semantic

import (
	"context"
	"errors"
	"testing"
)

// MockEmbedderForOffline simulates an embedder that can fail
type MockEmbedderForOffline struct {
	shouldFail bool
	failWith   error
	dimensions int
}

func (m *MockEmbedderForOffline) Embed(ctx context.Context, text string) ([]float32, error) {
	if m.shouldFail {
		return nil, m.failWith
	}
	embedding := make([]float32, m.dimensions)
	hash := uint32(0)
	for _, ch := range text {
		hash = hash*31 + uint32(ch)
	}
	for i := 0; i < m.dimensions; i++ {
		hash = hash*1103515245 + 12345
		embedding[i] = float32(hash%1000) / 1000.0
	}
	return embedding, nil
}

func (m *MockEmbedderForOffline) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	if m.shouldFail {
		return nil, m.failWith
	}
	results := make([][]float32, len(texts))
	for i, text := range texts {
		emb, err := m.Embed(ctx, text)
		if err != nil {
			return nil, err
		}
		results[i] = emb
	}
	return results, nil
}

func (m *MockEmbedderForOffline) Dimensions() int {
	return m.dimensions
}

func (m *MockEmbedderForOffline) Model() string {
	return "mock-offline-embedder"
}

func TestOfflineEmbedder_OnlineMode(t *testing.T) {
	mockEmb := &MockEmbedderForOffline{
		shouldFail: false,
		dimensions: 1024,
	}
	offlineEmb := NewOfflineEmbedder(mockEmb, 1024)

	ctx := context.Background()
	embedding, err := offlineEmb.Embed(ctx, "test query")

	if err != nil {
		t.Fatalf("Embed failed: %v", err)
	}

	if len(embedding) != 1024 {
		t.Errorf("Embedding length = %d, want 1024", len(embedding))
	}

	if offlineEmb.IsOffline() {
		t.Error("Should be in online mode")
	}
}

func TestOfflineEmbedder_FallbackOnNetworkError(t *testing.T) {
	mockEmb := &MockEmbedderForOffline{
		shouldFail: true,
		failWith:   errors.New("dial tcp: connection refused"),
		dimensions: 1024,
	}
	offlineEmb := NewOfflineEmbedder(mockEmb, 1024)

	ctx := context.Background()
	embedding, err := offlineEmb.Embed(ctx, "test query")

	if err != nil {
		t.Fatalf("Embed should not fail with fallback: %v", err)
	}

	if len(embedding) != 1024 {
		t.Errorf("Fallback embedding length = %d, want 1024", len(embedding))
	}

	if !offlineEmb.IsOffline() {
		t.Error("Should be in offline mode after network error")
	}
}

func TestOfflineEmbedder_NoFallbackOnOtherErrors(t *testing.T) {
	mockEmb := &MockEmbedderForOffline{
		shouldFail: true,
		failWith:   errors.New("API error: rate limit exceeded"),
		dimensions: 1024,
	}
	offlineEmb := NewOfflineEmbedder(mockEmb, 1024)

	ctx := context.Background()
	_, err := offlineEmb.Embed(ctx, "test query")

	if err == nil {
		t.Error("Should fail on non-network error")
	}

	if offlineEmb.IsOffline() {
		t.Error("Should not be in offline mode for non-network errors")
	}
}

func TestOfflineEmbedder_BatchFallback(t *testing.T) {
	mockEmb := &MockEmbedderForOffline{
		shouldFail: true,
		failWith:   errors.New("connection timed out"),
		dimensions: 512,
	}
	offlineEmb := NewOfflineEmbedder(mockEmb, 512)

	ctx := context.Background()
	texts := []string{"query 1", "query 2", "query 3"}
	embeddings, err := offlineEmb.EmbedBatch(ctx, texts)

	if err != nil {
		t.Fatalf("EmbedBatch should not fail with fallback: %v", err)
	}

	if len(embeddings) != 3 {
		t.Errorf("Got %d embeddings, want 3", len(embeddings))
	}

	for i, emb := range embeddings {
		if len(emb) != 512 {
			t.Errorf("Embedding %d length = %d, want 512", i, len(emb))
		}
	}
}

func TestOfflineEmbedder_KeywordEmbeddingSimilarity(t *testing.T) {
	offlineEmb := NewOfflineEmbedder(&MockEmbedderForOffline{
		shouldFail: true,
		failWith:   errors.New("connection refused"),
		dimensions: 256,
	}, 256)

	ctx := context.Background()

	// Similar texts should have higher similarity
	emb1, _ := offlineEmb.Embed(ctx, "user authentication login")
	emb2, _ := offlineEmb.Embed(ctx, "user login authentication")
	emb3, _ := offlineEmb.Embed(ctx, "database query optimization")

	sim12 := cosineSimilarity(emb1, emb2)
	sim13 := cosineSimilarity(emb1, emb3)

	// Similar texts should have higher similarity than dissimilar
	if sim12 <= sim13 {
		t.Errorf("Similar text similarity (%f) should be higher than dissimilar (%f)", sim12, sim13)
	}
}

func TestIsNetworkError(t *testing.T) {
	testCases := []struct {
		err      error
		expected bool
	}{
		{errors.New("dial tcp: connection refused"), true},
		{errors.New("network is unreachable"), true},
		{errors.New("connection timed out"), true},
		{errors.New("no such host"), true},
		{errors.New("unexpected EOF"), true}, // EOF with context
		{errors.New("API error: rate limit"), false},
		{errors.New("invalid request"), false},
		{nil, false},
	}

	for _, tc := range testCases {
		result := isNetworkError(tc.err)
		if result != tc.expected {
			t.Errorf("isNetworkError(%v) = %v, want %v", tc.err, result, tc.expected)
		}
	}
}

func TestKeywordEmbedding_Consistency(t *testing.T) {
	offlineEmb := NewOfflineEmbedder(&MockEmbedderForOffline{
		shouldFail: true,
		failWith:   errors.New("connection refused"),
		dimensions: 256,
	}, 256)

	ctx := context.Background()

	// Same text should produce same embedding
	text := "func processData(data []byte) error"
	emb1, _ := offlineEmb.Embed(ctx, text)
	emb2, _ := offlineEmb.Embed(ctx, text)

	for i := range emb1 {
		if emb1[i] != emb2[i] {
			t.Errorf("Embeddings differ at index %d: %f vs %f", i, emb1[i], emb2[i])
			break
		}
	}
}

func TestOfflineEmbedder_Dimensions(t *testing.T) {
	mockEmb := &MockEmbedderForOffline{
		shouldFail: false,
		dimensions: 768,
	}
	offlineEmb := NewOfflineEmbedder(mockEmb, 1024)

	// Should use embedder's dimensions when available
	dims := offlineEmb.Dimensions()
	if dims != 768 {
		t.Errorf("Dimensions = %d, want 768", dims)
	}

	// With zero-dimension embedder, should use fallback
	mockEmb.dimensions = 0
	offlineEmb = NewOfflineEmbedder(mockEmb, 1024)
	dims = offlineEmb.Dimensions()
	if dims != 1024 {
		t.Errorf("Fallback dimensions = %d, want 1024", dims)
	}
}
