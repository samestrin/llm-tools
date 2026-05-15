package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/samestrin/llm-tools/internal/support/multireview"
)

// initFixtureRepoMR mirrors the helper from the multireview package — a small
// 2-commit git repo with a tag so the multi_review command can bundle it.
func initFixtureRepoMR(t *testing.T) string {
	t.Helper()
	repoPath := t.TempDir()
	mustRun := func(args ...string) {
		c := exec.Command("git", args...)
		c.Dir = repoPath
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	mustRun("init", "-q", "-b", "main")
	mustRun("config", "user.email", "test@example.com")
	mustRun("config", "user.name", "Test")
	mustRun("config", "commit.gpgsign", "false")
	if err := os.WriteFile(filepath.Join(repoPath, "a.txt"), []byte("first"), 0o644); err != nil {
		t.Fatal(err)
	}
	mustRun("add", "a.txt")
	mustRun("commit", "-q", "-m", "first")
	mustRun("tag", "v0.1.0")
	if err := os.WriteFile(filepath.Join(repoPath, "b.txt"), []byte("second"), 0o644); err != nil {
		t.Fatal(err)
	}
	mustRun("add", "b.txt")
	mustRun("commit", "-q", "-m", "second")
	return repoPath
}

// withMockInvoker swaps the invokeReviewerFn used by the multi_review command
// for a deterministic stub and restores the original on cleanup.
func withMockInvoker(t *testing.T, fn func(ctx context.Context, p multireview.InvokeReviewerParams) (multireview.InvokeReviewerResult, error)) {
	t.Helper()
	orig := invokeReviewerFn
	invokeReviewerFn = fn
	t.Cleanup(func() { invokeReviewerFn = orig })
}

// withMockShipBundle swaps the shipBundleFn so tests don't actually SSH.
func withMockShipBundle(t *testing.T) {
	t.Helper()
	orig := shipBundleFn
	shipBundleFn = func(ctx context.Context, p multireview.ShipBundleParams) (multireview.ShipBundleResult, error) {
		return multireview.ShipBundleResult{
			LocalBundlePath:       "/tmp/mock-bundle.git",
			HostStagingBundlePath: "/tmp/mock-host-staging/bundle.git",
			RemoteRepoPath:        "/tmp/mock-container-work/" + p.RepoName,
			BundleSize:            1024,
		}, nil
	}
	t.Cleanup(func() { shipBundleFn = orig })
}

// withMockPreComputeDiff swaps preComputeDiffFn so tests get a deterministic
// diff result without invoking ssh. Default mock returns a 1234-byte / 50-line
// diff at <RemoteWorkdir>/diff.txt.
func withMockPreComputeDiff(t *testing.T) {
	t.Helper()
	orig := preComputeDiffFn
	preComputeDiffFn = func(ctx context.Context, p multireview.PreComputeDiffParams) (multireview.PreComputeDiffResult, error) {
		return multireview.PreComputeDiffResult{
			DiffPath:  p.RemoteWorkdir + "/diff.txt",
			SizeBytes: 1234,
			LineCount: 50,
			Empty:     false,
		}, nil
	}
	t.Cleanup(func() { preComputeDiffFn = orig })
}

func mockResultFor(agent string, tdLine string) multireview.InvokeReviewerResult {
	return multireview.InvokeReviewerResult{
		AgentName:   agent,
		Status:      "ok",
		Model:       "mock-" + agent,
		DurationMS:  1000,
		ReviewProse: "Verdict: ship\n\n" + tdLine + "\n",
		RawJSON:     `{"runId":"x"}`,
	}
}

func TestMultiReview_HappyPath(t *testing.T) {
	repo := initFixtureRepoMR(t)
	outDir := filepath.Join(t.TempDir(), "out")

	withMockShipBundle(t)
	withMockPreComputeDiff(t)
	withMockInvoker(t, func(ctx context.Context, p multireview.InvokeReviewerParams) (multireview.InvokeReviewerResult, error) {
		return mockResultFor(p.AgentName, "MEDIUM|src/a.go:1|test problem|test fix|robustness"), nil
	})

	cmd := newMultiReviewCmd()
	cmd.SetArgs([]string{
		"--reviewers", "bruce,greta",
		"--repo", repo,
		"--openclaw-host", "user@example.lan",
		"--output-dir", outDir,
		"--base", "v1",
		"--timeout-seconds", "30",
	})
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	for _, agent := range []string{"bruce", "greta"} {
		td := filepath.Join(outDir, "raw", agent, "td-stream.txt")
		if _, err := os.Stat(td); err != nil {
			t.Errorf("missing %s: %v", td, err)
		}
	}
	merged := filepath.Join(outDir, "td-stream.txt")
	if _, err := os.Stat(merged); err != nil {
		t.Errorf("missing merged td-stream.txt at output root: %v", err)
	}
	summary := filepath.Join(outDir, "multi-review-summary.json")
	if _, err := os.Stat(summary); err != nil {
		t.Errorf("missing summary: %v", err)
	}

	summaryData, _ := os.ReadFile(summary)
	var s struct {
		Reviewers []struct {
			Agent  string `json:"agent"`
			Status string `json:"status"`
		} `json:"reviewers"`
		TotalFindings int  `json:"totalFindings"`
		Partial       bool `json:"partial"`
	}
	if err := json.Unmarshal(summaryData, &s); err != nil {
		t.Fatalf("parse summary: %v", err)
	}
	if len(s.Reviewers) != 2 {
		t.Errorf("expected 2 reviewers in summary, got %d", len(s.Reviewers))
	}
	if s.TotalFindings != 2 {
		t.Errorf("expected 2 findings total, got %d", s.TotalFindings)
	}
	if s.Partial {
		t.Errorf("partial should be false in happy path")
	}
}

func TestMultiReview_RequiresFlags(t *testing.T) {
	repo := initFixtureRepoMR(t)
	cases := []struct {
		name string
		args []string
	}{
		{"missing reviewers", []string{"--repo", repo, "--openclaw-host", "h", "--output-dir", "/tmp/x"}},
		{"missing repo", []string{"--reviewers", "bruce", "--openclaw-host", "h", "--output-dir", "/tmp/x"}},
		{"missing host", []string{"--reviewers", "bruce", "--repo", repo, "--output-dir", "/tmp/x"}},
		{"missing output-dir", []string{"--reviewers", "bruce", "--repo", repo, "--openclaw-host", "h"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cmd := newMultiReviewCmd()
			cmd.SetArgs(c.args)
			cmd.SetOut(new(bytes.Buffer))
			cmd.SetErr(new(bytes.Buffer))
			if err := cmd.Execute(); err == nil {
				t.Errorf("expected error for %s", c.name)
			}
		})
	}
}

func TestMultiReview_TwoLane(t *testing.T) {
	repo := initFixtureRepoMR(t)
	outDir := filepath.Join(t.TempDir(), "out")

	withMockShipBundle(t)
	withMockPreComputeDiff(t)
	withMockInvoker(t, func(ctx context.Context, p multireview.InvokeReviewerParams) (multireview.InvokeReviewerResult, error) {
		return mockResultFor(p.AgentName, "LOW|f:1|p|x|c"), nil
	})

	cmd := newMultiReviewCmd()
	cmd.SetArgs([]string{
		"--reviewers", "bruce,greta,kai",
		"--serial-reviewers", "greta",
		"--repo", repo,
		"--openclaw-host", "user@example.lan",
		"--output-dir", outDir,
		"--base", "v1",
		"--timeout-seconds", "30",
	})
	cmd.SetOut(new(bytes.Buffer))
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	for _, agent := range []string{"bruce", "greta", "kai"} {
		dir := filepath.Join(outDir, "raw", agent)
		if _, err := os.Stat(dir); err != nil {
			t.Errorf("missing %s: %v", dir, err)
		}
	}
}

func TestMultiReview_PartialFailure(t *testing.T) {
	repo := initFixtureRepoMR(t)
	outDir := filepath.Join(t.TempDir(), "out")

	withMockShipBundle(t)
	withMockPreComputeDiff(t)
	withMockInvoker(t, func(ctx context.Context, p multireview.InvokeReviewerParams) (multireview.InvokeReviewerResult, error) {
		if p.AgentName == "greta" {
			return multireview.InvokeReviewerResult{AgentName: p.AgentName}, fmt.Errorf("simulated failure")
		}
		return mockResultFor(p.AgentName, "MEDIUM|f:1|p|x|c"), nil
	})

	cmd := newMultiReviewCmd()
	cmd.SetArgs([]string{
		"--reviewers", "bruce,greta",
		"--repo", repo,
		"--openclaw-host", "user@example.lan",
		"--output-dir", outDir,
		"--base", "v1",
		"--timeout-seconds", "30",
	})
	cmd.SetOut(new(bytes.Buffer))
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v (partial failure should not be fatal)", err)
	}

	summaryData, err := os.ReadFile(filepath.Join(outDir, "multi-review-summary.json"))
	if err != nil {
		t.Fatal(err)
	}
	var s struct {
		Partial   bool `json:"partial"`
		Reviewers []struct {
			Agent  string `json:"agent"`
			Status string `json:"status"`
		} `json:"reviewers"`
	}
	if err := json.Unmarshal(summaryData, &s); err != nil {
		t.Fatal(err)
	}
	if !s.Partial {
		t.Error("partial should be true")
	}
	var failed, ok int
	for _, r := range s.Reviewers {
		if r.Status == "ok" {
			ok++
		} else {
			failed++
		}
	}
	if ok != 1 || failed != 1 {
		t.Errorf("expected 1 ok + 1 failed, got ok=%d failed=%d", ok, failed)
	}
}

