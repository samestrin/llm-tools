package semantic

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestHuggingFaceEmbedder_Embed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		if r.Method != "POST" {
			t.Errorf("Expected POST, got %s", r.Method)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("Expected Bearer test-key, got %s", r.Header.Get("Authorization"))
		}

		// Return mock embedding (HF returns [[embedding]] for single input)
		resp := [][]float64{{0.1, 0.2, 0.3, 0.4}}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	embedder, err := NewHuggingFaceEmbedder(HuggingFaceConfig{
		APIKey:  "test-key",
		Model:   "sentence-transformers/all-MiniLM-L6-v2",
		BaseURL: server.URL,
	})
	if err != nil {
		t.Fatalf("NewHuggingFaceEmbedder() error = %v", err)
	}

	embedding, err := embedder.Embed(context.Background(), "test text")
	if err != nil {
		t.Fatalf("Embed() error = %v", err)
	}

	if len(embedding) != 4 {
		t.Errorf("Embed() returned %d dimensions, want 4", len(embedding))
	}
}

func TestHuggingFaceEmbedder_EmbedBatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Parse request
		var req huggingFaceRequest
		json.NewDecoder(r.Body).Decode(&req)

		// Return embeddings for each input
		var embeddings [][]float64
		switch inputs := req.Inputs.(type) {
		case []interface{}:
			for i := range inputs {
				embeddings = append(embeddings, []float64{float64(i) * 0.1, 0.2, 0.3, 0.4})
			}
		case string:
			embeddings = [][]float64{{0.1, 0.2, 0.3, 0.4}}
		}

		json.NewEncoder(w).Encode(embeddings)
	}))
	defer server.Close()

	embedder, _ := NewHuggingFaceEmbedder(HuggingFaceConfig{
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

func TestHuggingFaceEmbedder_ConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  HuggingFaceConfig
		wantErr bool
	}{
		{
			name: "valid config",
			config: HuggingFaceConfig{
				APIKey: "test-key",
				Model:  "sentence-transformers/all-MiniLM-L6-v2",
			},
			wantErr: false,
		},
		{
			name: "missing API key",
			config: HuggingFaceConfig{
				Model: "sentence-transformers/all-MiniLM-L6-v2",
			},
			wantErr: true,
		},
		{
			name: "default model",
			config: HuggingFaceConfig{
				APIKey: "test-key",
			},
			wantErr: false, // Should use default model
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewHuggingFaceEmbedder(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewHuggingFaceEmbedder() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestHuggingFaceEmbedder_ErrorHandling(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Invalid token",
		})
	}))
	defer server.Close()

	embedder, _ := NewHuggingFaceEmbedder(HuggingFaceConfig{
		APIKey:  "bad-key",
		BaseURL: server.URL,
	})

	_, err := embedder.Embed(context.Background(), "test")
	if err == nil {
		t.Error("Expected error for unauthorized request")
	}
}

func TestHuggingFaceEmbedder_ModelLoading(t *testing.T) {
	// Simulate model loading response (503 with estimated time)
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			// First call returns loading status
			w.WriteHeader(http.StatusServiceUnavailable)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error":          "Model is loading",
				"estimated_time": 20.0,
			})
			return
		}
		// Subsequent calls succeed
		resp := [][]float64{{0.1, 0.2, 0.3}}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	embedder, _ := NewHuggingFaceEmbedder(HuggingFaceConfig{
		APIKey:       "test-key",
		BaseURL:      server.URL,
		WaitForModel: true,
	})

	// This should retry and eventually succeed
	_, err := embedder.Embed(context.Background(), "test")
	if err != nil {
		t.Errorf("Expected success after retry, got error: %v", err)
	}
}

func TestHuggingFaceEmbedder_Dimensions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := [][]float64{{0.1, 0.2, 0.3, 0.4, 0.5}}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	embedder, _ := NewHuggingFaceEmbedder(HuggingFaceConfig{
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

func TestHuggingFaceEmbedder_FromEnv(t *testing.T) {
	apiKey := os.Getenv("HUGGING_FACE_API_KEY")
	if apiKey == "" {
		apiKey = os.Getenv("HF_TOKEN")
	}
	if apiKey == "" {
		t.Skip("Skipping: HUGGING_FACE_API_KEY or HF_TOKEN not set")
	}

	embedder, err := NewHuggingFaceEmbedderFromEnv()
	if err != nil {
		t.Fatalf("NewHuggingFaceEmbedderFromEnv() error = %v", err)
	}

	if embedder == nil {
		t.Error("Expected non-nil embedder")
	}
}

func TestHuggingFaceEmbedder_NestedResponse(t *testing.T) {
	// Some HF models return nested arrays [[[embedding]]]
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return a 3D array (tokens x dimensions)
		resp := [][][]float64{{{0.1, 0.2, 0.3, 0.4}}}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	embedder, _ := NewHuggingFaceEmbedder(HuggingFaceConfig{
		APIKey:  "test-key",
		BaseURL: server.URL,
	})

	embedding, err := embedder.Embed(context.Background(), "test")
	if err != nil {
		t.Fatalf("Embed() error = %v", err)
	}

	if len(embedding) != 4 {
		t.Errorf("Embed() returned %d dimensions, want 4", len(embedding))
	}
}
