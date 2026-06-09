package multireview

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/samestrin/llm-tools/pkg/llmapi"
)

// testRegistry creates an in-memory registry for testing
func testRegistry(serverURL string) *Registry {
	return &Registry{
		Providers: map[string]ProviderConfig{
			"test-provider": {
				Name:      "test-provider",
				APIKeyEnv: "TEST_API_KEY",
				BaseURL:   serverURL,
			},
		},
		Agents: map[string]AgentConfig{
			"agent-a": {
				Name:        "agent-a",
				Provider:    "test-provider",
				Model:       "test-model",
				Temperature: 0.3,
			},
			"agent-b": {
				Name:        "agent-b",
				Provider:    "test-provider",
				Model:       "test-model",
				Temperature: 0.3,
			},
			"agent-c": {
				Name:        "agent-c",
				Provider:    "test-provider",
				Model:       "test-model",
				Temperature: 0.3,
			},
		},
	}
}

func TestFanout_ParallelOnly(t *testing.T) {
	os.Setenv("TEST_API_KEY", "test-key")
	defer os.Unsetenv("TEST_API_KEY")

	// Track concurrent requests
	var concurrentCount int32
	var maxConcurrent int32
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		current := atomic.AddInt32(&concurrentCount, 1)
		defer atomic.AddInt32(&concurrentCount, -1)

		mu.Lock()
		if current > maxConcurrent {
			maxConcurrent = current
		}
		mu.Unlock()

		// Small delay to allow concurrency
		time.Sleep(50 * time.Millisecond)

		resp := llmapi.ChatResponse{
			Choices: []llmapi.Choice{{
				Message: llmapi.Message{Content: "Review: LGTM"},
			}},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	registry := testRegistry(server.URL)
	tempDir := t.TempDir()

	result, err := Fanout(context.Background(), FanoutParams{
		Registry:       registry,
		ParallelAgents: []string{"agent-a", "agent-b", "agent-c"},
		SerialAgents:   []string{},
		TaskMessage:    "Review this diff",
		GlobalTimeout:  30 * time.Second,
		OutputDir:      tempDir,
	})

	if err != nil {
		t.Fatalf("Fanout error: %v", err)
	}

	// All agents should succeed
	if result.SuccessCount != 3 {
		t.Errorf("SuccessCount = %d, want 3", result.SuccessCount)
	}
	if result.FailedCount != 0 {
		t.Errorf("FailedCount = %d, want 0", result.FailedCount)
	}
	if len(result.Results) != 3 {
		t.Errorf("len(Results) = %d, want 3", len(result.Results))
	}

	// Should have run concurrently
	mu.Lock()
	concurrent := maxConcurrent
	mu.Unlock()
	if concurrent < 2 {
		t.Errorf("maxConcurrent = %d, want >= 2 (parallel execution)", concurrent)
	}
}

func TestFanout_SerialOnly(t *testing.T) {
	os.Setenv("TEST_API_KEY", "test-key")
	defer os.Unsetenv("TEST_API_KEY")

	// Track concurrent requests
	var concurrentCount int32
	var maxConcurrent int32
	var mu sync.Mutex
	var callOrder []string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		current := atomic.AddInt32(&concurrentCount, 1)
		defer atomic.AddInt32(&concurrentCount, -1)

		mu.Lock()
		if current > maxConcurrent {
			maxConcurrent = current
		}
		callOrder = append(callOrder, r.URL.Path)
		mu.Unlock()

		time.Sleep(20 * time.Millisecond)

		resp := llmapi.ChatResponse{
			Choices: []llmapi.Choice{{
				Message: llmapi.Message{Content: "Review: OK"},
			}},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	registry := testRegistry(server.URL)
	tempDir := t.TempDir()

	result, err := Fanout(context.Background(), FanoutParams{
		Registry:       registry,
		ParallelAgents: []string{},
		SerialAgents:   []string{"agent-a", "agent-b"},
		TaskMessage:    "Review this diff",
		GlobalTimeout:  30 * time.Second,
		OutputDir:      tempDir,
	})

	if err != nil {
		t.Fatalf("Fanout error: %v", err)
	}

	if result.SuccessCount != 2 {
		t.Errorf("SuccessCount = %d, want 2", result.SuccessCount)
	}

	// Should have run serially (max 1 concurrent)
	mu.Lock()
	concurrent := maxConcurrent
	order := callOrder
	mu.Unlock()
	if concurrent != 1 {
		t.Errorf("maxConcurrent = %d, want 1 (serial execution)", concurrent)
	}
	if len(order) != 2 {
		t.Errorf("callOrder length = %d, want 2", len(order))
	}
}

func TestFanout_Mixed(t *testing.T) {
	os.Setenv("TEST_API_KEY", "test-key")
	defer os.Unsetenv("TEST_API_KEY")

	var parallelCount int32
	var serialCount int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(30 * time.Millisecond)

		resp := llmapi.ChatResponse{
			Choices: []llmapi.Choice{{
				Message: llmapi.Message{Content: "Review: Mixed test"},
			}},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	registry := testRegistry(server.URL)
	tempDir := t.TempDir()

	startTime := time.Now()
	result, err := Fanout(context.Background(), FanoutParams{
		Registry:       registry,
		ParallelAgents: []string{"agent-a", "agent-b"},
		SerialAgents:   []string{"agent-c"},
		TaskMessage:    "Review",
		GlobalTimeout:  30 * time.Second,
		OutputDir:      tempDir,
	})
	elapsed := time.Since(startTime)

	if err != nil {
		t.Fatalf("Fanout error: %v", err)
	}

	if result.SuccessCount != 3 {
		t.Errorf("SuccessCount = %d, want 3", result.SuccessCount)
	}

	// Mixed execution: parallel agents run together, then serial
	// Minimum time is ~60ms (parallel 30ms + serial 30ms)
	// If all were serial, would be ~90ms
	if elapsed < 50*time.Millisecond || elapsed > 200*time.Millisecond {
		t.Logf("elapsed = %v (expected ~60-120ms for mixed execution)", elapsed)
	}

	_ = parallelCount
	_ = serialCount
}

func TestFanout_OutputFiles(t *testing.T) {
	os.Setenv("TEST_API_KEY", "test-key")
	defer os.Unsetenv("TEST_API_KEY")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := llmapi.ChatResponse{
			Choices: []llmapi.Choice{{
				Message: llmapi.Message{Content: "Review: LGTM\n\nTD_STREAM\nHIGH|main.go:42|Bug|Fix|error"},
			}},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	registry := testRegistry(server.URL)
	tempDir := t.TempDir()

	_, err := Fanout(context.Background(), FanoutParams{
		Registry:       registry,
		ParallelAgents: []string{"agent-a"},
		TaskMessage:    "Review",
		GlobalTimeout:  30 * time.Second,
		OutputDir:      tempDir,
	})

	if err != nil {
		t.Fatalf("Fanout error: %v", err)
	}

	// Check output files exist
	reviewPath := filepath.Join(tempDir, "raw", "agent-a", "review.md")
	if _, err := os.Stat(reviewPath); os.IsNotExist(err) {
		t.Errorf("review.md not created at %s", reviewPath)
	}

	statusPath := filepath.Join(tempDir, "raw", "agent-a", "status.json")
	if _, err := os.Stat(statusPath); os.IsNotExist(err) {
		t.Errorf("status.json not created at %s", statusPath)
	}
}

// Adversarial tests

func TestFanout_PartialFailure(t *testing.T) {
	os.Setenv("TEST_API_KEY", "test-key")
	defer os.Unsetenv("TEST_API_KEY")

	var requestCount int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&requestCount, 1)

		// Every other request fails
		if count%2 == 0 {
			w.WriteHeader(http.StatusUnauthorized)
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

	registry := testRegistry(server.URL)
	tempDir := t.TempDir()

	result, err := Fanout(context.Background(), FanoutParams{
		Registry:       registry,
		ParallelAgents: []string{"agent-a", "agent-b"},
		TaskMessage:    "Review",
		GlobalTimeout:  30 * time.Second,
		OutputDir:      tempDir,
	})

	// Should not return error for partial failure
	if err != nil {
		t.Fatalf("Fanout should not error on partial failure: %v", err)
	}

	// Should have mixed results
	if result.SuccessCount+result.FailedCount != 2 {
		t.Errorf("total = %d, want 2", result.SuccessCount+result.FailedCount)
	}
	if result.FailedCount == 0 {
		t.Error("expected at least 1 failure")
	}
}

func TestFanout_AllFail(t *testing.T) {
	os.Setenv("TEST_API_KEY", "test-key")
	defer os.Unsetenv("TEST_API_KEY")

	// Server always fails
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	registry := testRegistry(server.URL)
	tempDir := t.TempDir()

	result, err := Fanout(context.Background(), FanoutParams{
		Registry:       registry,
		ParallelAgents: []string{"agent-a", "agent-b"},
		TaskMessage:    "Review",
		GlobalTimeout:  30 * time.Second,
		OutputDir:      tempDir,
	})

	// Should return error when all fail
	if err == nil {
		t.Error("expected error when all agents fail")
	}

	if result.SuccessCount != 0 {
		t.Errorf("SuccessCount = %d, want 0", result.SuccessCount)
	}
	if result.FailedCount != 2 {
		t.Errorf("FailedCount = %d, want 2", result.FailedCount)
	}
}

func TestFanout_GlobalTimeout(t *testing.T) {
	os.Setenv("TEST_API_KEY", "test-key")
	defer os.Unsetenv("TEST_API_KEY")

	// Server responds slowly
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		resp := llmapi.ChatResponse{
			Choices: []llmapi.Choice{{
				Message: llmapi.Message{Content: "Too slow"},
			}},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	registry := testRegistry(server.URL)
	tempDir := t.TempDir()

	start := time.Now()
	result, err := Fanout(context.Background(), FanoutParams{
		Registry:       registry,
		ParallelAgents: []string{"agent-a", "agent-b"},
		TaskMessage:    "Review",
		GlobalTimeout:  200 * time.Millisecond,
		OutputDir:      tempDir,
	})
	elapsed := time.Since(start)

	// Should return error on global timeout
	if err == nil {
		t.Error("expected error on global timeout")
	}

	// All should be timed out
	for _, r := range result.Results {
		if r.Status != "timeout" {
			t.Errorf("agent %s Status = %q, want timeout", r.AgentName, r.Status)
		}
	}

	// Should complete quickly (not wait for slow server)
	if elapsed > 1*time.Second {
		t.Errorf("elapsed = %v, expected < 1s due to global timeout", elapsed)
	}
}

func TestFanout_MissingAgent(t *testing.T) {
	os.Setenv("TEST_API_KEY", "test-key")
	defer os.Unsetenv("TEST_API_KEY")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := llmapi.ChatResponse{
			Choices: []llmapi.Choice{{
				Message: llmapi.Message{Content: "Review: OK"},
			}},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	registry := testRegistry(server.URL)
	tempDir := t.TempDir()

	result, err := Fanout(context.Background(), FanoutParams{
		Registry:       registry,
		ParallelAgents: []string{"agent-a", "nonexistent-agent"},
		TaskMessage:    "Review",
		GlobalTimeout:  30 * time.Second,
		OutputDir:      tempDir,
	})

	// Should continue with valid agents, fail for missing
	if err != nil {
		t.Logf("Error (expected for partial failure): %v", err)
	}

	// agent-a should succeed, nonexistent should fail
	foundSuccess := false
	foundFailure := false
	for _, r := range result.Results {
		if r.AgentName == "agent-a" && r.Status == "ok" {
			foundSuccess = true
		}
		if r.AgentName == "nonexistent-agent" && r.Status == "failed" {
			foundFailure = true
		}
	}

	if !foundSuccess {
		t.Error("agent-a should have succeeded")
	}
	if !foundFailure {
		t.Error("nonexistent-agent should have failed")
	}
}

func TestFanout_TotalDuration(t *testing.T) {
	os.Setenv("TEST_API_KEY", "test-key")
	defer os.Unsetenv("TEST_API_KEY")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond)
		resp := llmapi.ChatResponse{
			Choices: []llmapi.Choice{{
				Message: llmapi.Message{Content: "Review: OK"},
			}},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	registry := testRegistry(server.URL)
	tempDir := t.TempDir()

	result, err := Fanout(context.Background(), FanoutParams{
		Registry:       registry,
		ParallelAgents: []string{"agent-a"},
		TaskMessage:    "Review",
		GlobalTimeout:  30 * time.Second,
		OutputDir:      tempDir,
	})

	if err != nil {
		t.Fatalf("Fanout error: %v", err)
	}

	// Should have recorded total duration
	if result.TotalDurationMS < 50 {
		t.Errorf("TotalDurationMS = %d, want >= 50", result.TotalDurationMS)
	}
}

func TestFanout_EmptyAgents(t *testing.T) {
	registry := &Registry{}
	tempDir := t.TempDir()

	result, err := Fanout(context.Background(), FanoutParams{
		Registry:       registry,
		ParallelAgents: []string{},
		SerialAgents:   []string{},
		TaskMessage:    "Review",
		GlobalTimeout:  30 * time.Second,
		OutputDir:      tempDir,
	})

	// Should return error for no agents
	if err == nil {
		t.Error("expected error when no agents specified")
	}
	if !strings.Contains(err.Error(), "no agents") {
		t.Errorf("error = %q, should mention no agents", err.Error())
	}
	_ = result
}

// Fallback tests

func TestFanout_FallbackTriggered(t *testing.T) {
	os.Setenv("PRIMARY_API_KEY", "primary-key")
	os.Setenv("FALLBACK_API_KEY", "fallback-key")
	defer os.Unsetenv("PRIMARY_API_KEY")
	defer os.Unsetenv("FALLBACK_API_KEY")

	// Primary server always fails
	primaryServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer primaryServer.Close()

	// Fallback server always succeeds
	fallbackServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := llmapi.ChatResponse{
			Choices: []llmapi.Choice{{
				Message: llmapi.Message{Content: "Review from fallback: LGTM"},
			}},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer fallbackServer.Close()

	registry := &Registry{
		Providers: map[string]ProviderConfig{
			"primary-provider": {
				Name:      "primary-provider",
				APIKeyEnv: "PRIMARY_API_KEY",
				BaseURL:   primaryServer.URL,
			},
			"fallback-provider": {
				Name:      "fallback-provider",
				APIKeyEnv: "FALLBACK_API_KEY",
				BaseURL:   fallbackServer.URL,
			},
		},
		Agents: map[string]AgentConfig{
			"primary-agent": {
				Name:        "primary-agent",
				Provider:    "primary-provider",
				Model:       "primary-model",
				Temperature: 0.3,
				TimeoutSecs: 2, // Short timeout to speed up test
				Fallback:    "fallback-agent",
			},
			"fallback-agent": {
				Name:        "fallback-agent",
				Provider:    "fallback-provider",
				Model:       "fallback-model",
				Temperature: 0.3,
			},
		},
	}
	tempDir := t.TempDir()

	result, err := Fanout(context.Background(), FanoutParams{
		Registry:       registry,
		ParallelAgents: []string{"primary-agent"},
		TaskMessage:    "Review this",
		GlobalTimeout:  30 * time.Second,
		OutputDir:      tempDir,
	})

	if err != nil {
		t.Fatalf("Fanout error: %v", err)
	}

	// Should succeed via fallback
	if result.SuccessCount != 1 {
		t.Errorf("SuccessCount = %d, want 1", result.SuccessCount)
	}

	// Check fallback was used
	if len(result.Results) != 1 {
		t.Fatalf("len(Results) = %d, want 1", len(result.Results))
	}
	res := result.Results[0]
	if !res.FallbackUsed {
		t.Error("FallbackUsed = false, want true")
	}
	if res.FallbackFrom != "primary-agent" {
		t.Errorf("FallbackFrom = %q, want primary-agent", res.FallbackFrom)
	}
	if res.AgentName != "fallback-agent" {
		t.Errorf("AgentName = %q, want fallback-agent", res.AgentName)
	}
	if res.OriginalError == nil {
		t.Error("OriginalError should be set")
	}
	if res.Status != "ok" {
		t.Errorf("Status = %q, want ok", res.Status)
	}
}

func TestFanout_FallbackNotTriggeredOnSuccess(t *testing.T) {
	os.Setenv("TEST_API_KEY", "test-key")
	defer os.Unsetenv("TEST_API_KEY")

	var requestCount int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		resp := llmapi.ChatResponse{
			Choices: []llmapi.Choice{{
				Message: llmapi.Message{Content: "Review: LGTM"},
			}},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	registry := &Registry{
		Providers: map[string]ProviderConfig{
			"test-provider": {
				Name:      "test-provider",
				APIKeyEnv: "TEST_API_KEY",
				BaseURL:   server.URL,
			},
		},
		Agents: map[string]AgentConfig{
			"primary-agent": {
				Name:        "primary-agent",
				Provider:    "test-provider",
				Model:       "primary-model",
				Temperature: 0.3,
				Fallback:    "fallback-agent",
			},
			"fallback-agent": {
				Name:        "fallback-agent",
				Provider:    "test-provider",
				Model:       "fallback-model",
				Temperature: 0.3,
			},
		},
	}
	tempDir := t.TempDir()

	result, err := Fanout(context.Background(), FanoutParams{
		Registry:       registry,
		ParallelAgents: []string{"primary-agent"},
		TaskMessage:    "Review this",
		GlobalTimeout:  30 * time.Second,
		OutputDir:      tempDir,
	})

	if err != nil {
		t.Fatalf("Fanout error: %v", err)
	}

	// Should succeed with primary (only 1 request)
	if atomic.LoadInt32(&requestCount) != 1 {
		t.Errorf("requestCount = %d, want 1 (fallback should not be called)", requestCount)
	}

	res := result.Results[0]
	if res.FallbackUsed {
		t.Error("FallbackUsed = true, want false (primary succeeded)")
	}
	if res.AgentName != "primary-agent" {
		t.Errorf("AgentName = %q, want primary-agent", res.AgentName)
	}
}

func TestFanout_FallbackAlsoFails(t *testing.T) {
	os.Setenv("TEST_API_KEY", "test-key")
	defer os.Unsetenv("TEST_API_KEY")

	// All requests fail
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	registry := &Registry{
		Providers: map[string]ProviderConfig{
			"test-provider": {
				Name:      "test-provider",
				APIKeyEnv: "TEST_API_KEY",
				BaseURL:   server.URL,
			},
		},
		Agents: map[string]AgentConfig{
			"primary-agent": {
				Name:        "primary-agent",
				Provider:    "test-provider",
				Model:       "primary-model",
				Temperature: 0.3,
				Fallback:    "fallback-agent",
			},
			"fallback-agent": {
				Name:        "fallback-agent",
				Provider:    "test-provider",
				Model:       "fallback-model",
				Temperature: 0.3,
			},
		},
	}
	tempDir := t.TempDir()

	result, err := Fanout(context.Background(), FanoutParams{
		Registry:       registry,
		ParallelAgents: []string{"primary-agent"},
		TaskMessage:    "Review this",
		GlobalTimeout:  30 * time.Second,
		OutputDir:      tempDir,
	})

	// Should fail (all agents failed)
	if err == nil {
		t.Error("expected error when all agents fail")
	}

	if result.FailedCount != 1 {
		t.Errorf("FailedCount = %d, want 1", result.FailedCount)
	}

	// Error should mention fallback was tried
	res := result.Results[0]
	if res.Error == nil {
		t.Error("expected error in result")
	} else if !strings.Contains(res.Error.Error(), "fallback") {
		t.Errorf("error should mention fallback: %v", res.Error)
	}
}

func TestFanout_ChainedFallback(t *testing.T) {
	os.Setenv("PRIMARY_KEY", "key1")
	os.Setenv("SECONDARY_KEY", "key2")
	os.Setenv("TERTIARY_KEY", "key3")
	defer os.Unsetenv("PRIMARY_KEY")
	defer os.Unsetenv("SECONDARY_KEY")
	defer os.Unsetenv("TERTIARY_KEY")

	// Primary and secondary servers fail
	primaryServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer primaryServer.Close()

	secondaryServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer secondaryServer.Close()

	// Tertiary server succeeds
	tertiaryServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := llmapi.ChatResponse{
			Choices: []llmapi.Choice{{
				Message: llmapi.Message{Content: "Review from tertiary: LGTM"},
			}},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer tertiaryServer.Close()

	// Chain: primary -> secondary -> tertiary
	registry := &Registry{
		Providers: map[string]ProviderConfig{
			"primary-provider": {
				Name:      "primary-provider",
				APIKeyEnv: "PRIMARY_KEY",
				BaseURL:   primaryServer.URL,
			},
			"secondary-provider": {
				Name:      "secondary-provider",
				APIKeyEnv: "SECONDARY_KEY",
				BaseURL:   secondaryServer.URL,
			},
			"tertiary-provider": {
				Name:      "tertiary-provider",
				APIKeyEnv: "TERTIARY_KEY",
				BaseURL:   tertiaryServer.URL,
			},
		},
		Agents: map[string]AgentConfig{
			"primary": {
				Name:        "primary",
				Provider:    "primary-provider",
				Model:       "model-1",
				Temperature: 0.3,
				TimeoutSecs: 2,
				Fallback:    "secondary",
			},
			"secondary": {
				Name:        "secondary",
				Provider:    "secondary-provider",
				Model:       "model-2",
				Temperature: 0.3,
				TimeoutSecs: 2,
				Fallback:    "tertiary",
			},
			"tertiary": {
				Name:        "tertiary",
				Provider:    "tertiary-provider",
				Model:       "model-3",
				Temperature: 0.3,
			},
		},
	}
	tempDir := t.TempDir()

	result, err := Fanout(context.Background(), FanoutParams{
		Registry:       registry,
		ParallelAgents: []string{"primary"},
		TaskMessage:    "Review this",
		GlobalTimeout:  30 * time.Second,
		OutputDir:      tempDir,
	})

	if err != nil {
		t.Fatalf("Fanout error: %v", err)
	}

	// Should succeed via chained fallback
	if result.SuccessCount != 1 {
		t.Errorf("SuccessCount = %d, want 1", result.SuccessCount)
	}

	res := result.Results[0]
	if !res.FallbackUsed {
		t.Error("FallbackUsed = false, want true")
	}
	// FallbackFrom should be the original agent, not the intermediate
	if res.FallbackFrom != "primary" {
		t.Errorf("FallbackFrom = %q, want primary", res.FallbackFrom)
	}
	if res.AgentName != "tertiary" {
		t.Errorf("AgentName = %q, want tertiary", res.AgentName)
	}
}

func TestFanout_FallbackProviderError(t *testing.T) {
	// Primary agent has missing provider env var, should fallback
	os.Setenv("FALLBACK_API_KEY", "fallback-key")
	defer os.Unsetenv("FALLBACK_API_KEY")
	// Note: PRIMARY_API_KEY is NOT set

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := llmapi.ChatResponse{
			Choices: []llmapi.Choice{{
				Message: llmapi.Message{Content: "Review from fallback: LGTM"},
			}},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	registry := &Registry{
		Providers: map[string]ProviderConfig{
			"primary-provider": {
				Name:      "primary-provider",
				APIKeyEnv: "PRIMARY_API_KEY", // Not set!
				BaseURL:   server.URL,
			},
			"fallback-provider": {
				Name:      "fallback-provider",
				APIKeyEnv: "FALLBACK_API_KEY",
				BaseURL:   server.URL,
			},
		},
		Agents: map[string]AgentConfig{
			"primary-agent": {
				Name:        "primary-agent",
				Provider:    "primary-provider",
				Model:       "primary-model",
				Temperature: 0.3,
				Fallback:    "fallback-agent",
			},
			"fallback-agent": {
				Name:        "fallback-agent",
				Provider:    "fallback-provider",
				Model:       "fallback-model",
				Temperature: 0.3,
			},
		},
	}
	tempDir := t.TempDir()

	result, err := Fanout(context.Background(), FanoutParams{
		Registry:       registry,
		ParallelAgents: []string{"primary-agent"},
		TaskMessage:    "Review this",
		GlobalTimeout:  30 * time.Second,
		OutputDir:      tempDir,
	})

	if err != nil {
		t.Fatalf("Fanout error: %v", err)
	}

	// Should succeed via fallback (primary provider env var missing)
	if result.SuccessCount != 1 {
		t.Errorf("SuccessCount = %d, want 1", result.SuccessCount)
	}

	res := result.Results[0]
	if !res.FallbackUsed {
		t.Error("FallbackUsed = false, want true")
	}
	if res.AgentName != "fallback-agent" {
		t.Errorf("AgentName = %q, want fallback-agent", res.AgentName)
	}
	if res.OriginalError == nil {
		t.Error("OriginalError should be set")
	} else if !strings.Contains(res.OriginalError.Error(), "provider") {
		t.Errorf("OriginalError should mention provider: %v", res.OriginalError)
	}
}