func TestMultiReview_AllFail(t *testing.T) {
	repo := initFixtureRepoMR(t)
	outDir := filepath.Join(t.TempDir(), "out")

	withMockShipBundle(t)
	withMockPreComputeDiff(t)
	withMockInvoker(t, func(ctx context.Context, p multireview.InvokeReviewerParams) (multireview.InvokeReviewerResult, error) {
		return multireview.InvokeReviewerResult{AgentName: p.AgentName}, fmt.Errorf("simulated failure")
	})

	cmd := newMultiReviewCmd()
	cmd.SetArgs([]string{
		"--reviewers", "bruce",
		"--repo", repo,
		"--openclaw-host", "user@example.lan",
		"--output-dir", outDir,
		"--base", "v1",
		"--timeout-seconds", "30",
	})
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(new(bytes.Buffer))
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when all reviewers fail")
	}
	if !strings.Contains(err.Error(), "all reviewers failed") && !strings.Contains(err.Error(), "no successful") {
		t.Errorf("error %q should mention total failure", err.Error())
	}
}

func TestMultiReview_ShipFailureHardStops(t *testing.T) {
	repo := initFixtureRepoMR(t)
	outDir := filepath.Join(t.TempDir(), "out")

	// Override shipBundleFn to fail
	orig := shipBundleFn
	shipBundleFn = func(ctx context.Context, p multireview.ShipBundleParams) (multireview.ShipBundleResult, error) {
		return multireview.ShipBundleResult{}, fmt.Errorf("ssh: connection refused")
	}
	t.Cleanup(func() { shipBundleFn = orig })

	cmd := newMultiReviewCmd()
	cmd.SetArgs([]string{
		"--reviewers", "bruce,greta",
		"--repo", repo,
		"--openclaw-host", "user@example.lan",
		"--output-dir", outDir,
		"--base", "v1",
		"--timeout-seconds", "30",
	})
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(new(bytes.Buffer))
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected hard-stop on ship failure")
	}
}

