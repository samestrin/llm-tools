package llmapi

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// sseChunkBody builds one SSE data line carrying a delta content fragment.
func sseChunkBody(content string) string {
	b, _ := json.Marshal(map[string]interface{}{
		"choices": []map[string]interface{}{
			{"delta": map[string]string{"content": content}},
		},
	})
	return "data: " + string(b) + "\n\n"
}

// sseHandler writes SSE headers and returns a flusher.
func sseHandler(t *testing.T, w http.ResponseWriter) http.Flusher {
	t.Helper()
	w.Header().Set("Content-Type", "text/event-stream")
	w.WriteHeader(http.StatusOK)
	flusher, ok := w.(http.Flusher)
	if !ok {
		t.Fatal("response writer does not support flushing")
	}
	return flusher
}

func TestLLMClient_Stream_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req ChatRequest
		json.Unmarshal(body, &req)
		if !req.Stream {
			t.Error("expected stream: true in request body")
		}

		flusher := sseHandler(t, w)
		for _, part := range []string{"Hello", ", ", "world"} {
			fmt.Fprint(w, sseChunkBody(part))
			flusher.Flush()
		}
		fmt.Fprint(w, "data: [DONE]\n\n")
		flusher.Flush()
	}))
	defer server.Close()

	client := NewLLMClient("test-key", server.URL, "test-model")
	result, err := client.CompleteMessagesStream(t.Context(), []Message{{Role: "user", Content: "hi"}}, 5*time.Second)

	if err != nil {
		t.Fatalf("CompleteMessagesStream failed: %v", err)
	}
	if result != "Hello, world" {
		t.Errorf("result = %q, want \"Hello, world\"", result)
	}
}

func TestLLMClient_Stream_TotalLongerThanIdleWindow(t *testing.T) {
	// Total stream duration (~600ms) exceeds the idle window (250ms), but
	// every inter-chunk gap stays inside it: the stream must complete. This
	// is the scaled equivalent of AC3's "streaming slowly past the old 120s
	// mark completes within the per-agent budget".
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		flusher := sseHandler(t, w)
		for i := 0; i < 10; i++ {
			time.Sleep(60 * time.Millisecond)
			fmt.Fprint(w, sseChunkBody("x"))
			flusher.Flush()
		}
		fmt.Fprint(w, "data: [DONE]\n\n")
		flusher.Flush()
	}))
	defer server.Close()

	client := NewLLMClient("test-key", server.URL, "test-model")
	result, err := client.CompleteMessagesStream(t.Context(), []Message{{Role: "user", Content: "hi"}}, 250*time.Millisecond)

	if err != nil {
		t.Fatalf("CompleteMessagesStream failed: %v", err)
	}
	if result != strings.Repeat("x", 10) {
		t.Errorf("result = %q, want 10 x's", result)
	}
}

func TestLLMClient_Stream_IdleTimeout_KillsStalledStream(t *testing.T) {
	var mu sync.Mutex
	callCount := 0
	done := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		callCount++
		mu.Unlock()
		flusher := sseHandler(t, w)
		fmt.Fprint(w, sseChunkBody("partial"))
		flusher.Flush()
		<-done // stall forever
	}))
	defer server.Close()
	defer close(done)

	client := NewLLMClient("test-key", server.URL, "test-model")
	client.RetryDelay = 10 * time.Millisecond

	start := time.Now()
	_, err := client.CompleteMessagesStream(t.Context(), []Message{{Role: "user", Content: "hi"}}, 150*time.Millisecond)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected idle timeout error for stalled stream")
	}
	if !isTimeoutErr(err) {
		t.Errorf("error %v is not classified as a timeout", err)
	}
	if elapsed > 2*time.Second {
		t.Errorf("stalled stream took %v to fail, want ~150ms idle timeout", elapsed)
	}
	mu.Lock()
	got := callCount
	mu.Unlock()
	if got != 1 {
		t.Errorf("server invoked %d times, want exactly 1 (idle timeout must not be retried)", got)
	}
}

func TestLLMClient_Stream_PlainJSONFallback(t *testing.T) {
	// Some OpenAI-compatible gateways ignore stream:true and return a plain
	// JSON completion; the client must still extract the content.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(ChatResponse{
			Choices: []Choice{{Message: Message{Content: "non-streamed answer"}}},
		})
	}))
	defer server.Close()

	client := NewLLMClient("test-key", server.URL, "test-model")
	result, err := client.CompleteMessagesStream(t.Context(), []Message{{Role: "user", Content: "hi"}}, 5*time.Second)

	if err != nil {
		t.Fatalf("CompleteMessagesStream failed: %v", err)
	}
	if result != "non-streamed answer" {
		t.Errorf("result = %q, want \"non-streamed answer\"", result)
	}
}

func TestLLMClient_Stream_APIError_NonRetryable(t *testing.T) {
	var mu sync.Mutex
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		callCount++
		mu.Unlock()
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]string{"message": "Invalid API key"},
		})
	}))
	defer server.Close()

	client := NewLLMClient("invalid-key", server.URL, "test-model")
	client.RetryDelay = 10 * time.Millisecond

	_, err := client.CompleteMessagesStream(t.Context(), []Message{{Role: "user", Content: "hi"}}, 5*time.Second)

	if err == nil {
		t.Fatal("expected error for 401 response")
	}
	if !strings.Contains(err.Error(), "Invalid API key") {
		t.Errorf("error %v does not name the actual cause", err)
	}
	mu.Lock()
	got := callCount
	mu.Unlock()
	if got != 1 {
		t.Errorf("server invoked %d times, want exactly 1 (401 is not retryable)", got)
	}
}

func TestLLMClient_Stream_RetryableStatusBeforeContent(t *testing.T) {
	var mu sync.Mutex
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		callCount++
		n := callCount
		mu.Unlock()
		if n < 2 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		flusher := sseHandler(t, w)
		fmt.Fprint(w, sseChunkBody("recovered"))
		fmt.Fprint(w, "data: [DONE]\n\n")
		flusher.Flush()
	}))
	defer server.Close()

	client := NewLLMClient("test-key", server.URL, "test-model")
	client.RetryDelay = 10 * time.Millisecond

	result, err := client.CompleteMessagesStream(t.Context(), []Message{{Role: "user", Content: "hi"}}, 5*time.Second)

	if err != nil {
		t.Fatalf("CompleteMessagesStream failed: %v", err)
	}
	if result != "recovered" {
		t.Errorf("result = %q, want \"recovered\"", result)
	}
	mu.Lock()
	got := callCount
	mu.Unlock()
	if got < 2 {
		t.Errorf("server invoked %d times, want >= 2 (503 before content is retryable)", got)
	}
}
