//go:build integration
// +build integration

package semantic

import (
	"context"
	"os"
	"testing"
	"time"
)

// Run with: go test -tags=integration -v ./internal/semantic/... -run Integration

func TestIntegration_CohereEmbedder(t *testing.T) {
	apiKey := os.Getenv("COHERE_API_KEY")
	if apiKey == "" {
		t.Skip("COHERE_API_KEY not set")
	}

	embedder, err := NewCohereEmbedder(CohereConfig{
		APIKey: apiKey,
	})
	if err != nil {
		t.Fatalf("Failed to create embedder: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	embedding, err := embedder.Embed(ctx, "Hello world, this is a test of the Cohere embedding API")
	if err != nil {
		t.Fatalf("Failed to embed: %v", err)
	}

	t.Logf("Cohere embedding dimensions: %d", len(embedding))
	if len(embedding) == 0 {
		t.Error("Expected non-empty embedding")
	}
}

func TestIntegration_HuggingFaceEmbedder(t *testing.T) {
	apiKey := os.Getenv("HUGGINGFACE_API_KEY")
	if apiKey == "" {
		apiKey = os.Getenv("HUGGING_FACE_API_KEY")
	}
	if apiKey == "" {
		apiKey = os.Getenv("HF_TOKEN")
	}
	if apiKey == "" {
		t.Skip("HUGGINGFACE_API_KEY not set")
	}

	embedder, err := NewHuggingFaceEmbedder(HuggingFaceConfig{
		APIKey:       apiKey,
		WaitForModel: true,
	})
	if err != nil {
		t.Fatalf("Failed to create embedder: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	embedding, err := embedder.Embed(ctx, "Hello world, this is a test of the HuggingFace embedding API")
	if err != nil {
		t.Fatalf("Failed to embed: %v", err)
	}

	t.Logf("HuggingFace embedding dimensions: %d", len(embedding))
	if len(embedding) == 0 {
		t.Error("Expected non-empty embedding")
	}
}

func TestIntegration_OpenRouterEmbedder(t *testing.T) {
	apiKey := os.Getenv("OPENROUTER_API_KEY")
	if apiKey == "" {
		t.Skip("OPENROUTER_API_KEY not set")
	}

	embedder, err := NewOpenRouterEmbedder(OpenRouterConfig{
		APIKey: apiKey,
	})
	if err != nil {
		t.Fatalf("Failed to create embedder: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	embedding, err := embedder.Embed(ctx, "Hello world, this is a test of the OpenRouter embedding API")
	if err != nil {
		t.Fatalf("Failed to embed: %v", err)
	}

	t.Logf("OpenRouter embedding dimensions: %d", len(embedding))
	if len(embedding) == 0 {
		t.Error("Expected non-empty embedding")
	}
}

func TestIntegration_QdrantStorage(t *testing.T) {
	apiKey := os.Getenv("QDRANT_API_KEY")
	apiURL := os.Getenv("QDRANT_API_URL")
	if apiKey == "" || apiURL == "" {
		t.Skip("QDRANT_API_KEY or QDRANT_API_URL not set")
	}

	// Use a test collection name to avoid conflicts
	storage, err := NewQdrantStorage(QdrantConfig{
		APIKey:         apiKey,
		URL:            apiURL,
		CollectionName: "llm_semantic_test",
		EmbeddingDim:   384, // Common dimension for many models
	})
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer storage.Close()

	// Cleanup test collection at the end
	defer storage.DeleteCollection()

	ctx := context.Background()

	// Test create
	chunk := Chunk{
		ID:        "test-chunk-1",
		FilePath:  "/test/file.go",
		Type:      ChunkFunction,
		Name:      "TestFunction",
		Content:   "func TestFunction() {}",
		StartLine: 1,
		EndLine:   3,
		Language:  "go",
	}
	embedding := make([]float32, 384)
	for i := range embedding {
		embedding[i] = float32(i) / 384.0
	}

	err = storage.Create(ctx, chunk, embedding)
	if err != nil {
		t.Fatalf("Failed to create chunk: %v", err)
	}

	// Test read
	readChunk, err := storage.Read(ctx, "test-chunk-1")
	if err != nil {
		t.Fatalf("Failed to read chunk: %v", err)
	}

	if readChunk.Name != "TestFunction" {
		t.Errorf("Expected name 'TestFunction', got '%s'", readChunk.Name)
	}

	// Test stats
	stats, err := storage.Stats(ctx)
	if err != nil {
		t.Fatalf("Failed to get stats: %v", err)
	}

	t.Logf("Qdrant stats: %d chunks, %d files", stats.ChunksTotal, stats.FilesIndexed)

	// Test delete
	err = storage.Delete(ctx, "test-chunk-1")
	if err != nil {
		t.Fatalf("Failed to delete chunk: %v", err)
	}

	t.Log("Qdrant integration test passed!")
}