// ---- diff-precompute tests ----

func TestMultiReview_DiffFailureHardStops(t *testing.T) {
	// preComputeDiffFn errors → run aborts before any reviewer is invoked.
	repo := initFixtureRepoMR(t)
	outDir := filepath.Join(t.TempDir(), "out")

	withMockShipBundle(t)
	origDiff := preComputeDiffFn
	preComputeDiffFn = func(ctx context.Context, p multireview.PreComputeDiffParams) (multireview.PreComputeDiffResult, error) {
		return multireview.PreComputeDiffResult{}, fmt.Errorf("diff: git exit 128, stderr: fatal: bad revision 'badref'")
	}
	t.Cleanup(func() { preComputeDiffFn = origDiff })

	// invokeReviewerFn should never be called — fail loud if it is.
	withMockInvoker(t, func(ctx context.Context, p multireview.InvokeReviewerParams) (multireview.InvokeReviewerResult, error) {
		t.Errorf("invokeReviewerFn called after diff failure — should have hard-stopped first")
		return multireview.InvokeReviewerResult{AgentName: p.AgentName}, nil
	})

	cmd := newMultiReviewCmd()
	cmd.SetArgs([]string{
		"--reviewers", "bruce,greta",
		"--repo", repo,
		"--openclaw-host", "user@example.lan",
		"--output-dir", outDir,
		"--base", "v1",
		"--timeout-seconds", "30",
	})
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(new(bytes.Buffer))
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected hard-stop when diff fails")
	}
	// Summary should NOT be written when we hard-stop before invoking.
	if _, err := os.Stat(filepath.Join(outDir, "multi-review-summary.json")); err == nil {
		t.Error("summary should not exist after pre-diff hard-stop")
	}
}

func TestMultiReview_EmptyBaseRefRejected(t *testing.T) {
	repo := initFixtureRepoMR(t)
	outDir := filepath.Join(t.TempDir(), "out")

	// We don't need any of the mocks — validation should reject before then.
	// Ensure invokeReviewerFn and shipBundleFn are never called by failing loud.
	origShip := shipBundleFn
	shipBundleFn = func(ctx context.Context, p multireview.ShipBundleParams) (multireview.ShipBundleResult, error) {
		t.Errorf("shipBundleFn called — should have validated --base before this")
		return multireview.ShipBundleResult{}, nil
	}
	t.Cleanup(func() { shipBundleFn = origShip })

	cmd := newMultiReviewCmd()
	cmd.SetArgs([]string{
		"--reviewers", "bruce",
		"--repo", repo,
		"--openclaw-host", "user@example.lan",
		"--output-dir", outDir,
		// no --base passed
		"--timeout-seconds", "30",
	})
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(new(bytes.Buffer))
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected validation error when --base is missing")
	}
}

func TestMultiReview_DiffPathInTaskMessage(t *testing.T) {
	// Verify the task message passed to reviewers includes `cat <diffPath>`
	// and the anti-hallucination clause.
	repo := initFixtureRepoMR(t)
	outDir := filepath.Join(t.TempDir(), "out")

	withMockShipBundle(t)
	withMockPreComputeDiff(t)

	var capturedTaskMessage string
	withMockInvoker(t, func(ctx context.Context, p multireview.InvokeReviewerParams) (multireview.InvokeReviewerResult, error) {
		capturedTaskMessage = p.TaskMessage
		return mockResultFor(p.AgentName, "LOW|f:1|p|x|c"), nil
	})

	cmd := newMultiReviewCmd()
	cmd.SetArgs([]string{
		"--reviewers", "bruce",
		"--repo", repo,
		"--openclaw-host", "user@example.lan",
		"--output-dir", outDir,
		"--base", "v1",
		"--timeout-seconds", "30",
	})
	cmd.SetOut(new(bytes.Buffer))
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	// The diff path should appear in the task message. The mock builds it as
	// <RemoteWorkdir>/diff.txt; RemoteWorkdir is `/tmp/multi-review-<unix>`
	// in production, so we just check for the suffix.
	if !strings.Contains(capturedTaskMessage, "/diff.txt") {
		t.Errorf("task message missing diff path suffix `/diff.txt`. Got:\n%s", capturedTaskMessage)
	}
	if !strings.Contains(capturedTaskMessage, "/tmp/multi-review-") {
		t.Errorf("task message missing workdir prefix `/tmp/multi-review-`. Got:\n%s", capturedTaskMessage)
	}
	// The cat instruction should be present
	if !strings.Contains(capturedTaskMessage, "cat ") {
		t.Errorf("task message missing `cat` instruction. Got:\n%s", capturedTaskMessage)
	}
	// The anti-hallucination clause should be present
	if !strings.Contains(capturedTaskMessage, "Hallucinating") && !strings.Contains(capturedTaskMessage, "Do NOT report") {
		t.Errorf("task message missing anti-hallucination clause. Got:\n%s", capturedTaskMessage)
	}
	// Old `git diff <base>..<head>` instruction should NOT be present (reviewers shouldn't run git themselves anymore)
	if strings.Contains(capturedTaskMessage, "git diff v1..") {
		t.Errorf("task message should not include git diff instruction. Got:\n%s", capturedTaskMessage)
	}
}

