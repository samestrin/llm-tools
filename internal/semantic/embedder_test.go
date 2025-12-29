package semantic

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestEmbedder_Embed(t *testing.T) {
	// Create a mock OpenAI-compatible embedding server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/embeddings" {
			t.Errorf("Expected path /v1/embeddings, got %s", r.URL.Path)
		}

		// Parse request
		body, _ := io.ReadAll(r.Body)
		var req map[string]interface{}
		json.Unmarshal(body, &req)

		// Return mock embedding
		resp := map[string]interface{}{
			"object": "list",
			"data": []map[string]interface{}{
				{
					"object":    "embedding",
					"index":     0,
					"embedding": []float64{0.1, 0.2, 0.3, 0.4},
				},
			},
			"model": "test-model",
			"usage": map[string]interface{}{
				"prompt_tokens": 5,
				"total_tokens":  5,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	cfg := EmbedderConfig{
		APIURL: server.URL,
		Model:  "test-model",
	}

	embedder, err := NewEmbedder(cfg)
	if err != nil {
		t.Fatalf("NewEmbedder() error = %v", err)
	}

	embedding, err := embedder.Embed(context.Background(), "test text")
	if err != nil {
		t.Fatalf("Embed() error = %v", err)
	}

	if len(embedding) != 4 {
		t.Errorf("Embed() returned %d dimensions, want 4", len(embedding))
	}

	expected := []float32{0.1, 0.2, 0.3, 0.4}
	for i, v := range embedding {
		if v != expected[i] {
			t.Errorf("Embed()[%d] = %v, want %v", i, v, expected[i])
		}
	}
}

func TestEmbedder_EmbedBatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req map[string]interface{}
		json.Unmarshal(body, &req)

		// Get input - could be string or array
		inputs, ok := req["input"].([]interface{})
		if !ok {
			// Single input converted to array
			inputs = []interface{}{req["input"]}
		}

		// Return embeddings for each input
		data := make([]map[string]interface{}, len(inputs))
		for i := range inputs {
			data[i] = map[string]interface{}{
				"object":    "embedding",
				"index":     i,
				"embedding": []float64{float64(i) * 0.1, float64(i)*0.1 + 0.1, 0.3, 0.4},
			}
		}

		resp := map[string]interface{}{
			"object": "list",
			"data":   data,
			"model":  "test-model",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	cfg := EmbedderConfig{
		APIURL: server.URL,
		Model:  "test-model",
	}

	embedder, err := NewEmbedder(cfg)
	if err != nil {
		t.Fatalf("NewEmbedder() error = %v", err)
	}

	texts := []string{"text 1", "text 2", "text 3"}
	embeddings, err := embedder.EmbedBatch(context.Background(), texts)
	if err != nil {
		t.Fatalf("EmbedBatch() error = %v", err)
	}

	if len(embeddings) != 3 {
		t.Errorf("EmbedBatch() returned %d embeddings, want 3", len(embeddings))
	}

	for i, emb := range embeddings {
		if len(emb) != 4 {
			t.Errorf("EmbedBatch()[%d] has %d dimensions, want 4", i, len(emb))
		}
	}
}

func TestEmbedderConfig_Defaults(t *testing.T) {
	cfg := DefaultEmbedderConfig()

	if cfg.APIURL == "" {
		t.Error("Default APIURL should not be empty")
	}

	if cfg.Model == "" {
		t.Error("Default Model should not be empty")
	}

	if cfg.Timeout <= 0 {
		t.Error("Default Timeout should be positive")
	}

	if cfg.MaxRetries < 0 {
		t.Error("Default MaxRetries should be non-negative")
	}
}

func TestEmbedderConfig_WithAPIKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader != "Bearer test-api-key" {
			t.Errorf("Expected Authorization header 'Bearer test-api-key', got %q", authHeader)
		}

		resp := map[string]interface{}{
			"object": "list",
			"data": []map[string]interface{}{
				{"object": "embedding", "index": 0, "embedding": []float64{0.1, 0.2, 0.3, 0.4}},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	cfg := EmbedderConfig{
		APIURL: server.URL,
		Model:  "test-model",
		APIKey: "test-api-key",
	}

	embedder, err := NewEmbedder(cfg)
	if err != nil {
		t.Fatalf("NewEmbedder() error = %v", err)
	}

	_, err = embedder.Embed(context.Background(), "test")
	if err != nil {
		t.Fatalf("Embed() error = %v", err)
	}
}

func TestEmbedder_Dimensions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"object": "list",
			"data": []map[string]interface{}{
				{"object": "embedding", "index": 0, "embedding": []float64{0.1, 0.2, 0.3, 0.4}},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	cfg := EmbedderConfig{
		APIURL: server.URL,
		Model:  "test-model",
	}

	embedder, err := NewEmbedder(cfg)
	if err != nil {
		t.Fatalf("NewEmbedder() error = %v", err)
	}

	dims := embedder.Dimensions()
	// Initial dimensions should be 0 until first embedding is retrieved
	// or should reflect model default
	if dims < 0 {
		t.Errorf("Dimensions() = %d, should be non-negative", dims)
	}
}

func TestEmbedder_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": {"message": "server error"}}`))
	}))
	defer server.Close()

	cfg := EmbedderConfig{
		APIURL:     server.URL,
		Model:      "test-model",
		MaxRetries: 0, // No retries for test
	}

	embedder, err := NewEmbedder(cfg)
	if err != nil {
		t.Fatalf("NewEmbedder() error = %v", err)
	}

	_, err = embedder.Embed(context.Background(), "test")
	if err == nil {
		t.Error("Expected error for server error response")
	}
}

func TestEmbedder_InvalidURL(t *testing.T) {
	cfg := EmbedderConfig{
		APIURL: "http://invalid-host-that-does-not-exist:12345",
		Model:  "test-model",
	}

	embedder, err := NewEmbedder(cfg)
	if err != nil {
		t.Fatalf("NewEmbedder() error = %v", err)
	}

	ctx := context.Background()
	_, err = embedder.Embed(ctx, "test")
	if err == nil {
		t.Error("Expected error for invalid URL")
	}
}

func TestEmbedder_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate slow response - but we'll cancel before it responds
		select {}
	}))
	defer server.Close()

	cfg := EmbedderConfig{
		APIURL: server.URL,
		Model:  "test-model",
	}

	embedder, err := NewEmbedder(cfg)
	if err != nil {
		t.Fatalf("NewEmbedder() error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err = embedder.Embed(ctx, "test")
	if err == nil {
		t.Error("Expected error for cancelled context")
	}
}
