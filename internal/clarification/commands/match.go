package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/samestrin/llm-tools/internal/clarification/storage"
	"github.com/samestrin/llm-tools/internal/clarification/tracking"
	"github.com/samestrin/llm-tools/pkg/llmapi"

	"github.com/spf13/cobra"
)

var matchClarificationCmd = &cobra.Command{
	Use:   "match-clarification",
	Short: "Find matching clarification using LLM",
	Long:  `Use an LLM to find if a question matches any existing clarifications.`,
	RunE:  runMatchClarification,
}

var (
	matchFile     string
	matchQuestion string
)

func init() {
	rootCmd.AddCommand(matchClarificationCmd)
	matchClarificationCmd.Flags().StringVarP(&matchFile, "file", "f", "", "Tracking file path (required)")
	matchClarificationCmd.Flags().StringVarP(&matchQuestion, "question", "q", "", "Question to match (required)")
	matchClarificationCmd.MarkFlagRequired("file")
	matchClarificationCmd.MarkFlagRequired("question")
}

// MatchResult represents the JSON output of the match-clarification command.
type MatchResult struct {
	Status     string  `json:"status"`
	MatchID    string  `json:"match_id,omitempty"`
	Confidence float64 `json:"confidence,omitempty"`
	Reason     string  `json:"reason,omitempty"`
	Question   string  `json:"question,omitempty"`
	Answer     string  `json:"answer,omitempty"`
}

// LLMClientInterface defines the interface for LLM clients.
type LLMClientInterface interface {
	Complete(prompt string, timeout time.Duration) (string, error)
}

// llmClient is the current LLM client (can be mocked for testing).
var llmClient LLMClientInterface

// SetLLMClient sets the LLM client (for testing).
func SetLLMClient(client LLMClientInterface) {
	llmClient = client
}

// getLLMClient returns the current LLM client, creating a default one if needed.
func getLLMClient() (LLMClientInterface, error) {
	if llmClient != nil {
		return llmClient, nil
	}

	config := llmapi.GetAPIConfig()
	if err := config.Validate(); err != nil {
		return nil, err
	}

	llmClient = llmapi.NewLLMClient(config.APIKey, config.BaseURL, config.Model)
	return llmClient, nil
}

func runMatchClarification(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Get storage instance
	store, err := GetStorageOrError(ctx, matchFile)
	if err != nil {
		return err
	}
	defer store.Close()

	// Get all entries
	entries, err := store.List(ctx, storage.ListFilter{})
	if err != nil {
		return fmt.Errorf("failed to list entries: %w", err)
	}

	// If no entries, return no_match immediately
	if len(entries) == 0 {
		result := MatchResult{
			Status: "no_match",
			Reason: "No existing clarifications to match against",
		}
		return outputJSON(cmd, result)
	}

	// Get LLM client
	client, err := getLLMClient()
	if err != nil {
		return fmt.Errorf("failed to initialize LLM client: %w", err)
	}

	// Build prompt
	prompt := buildMatchPrompt(matchQuestion, entries)

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
		MatchID    *string `json:"match_id"`
		Confidence float64 `json:"confidence"`
		Reason     string  `json:"reason"`
	}
	if err := json.Unmarshal([]byte(cleanedResponse), &llmResult); err != nil {
		return fmt.Errorf("failed to parse LLM response JSON: %w", err)
	}

	// Build result
	var result MatchResult
	if llmResult.MatchID == nil || *llmResult.MatchID == "" {
		result = MatchResult{
			Status:     "no_match",
			Confidence: llmResult.Confidence,
			Reason:     llmResult.Reason,
		}
	} else {
		// Find the matched entry
		var matchedEntry *tracking.Entry
		for i := range entries {
			if entries[i].ID == *llmResult.MatchID {
				matchedEntry = &entries[i]
				break
			}
		}

		result = MatchResult{
			Status:     "matched",
			MatchID:    *llmResult.MatchID,
			Confidence: llmResult.Confidence,
			Reason:     llmResult.Reason,
		}
		if matchedEntry != nil {
			result.Question = matchedEntry.CanonicalQuestion
			result.Answer = matchedEntry.CurrentAnswer
		}
	}

	return outputJSON(cmd, result)
}

// buildMatchPrompt creates the prompt for matching.
func buildMatchPrompt(question string, entries []tracking.Entry) string {
	entriesJSON, _ := json.MarshalIndent(entries, "", "  ")
	return fmt.Sprintf(`You are a question matching assistant. Given a new question and a list of existing clarifications, determine if the new question is semantically similar to any existing one.

NEW QUESTION: %s

EXISTING CLARIFICATIONS:
%s

Respond with JSON in this format:
{
  "match_id": "id of matching entry or null if no match",
  "confidence": 0.0 to 1.0,
  "reason": "explanation of match or why no match"
}

Only match if the questions are asking about the same topic and would have the same answer. A confidence of 0.7 or higher indicates a good match.`, question, string(entriesJSON))
}

// outputJSON marshals and outputs JSON to the command output.
func outputJSON(cmd *cobra.Command, v interface{}) error {
	output, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal result: %w", err)
	}
	fmt.Fprintln(cmd.OutOrStdout(), string(output))
	return nil
}
