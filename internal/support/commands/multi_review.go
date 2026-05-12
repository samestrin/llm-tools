package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/samestrin/llm-tools/internal/support/multireview"
	"github.com/spf13/cobra"
)

// invokeReviewerFn and shipBundleFn are package-level function variables that
// tests swap to inject deterministic behavior. Production points them at the
// real multireview package functions.
var (
	invokeReviewerFn = multireview.InvokeReviewer
	shipBundleFn     = multireview.ShipBundle
)

// Flag variables for the multi_review command.
var (
	mrReviewers        string
	mrSerialReviewers  string
	mrRepo             string
	mrBaseRef          string
	mrHeadRef          string
	mrOpenclawHost     string
	mrOutputDir        string
	mrTimeoutSeconds   int
	mrPerReviewerTO    int
	mrGatewayContainer string
	mrTaskMessage      string
	mrSkipCleanup      bool
)

// ReviewerStatus is one entry in the multi-review summary report.
type ReviewerStatus struct {
	Agent       string `json:"agent"`
	Model       string `json:"model,omitempty"`
	Status      string `json:"status"` // "ok" | "failed" | "skipped"
	DurationMS  int64  `json:"durationMs,omitempty"`
	TDLineCount int    `json:"tdLineCount"`
	Error       string `json:"error,omitempty"`
}

// MultiReviewSummary is what gets written to multi-review-summary.json.
type MultiReviewSummary struct {
	Reviewers       []ReviewerStatus `json:"reviewers"`
	TotalFindings   int              `json:"totalFindings"`
	Partial         bool             `json:"partial"`
	TotalDurationMS int64            `json:"totalDurationMs"`
	BundleSize      int64            `json:"bundleSize"`
	RemoteRepoPath  string           `json:"remoteRepoPath"`
	Timestamp       string           `json:"timestamp"`
}

func newMultiReviewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "multi_review",
		Short: "Fan out a code review to multiple openclaw reviewer agents",
		Long: `Bundle a local repo, ship it to an openclaw-hosting machine, and invoke
several reviewer agents in parallel (or in a serial lane for those that
share rate-limited providers). Collects per-reviewer findings and writes
a merged TD stream the /code-review command can consume.

Output layout:
  <output-dir>/raw/<agent>/{review.md,td-stream.txt,status.json,response.json}
  <output-dir>/td-stream-all.txt           (merged + reviewer-attributed)
  <output-dir>/multi-review-summary.json   (per-reviewer status + counts)

Failure semantics:
  - Bundle/ship failure  → hard-stop (no point invoking reviewers without
    the diff staged on the remote)
  - Per-reviewer failure → recorded as failed in summary; other reviewers
    continue; exit 0 with partial: true
  - All reviewers fail   → exit 1 with summary of what failed`,
		RunE: runMultiReview,
	}
	cmd.Flags().StringVar(&mrReviewers, "reviewers", "", "Comma-separated reviewer agent names (required)")
	cmd.Flags().StringVar(&mrSerialReviewers, "serial-reviewers", "", "Comma-separated subset that runs serially after the parallel lane")
	cmd.Flags().StringVar(&mrRepo, "repo", "", "Local repo path to bundle (required)")
	cmd.Flags().StringVar(&mrBaseRef, "base", "", "Base ref for the diff range (informational, included in task message)")
	cmd.Flags().StringVar(&mrHeadRef, "head", "HEAD", "Head ref for the diff range")
	cmd.Flags().StringVar(&mrOpenclawHost, "openclaw-host", "", "SSH target running openclaw-gateway (required)")
	cmd.Flags().StringVar(&mrOutputDir, "output-dir", "", "Where per-reviewer artifacts and merged stream land (required)")
	cmd.Flags().IntVar(&mrTimeoutSeconds, "timeout-seconds", 1200, "Total wall-clock budget for the entire fan-out")
	cmd.Flags().IntVar(&mrPerReviewerTO, "per-reviewer-timeout-seconds", 600, "Per-reviewer soft timeout")
	cmd.Flags().StringVar(&mrGatewayContainer, "gateway-container", "openclaw-gateway", "Docker container running openclaw")
	cmd.Flags().StringVar(&mrTaskMessage, "task-message", "", "Override the task message sent to each reviewer; default is auto-built from --base/--head/--repo")
	cmd.Flags().BoolVar(&mrSkipCleanup, "skip-cleanup", false, "Do not remove the remote workdir after running")
	return cmd
}