func TestMultiReview_LargeDiffWarning(t *testing.T) {
	// SizeBytes > 1_000_000 should append the --stat hint. Isolate from the
	// user's installed per-agent templates by pointing the loader at an empty
	// dir — this test asserts on the BUILTIN hardcoded message's behavior,
	// not on whatever templates the user has synced locally.
	t.Setenv("LLM_TOOLS_MULTI_REVIEW_PROMPTS", t.TempDir())

	repo := initFixtureRepoMR(t)
	outDir := filepath.Join(t.TempDir(), "out")

	withMockShipBundle(t)
	origDiff := preComputeDiffFn
	preComputeDiffFn = func(ctx context.Context, p multireview.PreComputeDiffParams) (multireview.PreComputeDiffResult, error) {
		return multireview.PreComputeDiffResult{
			DiffPath:  p.RemoteWorkdir + "/diff.txt",
			SizeBytes: 2_500_000, // 2.5 MB
			LineCount: 60000,
			Empty:     false,
		}, nil
	}
	t.Cleanup(func() { preComputeDiffFn = origDiff })

	var capturedTaskMessage string
	withMockInvoker(t, func(ctx context.Context, p multireview.InvokeReviewerParams) (multireview.InvokeReviewerResult, error) {
		capturedTaskMessage = p.TaskMessage
		return mockResultFor(p.AgentName, "LOW|f:1|p|x|c"), nil
	})

	cmd := newMultiReviewCmd()
	cmd.SetArgs([]string{
		"--reviewers", "bruce",
		"--repo", repo,
		"--openclaw-host", "user@example.lan",
		"--output-dir", outDir,
		"--base", "v1",
		"--timeout-seconds", "30",
	})
	cmd.SetOut(new(bytes.Buffer))
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	// New: large-diff workflow must be DIRECTIVE (not optional) and front-
	// load the stat-first approach. The previous "if your context budget
	// is tight" wording was observed in production to be too weak — kai
	// (kimi-k2.6-coding) burned its 30-min budget exploring a 1.2 MB diff
	// without producing findings.
	if !strings.Contains(capturedTaskMessage, "--stat") {
		t.Errorf("task message should include --stat hint for large diff. Got:\n%s", capturedTaskMessage)
	}
	if !strings.Contains(capturedTaskMessage, "LARGE DIFF WORKFLOW (REQUIRED") {
		t.Errorf("task message should label the large-diff workflow as REQUIRED, not optional. Got:\n%s", capturedTaskMessage)
	}
	if !strings.Contains(capturedTaskMessage, "Tool-call budget") {
		t.Errorf("task message should set a tool-call budget for large diffs. Got:\n%s", capturedTaskMessage)
	}
	if !strings.Contains(capturedTaskMessage, "Stop exploring") {
		t.Errorf("task message should explicitly tell reviewers to stop exploring. Got:\n%s", capturedTaskMessage)
	}
}

func TestMultiReview_AbortedReviewerReportsAbortedStatus(t *testing.T) {
	// When openclaw aborts an agent before it produces findings (e.g. hit
	// the harness's per-turn time ceiling), the reviewer's response carries
	// Aborted=true. Today this is silently reported as status: "ok" with 0
	// findings, which is indistinguishable from a clean review that found
	// nothing. Distinguish them: aborted reviewers report status: "aborted".
	repo := initFixtureRepoMR(t)
	outDir := filepath.Join(t.TempDir(), "out")

	withMockShipBundle(t)
	withMockPreComputeDiff(t)
	withMockInvoker(t, func(ctx context.Context, p multireview.InvokeReviewerParams) (multireview.InvokeReviewerResult, error) {
		// Simulate openclaw aborting kai with no findings produced.
		return multireview.InvokeReviewerResult{
			AgentName:   p.AgentName,
			Status:      "ok", // openclaw side reports ok with Aborted=true
			Model:       "mock-" + p.AgentName,
			DurationMS:  1800000,
			Aborted:     true,
			ReviewProse: "Tool failed", // truncated/abandoned output
			RawJSON:     `{"runId":"x","aborted":true}`,
		}, nil
	})

	cmd := newMultiReviewCmd()
	cmd.SetArgs([]string{
		"--reviewers", "kai",
		"--repo", repo,
		"--openclaw-host", "user@example.lan",
		"--output-dir", outDir,
		"--base", "v1",
		"--timeout-seconds", "30",
	})
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(new(bytes.Buffer))
	// All reviewers aborted → exit non-zero (treated like all-fail).
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected non-zero exit when sole reviewer is aborted with no findings")
	}

	summaryData, err := os.ReadFile(filepath.Join(outDir, "multi-review-summary.json"))
	if err != nil {
		t.Fatal(err)
	}
	var s struct {
		Reviewers []struct {
			Agent  string `json:"agent"`
			Status string `json:"status"`
		} `json:"reviewers"`
	}
	if err := json.Unmarshal(summaryData, &s); err != nil {
		t.Fatal(err)
	}
	if len(s.Reviewers) != 1 {
		t.Fatalf("expected 1 reviewer in summary, got %d", len(s.Reviewers))
	}
	if s.Reviewers[0].Status != "aborted" {
		t.Errorf("aborted reviewer should have status=aborted, got %q", s.Reviewers[0].Status)
	}
}

