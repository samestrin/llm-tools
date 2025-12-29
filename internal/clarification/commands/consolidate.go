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

var suggestConsolidationCmd = &cobra.Command{
	Use:   "suggest-consolidation",
	Short: "Suggest clarification merges using LLM",
	Long:  `Use an LLM to identify similar clarifications that could be consolidated.`,
	RunE:  runSuggestConsolidation,
}

var (
	consolidateFile    string
	consolidateJSON    bool
	consolidateMinimal bool
)

func init() {
	rootCmd.AddCommand(suggestConsolidationCmd)
	suggestConsolidationCmd.Flags().StringVarP(&consolidateFile, "file", "f", "", "Tracking file path (required)")
	suggestConsolidationCmd.Flags().BoolVar(&consolidateJSON, "json", false, "Output as JSON")
	suggestConsolidationCmd.Flags().BoolVar(&consolidateMinimal, "min", false, "Output in minimal/token-optimized format")
	suggestConsolidationCmd.MarkFlagRequired("file")
}

// ConsolidationResult represents the JSON output of the suggest-consolidation command.
type ConsolidationResult struct {
	Status      string                    `json:"status"`
	Suggestions []ConsolidationSuggestion `json:"suggestions"`
	Total       int                       `json:"total"`
}

// ConsolidationSuggestion represents a suggested merge.
type ConsolidationSuggestion struct {
	PrimaryID      string   `json:"primary_id"`
	MergeIDs       []string `json:"merge_ids"`
	Reason         string   `json:"reason"`
	SuggestedMerge string   `json:"suggested_merge,omitempty"`
}

func runSuggestConsolidation(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Get storage instance
	store, err := GetStorageOrError(ctx, consolidateFile)
	if err != nil {
		return err
	}
	defer store.Close()

	// Get all entries
	entries, err := store.List(ctx, storage.ListFilter{})
	if err != nil {
		return fmt.Errorf("failed to list entries: %w", err)
	}

	// Need at least 2 entries to consolidate
	if len(entries) < 2 {
		result := ConsolidationResult{
			Status:      "no_suggestions",
			Suggestions: []ConsolidationSuggestion{},
			Total:       0,
		}
		formatter := output.New(consolidateJSON, consolidateMinimal, cmd.OutOrStdout())
		return formatter.Print(result, printConsolidationText)
	}

	// Get LLM client
	client, err := getLLMClient()
	if err != nil {
		return fmt.Errorf("failed to initialize LLM client: %w", err)
	}

	// Build prompt
	prompt := buildConsolidationPrompt(entries)

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
		Suggestions []ConsolidationSuggestion `json:"suggestions"`
	}
	if err := json.Unmarshal([]byte(cleanedResponse), &llmResult); err != nil {
		return fmt.Errorf("failed to parse LLM response JSON: %w", err)
	}

	status := "suggestions_found"
	if len(llmResult.Suggestions) == 0 {
		status = "no_suggestions"
	}

	result := ConsolidationResult{
		Status:      status,
		Suggestions: llmResult.Suggestions,
		Total:       len(llmResult.Suggestions),
	}

	formatter := output.New(consolidateJSON, consolidateMinimal, cmd.OutOrStdout())
	return formatter.Print(result, printConsolidationText)
}

func printConsolidationText(w io.Writer, data interface{}) {
	r := data.(ConsolidationResult)
	fmt.Fprintf(w, "STATUS: %s\n", r.Status)
	fmt.Fprintf(w, "TOTAL: %d\n", r.Total)
	for i, s := range r.Suggestions {
		fmt.Fprintf(w, "\n[%d] Primary: %s\n", i+1, s.PrimaryID)
		fmt.Fprintf(w, "    Merge: %v\n", s.MergeIDs)
		fmt.Fprintf(w, "    Reason: %s\n", s.Reason)
		if s.SuggestedMerge != "" {
			fmt.Fprintf(w, "    Suggested: %s\n", s.SuggestedMerge)
		}
	}
}

func buildConsolidationPrompt(entries []tracking.Entry) string {
	entriesJSON, _ := json.MarshalIndent(entries, "", "  ")
	return fmt.Sprintf(`You are a documentation analyst. Review these clarification entries and identify groups of similar questions that could be consolidated into single entries.

ENTRIES:
%s

Look for:
- Questions asking about the same topic in different words
- Questions that would have the same or very similar answers
- Duplicates or near-duplicates

Respond with JSON:
{
  "suggestions": [
    {
      "primary_id": "id of the entry to keep",
      "merge_ids": ["ids", "of", "entries", "to", "merge"],
      "reason": "why these should be consolidated",
      "suggested_merge": "optional: suggested combined question text"
    }
  ]
}

Only include groupings with high confidence (genuinely similar questions).`, string(entriesJSON))
}
