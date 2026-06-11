package multireview

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/samestrin/llm-tools/pkg/llmapi"
)

func TestInvokeDirect_Success(t *testing.T) {
	// Mock HTTP server returning valid review with TD_STREAM
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request structure
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/chat/completions") {
			t.Errorf("expected /chat/completions path, got %s", r.URL.Path)
		}

		resp := llmapi.ChatResponse{
			ID:    "test-id",
			Model: "gpt-4o",
			Choices: []llmapi.Choice{{
				Message: llmapi.Message{
					Role:    "assistant",
					Content: "Review: LGTM with minor issues.\n\nTD_STREAM\nMEDIUM|src/main.go:42|Missing error check|Add err != nil check|error-handling",
				},
				FinishReason: "stop",
			}},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	result := InvokeDirect(context.Background(), InvokeDirectParams{
		AgentName: "test-agent",
		AgentConfig: AgentConfig{
			Name:        "test-agent",
			Model:       "gpt-4o",
			Temperature: 0.3,
		},
		APIConfig: &llmapi.APIConfig{
			APIKey:  "test-key",
			BaseURL: server.URL,
			Model:   "gpt-4o",
		},
		TaskMessage: "Review this diff:\n```diff\n+func foo() {}\n```",
		Timeout:     30 * time.Second,
	})

	if result.Status != "ok" {
		t.Errorf("Status = %q, want ok", result.Status)
	}
	if result.AgentName != "test-agent" {
		t.Errorf("AgentName = %q, want test-agent", result.AgentName)
	}
	if result.Error != nil {
		t.Errorf("Error = %v, want nil", result.Error)
	}
	if !strings.Contains(result.ReviewProse, "TD_STREAM") {
		t.Errorf("ReviewProse missing TD_STREAM section")
	}
	if !strings.Contains(result.ReviewProse, "MEDIUM|src/main.go:42") {
		t.Errorf("ReviewProse missing TD entry")
	}
	if result.DurationMS < 0 {
		t.Errorf("DurationMS = %d, want >= 0", result.DurationMS)
	}
}

func TestInvokeDirect_Timeout(t *testing.T) {
	// Server that sleeps longer than timeout
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond)
		resp := llmapi.ChatResponse{
			Choices: []llmapi.Choice{{
				Message: llmapi.Message{Content: "too late"},
			}},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	result := InvokeDirect(ctx, InvokeDirectParams{
		AgentName: "slow-agent",
		AgentConfig: AgentConfig{
			Name:  "slow-agent",
			Model: "gpt-4o",
		},
		APIConfig: &llmapi.APIConfig{
			APIKey:  "test-key",
			BaseURL: server.URL,
			Model:   "gpt-4o",
		},
		TaskMessage: "Review",
		Timeout:     100 * time.Millisecond,
	})

	if result.Status != "timeout" {
		t.Errorf("Status = %q, want timeout", result.Status)
	}
	if result.Error == nil {
		t.Error("Error = nil, want context deadline exceeded")
	}
}

func TestInvokeDirect_APIError401(t *testing.T) {
	// Server returns 401 Unauthorized
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]string{
				"message": "Invalid API key",
				"type":    "invalid_request_error",
			},
		})
	}))
	defer server.Close()

	result := InvokeDirect(context.Background(), InvokeDirectParams{
		AgentName: "auth-fail-agent",
		AgentConfig: AgentConfig{
			Name:  "auth-fail-agent",
			Model: "gpt-4o",
		},
		APIConfig: &llmapi.APIConfig{
			APIKey:  "invalid-key",
			BaseURL: server.URL,
			Model:   "gpt-4o",
		},
		TaskMessage: "Review",
		Timeout:     10 * time.Second,
	})

	if result.Status != "failed" {
		t.Errorf("Status = %q, want failed", result.Status)
	}
	if result.Error == nil {
		t.Error("Error = nil, want error")
	}
}

