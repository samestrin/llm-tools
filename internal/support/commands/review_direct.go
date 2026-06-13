package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/samestrin/llm-tools/internal/support/gitrange"
	"github.com/samestrin/llm-tools/internal/support/multireview"
	"github.com/spf13/cobra"
)

// Flag variables for the review_direct command.
var (
	rdReviewers       string
	rdSerialReviewers string
	rdDiffFile        string
	rdRepo            string
	rdBase            string
	rdHead            string
	rdMergeCommit     string
	rdOutputDir       string
	rdTimeoutSeconds  int
	rdRegistryDir     string
	rdTaskMessage     string
	rdConfig          string
	rdSprintPlan      string
	rdExclude         string
)

func newReviewDirectCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "review_direct",
		Short: "Direct LLM provider multi-agent code review (no SSH/Docker)",
		Long: `Invoke multiple LLM provider agents directly via their OpenAI-compatible
APIs to perform parallel code review. Unlike multi_review, this command runs
entirely locally without SSH, Docker, or openclaw infrastructure.

Agent configuration is loaded from a global registry at ~/.config/llm-tools/agents/
(or --registry-dir). Each agent has a provider (openai, anthropic, google), model,
and system prompt loaded from <agent>.md files.

The diff can be supplied pre-computed (--diff-file) or computed self-serve
from a local repository (--repo with optional --base/--head/--merge-commit;
the resolved diff is written to <output-dir>/diff.txt). Self-serve range
resolution matches 'llm-support review_range': --merge-commit reviews
sha^..sha (already-merged branches), explicit refs are used as given, and
with neither the range is the merge-base of HEAD against the default branch.
An empty diff is a hard error — nothing to review.

With --sprint-plan, the file's content is injected into reviewer prompts as
a scope constraint so findings stay within the sprint's work items (matching
multi_review). A missing or whitespace-only file is ignored and an unreadable
file warns — in both cases reviewers fall back to diff-touched-line scoping.
--task-message overrides the entire task message, including the scope block.

In self-serve mode, paths matching --exclude globs are dropped from the diff
(git :(exclude) pathspecs) before reviewers see it. The default,
.planning/**,CHANGELOG.md, removes planning/tracking artifacts that are noise
to a reviewer. A non-empty --exclude replaces the default; --exclude='' disables
it. Excludes do not apply to a pre-computed --diff-file.

Output layout matches multi_review for compatibility:
  <output-dir>/raw/<agent>/{review.md,status.json}
  <output-dir>/multi-review-summary.json

Examples:
  llm-support review_direct \
    --reviewers "bruce,greta,otto" \
    --diff-file /path/to/diff.txt \
    --output-dir code-review/multi-agent/ \
    --timeout-seconds 900

  llm-support review_direct \
    --reviewers "bruce,greta" \
    --repo . --merge-commit 9e013e7 \
    --output-dir code-review/multi-agent/`,
		RunE: runReviewDirect,
	}
	cmd.Flags().StringVar(&rdReviewers, "reviewers", "", "Comma-separated reviewer agent names (required)")
	cmd.Flags().StringVar(&rdSerialReviewers, "serial-reviewers", "", "Comma-separated subset that runs serially after the parallel lane")
	cmd.Flags().StringVar(&rdDiffFile, "diff-file", "", "Path to a pre-computed diff file (mutually exclusive with --base/--head/--merge-commit)")
	cmd.Flags().StringVar(&rdRepo, "repo", ".", "Repository path for self-serve diff computation")
	cmd.Flags().StringVar(&rdBase, "base", "", "Base ref for self-serve diff (default: merge-base against the default branch)")
	cmd.Flags().StringVar(&rdHead, "head", "", "Head ref for self-serve diff (default HEAD)")
	cmd.Flags().StringVar(&rdMergeCommit, "merge-commit", "", "Merge/squash commit SHA; reviews sha^..sha (for already-merged branches)")
	cmd.Flags().StringVar(&rdOutputDir, "output-dir", "", "Where per-reviewer artifacts land (required)")
	cmd.Flags().IntVar(&rdTimeoutSeconds, "timeout-seconds", 900, "Total wall-clock budget for the entire fan-out")
	cmd.Flags().StringVar(&rdRegistryDir, "registry-dir", multireview.DefaultRegistryDir(), "Directory containing registry.yaml and agent prompts")
	cmd.Flags().StringVar(&rdTaskMessage, "task-message", "", "Override the task message sent to each reviewer (suppresses --sprint-plan scoping)")
	cmd.Flags().StringVar(&rdConfig, "config", "", "Optional config.yaml; review.direct.* keys supply defaults for unset flags")
	cmd.Flags().StringVar(&rdSprintPlan, "sprint-plan", "", "Path to sprint-plan.md or epic file; content is injected into reviewer prompts to scope findings")
	cmd.Flags().StringVar(&rdExclude, "exclude", strings.Join(gitrange.DefaultExcludeGlobs, ","),
		"Comma-separated path globs dropped from the diff before review (self-serve mode only). "+
			"Replaces the built-in default; pass --exclude='' to disable and review every file.")
	return cmd
}

