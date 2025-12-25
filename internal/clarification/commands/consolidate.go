package commands

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/samestrin/llm-tools/internal/clarification/tracking"
	"github.com/samestrin/llm-tools/pkg/llmapi"

	"github.com/spf13/cobra"
)

var suggestConsolidationCmd = &cobra.Command{
	Use:   "suggest-consolidation",
	Short: "Suggest clarification merges using LLM",
	Long:  `Use an LLM to identify similar clarifications that could be consolidated.`,
	RunE:  runSuggestConsolidation,
}

var (
	consolidateFile string
)

func init() {
	rootCmd.AddCommand(suggestConsolidationCmd)
	suggestConsolidationCmd.Flags().StringVarP(&consolidateFile, "file", "f", "", "Tracking file path (required)")
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
	// Load tracking file
	if !tracking.FileExists(consolidateFile) {
		return fmt.Errorf("tracking file not found: %s", consolidateFile)
	}

	tf, err := tracking.LoadTrackingFile(consolidateFile)
	if err != nil {
		return fmt.Errorf("failed to load tracking file: %w", err)
	}

	// Need at least 2 entries to consolidate
	if len(tf.Entries) < 2 {
		result := ConsolidationResult{
			Status:      "no_suggestions",
			Suggestions: []ConsolidationSuggestion{},
			Total:       0,
		}
		return outputJSON(cmd, result)
	}

	// Get LLM client
	client, err := getLLMClient()
	if err != nil {
		return fmt.Errorf("failed to initialize LLM client: %w", err)
	}

	// Build prompt
	prompt := buildConsolidationPrompt(tf.Entries)

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

	return outputJSON(cmd, result)
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
