package semantic

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestOpenRouterEmbedder_Embed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		if r.Method != "POST" {
			t.Errorf("Expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/v1/embeddings" {
			t.Errorf("Expected /api/v1/embeddings, got %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("Expected Bearer test-key, got %s", r.Header.Get("Authorization"))
		}

		// Parse and verify request body
		var req openRouterEmbedRequest
		json.NewDecoder(r.Body).Decode(&req)

		if req.Model != "mistralai/codestral-embed-2505" {
			t.Errorf("Expected mistralai/codestral-embed-2505, got %s", req.Model)
		}
		if req.EncodingFormat != "float" {
			t.Errorf("Expected encoding_format=float, got %s", req.EncodingFormat)
		}

		// Return OpenAI-compatible response
		resp := openRouterEmbedResponse{
			Object: "list",
			Data: []openRouterEmbedData{
				{
					Object:    "embedding",
					Index:     0,
					Embedding: []float64{0.1, 0.2, 0.3, 0.4},
				},
			},
			Model: "mistralai/codestral-embed-2505",
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	embedder, err := NewOpenRouterEmbedder(OpenRouterConfig{
		APIKey:  "test-key",
		Model:   "mistralai/codestral-embed-2505",
		BaseURL: server.URL,
	})
	if err != nil {
		t.Fatalf("NewOpenRouterEmbedder() error = %v", err)
	}

	embedding, err := embedder.Embed(context.Background(), "test text")
	if err != nil {
		t.Fatalf("Embed() error = %v", err)
	}

	if len(embedding) != 4 {
		t.Errorf("Embed() returned %d dimensions, want 4", len(embedding))
	}
}

func TestOpenRouterEmbedder_EmbedBatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req openRouterEmbedRequest
		json.NewDecoder(r.Body).Decode(&req)

		// Return embeddings for each input
		inputs, ok := req.Input.([]interface{})
		if !ok {
			// Single input
			resp := openRouterEmbedResponse{
				Object: "list",
				Data: []openRouterEmbedData{
					{Object: "embedding", Index: 0, Embedding: []float64{0.1, 0.2, 0.3, 0.4}},
				},
			}
			json.NewEncoder(w).Encode(resp)
			return
		}

		// Batch input
		data := make([]openRouterEmbedData, len(inputs))
		for i := range inputs {
			data[i] = openRouterEmbedData{
				Object:    "embedding",
				Index:     i,
				Embedding: []float64{float64(i) * 0.1, 0.2, 0.3, 0.4},
			}
		}

		resp := openRouterEmbedResponse{
			Object: "list",
			Data:   data,
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	embedder, _ := NewOpenRouterEmbedder(OpenRouterConfig{
		APIKey:  "test-key",
		BaseURL: server.URL,
	})

	texts := []string{"text1", "text2", "text3"}
	embeddings, err := embedder.EmbedBatch(context.Background(), texts)
	if err != nil {
		t.Fatalf("EmbedBatch() error = %v", err)
	}

	if len(embeddings) != 3 {
		t.Errorf("EmbedBatch() returned %d embeddings, want 3", len(embeddings))
	}
}

func TestOpenRouterEmbedder_ConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  OpenRouterConfig
		wantErr bool
	}{
		{
			name: "valid config",
			config: OpenRouterConfig{
				APIKey: "test-key",
				Model:  "mistralai/codestral-embed-2505",
			},
			wantErr: false,
		},
		{
			name: "missing API key",
			config: OpenRouterConfig{
				Model: "mistralai/codestral-embed-2505",
			},
			wantErr: true,
		},
		{
			name: "default model applied",
			config: OpenRouterConfig{
				APIKey: "test-key",
			},
			wantErr: false, // Should use default model
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewOpenRouterEmbedder(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewOpenRouterEmbedder() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestOpenRouterEmbedder_ErrorHandling(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]string{
				"message": "Invalid API key",
				"type":    "authentication_error",
			},
		})
	}))
	defer server.Close()

	embedder, _ := NewOpenRouterEmbedder(OpenRouterConfig{
		APIKey:  "bad-key",
		BaseURL: server.URL,
	})

	_, err := embedder.Embed(context.Background(), "test")
	if err == nil {
		t.Error("Expected error for unauthorized request")
	}
}

func TestOpenRouterEmbedder_RateLimiting(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount <= 2 {
			// First two calls return rate limit error
			w.WriteHeader(http.StatusTooManyRequests)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": map[string]string{
					"message": "Rate limit exceeded",
					"type":    "rate_limit_error",
				},
			})
			return
		}
		// Third call succeeds
		resp := openRouterEmbedResponse{
			Object: "list",
			Data: []openRouterEmbedData{
				{Object: "embedding", Index: 0, Embedding: []float64{0.1, 0.2, 0.3}},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	embedder, _ := NewOpenRouterEmbedder(OpenRouterConfig{
		APIKey:     "test-key",
		BaseURL:    server.URL,
		MaxRetries: 3,
	})

	_, err := embedder.Embed(context.Background(), "test")
	if err != nil {
		t.Errorf("Expected success after retry, got error: %v", err)
	}

	if callCount != 3 {
		t.Errorf("Expected 3 calls (2 retries + success), got %d", callCount)
	}
}

func TestOpenRouterEmbedder_Dimensions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := openRouterEmbedResponse{
			Object: "list",
			Data: []openRouterEmbedData{
				{Object: "embedding", Index: 0, Embedding: []float64{0.1, 0.2, 0.3, 0.4, 0.5}},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	embedder, _ := NewOpenRouterEmbedder(OpenRouterConfig{
		APIKey:  "test-key",
		BaseURL: server.URL,
	})

	// Dimensions should be 0 before first call
	if embedder.Dimensions() != 0 {
		t.Errorf("Initial Dimensions() = %d, want 0", embedder.Dimensions())
	}

	embedder.Embed(context.Background(), "test")

	// Dimensions should be set after first call
	if embedder.Dimensions() != 5 {
		t.Errorf("Dimensions() after embed = %d, want 5", embedder.Dimensions())
	}
}

func TestOpenRouterEmbedder_FromEnv(t *testing.T) {
	apiKey := os.Getenv("OPENROUTER_API_KEY")
	if apiKey == "" {
		t.Skip("Skipping: OPENROUTER_API_KEY not set")
	}

	embedder, err := NewOpenRouterEmbedderFromEnv()
	if err != nil {
		t.Fatalf("NewOpenRouterEmbedderFromEnv() error = %v", err)
	}

	if embedder == nil {
		t.Error("Expected non-nil embedder")
	}
}

func TestOpenRouterEmbedder_DefaultModel(t *testing.T) {
	embedder, err := NewOpenRouterEmbedder(OpenRouterConfig{
		APIKey: "test-key",
	})
	if err != nil {
		t.Fatalf("NewOpenRouterEmbedder() error = %v", err)
	}

	if embedder.Model() != "mistralai/codestral-embed-2505" {
		t.Errorf("Model() = %s, want mistralai/codestral-embed-2505", embedder.Model())
	}
}
