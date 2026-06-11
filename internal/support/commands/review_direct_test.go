package commands

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
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

func TestReviewDirectCmd_EmptyDiffFile(t *testing.T) {
	cases := []struct {
		name    string
		content string
	}{
		{"zero byte", ""},
		{"whitespace only", "   \n\t\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			diffFile := filepath.Join(t.TempDir(), "diff.txt")
			if err := os.WriteFile(diffFile, []byte(tc.content), 0644); err != nil {
				t.Fatal(err)
			}
			cmd := newReviewDirectCmd()
			cmd.SetArgs([]string{
				"--reviewers", "alice",
				"--diff-file", diffFile,
				"--output-dir", t.TempDir(),
			})
			cmd.SetOut(new(bytes.Buffer))
			cmd.SetErr(new(bytes.Buffer))
			err := cmd.Execute()
			if err == nil {
				t.Fatal("expected error for empty diff file")
			}
			if !bytes.Contains([]byte(err.Error()), []byte("empty")) {
				t.Errorf("error %q should mention empty diff", err.Error())
			}
		})
	}
}

// newDirectTestEnv builds a mock LLM server + agent registry for self-serve
// diff tests. Returns the registry dir; the server is closed on cleanup.
func newDirectTestEnv(t *testing.T) string {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := llmapi.ChatResponse{
			Choices: []llmapi.Choice{{
				Message: llmapi.Message{Content: "Review: LGTM\n\nTD_STREAM\nMEDIUM|main.go:42|Issue|Fix|error"},
			}},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	t.Cleanup(server.Close)

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
	t.Setenv("TEST_API_KEY", "test-key")
	return registryDir
}

func TestReviewDirectCmd_SelfServeDiff(t *testing.T) {
	repo := initRangeFixtureRepo(t) // on feature branch, 2 commits ahead of main
	registryDir := newDirectTestEnv(t)
	outputDir := filepath.Join(t.TempDir(), "out")

	cmd := newReviewDirectCmd()
	cmd.SetArgs([]string{
		"--reviewers", "alice",
		"--repo", repo,
		"--output-dir", outputDir,
		"--registry-dir", registryDir,
		"--timeout-seconds", "60",
	})
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(new(bytes.Buffer))
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	diffPath := filepath.Join(outputDir, "diff.txt")
	data, err := os.ReadFile(diffPath)
	if err != nil {
		t.Fatalf("self-serve mode should write %s: %v", diffPath, err)
	}
	if !bytes.Contains(data, []byte("c.txt")) || !bytes.Contains(data, []byte("d.txt")) {
		t.Errorf("diff.txt should contain feature-branch changes:\n%s", data)
	}
	if _, err := os.Stat(filepath.Join(outputDir, "raw", "alice", "review.md")); err != nil {
		t.Errorf("review artifacts missing: %v", err)
	}
}

func TestReviewDirectCmd_SelfServeMergeCommit(t *testing.T) {
	repo := initRangeFixtureRepo(t)
	gitInDir(t, repo, "checkout", "-q", "main")
	gitInDir(t, repo, "merge", "--squash", "-q", "feature")
	gitInDir(t, repo, "commit", "-q", "-m", "squash feature")
	sha := gitInDir(t, repo, "rev-parse", "HEAD")

	registryDir := newDirectTestEnv(t)
	outputDir := filepath.Join(t.TempDir(), "out")

	cmd := newReviewDirectCmd()
	cmd.SetArgs([]string{
		"--reviewers", "alice",
		"--repo", repo,
		"--merge-commit", sha,
		"--output-dir", outputDir,
		"--registry-dir", registryDir,
		"--timeout-seconds", "60",
	})
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(new(bytes.Buffer))
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(outputDir, "diff.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(data, []byte("c.txt")) {
		t.Errorf("merge-commit diff should contain squashed changes:\n%s", data)
	}
}

func TestReviewDirectCmd_DiffFileMutuallyExclusive(t *testing.T) {
	for _, extra := range [][]string{
		{"--base", "main"},
		{"--head", "feature"},
		{"--merge-commit", "abc1234"},
	} {
		cmd := newReviewDirectCmd()
		args := append([]string{
			"--reviewers", "alice",
			"--diff-file", "/tmp/x.txt",
			"--output-dir", "/tmp/y",
		}, extra...)
		cmd.SetArgs(args)
		cmd.SetOut(new(bytes.Buffer))
		cmd.SetErr(new(bytes.Buffer))
		err := cmd.Execute()
		if err == nil {
			t.Errorf("%v: expected mutual-exclusion error", extra)
		} else if !bytes.Contains([]byte(err.Error()), []byte("mutually exclusive")) {
			t.Errorf("%v: error %q should say mutually exclusive", extra, err.Error())
		}
	}
}

func TestReviewDirectCmd_SelfServeEmptyRange(t *testing.T) {
	repo := initRangeFixtureRepo(t)
	gitInDir(t, repo, "checkout", "-q", "main") // HEAD == main → empty range

	cmd := newReviewDirectCmd()
	cmd.SetArgs([]string{
		"--reviewers", "alice",
		"--repo", repo,
		"--output-dir", filepath.Join(t.TempDir(), "out"),
	})
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(new(bytes.Buffer))
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for empty self-serve range")
	}
	if !bytes.Contains([]byte(err.Error()), []byte("empty")) {
		t.Errorf("error %q should mention empty", err.Error())
	}
}