func TestMultiReview_AbortedReviewerCountedAsNotOk(t *testing.T) {
	// Mixed pool: one clean reviewer, one aborted. The aborted one should
	// NOT count toward okCount, so partial:true should fire and the merged
	// stream should include only the clean reviewer's findings.
	repo := initFixtureRepoMR(t)
	outDir := filepath.Join(t.TempDir(), "out")

	withMockShipBundle(t)
	withMockPreComputeDiff(t)
	withMockInvoker(t, func(ctx context.Context, p multireview.InvokeReviewerParams) (multireview.InvokeReviewerResult, error) {
		if p.AgentName == "kai" {
			return multireview.InvokeReviewerResult{
				AgentName:   p.AgentName,
				Status:      "ok",
				Model:       "mock-kai",
				DurationMS:  1800000,
				Aborted:     true,
				ReviewProse: "Tool failed",
			}, nil
		}
		return mockResultFor(p.AgentName, "MEDIUM|src/a.go:1|p|x|cat"), nil
	})

	cmd := newMultiReviewCmd()
	cmd.SetArgs([]string{
		"--reviewers", "bruce,kai",
		"--repo", repo,
		"--openclaw-host", "user@example.lan",
		"--output-dir", outDir,
		"--base", "v1",
		"--timeout-seconds", "30",
	})
	cmd.SetOut(new(bytes.Buffer))
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v (mixed pool with one aborted should not be fatal)", err)
	}

	summaryData, err := os.ReadFile(filepath.Join(outDir, "multi-review-summary.json"))
	if err != nil {
		t.Fatal(err)
	}
	var s struct {
		Partial   bool `json:"partial"`
		Reviewers []struct {
			Agent  string `json:"agent"`
			Status string `json:"status"`
		} `json:"reviewers"`
	}
	if err := json.Unmarshal(summaryData, &s); err != nil {
		t.Fatal(err)
	}
	if !s.Partial {
		t.Error("mixed pool with one aborted should set partial:true")
	}
	statusByAgent := make(map[string]string)
	for _, r := range s.Reviewers {
		statusByAgent[r.Agent] = r.Status
	}
	if statusByAgent["kai"] != "aborted" {
		t.Errorf("kai should be aborted, got %q", statusByAgent["kai"])
	}
	if statusByAgent["bruce"] != "ok" {
		t.Errorf("bruce should be ok, got %q", statusByAgent["bruce"])
	}
}

// ---- per-agent prompt template tests ----

// withPromptDir writes the given (filename → content) entries into a temp
// dir and points LLM_TOOLS_MULTI_REVIEW_PROMPTS at it for the duration of
// the test. The dir is automatically cleaned up.
func withPromptDir(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}
	t.Setenv("LLM_TOOLS_MULTI_REVIEW_PROMPTS", dir)
	return dir
}

func TestMultiReview_PerAgentPromptIsLoaded(t *testing.T) {
	// When ~/.llm-tools/multi-review/prompts/<agent>.md exists, the rendered
	// content of that file is sent as the task message — not the hardcoded
	// buildDefaultTaskMessage output.
	repo := initFixtureRepoMR(t)
	outDir := filepath.Join(t.TempDir(), "out")
	withPromptDir(t, map[string]string{
		"_base.md": "BASE for {{.AgentName}}",
		"bruce.md": "PER-AGENT bruce prompt — diff at {{.DiffPath}}, repo {{.RemoteRepo}}",
	})

	withMockShipBundle(t)
	withMockPreComputeDiff(t)

	var captured = map[string]string{}
	var capturedMu sync.Mutex
	withMockInvoker(t, func(ctx context.Context, p multireview.InvokeReviewerParams) (multireview.InvokeReviewerResult, error) {
		capturedMu.Lock()
		captured[p.AgentName] = p.TaskMessage
		capturedMu.Unlock()
		return mockResultFor(p.AgentName, "LOW|f:1|p|x|c"), nil
	})

	cmd := newMultiReviewCmd()
	cmd.SetArgs([]string{
		"--reviewers", "bruce,greta",
		"--repo", repo,
		"--openclaw-host", "user@example.lan",
		"--output-dir", outDir,
		"--base", "v1",
		"--timeout-seconds", "30",
	})
	cmd.SetOut(new(bytes.Buffer))
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	// Bruce: has a per-agent file. Should see the bruce.md content (rendered).
	if !strings.Contains(captured["bruce"], "PER-AGENT bruce prompt") {
		t.Errorf("bruce should get per-agent file content, got: %q", captured["bruce"])
	}
	if !strings.Contains(captured["bruce"], "/diff.txt") {
		t.Errorf("bruce per-agent template should render DiffPath, got: %q", captured["bruce"])
	}

	// Greta: no greta.md, only _base.md. Should see the _base.md content.
	if !strings.Contains(captured["greta"], "BASE for greta") {
		t.Errorf("greta should fall back to _base.md, got: %q", captured["greta"])
	}
	if strings.Contains(captured["greta"], "PER-AGENT bruce") {
		t.Errorf("greta must not see bruce's per-agent content, got: %q", captured["greta"])
	}
}

func TestMultiReview_TaskMessageFlagBeatsPerAgentPrompt(t *testing.T) {
	// --task-message is the explicit user override and wins over per-agent
	// files. Same message goes to every reviewer.
	repo := initFixtureRepoMR(t)
	outDir := filepath.Join(t.TempDir(), "out")
	withPromptDir(t, map[string]string{
		"_base.md": "BASE",
		"bruce.md": "PER-AGENT bruce",
		"greta.md": "PER-AGENT greta",
	})

	withMockShipBundle(t)
	withMockPreComputeDiff(t)

	var captured = map[string]string{}
	var capturedMu sync.Mutex
	withMockInvoker(t, func(ctx context.Context, p multireview.InvokeReviewerParams) (multireview.InvokeReviewerResult, error) {
		capturedMu.Lock()
		captured[p.AgentName] = p.TaskMessage
		capturedMu.Unlock()
		return mockResultFor(p.AgentName, "LOW|f:1|p|x|c"), nil
	})

	cmd := newMultiReviewCmd()
	cmd.SetArgs([]string{
		"--reviewers", "bruce,greta",
		"--repo", repo,
		"--openclaw-host", "user@example.lan",
		"--output-dir", outDir,
		"--base", "v1",
		"--timeout-seconds", "30",
		"--task-message", "CLI OVERRIDE - same for everyone",
	})
	cmd.SetOut(new(bytes.Buffer))
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	for _, agent := range []string{"bruce", "greta"} {
		if captured[agent] != "CLI OVERRIDE - same for everyone" {
			t.Errorf("%s should get CLI override, got: %q", agent, captured[agent])
		}
	}
}

