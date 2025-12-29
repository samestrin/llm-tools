package semantic

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestCohereEmbedder_Embed(t *testing.T) {
	// Mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		if r.Method != "POST" {
			t.Errorf("Expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/v1/embed" {
			t.Errorf("Expected /v1/embed, got %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("Expected Bearer test-key, got %s", r.Header.Get("Authorization"))
		}

		// Parse request body
		var req cohereEmbedRequest
		json.NewDecoder(r.Body).Decode(&req)

		if req.Model != "embed-english-v3.0" {
			t.Errorf("Expected embed-english-v3.0, got %s", req.Model)
		}
		if len(req.Texts) != 1 || req.Texts[0] != "test text" {
			t.Errorf("Expected ['test text'], got %v", req.Texts)
		}

		// Return mock response
		resp := cohereEmbedResponse{
			ID:         "test-id",
			Embeddings: [][]float64{{0.1, 0.2, 0.3, 0.4}},
			Texts:      []string{"test text"},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	embedder, err := NewCohereEmbedder(CohereConfig{
		APIKey:  "test-key",
		Model:   "embed-english-v3.0",
		BaseURL: server.URL,
	})
	if err != nil {
		t.Fatalf("NewCohereEmbedder() error = %v", err)
	}

	embedding, err := embedder.Embed(context.Background(), "test text")
	if err != nil {
		t.Fatalf("Embed() error = %v", err)
	}

	if len(embedding) != 4 {
		t.Errorf("Embed() returned %d dimensions, want 4", len(embedding))
	}
}

func TestCohereEmbedder_EmbedBatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req cohereEmbedRequest
		json.NewDecoder(r.Body).Decode(&req)

		// Return embeddings for each text
		embeddings := make([][]float64, len(req.Texts))
		for i := range req.Texts {
			embeddings[i] = []float64{float64(i) * 0.1, 0.2, 0.3, 0.4}
		}

		resp := cohereEmbedResponse{
			ID:         "test-id",
			Embeddings: embeddings,
			Texts:      req.Texts,
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	embedder, _ := NewCohereEmbedder(CohereConfig{
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

func TestCohereEmbedder_ConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  CohereConfig
		wantErr bool
	}{
		{
			name: "valid config",
			config: CohereConfig{
				APIKey: "test-key",
				Model:  "embed-english-v3.0",
			},
			wantErr: false,
		},
		{
			name: "missing API key",
			config: CohereConfig{
				Model: "embed-english-v3.0",
			},
			wantErr: true,
		},
		{
			name: "default model",
			config: CohereConfig{
				APIKey: "test-key",
			},
			wantErr: false, // Should use default model
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewCohereEmbedder(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewCohereEmbedder() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCohereEmbedder_ErrorHandling(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{
			"message": "invalid api token",
		})
	}))
	defer server.Close()

	embedder, _ := NewCohereEmbedder(CohereConfig{
		APIKey:  "bad-key",
		BaseURL: server.URL,
	})

	_, err := embedder.Embed(context.Background(), "test")
	if err == nil {
		t.Error("Expected error for unauthorized request")
	}
}

func TestCohereEmbedder_InputType(t *testing.T) {
	var receivedInputType string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req cohereEmbedRequest
		json.NewDecoder(r.Body).Decode(&req)
		receivedInputType = req.InputType

		resp := cohereEmbedResponse{
			Embeddings: [][]float64{{0.1, 0.2, 0.3}},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	embedder, _ := NewCohereEmbedder(CohereConfig{
		APIKey:    "test-key",
		BaseURL:   server.URL,
		InputType: "search_query",
	})

	embedder.Embed(context.Background(), "test")

	if receivedInputType != "search_query" {
		t.Errorf("InputType = %s, want search_query", receivedInputType)
	}
}

func TestCohereEmbedder_Dimensions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := cohereEmbedResponse{
			Embeddings: [][]float64{{0.1, 0.2, 0.3, 0.4, 0.5}},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	embedder, _ := NewCohereEmbedder(CohereConfig{
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

func TestCohereEmbedder_FromEnv(t *testing.T) {
	// Skip if env vars are not set
	apiKey := os.Getenv("COHERE_API_KEY")
	if apiKey == "" {
		t.Skip("Skipping: COHERE_API_KEY not set")
	}

	embedder, err := NewCohereEmbedderFromEnv()
	if err != nil {
		t.Fatalf("NewCohereEmbedderFromEnv() error = %v", err)
	}

	if embedder == nil {
		t.Error("Expected non-nil embedder")
	}
}
