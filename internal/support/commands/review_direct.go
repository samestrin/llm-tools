package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/samestrin/llm-tools/internal/support/multireview"
	"github.com/spf13/cobra"
)

// Flag variables for the review_direct command.
var (
	rdReviewers       string
	rdSerialReviewers string
	rdDiffFile        string
	rdOutputDir       string
	rdTimeoutSeconds  int
	rdRegistryDir     string
	rdTaskMessage     string
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

Output layout matches multi_review for compatibility:
  <output-dir>/raw/<agent>/{review.md,status.json}
  <output-dir>/multi-review-summary.json

Example:
  llm-support review_direct \
    --reviewers "bruce,greta,otto" \
    --diff-file /path/to/diff.txt \
    --output-dir code-review/multi-agent/ \
    --timeout-seconds 900`,
		RunE: runReviewDirect,
	}
	cmd.Flags().StringVar(&rdReviewers, "reviewers", "", "Comma-separated reviewer agent names (required)")
	cmd.Flags().StringVar(&rdSerialReviewers, "serial-reviewers", "", "Comma-separated subset that runs serially after the parallel lane")
	cmd.Flags().StringVar(&rdDiffFile, "diff-file", "", "Path to diff file to review (required)")
	cmd.Flags().StringVar(&rdOutputDir, "output-dir", "", "Where per-reviewer artifacts land (required)")
	cmd.Flags().IntVar(&rdTimeoutSeconds, "timeout-seconds", 900, "Total wall-clock budget for the entire fan-out")
	cmd.Flags().StringVar(&rdRegistryDir, "registry-dir", multireview.DefaultRegistryDir(), "Directory containing registry.yaml and agent prompts")
	cmd.Flags().StringVar(&rdTaskMessage, "task-message", "", "Override the task message sent to each reviewer")
	return cmd
}

func init() {
	RootCmd.AddCommand(newReviewDirectCmd())
}

func runReviewDirect(cmd *cobra.Command, _ []string) error {
	// Flag validation
	if rdReviewers == "" {
		return fmt.Errorf("--reviewers required")
	}
	if rdDiffFile == "" {
		return fmt.Errorf("--diff-file required")
	}
	if rdOutputDir == "" {
		return fmt.Errorf("--output-dir required")
	}

	// Read diff file
	diffContent, err := os.ReadFile(rdDiffFile)
	if err != nil {
		return fmt.Errorf("failed to read diff file: %w", err)
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

	// Build task message
	taskMessage := rdTaskMessage
	if taskMessage == "" {
		taskMessage = buildReviewTaskMessage(string(diffContent))
	}

	// Create output directory
	if err := os.MkdirAll(rdOutputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output dir: %w", err)
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

func buildReviewTaskMessage(diffContent string) string {
	return fmt.Sprintf(`Review the following diff and identify any issues.

Output your findings in TD_STREAM format:
SEVERITY|FILE:LINE|PROBLEM|FIX|CATEGORY|EST_MINUTES|EVIDENCE|REVIEWER

Where:
- SEVERITY: HIGH, MEDIUM, or LOW
- FILE:LINE: The file path and line number
- PROBLEM: Brief description of the issue
- FIX: How to fix it
- CATEGORY: error-handling, security, performance, maintainability, correctness
- EST_MINUTES: Estimated time to fix (integer)
- EVIDENCE: Key code snippet or reason
- REVIEWER: Your agent name

Example:
HIGH|src/auth.go:42|Missing input validation|Add length check|security|5|user input passed directly to query|bruce

Diff to review:
%s`, diffContent)
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
	count := 0
	inTDStream := false
	for _, line := range strings.Split(prose, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "TD_STREAM") {
			inTDStream = true
			continue
		}
		if inTDStream && line != "" && !strings.HasPrefix(line, "#") {
			// Count pipe-delimited lines with at least 4 pipes
			if strings.Count(line, "|") >= 4 {
				count++
			}
		}
	}
	return count
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

func extractTDLines(prose, agentName string) []string {
	var lines []string
	inTDStream := false

	for _, line := range strings.Split(prose, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "TD_STREAM") {
			inTDStream = true
			continue
		}
		if inTDStream && line != "" && !strings.HasPrefix(line, "#") {
			// Validate it looks like a TD line
			if strings.Count(line, "|") >= 4 {
				// Ensure REVIEWER column has agent name
				parts := strings.Split(line, "|")
				if len(parts) >= 8 && parts[7] == "" {
					parts[7] = agentName
					line = strings.Join(parts, "|")
				} else if len(parts) == 7 {
					line = line + "|" + agentName
				}
				lines = append(lines, line)
			}
		}
	}

	return lines
}