func init() {
	RootCmd.AddCommand(newReviewDirectCmd())
}

func runReviewDirect(cmd *cobra.Command, _ []string) error {
	// Merge config defaults (explicit flags win) before validation so config
	// alone can satisfy required flags.
	if rdConfig != "" {
		if err := applyReviewDirectConfig(cmd, rdConfig); err != nil {
			return err
		}
	}

	// Flag validation
	if cmd.Flags().Changed("diff-file") && rdDiffFile == "" {
		return fmt.Errorf("--diff-file was provided but is empty (unset shell variable?) — " +
			"pass a real path, or omit it and use --repo/--base/--head/--merge-commit for self-serve mode")
	}
	if rdReviewers == "" {
		if rdConfig != "" {
			return fmt.Errorf("--reviewers required (--config %s has no review.direct.agents)", rdConfig)
		}
		return fmt.Errorf("--reviewers required")
	}
	if rdOutputDir == "" {
		return fmt.Errorf("--output-dir required")
	}
	if rdDiffFile != "" && (rdBase != "" || rdHead != "" || rdMergeCommit != "") {
		return fmt.Errorf("--diff-file is mutually exclusive with --base/--head/--merge-commit")
	}

	// Create output directory early — self-serve mode writes diff.txt into it.
	if err := os.MkdirAll(rdOutputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output dir: %w", err)
	}

	// Acquire the diff: pre-computed file, or self-serve via local git.
	var diffContent []byte
	if rdDiffFile != "" {
		var err error
		diffContent, err = os.ReadFile(rdDiffFile)
		if err != nil {
			return fmt.Errorf("failed to read diff file: %w", err)
		}
		if strings.TrimSpace(string(diffContent)) == "" {
			return fmt.Errorf("diff file %s is empty — nothing to review. "+
				"If the branch is already merged, diagnose with `llm-support review_range --repo <repo>` "+
				"or re-run review_direct with --merge-commit <sha>", rdDiffFile)
		}
	} else {
		rangeRes, err := gitrange.Resolve(gitrange.Params{
			RepoPath:    rdRepo,
			BaseRef:     rdBase,
			HeadRef:     rdHead,
			MergeCommit: rdMergeCommit,
		})
		if err != nil {
			return err
		}
		if rangeRes.Empty {
			return fmt.Errorf("empty diff for %s..%s (detection: %s) — nothing to review. %s",
				gitrange.Short(rangeRes.Base), gitrange.Short(rangeRes.Head), rangeRes.Detection, rangeRes.Message)
		}
		excludeGlobs := splitAndTrim(rdExclude)
		diffText, err := gitrange.DiffExcluding(rdRepo, rangeRes.Base, rangeRes.Head, excludeGlobs)
		if err != nil {
			return fmt.Errorf("failed to compute diff: %w", err)
		}
		// Compute what the exclusion dropped, for the report and the empty-diff
		// message (so an all-excluded range names the cause instead of blaming
		// "commits without content changes").
		excluded, exErr := gitrange.ExcludedFileNames(rdRepo, rangeRes.Base, rangeRes.Head, excludeGlobs)
		if exErr != nil {
			return fmt.Errorf("failed to compute excluded files: %w", exErr)
		}
		if strings.TrimSpace(diffText) == "" {
			if len(excluded) > 0 {
				return fmt.Errorf("diff for %s..%s is empty after excluding %d file(s) via [%s] — "+
					"every changed file was excluded; loosen --exclude or pass --exclude='' to review them",
					gitrange.Short(rangeRes.Base), gitrange.Short(rangeRes.Head), len(excluded), strings.Join(excludeGlobs, ", "))
			}
			return fmt.Errorf("diff for %s..%s is empty (commits without content changes) — nothing to review",
				gitrange.Short(rangeRes.Base), gitrange.Short(rangeRes.Head))
		}
		diffContent = []byte(diffText)
		diffPath := filepath.Join(rdOutputDir, "diff.txt")
		if err := os.WriteFile(diffPath, diffContent, 0644); err != nil {
			return fmt.Errorf("failed to write %s: %w", diffPath, err)
		}
		excludeNote := ""
		if len(excludeGlobs) > 0 {
			excludeNote = fmt.Sprintf("; excluded %d file(s) via [%s]", len(excluded), strings.Join(excludeGlobs, ", "))
		}
		fmt.Fprintf(cmd.OutOrStdout(), "diff: %s (%d bytes, %s..%s via %s%s)\n",
			diffPath, len(diffContent), gitrange.Short(rangeRes.Base), gitrange.Short(rangeRes.Head), rangeRes.Detection, excludeNote)
	}

	// Load registry
	registry, err := multireview.LoadRegistry(rdRegistryDir)
	if err != nil {
		return fmt.Errorf("failed to load registry from %s: %w", rdRegistryDir, err)
	}

	// Parse reviewers
	allReviewers := splitAndTrim(rdReviewers)
	serial := splitAndTrim(rdSerialReviewers)
	parallel := subtract(allReviewers, serial)

	if len(allReviewers) == 0 {
		return fmt.Errorf("--reviewers must list at least one agent")
	}

	// Read the sprint plan unconditionally (matching multi_review) so an
	// unreadable file warns even when --task-message overrides the message.
	sprintPlanContent := readSprintPlan(rdSprintPlan, cmd.ErrOrStderr())

	// Build task message. --task-message is a full override (matching
	// multi_review): the sprint-plan scope block is not appended to it.
	taskMessage := rdTaskMessage
	if taskMessage == "" {
		taskMessage = buildReviewTaskMessage(string(diffContent), sprintPlanContent)
	}

	// Run fan-out
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(rdTimeoutSeconds)*time.Second)
	defer cancel()

	result, err := multireview.Fanout(ctx, multireview.FanoutParams{
		Registry:       registry,
		ParallelAgents: parallel,
		SerialAgents:   serial,
		TaskMessage:    taskMessage,
		GlobalTimeout:  time.Duration(rdTimeoutSeconds) * time.Second,
		OutputDir:      rdOutputDir,
	})

	// Build summary (even on partial failure)
	summary := buildDirectReviewSummary(result, allReviewers)

	// Write summary
	summaryPath := filepath.Join(rdOutputDir, "multi-review-summary.json")
	summaryJSON, _ := json.MarshalIndent(summary, "", "  ")
	if writeErr := os.WriteFile(summaryPath, summaryJSON, 0644); writeErr != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to write summary: %v\n", writeErr)
	}

	// Extract and merge TD streams
	if mergeErr := mergeDirectTDStreams(rdOutputDir, result.Results); mergeErr != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to merge TD streams: %v\n", mergeErr)
	}

	// Print summary to stdout
	fmt.Printf("Direct review complete: %d succeeded, %d failed\n",
		result.SuccessCount, result.FailedCount)
	fmt.Printf("Total duration: %dms\n", result.TotalDurationMS)
	fmt.Printf("Output: %s\n", rdOutputDir)

	return err // Returns nil on partial success, error if all fail
}

