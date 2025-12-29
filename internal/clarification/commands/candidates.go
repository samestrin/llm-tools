package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/samestrin/llm-tools/internal/clarification/storage"
	"github.com/samestrin/llm-tools/internal/clarification/tracking"
	"github.com/samestrin/llm-tools/pkg/llmapi"
	"github.com/samestrin/llm-tools/pkg/output"

	"github.com/spf13/cobra"
)

var identifyCandidatesCmd = &cobra.Command{
	Use:   "identify-candidates",
	Short: "Find promotion candidates using LLM",
	Long:  `Use an LLM to identify clarifications that should be promoted to CLAUDE.md.`,
	RunE:  runIdentifyCandidates,
}

var (
	candidatesFile           string
	candidatesMinOccurrences int
	candidatesJSON           bool
	candidatesMinimal        bool
)

func init() {
	rootCmd.AddCommand(identifyCandidatesCmd)
	identifyCandidatesCmd.Flags().StringVarP(&candidatesFile, "file", "f", "", "Tracking file path (required)")
	identifyCandidatesCmd.Flags().IntVar(&candidatesMinOccurrences, "min-occurrences", 3, "Minimum occurrences to consider")
	identifyCandidatesCmd.Flags().BoolVar(&candidatesJSON, "json", false, "Output as JSON")
	identifyCandidatesCmd.Flags().BoolVar(&candidatesMinimal, "min", false, "Output in minimal/token-optimized format")
	identifyCandidatesCmd.MarkFlagRequired("file")
}

// CandidatesResult represents the JSON output of the identify-candidates command.
type CandidatesResult struct {
	Status     string      `json:"status"`
	Candidates []Candidate `json:"candidates"`
	Total      int         `json:"total"`
}

// Candidate represents a promotion candidate.
type Candidate struct {
	ID          string  `json:"id"`
	Question    string  `json:"question"`
	Occurrences int     `json:"occurrences"`
	Confidence  float64 `json:"confidence"`
	Reason      string  `json:"reason"`
}

func runIdentifyCandidates(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Get storage instance
	store, err := GetStorageOrError(ctx, candidatesFile)
	if err != nil {
		return err
	}
	defer store.Close()

	// Get all entries and filter eligible ones (pending status, meets occurrence threshold)
	entries, err := store.List(ctx, storage.ListFilter{
		Status:         "pending",
		MinOccurrences: candidatesMinOccurrences,
	})
	if err != nil {
		return fmt.Errorf("failed to list entries: %w", err)
	}

	eligibleEntries := entries

	if len(eligibleEntries) == 0 {
		result := CandidatesResult{
			Status:     "no_candidates",
			Candidates: []Candidate{},
			Total:      0,
		}
		formatter := output.New(candidatesJSON, candidatesMinimal, cmd.OutOrStdout())
		return formatter.Print(result, printCandidatesText)
	}

	// Get LLM client
	client, err := getLLMClient()
	if err != nil {
		return fmt.Errorf("failed to initialize LLM client: %w", err)
	}

	// Build prompt
	prompt := buildCandidatesPrompt(eligibleEntries)

	// Call LLM
	response, err := client.Complete(prompt, 30*time.Second)
	if err != nil {
		return fmt.Errorf("LLM request failed: %w", err)
	}

	// Parse response
	cleanedResponse, err := llmapi.ExtractJSON(response)
	if err != nil {
		return fmt.Errorf("failed to parse LLM response: %w", err)
	}

	var llmResult struct {
		Candidates []struct {
			ID         string  `json:"id"`
			Confidence float64 `json:"confidence"`
			Reason     string  `json:"reason"`
		} `json:"candidates"`
	}
	if err := json.Unmarshal([]byte(cleanedResponse), &llmResult); err != nil {
		return fmt.Errorf("failed to parse LLM response JSON: %w", err)
	}

	// Build result with full entry details
	var candidates []Candidate
	for _, llmCand := range llmResult.Candidates {
		for _, entry := range eligibleEntries {
			if entry.ID == llmCand.ID {
				candidates = append(candidates, Candidate{
					ID:          entry.ID,
					Question:    entry.CanonicalQuestion,
					Occurrences: entry.Occurrences,
					Confidence:  llmCand.Confidence,
					Reason:      llmCand.Reason,
				})
				break
			}
		}
	}

	result := CandidatesResult{
		Status:     "candidates_found",
		Candidates: candidates,
		Total:      len(candidates),
	}

	formatter := output.New(candidatesJSON, candidatesMinimal, cmd.OutOrStdout())
	return formatter.Print(result, printCandidatesText)
}

func printCandidatesText(w io.Writer, data interface{}) {
	r := data.(CandidatesResult)
	fmt.Fprintf(w, "STATUS: %s\n", r.Status)
	fmt.Fprintf(w, "TOTAL: %d\n", r.Total)
	for _, c := range r.Candidates {
		fmt.Fprintf(w, "\n%s (confidence: %.2f)\n", c.ID, c.Confidence)
		fmt.Fprintf(w, "  QUESTION: %s\n", c.Question)
		fmt.Fprintf(w, "  OCCURRENCES: %d\n", c.Occurrences)
		fmt.Fprintf(w, "  REASON: %s\n", c.Reason)
	}
}

func buildCandidatesPrompt(entries []tracking.Entry) string {
	entriesJSON, _ := json.MarshalIndent(entries, "", "  ")
	return fmt.Sprintf(`You are a documentation analyst. Review these clarification entries and identify which ones should be promoted to the project's main documentation (CLAUDE.md).

ENTRIES:
%s

Consider:
- Is the answer stable and unlikely to change?
- Is it broadly applicable across the project?
- Is the answer clear and well-formulated?

Respond with JSON:
{
  "candidates": [
    {
      "id": "entry id",
      "confidence": 0.0 to 1.0,
      "reason": "why this should be promoted"
    }
  ]
}

Include only entries with confidence >= 0.7.`, string(entriesJSON))
}
