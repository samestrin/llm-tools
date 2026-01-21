package semantic

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewReranker_RequiresAPIURL(t *testing.T) {
	cfg := RerankerConfig{
		APIURL: "",
		Model:  "test-model",
	}

	_, err := NewReranker(cfg)
	if err == nil {
		t.Error("NewReranker() should return error when APIURL is empty")
	}
}

func TestNewReranker_DefaultModel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(rerankResponse{
			Results: []rerankResult{},
		})
	}))
	defer server.Close()

	cfg := RerankerConfig{
		APIURL: server.URL,
		Model:  "", // Empty - should get default
	}

	reranker, err := NewReranker(cfg)
	if err != nil {
		t.Fatalf("NewReranker() error = %v", err)
	}

	// Should get default model
	if reranker.Model() != "Qwen/Qwen3-Reranker-0.6B" {
		t.Errorf("NewReranker() default model = %s, want Qwen/Qwen3-Reranker-0.6B", reranker.Model())
	}
}

func TestReranker_Rerank(t *testing.T) {
	// Create a mock server that returns expected reranker response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request path
		if r.URL.Path != "/v1/rerank" {
			t.Errorf("Expected path /v1/rerank, got %s", r.URL.Path)
		}

		// Verify method
		if r.Method != "POST" {
			t.Errorf("Expected POST method, got %s", r.Method)
		}

		// Verify content type
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Expected Content-Type application/json, got %s", r.Header.Get("Content-Type"))
		}

		// Decode request
		var req rerankRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("Failed to decode request: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// Verify request content
		if req.Query != "test query" {
			t.Errorf("Expected query 'test query', got %s", req.Query)
		}
		if len(req.Documents) != 3 {
			t.Errorf("Expected 3 documents, got %d", len(req.Documents))
		}

		// Return mock response
		// Scores in original document order
		response := rerankResponse{
			Results: []rerankResult{
				{Index: 0, RelevanceScore: 0.5},
				{Index: 1, RelevanceScore: 0.9}, // Highest
				{Index: 2, RelevanceScore: 0.3},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	cfg := RerankerConfig{
		APIURL:  server.URL,
		Model:   "test-model",
		Timeout: 10 * time.Second,
	}

	reranker, err := NewReranker(cfg)
	if err != nil {
		t.Fatalf("NewReranker() error = %v", err)
	}

	documents := []string{"doc1", "doc2", "doc3"}
	scores, err := reranker.Rerank(context.Background(), "test query", documents)
	if err != nil {
		t.Fatalf("Rerank() error = %v", err)
	}

	if len(scores) != 3 {
		t.Fatalf("Rerank() returned %d scores, want 3", len(scores))
	}

	// Verify scores are in original document order
	if scores[0] != 0.5 {
		t.Errorf("scores[0] = %f, want 0.5", scores[0])
	}
	if scores[1] != 0.9 {
		t.Errorf("scores[1] = %f, want 0.9", scores[1])
	}
	if scores[2] != 0.3 {
		t.Errorf("scores[2] = %f, want 0.3", scores[2])
	}
}

func TestReranker_EmptyDocuments(t *testing.T) {
	cfg := RerankerConfig{
		APIURL: "http://example.com", // Won't be called
		Model:  "test-model",
	}

	reranker, err := NewReranker(cfg)
	if err != nil {
		t.Fatalf("NewReranker() error = %v", err)
	}

	scores, err := reranker.Rerank(context.Background(), "query", []string{})
	if err != nil {
		t.Fatalf("Rerank() error = %v", err)
	}

	if scores != nil {
		t.Errorf("Rerank() with empty documents should return nil, got %v", scores)
	}
}

func TestReranker_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
	}))
	defer server.Close()

	cfg := RerankerConfig{
		APIURL:     server.URL,
		Model:      "test-model",
		Timeout:    10 * time.Second,
		MaxRetries: 0, // No retries for faster test
	}

	reranker, err := NewReranker(cfg)
	if err != nil {
		t.Fatalf("NewReranker() error = %v", err)
	}

	_, err = reranker.Rerank(context.Background(), "query", []string{"doc1"})
	if err == nil {
		t.Error("Rerank() should return error on API failure")
	}
}

func TestReranker_AuthorizationHeader(t *testing.T) {
	receivedAuth := ""
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(rerankResponse{
			Results: []rerankResult{{Index: 0, RelevanceScore: 0.5}},
		})
	}))
	defer server.Close()

	cfg := RerankerConfig{
		APIURL: server.URL,
		Model:  "test-model",
		APIKey: "test-api-key",
	}

	reranker, err := NewReranker(cfg)
	if err != nil {
		t.Fatalf("NewReranker() error = %v", err)
	}

	_, err = reranker.Rerank(context.Background(), "query", []string{"doc1"})
	if err != nil {
		t.Fatalf("Rerank() error = %v", err)
	}

	if receivedAuth != "Bearer test-api-key" {
		t.Errorf("Authorization header = %s, want 'Bearer test-api-key'", receivedAuth)
	}
}

func TestDefaultRerankerConfig(t *testing.T) {
	cfg := DefaultRerankerConfig()

	if cfg.Model != "Qwen/Qwen3-Reranker-0.6B" {
		t.Errorf("DefaultRerankerConfig().Model = %s, want Qwen/Qwen3-Reranker-0.6B", cfg.Model)
	}
	if cfg.Timeout != 60*time.Second {
		t.Errorf("DefaultRerankerConfig().Timeout = %v, want 60s", cfg.Timeout)
	}
	if cfg.MaxRetries != 3 {
		t.Errorf("DefaultRerankerConfig().MaxRetries = %d, want 3", cfg.MaxRetries)
	}
	if cfg.Instruction == "" {
		t.Error("DefaultRerankerConfig().Instruction should not be empty")
	}
}
