package llmapi

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestLLMClient_Complete_Success(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/chat/completions") {
			t.Errorf("expected path to end with /chat/completions, got %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("expected Authorization header 'Bearer test-key', got %s", r.Header.Get("Authorization"))
		}

		// Verify request body
		body, _ := io.ReadAll(r.Body)
		var req ChatRequest
		json.Unmarshal(body, &req)
		if req.Model != "test-model" {
			t.Errorf("expected model 'test-model', got %s", req.Model)
		}

		// Return mock response
		resp := ChatResponse{
			Choices: []Choice{
				{
					Message: Message{
						Role:    "assistant",
						Content: "Test response content",
					},
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewLLMClient("test-key", server.URL, "test-model")
	result, err := client.Complete("Test prompt", 10*time.Second)

	if err != nil {
		t.Fatalf("Complete failed: %v", err)
	}
	if result != "Test response content" {
		t.Errorf("expected 'Test response content', got %s", result)
	}
}

func TestLLMClient_Complete_APIError(t *testing.T) {
	// Create mock server that returns error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]string{
				"message": "Invalid API key",
			},
		})
	}))
	defer server.Close()

	client := NewLLMClient("invalid-key", server.URL, "test-model")
	_, err := client.Complete("Test prompt", 10*time.Second)

	if err == nil {
		t.Error("expected error for 401 response")
	}
}

func TestLLMClient_Complete_Timeout(t *testing.T) {
	// Create slow mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		json.NewEncoder(w).Encode(ChatResponse{})
	}))
	defer server.Close()

	client := NewLLMClient("test-key", server.URL, "test-model")
	_, err := client.Complete("Test prompt", 50*time.Millisecond)

	if err == nil {
		t.Error("expected timeout error")
	}
}

func TestLLMClient_Complete_InvalidJSON(t *testing.T) {
	// Create mock server that returns invalid JSON
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not json"))
	}))
	defer server.Close()

	client := NewLLMClient("test-key", server.URL, "test-model")
	_, err := client.Complete("Test prompt", 10*time.Second)

	if err == nil {
		t.Error("expected error for invalid JSON response")
	}
}

func TestLLMClient_Complete_EmptyResponse(t *testing.T) {
	// Create mock server that returns empty choices
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := ChatResponse{Choices: []Choice{}}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewLLMClient("test-key", server.URL, "test-model")
	_, err := client.Complete("Test prompt", 10*time.Second)

	if err == nil {
		t.Error("expected error for empty choices")
	}
}

func TestNewLLMClient(t *testing.T) {
	client := NewLLMClient("api-key", "https://api.example.com", "model-1")

	if client.APIKey != "api-key" {
		t.Errorf("expected APIKey 'api-key', got %s", client.APIKey)
	}
	if client.BaseURL != "https://api.example.com" {
		t.Errorf("expected BaseURL 'https://api.example.com', got %s", client.BaseURL)
	}
	if client.Model != "model-1" {
		t.Errorf("expected Model 'model-1', got %s", client.Model)
	}
	if client.HTTPClient == nil {
		t.Error("expected HTTPClient to be initialized")
	}
}