func init() {
	RootCmd.AddCommand(newMultiReviewCmd())
}

func runMultiReview(cmd *cobra.Command, _ []string) error {
	// Flag validation
	if mrReviewers == "" {
		return fmt.Errorf("--reviewers required")
	}
	if mrRepo == "" {
		return fmt.Errorf("--repo required")
	}
	if mrOpenclawHost == "" {
		return fmt.Errorf("--openclaw-host required")
	}
	if mrOutputDir == "" {
		return fmt.Errorf("--output-dir required")
	}

	allReviewers := splitAndTrim(mrReviewers)
	serial := splitAndTrim(mrSerialReviewers)
	if len(allReviewers) == 0 {
		return fmt.Errorf("--reviewers must list at least one agent")
	}
	parallel := subtract(allReviewers, serial)

	rawDir := filepath.Join(mrOutputDir, "raw")
	if err := os.MkdirAll(rawDir, 0o755); err != nil {
		return fmt.Errorf("mkdir output: %w", err)
	}

	start := time.Now()
	totalCtx, cancelTotal := context.WithTimeout(context.Background(), time.Duration(mrTimeoutSeconds)*time.Second)
	defer cancelTotal()

	// 1. Ship the bundle. Hard-stop on failure.
	repoName := filepath.Base(strings.TrimRight(mrRepo, "/"))
	remoteWorkdir := fmt.Sprintf("/tmp/multi-review-%d", start.Unix())
	shipRes, err := shipBundleFn(totalCtx, multireview.ShipBundleParams{
		LocalRepo:     mrRepo,
		Host:          mrOpenclawHost,
		RemoteWorkdir: remoteWorkdir,
		RepoName:      repoName,
		Timeout:       time.Duration(mrTimeoutSeconds) * time.Second,
	})
	if err != nil {
		return fmt.Errorf("ship bundle to %s: %w", mrOpenclawHost, err)
	}
	defer func() {
		if !mrSkipCleanup {
			// Best-effort teardown of the remote workdir.
			cleanupCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			_, _ = multireview.SSHRun(cleanupCtx, multireview.SSHParams{
				Host:    mrOpenclawHost,
				Command: "rm -rf " + shellQuote(remoteWorkdir),
				Timeout: 30 * time.Second,
			})
		}
	}()

	// 2. Build the task message (auto if not overridden).
	taskMessage := mrTaskMessage
	if taskMessage == "" {
		taskMessage = buildDefaultTaskMessage(shipRes.RemoteRepoPath, repoName, mrBaseRef, mrHeadRef)
	}

	// 3. Invoke reviewers. Parallel lane first, then serial.
	statuses := make([]ReviewerStatus, 0, len(allReviewers))
	statusByAgent := make(map[string]ReviewerStatus)
	var statusMu sync.Mutex

	invokeOne := func(agent string) {
		res, err := invokeReviewerFn(totalCtx, multireview.InvokeReviewerParams{
			Host:             mrOpenclawHost,
			AgentName:        agent,
			TaskMessage:      taskMessage,
			Timeout:          time.Duration(mrPerReviewerTO) * time.Second,
			GatewayContainer: mrGatewayContainer,
		})
		st := ReviewerStatus{Agent: agent}
		if err != nil {
			st.Status = "failed"
			st.Error = err.Error()
		} else {
			_, werr := multireview.WriteReviewerOutput(rawDir, res)
			st.Model = res.Model
			st.DurationMS = res.DurationMS
			st.TDLineCount = len(multireview.ExtractTDLines(res.ReviewProse))
			if werr != nil {
				st.Status = "failed"
				st.Error = werr.Error()
			} else {
				st.Status = "ok"
			}
		}
		statusMu.Lock()
		statusByAgent[agent] = st
		statusMu.Unlock()
	}

	// Parallel lane
	if len(parallel) > 0 {
		var wg sync.WaitGroup
		for _, agent := range parallel {
			wg.Add(1)
			go func(a string) {
				defer wg.Done()
				invokeOne(a)
			}(agent)
		}
		wg.Wait()
	}

	// Serial lane (sequential after parallel completes)
	for _, agent := range serial {
		invokeOne(agent)
	}

	// 4. Preserve order from --reviewers in the final summary
	okCount := 0
	for _, agent := range allReviewers {
		s := statusByAgent[agent]
		statuses = append(statuses, s)
		if s.Status == "ok" {
			okCount++
		}
	}

	// 5. Merge streams from successful reviewers only
	successAgents := make([]string, 0)
	for _, s := range statuses {
		if s.Status == "ok" {
			successAgents = append(successAgents, s.Agent)
		}
	}
	totalFindings := 0
	if len(successAgents) > 0 {
		_, n, err := multireview.MergeStreams(rawDir, successAgents)
		if err != nil {
			return fmt.Errorf("merge streams: %w", err)
		}
		totalFindings = n
		// Also write the merged stream up one level as td-stream.txt so
		// /reconcile-code-review's auto-discovery (any code-review/<source>/
		// dir with a td-stream.txt is a source) treats multi-agent as one
		// unified source rather than peeking into raw/<agent>/.
		srcMerged := filepath.Join(rawDir, "td-stream-all.txt")
		dstMerged := filepath.Join(mrOutputDir, "td-stream.txt")
		if data, err := os.ReadFile(srcMerged); err == nil {
			_ = os.WriteFile(dstMerged, data, 0o644)
		}
	}

	// 6. Write summary
	summary := MultiReviewSummary{
		Reviewers:       statuses,
		TotalFindings:   totalFindings,
		Partial:         okCount > 0 && okCount < len(allReviewers),
		TotalDurationMS: time.Since(start).Milliseconds(),
		BundleSize:      shipRes.BundleSize,
		RemoteRepoPath:  shipRes.RemoteRepoPath,
		Timestamp:       start.UTC().Format(time.RFC3339),
	}
	summaryPath := filepath.Join(mrOutputDir, "multi-review-summary.json")
	summaryBytes, _ := json.MarshalIndent(summary, "", "  ")
	if err := os.WriteFile(summaryPath, summaryBytes, 0o644); err != nil {
		return fmt.Errorf("write summary: %w", err)
	}

	// Human-readable output to stdout (cmd.OutOrStdout())
	fmt.Fprintf(cmd.OutOrStdout(),
		"multi_review: %d/%d reviewers succeeded, %d findings, total %s\n",
		okCount, len(allReviewers), totalFindings, time.Since(start).Round(time.Second),
	)
	for _, s := range statuses {
		fmt.Fprintf(cmd.OutOrStdout(), "  %-10s %-6s %3d findings  %s\n",
			s.Agent, s.Status, s.TDLineCount, s.Model)
		if s.Error != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "             err: %s\n", s.Error)
		}
	}

	if okCount == 0 {
		return fmt.Errorf("all reviewers failed (no successful runs); see %s", summaryPath)
	}
	return nil
}