func TestMultiReview_FallsBackToHardcodedWhenNoPrompts(t *testing.T) {
	// When neither <agent>.md nor _base.md exists (empty prompts dir),
	// the binary falls back to buildDefaultTaskMessage and the reviewer
	// still gets a valid task message. Critical for fresh installs that
	// haven't run update-prompts.sh yet — nothing must break.
	repo := initFixtureRepoMR(t)
	outDir := filepath.Join(t.TempDir(), "out")
	// Point env at a temp dir with NO files.
	withPromptDir(t, map[string]string{})

	withMockShipBundle(t)
	withMockPreComputeDiff(t)

	var captured = map[string]string{}
	var capturedMu sync.Mutex
	withMockInvoker(t, func(ctx context.Context, p multireview.InvokeReviewerParams) (multireview.InvokeReviewerResult, error) {
		capturedMu.Lock()
		captured[p.AgentName] = p.TaskMessage
		capturedMu.Unlock()
		return mockResultFor(p.AgentName, "LOW|f:1|p|x|c"), nil
	})

	cmd := newMultiReviewCmd()
	cmd.SetArgs([]string{
		"--reviewers", "bruce",
		"--repo", repo,
		"--openclaw-host", "user@example.lan",
		"--output-dir", outDir,
		"--base", "v1",
		"--timeout-seconds", "30",
	})
	cmd.SetOut(new(bytes.Buffer))
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	// Hardcoded buildDefaultTaskMessage has these signature strings.
	if !strings.Contains(captured["bruce"], "A pre-computed unified diff is at:") {
		t.Errorf("bruce should get hardcoded fallback, got: %q", captured["bruce"])
	}
	if !strings.Contains(captured["bruce"], "INSTRUCTIONS — follow exactly:") {
		t.Errorf("hardcoded fallback should have INSTRUCTIONS header, got: %q", captured["bruce"])
	}
}

func TestMultiReview_FallsBackToHardcodedWhenDirMissing(t *testing.T) {
	// When LLM_TOOLS_MULTI_REVIEW_PROMPTS points at a non-existent dir,
	// loader returns empty and binary uses hardcoded fallback.
	repo := initFixtureRepoMR(t)
	outDir := filepath.Join(t.TempDir(), "out")
	t.Setenv("LLM_TOOLS_MULTI_REVIEW_PROMPTS", "/nonexistent/dir/never-created")

	withMockShipBundle(t)
	withMockPreComputeDiff(t)

	var captured = map[string]string{}
	var capturedMu sync.Mutex
	withMockInvoker(t, func(ctx context.Context, p multireview.InvokeReviewerParams) (multireview.InvokeReviewerResult, error) {
		capturedMu.Lock()
		captured[p.AgentName] = p.TaskMessage
		capturedMu.Unlock()
		return mockResultFor(p.AgentName, "LOW|f:1|p|x|c"), nil
	})

	cmd := newMultiReviewCmd()
	cmd.SetArgs([]string{
		"--reviewers", "bruce",
		"--repo", repo,
		"--openclaw-host", "user@example.lan",
		"--output-dir", outDir,
		"--base", "v1",
		"--timeout-seconds", "30",
	})
	cmd.SetOut(new(bytes.Buffer))
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(captured["bruce"], "A pre-computed unified diff is at:") {
		t.Errorf("missing dir should fall back to hardcoded, got: %q", captured["bruce"])
	}
}

// ---- container-routing tests ----

// withCaptureSSHRun swaps the sshRunFn used for cleanup so tests can assert
// on which paths the run tried to rm -rf at teardown.
func withCaptureSSHRun(t *testing.T, captured *[]string) {
	t.Helper()
	orig := sshRunFn
	sshRunFn = func(ctx context.Context, p multireview.SSHParams) (multireview.SSHResult, error) {
		*captured = append(*captured, p.Command)
		return multireview.SSHResult{}, nil
	}
	t.Cleanup(func() { sshRunFn = orig })
}

// withCaptureContainerExec swaps containerExecFn for capturing cleanup
// commands routed inside the container.
func withCaptureContainerExec(t *testing.T, captured *[]string) {
	t.Helper()
	orig := containerExecFn
	containerExecFn = func(ctx context.Context, p multireview.ContainerExecParams) (multireview.SSHResult, error) {
		*captured = append(*captured, p.Command)
		return multireview.SSHResult{}, nil
	}
	t.Cleanup(func() { containerExecFn = orig })
}

