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
			LocalBundlePath:  "/tmp/mock-bundle.git",
			RemoteBundlePath: "/tmp/mock-remote/bundle.git",
			RemoteRepoPath:   "/tmp/mock-remote/" + p.RepoName,
			BundleSize:       1024,
		}, nil
	}
	t.Cleanup(func() { shipBundleFn = orig })
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
	withMockInvoker(t, func(ctx context.Context, p multireview.InvokeReviewerParams) (multireview.InvokeReviewerResult, error) {
		return mockResultFor(p.AgentName, "MEDIUM|src/a.go:1|test problem|test fix|robustness"), nil
	})

	cmd := newMultiReviewCmd()
	cmd.SetArgs([]string{
		"--reviewers", "bruce,greta",
		"--repo", repo,
		"--openclaw-host", "user@example.lan",
		"--output-dir", outDir,
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
	merged := filepath.Join(outDir, "td-stream-all.txt")
	if _, err := os.Stat(merged); err != nil {
		t.Errorf("missing merged: %v", err)
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
	withMockInvoker(t, func(ctx context.Context, p multireview.InvokeReviewerParams) (multireview.InvokeReviewerResult, error) {
		return multireview.InvokeReviewerResult{AgentName: p.AgentName}, fmt.Errorf("simulated failure")
	})

	cmd := newMultiReviewCmd()
	cmd.SetArgs([]string{
		"--reviewers", "bruce",
		"--repo", repo,
		"--openclaw-host", "user@example.lan",
		"--output-dir", outDir,
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
		"--timeout-seconds", "30",
	})
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(new(bytes.Buffer))
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected hard-stop on ship failure")
	}
}
