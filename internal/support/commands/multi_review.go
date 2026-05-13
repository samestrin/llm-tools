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

// invokeReviewerFn, shipBundleFn, preComputeDiffFn, sshRunFn, and
// containerExecFn are package-level function variables that tests swap to
// inject deterministic behavior. Production points them at the real
// multireview package functions.
//
// sshRunFn and containerExecFn cover the two cleanup channels (host staging
// dir via raw ssh, container workdir via docker exec) so tests can assert
// both fire on success and neither fires under --skip-cleanup.
var (
	invokeReviewerFn = multireview.InvokeReviewer
	shipBundleFn     = multireview.ShipBundle
	preComputeDiffFn = multireview.PreComputeDiff
	sshRunFn         = multireview.SSHRun
	containerExecFn  = multireview.ContainerExec
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
		Long: `Bundle a local repo, ship it INTO the openclaw-gateway container on
the remote host, pre-compute the diff once inside the container, and
invoke several reviewer agents in parallel (or in a serial lane for
those that share rate-limited providers). Collects per-reviewer findings
and writes a merged TD stream the /code-review command can consume.

Container routing: filesystem ops route through 'docker exec' because
the gateway container has no /tmp bind mount from the host. The bundle
lands briefly on the host as a staging scp pad, then 'docker cp' moves
it into the container where the clone and pre-computed diff live —
where reviewers (also inside the container) can actually read them.

Pre-compute step: --base is REQUIRED. After the bundle is shipped, we
run 'git diff <base>..<head>' inside the container and write the
result to <container-workdir>/diff.txt. Reviewers are told to 'cat'
that file rather than running git themselves — observed in production,
weaker reviewers hallucinate "clone missing" failures rather than
persisting through a multi-step git invocation.

Output layout:
  <output-dir>/raw/<agent>/{review.md,td-stream.txt,status.json,response.json}
  <output-dir>/raw/td-stream-all.txt        (cross-reviewer merge, inside raw/)
  <output-dir>/td-stream.txt                (same merged content at root, where
                                             /reconcile-code-review auto-discovers
                                             this directory as one unified source)
  <output-dir>/multi-review-summary.json    (per-reviewer status + counts)

Failure semantics:
  - --base missing       → validation error, no remote calls
  - Bundle/ship failure  → hard-stop (no point invoking reviewers without
    the diff staged on the remote)
  - Diff pre-compute failure (bad ref, etc.) → hard-stop before reviewers
    are invoked; surfaces git stderr in the error message
  - Per-reviewer failure → recorded as failed in summary; other reviewers
    continue; exit 0 with partial: true
  - All reviewers fail   → exit 1 with summary of what failed`,
		RunE: runMultiReview,
	}
	cmd.Flags().StringVar(&mrReviewers, "reviewers", "", "Comma-separated reviewer agent names (required)")
	cmd.Flags().StringVar(&mrSerialReviewers, "serial-reviewers", "", "Comma-separated subset that runs serially after the parallel lane")
	cmd.Flags().StringVar(&mrRepo, "repo", "", "Local repo path to bundle (required)")
	cmd.Flags().StringVar(&mrBaseRef, "base", "", "Base ref for the diff range — REQUIRED (we pre-compute git diff <base>..<head> on the remote)")
	cmd.Flags().StringVar(&mrHeadRef, "head", "HEAD", "Head ref for the diff range")
	cmd.Flags().StringVar(&mrOpenclawHost, "openclaw-host", "", "SSH target running openclaw-gateway (required)")
	cmd.Flags().StringVar(&mrOutputDir, "output-dir", "", "Where per-reviewer artifacts and merged stream land (required)")
	cmd.Flags().IntVar(&mrTimeoutSeconds, "timeout-seconds", 1200, "Total wall-clock budget for the entire fan-out")
	cmd.Flags().IntVar(&mrPerReviewerTO, "per-reviewer-timeout-seconds", 1200, "Per-reviewer soft timeout (default 1200s — observed 600s default timing out for real sprints)")
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
	if mrBaseRef == "" {
		return fmt.Errorf("--base required (working-tree mode is not supported — pass --base=<ref> so we can pre-compute the diff for reviewers)")
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
	//
	// remoteWorkdir is a path INSIDE the openclaw-gateway container — that's
	// where the clone and the pre-computed diff need to live, because the
	// container's /tmp is overlay-only (no bind mount from host /tmp).
	// hostStagingDir is on the SSH target host; it exists only briefly as the
	// scp landing pad before `docker cp` moves the bundle into the container.
	repoName := filepath.Base(strings.TrimRight(mrRepo, "/"))
	remoteWorkdir := fmt.Sprintf("/tmp/multi-review-%d", start.Unix())
	hostStagingDir := fmt.Sprintf("/tmp/multi-review-staging-%d", start.Unix())

	// Register cleanup BEFORE ShipBundle so partial-state failures
	// (container mkdir succeeds, then scp fails) don't leak. rm -rf on a
	// non-existent path is a harmless no-op; cleanup results are
	// best-effort and intentionally ignored.
	defer func() {
		if !mrSkipCleanup {
			cleanupCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			_, _ = containerExecFn(cleanupCtx, multireview.ContainerExecParams{
				Host:             mrOpenclawHost,
				GatewayContainer: mrGatewayContainer,
				Command:          "rm -rf " + shellQuote(remoteWorkdir),
				Timeout:          30 * time.Second,
			})
		}
	}()
	defer func() {
		if !mrSkipCleanup {
			cleanupCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			_, _ = sshRunFn(cleanupCtx, multireview.SSHParams{
				Host:    mrOpenclawHost,
				Command: "rm -rf " + shellQuote(hostStagingDir),
				Timeout: 30 * time.Second,
			})
		}
	}()

	shipRes, err := shipBundleFn(totalCtx, multireview.ShipBundleParams{
		LocalRepo:        mrRepo,
		Host:             mrOpenclawHost,
		GatewayContainer: mrGatewayContainer,
		RemoteWorkdir:    remoteWorkdir,
		HostStagingDir:   hostStagingDir,
		RepoName:         repoName,
		Timeout:          time.Duration(mrTimeoutSeconds) * time.Second,
	})
	if err != nil {
		return fmt.Errorf("ship bundle to %s: %w", mrOpenclawHost, err)
	}

	// 2. Pre-compute the diff inside the container so reviewers only need
	//    to `cat <diffPath>`. Removes the hallucination surface where weaker
	//    reviewers were inventing "clone missing" failures rather than
	//    running `git diff` themselves AND the container-can't-see-host bug
	//    that made the diff invisible in PR #30.
	diffRes, err := preComputeDiffFn(totalCtx, multireview.PreComputeDiffParams{
		Host:             mrOpenclawHost,
		GatewayContainer: mrGatewayContainer,
		RemoteRepoPath:   shipRes.RemoteRepoPath,
		RemoteWorkdir:    remoteWorkdir,
		BaseRef:          mrBaseRef,
		HeadRef:          mrHeadRef,
		Timeout:          time.Duration(mrTimeoutSeconds) * time.Second,
	})
	if err != nil {
		return fmt.Errorf("pre-compute diff on %s: %w", mrOpenclawHost, err)
	}

	// 3. Build the task message (auto if not overridden).
	taskMessage := mrTaskMessage
	if taskMessage == "" {
		taskMessage = buildDefaultTaskMessage(shipRes.RemoteRepoPath, repoName, mrBaseRef, mrHeadRef, diffRes)
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

func buildDefaultTaskMessage(remoteRepo, repoName, base, head string, diff multireview.PreComputeDiffResult) string {
	if head == "" {
		head = "HEAD"
	}
	var b strings.Builder
	b.WriteString("Code review.\n\n")
	b.WriteString(fmt.Sprintf("A pre-computed unified diff is at:\n  %s\n", diff.DiffPath))
	b.WriteString(fmt.Sprintf("Size: %d bytes (%d lines). Range: %s..%s. Repo clone: %s/\n\n",
		diff.SizeBytes, diff.LineCount, base, head, remoteRepo))
	b.WriteString(`INSTRUCTIONS — follow exactly:
1. Run ` + "`cat " + diff.DiffPath + "`" + ` ONCE. That is the diff you must review.
2. If the cat output looks empty or wrong, run ` + "`ls -la " + diff.DiffPath + "`" + ` and
   ` + "`wc -l " + diff.DiffPath + "`" + ` and INCLUDE the literal output in your reply.
3. Do NOT report "repository missing", "clone failed", or "refs not found"
   unless step 2 actually shows the file is absent. Hallucinating an
   infrastructure failure when the file exists is worse than no review.
4. You may also ` + "`cd " + remoteRepo + "`" + ` and inspect individual files referenced
   in the diff for context. The clone IS there.

`)
	// Large-diff hint: tell reviewers with tight context to start with --stat.
	if diff.SizeBytes > 1_000_000 {
		mb := float64(diff.SizeBytes) / 1_000_000
		b.WriteString(fmt.Sprintf(`NOTE: Diff is large (%.1f MB). If your context budget is tight, get the
file-level summary first with `+"`git -C %s diff --stat %s..%s`"+`,
then focus on files with the most changes.

`, mb, remoteRepo, base, head))
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