func buildReviewTaskMessage(diffContent, sprintPlan string) string {
	var b strings.Builder
	b.WriteString(`Review the following diff and identify any issues.

Output ONLY pipe-delimited findings, one finding per line, in this exact format:
SEVERITY|FILE:LINE|PROBLEM|FIX|CATEGORY|EST_MINUTES|EVIDENCE|REVIEWER

Where:
- SEVERITY: HIGH, MEDIUM, or LOW
- FILE:LINE: The file path and line number
- PROBLEM: Brief description of the issue
- FIX: How to fix it
- CATEGORY: error-handling, security, performance, maintainability, correctness
- EST_MINUTES: Estimated time to fix (integer)
- EVIDENCE: A short code snippet or reason (keep it under ~20 words)
- REVIEWER: Your agent name

Output rules (strict):
- Emit ONLY the finding lines — no prose, no preamble, no summary, no markdown, no code fences, no headings.
- One finding per line; never wrap a finding across multiple lines.
- If you find no issues, output nothing at all.

Example:
HIGH|src/auth.go:42|Missing input validation|Add length check|security|5|user input passed directly to query|bruce
`)
	// Scope instructions go BEFORE the embedded diff — the diff can be huge
	// and instructions buried after it are easy for models to miss.
	b.WriteString(sprintPlanScopeBlock(sprintPlan))
	b.WriteString("\nDiff to review:\n")
	b.WriteString(diffContent)
	return b.String()
}