func TestReviewDirectCmd_ExplicitlyEmptyDiffFileRejected(t *testing.T) {
	// --diff-file "" (classic unset shell variable) must NOT silently fall
	// into self-serve mode against the cwd repo.
	cmd := newReviewDirectCmd()
	cmd.SetArgs([]string{
		"--reviewers", "alice",
		"--diff-file", "",
		"--output-dir", t.TempDir(),
	})
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(new(bytes.Buffer))
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for explicitly empty --diff-file")
	}
	if !bytes.Contains([]byte(err.Error()), []byte("--diff-file")) {
		t.Errorf("error %q should mention --diff-file", err.Error())
	}
}

// sprintPlanTestEnv stands up a mock provider that captures the request body,
// a single-agent registry, and a diff file. Returns the registry dir, diff
// path, and a pointer to the captured body (valid after cmd.Execute returns).
func sprintPlanTestEnv(t *testing.T) (registryDir, diffFile string, captured *bytes.Buffer) {
	t.Helper()
	captured = new(bytes.Buffer)
	var mu sync.Mutex
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		captured.Write(body)
		mu.Unlock()
		resp := llmapi.ChatResponse{
			Choices: []llmapi.Choice{{
				Message: llmapi.Message{Content: "Review: OK"},
			}},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	t.Cleanup(server.Close)

	registryDir = t.TempDir()
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
	t.Setenv("TEST_API_KEY", "test-key")

	diffFile = filepath.Join(t.TempDir(), "diff.txt")
	os.WriteFile(diffFile, []byte("+func foo() {}\n"), 0644)
	return registryDir, diffFile, captured
}

func runReviewDirectForSprintPlan(t *testing.T, registryDir, diffFile string, extraArgs ...string) error {
	t.Helper()
	cmd := newReviewDirectCmd()
	args := []string{
		"--reviewers", "alice",
		"--diff-file", diffFile,
		"--output-dir", t.TempDir(),
		"--registry-dir", registryDir,
		"--timeout-seconds", "60",
	}
	args = append(args, extraArgs...)
	cmd.SetArgs(args)
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(new(bytes.Buffer))
	return cmd.Execute()
}

func TestBuildReviewTaskMessage_NoPlanByteIdentity(t *testing.T) {
	// Pins the pre-refactor output: without a sprint plan the task message
	// must keep the exact "example line, blank line, diff" shape.
	msg := buildReviewTaskMessage("DIFF", "")
	wantTail := "HIGH|src/auth.go:42|Missing input validation|Add length check|security|5|user input passed directly to query|bruce\n\nDiff to review:\nDIFF"
	if !strings.HasSuffix(msg, wantTail) {
		t.Errorf("task message tail changed:\n...%q\nwant suffix:\n%q", msg[len(msg)-min(len(msg), 200):], wantTail)
	}
	if strings.Contains(msg, "SCOPE CONSTRAINT") {
		t.Error("unexpected scope block without sprint plan")
	}
}

func TestReviewDirectCmd_SprintPlanScopesTaskMessage(t *testing.T) {
	registryDir, diffFile, captured := sprintPlanTestEnv(t)

	planContent := "## Sprint 9.0 widget work\n- Task: add widget"
	planFile := filepath.Join(t.TempDir(), "sprint-plan.md")
	os.WriteFile(planFile, []byte(planContent), 0644)

	if err := runReviewDirectForSprintPlan(t, registryDir, diffFile,
		"--sprint-plan", planFile); err != nil {
		t.Fatalf("Execute error: %v", err)
	}

	body := captured.String()
	if !strings.Contains(body, "SCOPE CONSTRAINT") {
		t.Error("request body missing SCOPE CONSTRAINT block")
	}
	if !strings.Contains(body, "Sprint 9.0 widget work") {
		t.Error("request body missing sprint plan content")
	}
	// The scope block must precede the embedded diff: instructions before
	// (potentially huge) diff content.
	scopeIdx := strings.Index(body, "SCOPE CONSTRAINT")
	diffIdx := strings.Index(body, "Diff to review:")
	if diffIdx == -1 {
		t.Fatal("request body missing 'Diff to review:' section")
	}
	if scopeIdx > diffIdx {
		t.Errorf("scope block at %d must come before diff section at %d", scopeIdx, diffIdx)
	}
}

func TestReviewDirectCmd_NoSprintPlanNoScopeBlock(t *testing.T) {
	registryDir, diffFile, captured := sprintPlanTestEnv(t)

	if err := runReviewDirectForSprintPlan(t, registryDir, diffFile); err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if strings.Contains(captured.String(), "SCOPE CONSTRAINT") {
		t.Error("unexpected SCOPE CONSTRAINT block without --sprint-plan")
	}
}

func TestReviewDirectCmd_SprintPlanMissingFile(t *testing.T) {
	registryDir, diffFile, captured := sprintPlanTestEnv(t)

	err := runReviewDirectForSprintPlan(t, registryDir, diffFile,
		"--sprint-plan", filepath.Join(t.TempDir(), "does-not-exist.md"))
	if err != nil {
		t.Fatalf("missing sprint plan file must not fail the review: %v", err)
	}
	if strings.Contains(captured.String(), "SCOPE CONSTRAINT") {
		t.Error("unexpected SCOPE CONSTRAINT block for missing sprint plan file")
	}
}

func TestReviewDirectCmd_TaskMessageOverrideSuppressesScope(t *testing.T) {
	registryDir, diffFile, captured := sprintPlanTestEnv(t)

	planFile := filepath.Join(t.TempDir(), "sprint-plan.md")
	os.WriteFile(planFile, []byte("## Sprint 9.0"), 0644)

	if err := runReviewDirectForSprintPlan(t, registryDir, diffFile,
		"--sprint-plan", planFile,
		"--task-message", "custom review instructions"); err != nil {
		t.Fatalf("Execute error: %v", err)
	}

	body := captured.String()
	if !strings.Contains(body, "custom review instructions") {
		t.Error("request body missing --task-message override")
	}
	// --task-message is a full override, matching multi_review: the scope
	// block is NOT appended to it.
	if strings.Contains(body, "SCOPE CONSTRAINT") {
		t.Error("--task-message override must suppress the scope block")
	}
}
