package commands

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/samestrin/llm-tools/pkg/llmapi"
)

func TestReviewDirectCmd_Basic(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := llmapi.ChatResponse{
			Choices: []llmapi.Choice{{
				Message: llmapi.Message{Content: "Review: LGTM\n\nTD_STREAM\nMEDIUM|main.go:42|Issue|Fix|error"},
			}},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	// Create temp registry
	registryDir := t.TempDir()
	registryYAML := `
providers:
  test:
    api_key_env: TEST_API_KEY
    base_url: ` + server.URL + `

agents:
  alice:
    provider: test
    model: test-model
    timeout_secs: 60
`
	os.WriteFile(filepath.Join(registryDir, "registry.yaml"), []byte(registryYAML), 0644)
	os.WriteFile(filepath.Join(registryDir, "alice.md"), []byte("You are a code reviewer."), 0644)
	os.Setenv("TEST_API_KEY", "test-key")
	defer os.Unsetenv("TEST_API_KEY")

	// Create temp diff file
	diffFile := filepath.Join(t.TempDir(), "diff.txt")
	os.WriteFile(diffFile, []byte("+func foo() {}\n"), 0644)

	// Create output dir
	outputDir := t.TempDir()

	// Execute command
	cmd := newReviewDirectCmd()
	cmd.SetArgs([]string{
		"--reviewers", "alice",
		"--diff-file", diffFile,
		"--output-dir", outputDir,
		"--registry-dir", registryDir,
		"--timeout-seconds", "60",
	})

	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("Execute error: %v\nstderr: %s", err, stderr.String())
	}

	// Verify output files
	reviewPath := filepath.Join(outputDir, "raw", "alice", "review.md")
	if _, err := os.Stat(reviewPath); os.IsNotExist(err) {
		t.Errorf("review.md not created at %s", reviewPath)
	}

	statusPath := filepath.Join(outputDir, "raw", "alice", "status.json")
	if _, err := os.Stat(statusPath); os.IsNotExist(err) {
		t.Errorf("status.json not created at %s", statusPath)
	}

	summaryPath := filepath.Join(outputDir, "multi-review-summary.json")
	if _, err := os.Stat(summaryPath); os.IsNotExist(err) {
		t.Errorf("multi-review-summary.json not created at %s", summaryPath)
	}
}

func TestReviewDirectCmd_MissingFlags(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "missing reviewers",
			args:    []string{"--diff-file", "/tmp/x", "--output-dir", "/tmp/y"},
			wantErr: "--reviewers required",
		},
		{
			name:    "missing diff-file",
			args:    []string{"--reviewers", "alice", "--output-dir", "/tmp/y"},
			wantErr: "--diff-file required",
		},
		{
			name:    "missing output-dir",
			args:    []string{"--reviewers", "alice", "--diff-file", "/tmp/x"},
			wantErr: "--output-dir required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := newReviewDirectCmd()
			cmd.SetArgs(tt.args)

			var stderr bytes.Buffer
			cmd.SetErr(&stderr)

			err := cmd.Execute()
			if err == nil {
				t.Error("expected error")
			} else if !bytes.Contains([]byte(err.Error()), []byte(tt.wantErr)) {
				t.Errorf("error = %q, want to contain %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestReviewDirectCmd_MultipleReviewers(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := llmapi.ChatResponse{
			Choices: []llmapi.Choice{{
				Message: llmapi.Message{Content: "Review: OK"},
			}},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	// Create temp registry with multiple agents
	registryDir := t.TempDir()
	registryYAML := `
providers:
  test:
    api_key_env: TEST_API_KEY
    base_url: ` + server.URL + `

agents:
  alice:
    provider: test
    model: test-model
  bob:
    provider: test
    model: test-model
`
	os.WriteFile(filepath.Join(registryDir, "registry.yaml"), []byte(registryYAML), 0644)
	os.Setenv("TEST_API_KEY", "test-key")
	defer os.Unsetenv("TEST_API_KEY")

	// Create temp diff file
	diffFile := filepath.Join(t.TempDir(), "diff.txt")
	os.WriteFile(diffFile, []byte("+func foo() {}\n"), 0644)

	outputDir := t.TempDir()

	cmd := newReviewDirectCmd()
	cmd.SetArgs([]string{
		"--reviewers", "alice,bob",
		"--diff-file", diffFile,
		"--output-dir", outputDir,
		"--registry-dir", registryDir,
	})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}

	// Verify both agents have output
	for _, agent := range []string{"alice", "bob"} {
		reviewPath := filepath.Join(outputDir, "raw", agent, "review.md")
		if _, err := os.Stat(reviewPath); os.IsNotExist(err) {
			t.Errorf("%s/review.md not created", agent)
		}
	}

	// Verify summary
	summaryPath := filepath.Join(outputDir, "multi-review-summary.json")
	summaryBytes, err := os.ReadFile(summaryPath)
	if err != nil {
		t.Fatalf("failed to read summary: %v", err)
	}

	var summary MultiReviewSummary
	if err := json.Unmarshal(summaryBytes, &summary); err != nil {
		t.Fatalf("failed to parse summary: %v", err)
	}

	if len(summary.Reviewers) != 2 {
		t.Errorf("Reviewers count = %d, want 2", len(summary.Reviewers))
	}
}

func TestReviewDirectCmd_SerialReviewers(t *testing.T) {
	// Create mock server that tracks order
	var callOrder []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Capture call order via request body parsing (agent name in task message or similar)
		resp := llmapi.ChatResponse{
			Choices: []llmapi.Choice{{
				Message: llmapi.Message{Content: "Review: Serial OK"},
			}},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	registryDir := t.TempDir()
	registryYAML := `
providers:
  test:
    api_key_env: TEST_API_KEY
    base_url: ` + server.URL + `

agents:
  alice:
    provider: test
    model: test-model
  bob:
    provider: test
    model: test-model
    rate_limited: true
`
	os.WriteFile(filepath.Join(registryDir, "registry.yaml"), []byte(registryYAML), 0644)
	os.Setenv("TEST_API_KEY", "test-key")
	defer os.Unsetenv("TEST_API_KEY")

	diffFile := filepath.Join(t.TempDir(), "diff.txt")
	os.WriteFile(diffFile, []byte("+func foo() {}\n"), 0644)

	outputDir := t.TempDir()

	cmd := newReviewDirectCmd()
	cmd.SetArgs([]string{
		"--reviewers", "alice,bob",
		"--serial-reviewers", "bob",
		"--diff-file", diffFile,
		"--output-dir", outputDir,
		"--registry-dir", registryDir,
	})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}

	// Both should have output
	for _, agent := range []string{"alice", "bob"} {
		statusPath := filepath.Join(outputDir, "raw", agent, "status.json")
		if _, err := os.Stat(statusPath); os.IsNotExist(err) {
			t.Errorf("%s/status.json not created", agent)
		}
	}

	_ = callOrder // Would need more elaborate mock to verify serial order
}

func TestReviewDirectCmd_DefaultRegistry(t *testing.T) {
	// Test that command works with default registry location
	cmd := newReviewDirectCmd()

	// Just verify the flag default is set correctly
	registryDir, _ := cmd.Flags().GetString("registry-dir")
	if registryDir == "" {
		t.Error("registry-dir should have a default value")
	}
}