func TestInvokeDirect_APIError500_Retry(t *testing.T) {
	// Server returns 500 first, then succeeds
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount < 2 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		resp := llmapi.ChatResponse{
			Choices: []llmapi.Choice{{
				Message: llmapi.Message{Content: "Review: OK"},
			}},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	result := InvokeDirect(context.Background(), InvokeDirectParams{
		AgentName: "retry-agent",
		AgentConfig: AgentConfig{
			Name:  "retry-agent",
			Model: "gpt-4o",
		},
		APIConfig: &llmapi.APIConfig{
			APIKey:  "test-key",
			BaseURL: server.URL,
			Model:   "gpt-4o",
		},
		TaskMessage: "Review",
		Timeout:     30 * time.Second,
	})

	// Should succeed after retry
	if result.Status != "ok" {
		t.Errorf("Status = %q, want ok (after retry)", result.Status)
	}
	if callCount < 2 {
		t.Errorf("callCount = %d, expected >= 2 (retry)", callCount)
	}
}

func TestInvokeDirect_MalformedJSON(t *testing.T) {
	// Server returns invalid JSON
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not valid json {"))
	}))
	defer server.Close()

	result := InvokeDirect(context.Background(), InvokeDirectParams{
		AgentName: "malformed-agent",
		AgentConfig: AgentConfig{
			Name:  "malformed-agent",
			Model: "gpt-4o",
		},
		APIConfig: &llmapi.APIConfig{
			APIKey:  "test-key",
			BaseURL: server.URL,
			Model:   "gpt-4o",
		},
		TaskMessage: "Review",
		Timeout:     10 * time.Second,
	})

	if result.Status != "failed" {
		t.Errorf("Status = %q, want failed", result.Status)
	}
	if result.Error == nil {
		t.Error("Error = nil, want JSON parse error")
	}
}

func TestInvokeDirect_EmptyResponse(t *testing.T) {
	// Server returns valid JSON but no choices
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := llmapi.ChatResponse{
			ID:      "test-id",
			Model:   "gpt-4o",
			Choices: []llmapi.Choice{}, // Empty
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	result := InvokeDirect(context.Background(), InvokeDirectParams{
		AgentName: "empty-agent",
		AgentConfig: AgentConfig{
			Name:  "empty-agent",
			Model: "gpt-4o",
		},
		APIConfig: &llmapi.APIConfig{
			APIKey:  "test-key",
			BaseURL: server.URL,
			Model:   "gpt-4o",
		},
		TaskMessage: "Review",
		Timeout:     10 * time.Second,
	})

	if result.Status != "failed" {
		t.Errorf("Status = %q, want failed (no choices)", result.Status)
	}
}

func TestInvokeDirect_ToInvokeReviewerResult(t *testing.T) {
	result := InvokeDirectResult{
		AgentName:   "test",
		Model:       "gpt-4o",
		Status:      "ok",
		DurationMS:  1234,
		ReviewProse: "Review content",
		Error:       nil,
	}

	converted := result.ToInvokeReviewerResult()

	if converted.AgentName != "test" {
		t.Errorf("AgentName = %q", converted.AgentName)
	}
	if converted.Model != "gpt-4o" {
		t.Errorf("Model = %q", converted.Model)
	}
	if converted.Status != "ok" {
		t.Errorf("Status = %q", converted.Status)
	}
	if converted.DurationMS != 1234 {
		t.Errorf("DurationMS = %d", converted.DurationMS)
	}
	if converted.ReviewProse != "Review content" {
		t.Errorf("ReviewProse = %q", converted.ReviewProse)
	}
	if converted.Aborted {
		t.Error("Aborted = true, want false for ok status")
	}
}

func TestInvokeDirect_ToInvokeReviewerResult_Timeout(t *testing.T) {
	result := InvokeDirectResult{
		AgentName: "test",
		Status:    "timeout",
	}

	converted := result.ToInvokeReviewerResult()

	if !converted.Aborted {
		t.Error("Aborted = false, want true for timeout status")
	}
}

func TestInvokeDirect_SystemPromptIncluded(t *testing.T) {
	var receivedMessages []llmapi.Message

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req llmapi.ChatRequest
		json.NewDecoder(r.Body).Decode(&req)
		receivedMessages = req.Messages

		resp := llmapi.ChatResponse{
			Choices: []llmapi.Choice{{
				Message: llmapi.Message{Content: "Review: OK"},
			}},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	InvokeDirect(context.Background(), InvokeDirectParams{
		AgentName: "system-prompt-agent",
		AgentConfig: AgentConfig{
			Name:         "system-prompt-agent",
			Model:        "gpt-4o",
			SystemPrompt: "You are a meticulous code reviewer.",
		},
		APIConfig: &llmapi.APIConfig{
			APIKey:  "test-key",
			BaseURL: server.URL,
			Model:   "gpt-4o",
		},
		TaskMessage: "Review this code",
		Timeout:     10 * time.Second,
	})

	if len(receivedMessages) < 2 {
		t.Fatalf("expected at least 2 messages (system + user), got %d", len(receivedMessages))
	}

	// First message should be system
	if receivedMessages[0].Role != "system" {
		t.Errorf("first message role = %q, want system", receivedMessages[0].Role)
	}
	if receivedMessages[0].Content != "You are a meticulous code reviewer." {
		t.Errorf("system message content = %q", receivedMessages[0].Content)
	}

	// Second message should be user
	if receivedMessages[1].Role != "user" {
		t.Errorf("second message role = %q, want user", receivedMessages[1].Role)
	}
	if receivedMessages[1].Content != "Review this code" {
		t.Errorf("user message content = %q", receivedMessages[1].Content)
	}
}

// Adversarial tests

func TestInvokeDirect_ContextCancellation(t *testing.T) {
	// Server that responds slowly
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		resp := llmapi.ChatResponse{
			Choices: []llmapi.Choice{{
				Message: llmapi.Message{Content: "Review: OK"},
			}},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())

	// Cancel after 100ms
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	result := InvokeDirect(ctx, InvokeDirectParams{
		AgentName: "cancel-agent",
		AgentConfig: AgentConfig{
			Name:  "cancel-agent",
			Model: "gpt-4o",
		},
		APIConfig: &llmapi.APIConfig{
			APIKey:  "test-key",
			BaseURL: server.URL,
			Model:   "gpt-4o",
		},
		TaskMessage: "Review",
		Timeout:     0, // No additional timeout
	})

	// Should detect cancellation as timeout (context cancelled)
	if result.Status != "timeout" {
		t.Errorf("Status = %q, want timeout", result.Status)
	}
	if result.Error == nil {
		t.Error("Error = nil, want context cancelled")
	}
}

func TestInvokeDirect_RateLimited429_Retry(t *testing.T) {
	// Server returns 429 first two times, then succeeds
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount < 3 {
			w.WriteHeader(http.StatusTooManyRequests)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": map[string]string{
					"message": "Rate limit exceeded",
					"type":    "rate_limit_error",
				},
			})
			return
		}
		resp := llmapi.ChatResponse{
			Choices: []llmapi.Choice{{
				Message: llmapi.Message{Content: "Review: OK after rate limit"},
			}},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	result := InvokeDirect(context.Background(), InvokeDirectParams{
		AgentName: "ratelimit-agent",
		AgentConfig: AgentConfig{
			Name:  "ratelimit-agent",
			Model: "gpt-4o",
		},
		APIConfig: &llmapi.APIConfig{
			APIKey:  "test-key",
			BaseURL: server.URL,
			Model:   "gpt-4o",
		},
		TaskMessage: "Review",
		Timeout:     30 * time.Second,
	})

	// Should succeed after retries
	if result.Status != "ok" {
		t.Errorf("Status = %q, want ok (after rate limit retries)", result.Status)
	}
	if callCount < 3 {
		t.Errorf("callCount = %d, expected >= 3 (rate limit retries)", callCount)
	}
	if !strings.Contains(result.ReviewProse, "Review: OK") {
		t.Errorf("ReviewProse = %q, expected success message", result.ReviewProse)
	}
}