func buildDirectReviewSummary(result multireview.FanoutResult, allReviewers []string) MultiReviewSummary {
	summary := MultiReviewSummary{
		Reviewers:       make([]ReviewerStatus, 0, len(result.Results)),
		TotalDurationMS: result.TotalDurationMS,
		Timestamp:       time.Now().Format(time.RFC3339),
		Partial:         result.FailedCount > 0 && result.SuccessCount > 0,
	}

	for _, r := range result.Results {
		status := ReviewerStatus{
			Agent:      r.AgentName,
			Model:      r.Model,
			Status:     r.Status,
			DurationMS: r.DurationMS,
		}
		if r.Error != nil {
			status.Error = r.Error.Error()
		}
		// Count TD lines in review prose
		status.TDLineCount = countTDLines(r.ReviewProse)
		summary.TotalFindings += status.TDLineCount
		summary.Reviewers = append(summary.Reviewers, status)
	}

	return summary
}

func countTDLines(prose string) int {
	return len(multireview.ExtractTDLines(prose))
}

func mergeDirectTDStreams(outputDir string, results []multireview.InvokeDirectResult) error {
	var merged strings.Builder
	merged.WriteString("# Merged TD Stream from direct review\n")
	merged.WriteString("# SEVERITY|FILE:LINE|PROBLEM|FIX|CATEGORY|EST_MINUTES|EVIDENCE|REVIEWER\n\n")

	for _, r := range results {
		if r.Status != "ok" {
			continue
		}
		// Extract TD_STREAM lines
		lines := extractTDLines(r.ReviewProse, r.AgentName)
		for _, line := range lines {
			merged.WriteString(line)
			merged.WriteString("\n")
		}

		// Also write per-agent td-stream.txt
		agentTDPath := filepath.Join(outputDir, "raw", r.AgentName, "td-stream.txt")
		os.WriteFile(agentTDPath, []byte(strings.Join(lines, "\n")+"\n"), 0644)
	}

	// Write merged stream to both locations (raw/ and root for compatibility)
	rawMerged := filepath.Join(outputDir, "raw", "td-stream-all.txt")
	rootMerged := filepath.Join(outputDir, "td-stream.txt")

	content := merged.String()
	if err := os.WriteFile(rawMerged, []byte(content), 0644); err != nil {
		return err
	}
	return os.WriteFile(rootMerged, []byte(content), 0644)
}

// extractTDLines pulls pipe-delimited TD findings from a reviewer's raw output
// and ensures each row carries the agent name in the REVIEWER column (col 8).
//
// Detection is severity-anchored (shared with the openclaw path via
// multireview.ExtractTDLines): any line beginning with CRITICAL|HIGH|MEDIUM|LOW
// immediately followed by a pipe is a finding. It does NOT require a leading
// "TD_STREAM" sentinel — the reviewer prompt never asks for one (its example is
// a bare row), so a sentinel gate silently dropped every compliant reviewer's
// findings.
func extractTDLines(prose, agentName string) []string {
	raw := multireview.ExtractTDLines(prose)
	lines := make([]string, 0, len(raw))
	for _, line := range raw {
		parts := strings.Split(line, "|")
		if len(parts) >= 8 {
			// Already has a REVIEWER slot. Fill it only when empty — never
			// double-append when the reviewer already named itself (the
			// prompt's example row ends with the agent name).
			if parts[7] == "" {
				parts[7] = agentName
				line = strings.Join(parts, "|")
			}
		} else {
			// 7 or fewer columns: pad up to EVIDENCE (col 7), then append the
			// agent name as REVIEWER (col 8) — parity with the unified 8-col
			// format produced by multireview.WriteReviewerOutput.
			for len(parts) < 7 {
				parts = append(parts, "")
			}
			line = strings.Join(parts, "|") + "|" + agentName
		}
		lines = append(lines, line)
	}
	return lines
}