func TestMultiReview_GatewayContainerFlagsThreaded(t *testing.T) {
	// --gateway-container value must reach BOTH ShipBundleParams and
	// PreComputeDiffParams. We capture each via the mock function pointers.
	repo := initFixtureRepoMR(t)
	outDir := filepath.Join(t.TempDir(), "out")

	var capturedShip, capturedDiff string
	origShip := shipBundleFn
	shipBundleFn = func(ctx context.Context, p multireview.ShipBundleParams) (multireview.ShipBundleResult, error) {
		capturedShip = p.GatewayContainer
		return multireview.ShipBundleResult{
			RemoteRepoPath:        "/tmp/work/" + p.RepoName,
			HostStagingBundlePath: "/tmp/stage/bundle.git",
			BundleSize:            1,
		}, nil
	}
	t.Cleanup(func() { shipBundleFn = origShip })

	origDiff := preComputeDiffFn
	preComputeDiffFn = func(ctx context.Context, p multireview.PreComputeDiffParams) (multireview.PreComputeDiffResult, error) {
		capturedDiff = p.GatewayContainer
		return multireview.PreComputeDiffResult{DiffPath: p.RemoteWorkdir + "/diff.txt"}, nil
	}
	t.Cleanup(func() { preComputeDiffFn = origDiff })

	withMockInvoker(t, func(ctx context.Context, p multireview.InvokeReviewerParams) (multireview.InvokeReviewerResult, error) {
		return mockResultFor(p.AgentName, "LOW|f:1|p|x|c"), nil
	})

	cmd := newMultiReviewCmd()
	cmd.SetArgs([]string{
		"--reviewers", "bruce",
		"--repo", repo,
		"--openclaw-host", "user@example.lan",
		"--output-dir", outDir,
		"--base", "v1",
		"--gateway-container", "my-custom-container",
		"--timeout-seconds", "30",
	})
	cmd.SetOut(new(bytes.Buffer))
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if capturedShip != "my-custom-container" {
		t.Errorf("ShipBundle got GatewayContainer=%q, want my-custom-container", capturedShip)
	}
	if capturedDiff != "my-custom-container" {
		t.Errorf("PreComputeDiff got GatewayContainer=%q, want my-custom-container", capturedDiff)
	}
}

func TestMultiReview_HostStagingDirThreadsToShipBundle(t *testing.T) {
	// HostStagingDir must be set by runMultiReview and include the run
	// timestamp so concurrent runs don't collide.
	repo := initFixtureRepoMR(t)
	outDir := filepath.Join(t.TempDir(), "out")

	var capturedStaging, capturedWorkdir string
	origShip := shipBundleFn
	shipBundleFn = func(ctx context.Context, p multireview.ShipBundleParams) (multireview.ShipBundleResult, error) {
		capturedStaging = p.HostStagingDir
		capturedWorkdir = p.RemoteWorkdir
		return multireview.ShipBundleResult{
			RemoteRepoPath:        p.RemoteWorkdir + "/" + p.RepoName,
			HostStagingBundlePath: p.HostStagingDir + "/bundle.git",
			BundleSize:            1,
		}, nil
	}
	t.Cleanup(func() { shipBundleFn = origShip })

	withMockPreComputeDiff(t)
	withMockInvoker(t, func(ctx context.Context, p multireview.InvokeReviewerParams) (multireview.InvokeReviewerResult, error) {
		return mockResultFor(p.AgentName, "LOW|f:1|p|x|c"), nil
	})

	cmd := newMultiReviewCmd()
	cmd.SetArgs([]string{
		"--reviewers", "bruce",
		"--repo", repo,
		"--openclaw-host", "user@example.lan",
		"--output-dir", outDir,
		"--base", "v1",
		"--timeout-seconds", "30",
	})
	cmd.SetOut(new(bytes.Buffer))
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if capturedStaging == "" {
		t.Fatal("HostStagingDir was empty")
	}
	if !strings.HasPrefix(capturedStaging, "/tmp/multi-review-staging-") {
		t.Errorf("HostStagingDir should start with /tmp/multi-review-staging-, got: %s", capturedStaging)
	}
	// Staging and workdir must be different paths to keep cleanup distinct.
	if capturedStaging == capturedWorkdir {
		t.Errorf("HostStagingDir and RemoteWorkdir must differ; both were %q", capturedStaging)
	}
}

func TestMultiReview_BothCleanupsFire(t *testing.T) {
	// On successful run, cleanup must remove BOTH:
	//   1. The container workdir (via docker exec rm -rf)
	//   2. The host staging dir (via raw ssh rm -rf)
	repo := initFixtureRepoMR(t)
	outDir := filepath.Join(t.TempDir(), "out")

	withMockShipBundle(t)
	withMockPreComputeDiff(t)
	withMockInvoker(t, func(ctx context.Context, p multireview.InvokeReviewerParams) (multireview.InvokeReviewerResult, error) {
		return mockResultFor(p.AgentName, "LOW|f:1|p|x|c"), nil
	})

	var sshCommands, containerCommands []string
	withCaptureSSHRun(t, &sshCommands)
	withCaptureContainerExec(t, &containerCommands)

	cmd := newMultiReviewCmd()
	cmd.SetArgs([]string{
		"--reviewers", "bruce",
		"--repo", repo,
		"--openclaw-host", "user@example.lan",
		"--output-dir", outDir,
		"--base", "v1",
		"--timeout-seconds", "30",
	})
	cmd.SetOut(new(bytes.Buffer))
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	// Container cleanup: at least one container-exec call should be `rm -rf`
	// targeting a /tmp/multi-review-<ts> path (not the staging prefix).
	foundContainerRm := false
	for _, c := range containerCommands {
		if strings.Contains(c, "rm -rf") && strings.Contains(c, "/tmp/multi-review-") && !strings.Contains(c, "staging") {
			foundContainerRm = true
			break
		}
	}
	if !foundContainerRm {
		t.Errorf("expected container exec rm -rf for the workdir; got: %v", containerCommands)
	}

	// Host cleanup: at least one raw ssh call should be `rm -rf` targeting
	// the staging dir.
	foundHostRm := false
	for _, c := range sshCommands {
		if strings.Contains(c, "rm -rf") && strings.Contains(c, "/tmp/multi-review-staging-") {
			foundHostRm = true
			break
		}
	}
	if !foundHostRm {
		t.Errorf("expected raw ssh rm -rf for the host staging dir; got: %v", sshCommands)
	}
}