func TestInvokeDirect_LargeResponse(t *testing.T) {
	// Server returns a large response (100KB+)
	largeContent := strings.Repeat("This is a test line with some code review content. ", 2000)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := llmapi.ChatResponse{
			Choices: []llmapi.Choice{{
				Message: llmapi.Message{Content: largeContent + "\n\nTD_STREAM\nHIGH|src/main.go:100|Critical issue|Fix it|security"},
			}},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	result := InvokeDirect(context.Background(), InvokeDirectParams{
		AgentName: "large-response-agent",
		AgentConfig: AgentConfig{
			Name:  "large-response-agent",
			Model: "gpt-4o",
		},
		APIConfig: &llmapi.APIConfig{
			APIKey:  "test-key",
			BaseURL: server.URL,
			Model:   "gpt-4o",
		},
		TaskMessage: "Review large codebase",
		Timeout:     30 * time.Second,
	})

	if result.Status != "ok" {
		t.Errorf("Status = %q, want ok", result.Status)
	}
	// Should contain full content including TD_STREAM
	if !strings.Contains(result.ReviewProse, "TD_STREAM") {
		t.Error("ReviewProse missing TD_STREAM section in large response")
	}
	if len(result.ReviewProse) < 100000 {
		t.Errorf("ReviewProse length = %d, expected > 100000", len(result.ReviewProse))
	}
}

func TestNewDirectClient_NoHTTPClientTimeout(t *testing.T) {
	client := newDirectClient(InvokeDirectParams{
		AgentName: "budget-agent",
		AgentConfig: AgentConfig{
			Name:        "budget-agent",
			Model:       "gpt-4o",
			Temperature: 0.3,
		},
		APIConfig: &llmapi.APIConfig{
			APIKey:  "test-key",
			BaseURL: "http://example.invalid",
			Model:   "gpt-4o",
		},
		Timeout: 5 * time.Second,
	})

	// The per-agent budget governs via the context deadline; a non-zero
	// http.Client.Timeout would silently cap every attempt below the budget
	// (the 120s default killed all 12 agents in the observed fan-out).
	if client.HTTPClient.Timeout != 0 {
		t.Errorf("HTTPClient.Timeout = %v, want 0 (per-agent context deadline governs)", client.HTTPClient.Timeout)
	}
}

func TestInvokeDirect_UsesStreaming(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req llmapi.ChatRequest
		json.NewDecoder(r.Body).Decode(&req)
		if !req.Stream {
			t.Error("expected stream: true in review request body")
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher := w.(http.Flusher)
		for _, part := range []string{"Review: ", "streamed ", "findings"} {
			b, _ := json.Marshal(map[string]interface{}{
				"choices": []map[string]interface{}{
					{"delta": map[string]string{"content": part}},
				},
			})
			w.Write([]byte("data: " + string(b) + "\n\n"))
			flusher.Flush()
		}
		w.Write([]byte("data: [DONE]\n\n"))
		flusher.Flush()
	}))
	defer server.Close()

	result := InvokeDirect(context.Background(), InvokeDirectParams{
		AgentName: "stream-agent",
		AgentConfig: AgentConfig{
			Name:  "stream-agent",
			Model: "gpt-4o",
		},
		APIConfig: &llmapi.APIConfig{
			APIKey:  "test-key",
			BaseURL: server.URL,
			Model:   "gpt-4o",
		},
		TaskMessage: "Review this diff",
		Timeout:     10 * time.Second,
		IdleTimeout: 2 * time.Second,
	})

	if result.Status != "ok" {
		t.Fatalf("Status = %q (error: %v), want ok", result.Status, result.Error)
	}
	if result.ReviewProse != "Review: streamed findings" {
		t.Errorf("ReviewProse = %q, want assembled stream content", result.ReviewProse)
	}
}

