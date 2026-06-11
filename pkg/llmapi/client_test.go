package llmapi

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
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

func TestLLMClient_Timeout_NotRetried(t *testing.T) {
	var mu sync.Mutex
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		callCount++
		mu.Unlock()
		time.Sleep(300 * time.Millisecond) // hang past the per-attempt timeout
		json.NewEncoder(w).Encode(ChatResponse{Choices: []Choice{{Message: Message{Content: "late"}}}})
	}))
	defer server.Close()

	client := NewLLMClient("test-key", server.URL, "test-model")
	client.HTTPClient.Timeout = 50 * time.Millisecond
	client.RetryDelay = 10 * time.Millisecond

	_, err := client.Complete("Test prompt", 5*time.Second)

	if err == nil {
		t.Fatal("expected timeout error")
	}
	mu.Lock()
	got := callCount
	mu.Unlock()
	// A timed-out request re-sends the full prompt and forces the backend to
	// repeat the entire prefill — retrying it is deterministic waste.
	if got != 1 {
		t.Errorf("server invoked %d times, want exactly 1 (timeouts must not be retried)", got)
	}
}

func TestLLMClient_TransientNetworkError_StillRetried(t *testing.T) {
	var mu sync.Mutex
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		callCount++
		n := callCount
		mu.Unlock()
		if n < 2 {
			// Abort the connection without a response: the client sees a
			// non-timeout network error (connection reset), which must
			// remain retryable.
			hj, ok := w.(http.Hijacker)
			if !ok {
				t.Fatal("server does not support hijacking")
			}
			conn, _, _ := hj.Hijack()
			conn.Close()
			return
		}
		json.NewEncoder(w).Encode(ChatResponse{Choices: []Choice{{Message: Message{Content: "recovered"}}}})
	}))
	defer server.Close()

	client := NewLLMClient("test-key", server.URL, "test-model")
	client.RetryDelay = 10 * time.Millisecond

	result, err := client.Complete("Test prompt", 5*time.Second)

	if err != nil {
		t.Fatalf("Complete failed: %v (transient network errors must stay retryable)", err)
	}
	if result != "recovered" {
		t.Errorf("result = %q, want \"recovered\"", result)
	}
	mu.Lock()
	got := callCount
	mu.Unlock()
	if got < 2 {
		t.Errorf("server invoked %d times, want >= 2 (retry after connection reset)", got)
	}
}