func buildDefaultTaskMessage(remoteRepo, repoName, base, head string) string {
	if head == "" {
		head = "HEAD"
	}
	var b strings.Builder
	b.WriteString("Code review.\n\n")
	b.WriteString(fmt.Sprintf("Repository: a fresh clone of %s is on this host at %s/\n\n", repoName, remoteRepo))
	if base != "" {
		b.WriteString(fmt.Sprintf("Range to review: %s..%s\n\n", base, head))
		b.WriteString("To see the diff:\n")
		b.WriteString(fmt.Sprintf("  cd %s && git diff %s..%s\n\n", remoteRepo, base, head))
	} else {
		b.WriteString(fmt.Sprintf("Working tree at %s — review the current state.\n\n", remoteRepo))
	}
	b.WriteString(`Produce your normal review report (verdict + severity-graded findings + what was done well + out-of-scope).
Reply with the review body only — no preamble.

After your normal review, append a section titled "TD_STREAM" with each finding as a single pipe-delimited line in this format:

  SEVERITY|FILE:LINE|PROBLEM|FIX|CATEGORY

Where SEVERITY is HIGH/MEDIUM/LOW (map blocking->HIGH, significant->MEDIUM, minor->LOW). One line per finding. No header row, no commentary in this section.
`)
	return b.String()
}

func splitAndTrim(csv string) []string {
	if csv == "" {
		return nil
	}
	parts := strings.Split(csv, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		s := strings.TrimSpace(p)
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

func subtract(all, remove []string) []string {
	rm := make(map[string]bool, len(remove))
	for _, r := range remove {
		rm[r] = true
	}
	out := make([]string, 0, len(all))
	for _, a := range all {
		if !rm[a] {
			out = append(out, a)
		}
	}
	return out
}

// shellQuote here mirrors multireview.shellQuote — kept inline to avoid
// exposing the helper from that package.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
