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
