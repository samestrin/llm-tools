package commands

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/samestrin/llm-tools/pkg/llmapi"

	"github.com/spf13/cobra"
)

var normalizeClarificationCmd = &cobra.Command{
	Use:   "normalize-clarification",
	Short: "Normalize question wording using LLM",
	Long:  `Use an LLM to improve and standardize question wording.`,
	RunE:  runNormalizeClarification,
}

var (
	normalizeQuestion string
)

func init() {
	rootCmd.AddCommand(normalizeClarificationCmd)
	normalizeClarificationCmd.Flags().StringVarP(&normalizeQuestion, "question", "q", "", "Question to normalize (required)")
	normalizeClarificationCmd.MarkFlagRequired("question")
}

// NormalizeResult represents the JSON output of the normalize-clarification command.
type NormalizeResult struct {
	Status             string `json:"status"`
	OriginalQuestion   string `json:"original_question"`
	NormalizedQuestion string `json:"normalized_question"`
	Changes            string `json:"changes,omitempty"`
}

func runNormalizeClarification(cmd *cobra.Command, args []string) error {
	// Get LLM client
	client, err := getLLMClient()
	if err != nil {
		return fmt.Errorf("failed to initialize LLM client: %w", err)
	}

	// Build prompt
	prompt := buildNormalizePrompt(normalizeQuestion)

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
		NormalizedQuestion string `json:"normalized_question"`
		Changes            string `json:"changes"`
	}
	if err := json.Unmarshal([]byte(cleanedResponse), &llmResult); err != nil {
		return fmt.Errorf("failed to parse LLM response JSON: %w", err)
	}

	result := NormalizeResult{
		Status:             "normalized",
		OriginalQuestion:   normalizeQuestion,
		NormalizedQuestion: llmResult.NormalizedQuestion,
		Changes:            llmResult.Changes,
	}

	return outputJSON(cmd, result)
}

func buildNormalizePrompt(question string) string {
	return fmt.Sprintf(`You are a question normalization assistant. Your task is to improve the wording of a question to make it clear, concise, and standardized.

QUESTION: %s

Guidelines:
- Make it a complete, grammatical question
- Remove filler words and redundancy
- Use consistent terminology
- Keep the original meaning intact

Respond with JSON:
{
  "normalized_question": "the improved question",
  "changes": "brief description of changes made"
}`, question)
}