func TestInvokeDirect_StreamOutlastsIdleWindow(t *testing.T) {
	// Scaled AC3 scenario: the generation takes longer than the idle window
	// in total, but chunks keep arriving — the agent must complete within
	// its per-agent budget instead of being killed by a whole-response cap.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher := w.(http.Flusher)
		for i := 0; i < 8; i++ {
			time.Sleep(50 * time.Millisecond)
			b, _ := json.Marshal(map[string]interface{}{
				"choices": []map[string]interface{}{
					{"delta": map[string]string{"content": "x"}},
				},
			})
			w.Write([]byte("data: " + string(b) + "\n\n"))
			flusher.Flush()
		}
		w.Write([]byte("data: [DONE]\n\n"))
		flusher.Flush()
	}))
	defer server.Close()

	result := InvokeDirect(context.Background(), InvokeDirectParams{
		AgentName: "slow-stream-agent",
		AgentConfig: AgentConfig{
			Name:  "slow-stream-agent",
			Model: "gpt-4o",
		},
		APIConfig: &llmapi.APIConfig{
			APIKey:  "test-key",
			BaseURL: server.URL,
			Model:   "gpt-4o",
		},
		TaskMessage: "Review",
		Timeout:     5 * time.Second,
		IdleTimeout: 200 * time.Millisecond, // total stream ~400ms > idle window
	})

	if result.Status != "ok" {
		t.Fatalf("Status = %q (error: %v), want ok", result.Status, result.Error)
	}
	if result.ReviewProse != "xxxxxxxx" {
		t.Errorf("ReviewProse = %q, want 8 x's", result.ReviewProse)
	}
}

func TestInvokeDirect_StalledStream_TimesOutByIdle(t *testing.T) {
	done := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher := w.(http.Flusher)
		b, _ := json.Marshal(map[string]interface{}{
			"choices": []map[string]interface{}{
				{"delta": map[string]string{"content": "stuck"}},
			},
		})
		w.Write([]byte("data: " + string(b) + "\n\n"))
		flusher.Flush()
		<-done // stall forever
	}))
	defer server.Close()
	defer close(done)

	start := time.Now()
	result := InvokeDirect(context.Background(), InvokeDirectParams{
		AgentName: "stalled-agent",
		AgentConfig: AgentConfig{
			Name:  "stalled-agent",
			Model: "gpt-4o",
		},
		APIConfig: &llmapi.APIConfig{
			APIKey:  "test-key",
			BaseURL: server.URL,
			Model:   "gpt-4o",
		},
		TaskMessage: "Review",
		Timeout:     30 * time.Second,
		IdleTimeout: 150 * time.Millisecond,
	})
	elapsed := time.Since(start)

	if result.Status != "timeout" {
		t.Errorf("Status = %q, want timeout (idle stall)", result.Status)
	}
	if elapsed > 5*time.Second {
		t.Errorf("stalled stream took %v to fail, want ~150ms", elapsed)
	}
}