func TestMultiReview_CleanupFiresOnShipBundleFailure(t *testing.T) {
	// If ShipBundle creates the container workdir then fails (e.g. clone
	// errors after mkdir succeeds), we'd leak overlay /tmp space inside the
	// container. The defers must be registered BEFORE ShipBundle is called
	// so cleanup runs even on partial-state failure. rm -rf on a path that
	// was never created is a harmless no-op.
	repo := initFixtureRepoMR(t)
	outDir := filepath.Join(t.TempDir(), "out")

	origShip := shipBundleFn
	shipBundleFn = func(ctx context.Context, p multireview.ShipBundleParams) (multireview.ShipBundleResult, error) {
		return multireview.ShipBundleResult{}, fmt.Errorf("ship: container clone exit 128, stderr: fatal: clone destination exists")
	}
	t.Cleanup(func() { shipBundleFn = origShip })

	var sshCommands, containerCommands []string
	withCaptureSSHRun(t, &sshCommands)
	withCaptureContainerExec(t, &containerCommands)

	cmd := newMultiReviewCmd()
	cmd.SetArgs([]string{
		"--reviewers", "bruce",
		"--repo", repo,
		"--openclaw-host", "user@example.lan",
		"--output-dir", outDir,
		"--base", "v1",
		"--timeout-seconds", "30",
	})
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(new(bytes.Buffer))
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error from ship failure")
	}

	// Even on hard-stop, both cleanup channels must have fired.
	foundContainerRm := false
	for _, c := range containerCommands {
		if strings.Contains(c, "rm -rf") && strings.Contains(c, "/tmp/multi-review-") && !strings.Contains(c, "staging") {
			foundContainerRm = true
			break
		}
	}
	if !foundContainerRm {
		t.Errorf("container rm -rf should fire even on ship failure; got: %v", containerCommands)
	}
	foundHostRm := false
	for _, c := range sshCommands {
		if strings.Contains(c, "rm -rf") && strings.Contains(c, "/tmp/multi-review-staging-") {
			foundHostRm = true
			break
		}
	}
	if !foundHostRm {
		t.Errorf("host staging rm -rf should fire even on ship failure; got: %v", sshCommands)
	}
}

func TestMultiReview_SkipCleanupSkipsBoth(t *testing.T) {
	repo := initFixtureRepoMR(t)
	outDir := filepath.Join(t.TempDir(), "out")

	withMockShipBundle(t)
	withMockPreComputeDiff(t)
	withMockInvoker(t, func(ctx context.Context, p multireview.InvokeReviewerParams) (multireview.InvokeReviewerResult, error) {
		return mockResultFor(p.AgentName, "LOW|f:1|p|x|c"), nil
	})

	var sshCommands, containerCommands []string
	withCaptureSSHRun(t, &sshCommands)
	withCaptureContainerExec(t, &containerCommands)

	cmd := newMultiReviewCmd()
	cmd.SetArgs([]string{
		"--reviewers", "bruce",
		"--repo", repo,
		"--openclaw-host", "user@example.lan",
		"--output-dir", outDir,
		"--base", "v1",
		"--skip-cleanup",
		"--timeout-seconds", "30",
	})
	cmd.SetOut(new(bytes.Buffer))
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	// Neither cleanup channel should have any `rm -rf` invocation.
	for _, c := range containerCommands {
		if strings.Contains(c, "rm -rf") {
			t.Errorf("--skip-cleanup should suppress container rm -rf, got: %s", c)
		}
	}
	for _, c := range sshCommands {
		if strings.Contains(c, "rm -rf") {
			t.Errorf("--skip-cleanup should suppress host rm -rf, got: %s", c)
		}
	}
}

func TestMultiReview_SmallDiffNoWarning(t *testing.T) {
	// SizeBytes < 1_000_000 should NOT append the --stat hint.
	// Test isolation: point LLM_TOOLS_MULTI_REVIEW_PROMPTS at an empty temp
	// dir so the loader falls back to the hardcoded message (the only place
	// this test's "--stat" assertion is meaningful — per-agent template files
	// the user has installed locally may mention --stat in their operational
	// rules unconditionally).
	t.Setenv("LLM_TOOLS_MULTI_REVIEW_PROMPTS", t.TempDir())

	repo := initFixtureRepoMR(t)
	outDir := filepath.Join(t.TempDir(), "out")

	withMockShipBundle(t)
	withMockPreComputeDiff(t) // default mock: 1234 bytes

	var capturedTaskMessage string
	withMockInvoker(t, func(ctx context.Context, p multireview.InvokeReviewerParams) (multireview.InvokeReviewerResult, error) {
		capturedTaskMessage = p.TaskMessage
		return mockResultFor(p.AgentName, "LOW|f:1|p|x|c"), nil
	})

	cmd := newMultiReviewCmd()
	cmd.SetArgs([]string{
		"--reviewers", "bruce",
		"--repo", repo,
		"--openclaw-host", "user@example.lan",
		"--output-dir", outDir,
		"--base", "v1",
		"--timeout-seconds", "30",
	})
	cmd.SetOut(new(bytes.Buffer))
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if strings.Contains(capturedTaskMessage, "--stat") {
		t.Errorf("task message should not include --stat hint for small diff. Got:\n%s", capturedTaskMessage)
	}
}
