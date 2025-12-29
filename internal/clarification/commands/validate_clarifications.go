package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/samestrin/llm-tools/internal/clarification/storage"
	"github.com/samestrin/llm-tools/internal/clarification/tracking"
	"github.com/samestrin/llm-tools/internal/clarification/utils"
	"github.com/samestrin/llm-tools/pkg/llmapi"
	"github.com/samestrin/llm-tools/pkg/output"

	"github.com/spf13/cobra"
)

var validateClarificationsCmd = &cobra.Command{
	Use:   "validate-clarifications",
	Short: "Validate clarifications against current project state",
	Long:  `Flags entries that may be outdated or no longer applicable based on project context.`,
	RunE:  runValidateClarifications,
}

var (
	validateFile       string
	validateContext    string
	validateClarsJSON  bool
	validateClarsMin   bool
)

func init() {
	rootCmd.AddCommand(validateClarificationsCmd)
	validateClarificationsCmd.Flags().StringVarP(&validateFile, "file", "f", "", "Tracking file path (required)")
	validateClarificationsCmd.Flags().StringVarP(&validateContext, "context", "c", "", "Project context (optional, auto-detected if not provided)")
	validateClarificationsCmd.Flags().BoolVar(&validateClarsJSON, "json", false, "Output as JSON")
	validateClarificationsCmd.Flags().BoolVar(&validateClarsMin, "min", false, "Output in minimal/token-optimized format")
	validateClarificationsCmd.MarkFlagRequired("file")
}

// Validation represents the validation status of an entry.
type Validation struct {
	ID             string `json:"id"`
	Status         string `json:"status"`
	Reason         string `json:"reason,omitempty"`
	Recommendation string `json:"recommendation,omitempty"`
}

// ValidateClarificationsResult represents the JSON output of the validate-clarifications command.
type ValidateClarificationsResult struct {
	Status      string       `json:"status"`
	Validations []Validation `json:"validations"`
	ValidCount  int          `json:"valid_count"`
	StaleCount  int          `json:"stale_count"`
	ReviewCount int          `json:"review_count"`
}

func runValidateClarifications(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Get storage instance
	store, err := GetStorageOrError(ctx, validateFile)
	if err != nil {
		return err
	}
	defer store.Close()

	// Get all entries
	entries, err := store.List(ctx, storage.ListFilter{})
	if err != nil {
		return fmt.Errorf("failed to list entries: %w", err)
	}

	// No entries means nothing to validate
	if len(entries) == 0 {
		result := ValidateClarificationsResult{
			Status:      "no_entries",
			Validations: []Validation{},
			ValidCount:  0,
			StaleCount:  0,
			ReviewCount: 0,
		}
		formatter := output.New(validateClarsJSON, validateClarsMin, cmd.OutOrStdout())
		return formatter.Print(result, printValidateClarificationsText)
	}

	// Get LLM client
	client, err := getLLMClient()
	if err != nil {
		return fmt.Errorf("failed to initialize LLM client: %w", err)
	}

	// Get or detect project context
	projectContext := validateContext
	if projectContext == "" {
		projectContext = detectProjectContext()
	}

	// Build prompt
	prompt := buildValidatePrompt(entries, projectContext)

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
		Validations []Validation `json:"validations"`
		ValidCount  int          `json:"valid_count"`
		StaleCount  int          `json:"stale_count"`
		ReviewCount int          `json:"review_count"`
	}
	if err := json.Unmarshal([]byte(cleanedResponse), &llmResult); err != nil {
		return fmt.Errorf("failed to parse LLM response JSON: %w", err)
	}

	result := ValidateClarificationsResult{
		Status:      "validated",
		Validations: llmResult.Validations,
		ValidCount:  llmResult.ValidCount,
		StaleCount:  llmResult.StaleCount,
		ReviewCount: llmResult.ReviewCount,
	}

	formatter := output.New(validateClarsJSON, validateClarsMin, cmd.OutOrStdout())
	return formatter.Print(result, printValidateClarificationsText)
}

func printValidateClarificationsText(w io.Writer, data interface{}) {
	r := data.(ValidateClarificationsResult)
	fmt.Fprintf(w, "STATUS: %s\n", r.Status)
	fmt.Fprintf(w, "VALID_COUNT: %d\n", r.ValidCount)
	fmt.Fprintf(w, "STALE_COUNT: %d\n", r.StaleCount)
	fmt.Fprintf(w, "REVIEW_COUNT: %d\n", r.ReviewCount)
	for _, v := range r.Validations {
		fmt.Fprintf(w, "\n%s: %s\n", v.ID, v.Status)
		if v.Reason != "" {
			fmt.Fprintf(w, "  REASON: %s\n", v.Reason)
		}
		if v.Recommendation != "" {
			fmt.Fprintf(w, "  RECOMMENDATION: %s\n", v.Recommendation)
		}
	}
}

func detectProjectContext() string {
	contextParts := []string{}

	// Check for package.json (Node.js)
	if _, err := os.Stat("package.json"); err == nil {
		contextParts = append(contextParts, "Node.js project (package.json found)")
	}

	// Check for go.mod (Go)
	if _, err := os.Stat("go.mod"); err == nil {
		contextParts = append(contextParts, "Go project (go.mod found)")
	}

	// Check for requirements.txt (Python)
	if _, err := os.Stat("requirements.txt"); err == nil {
		contextParts = append(contextParts, "Python project (requirements.txt found)")
	}

	// Check for Cargo.toml (Rust)
	if _, err := os.Stat("Cargo.toml"); err == nil {
		contextParts = append(contextParts, "Rust project (Cargo.toml found)")
	}

	if len(contextParts) == 0 {
		return "Unknown project type"
	}

	result := ""
	for i, part := range contextParts {
		if i > 0 {
			result += "; "
		}
		result += part
	}
	return result
}

func buildValidatePrompt(entries []tracking.Entry, context string) string {
	// Format entries
	formattedEntries := ""
	for _, entry := range entries {
		answer := entry.CurrentAnswer
		if answer == "" {
			answer = "N/A"
		}
		lastSeen := entry.LastSeen
		if lastSeen == "" {
			lastSeen = "unknown"
		}
		formattedEntries += fmt.Sprintf("- ID: %s\n  Question: \"%s\"\n  Answer: \"%s\"\n  Last seen: %s\n  Occurrences: %d\n\n",
			entry.ID, entry.CanonicalQuestion, answer, lastSeen, entry.Occurrences)
	}

	today := utils.GetToday()

	return fmt.Sprintf(`Validate these clarification entries against the current project context.
Identify entries that may be stale, outdated, or need review.

PROJECT CONTEXT:
%s

ENTRIES:
%s
TODAY'S DATE: %s

INSTRUCTIONS:
- Flag entries that reference outdated technologies (deprecated libraries, old patterns)
- Flag entries with answers that may no longer be accurate
- Flag entries not seen in over 90 days for review
- Entries frequently used recently are likely still valid

Return ONLY valid JSON in this exact format (no markdown, no explanation):
{
  "validations": [
    {
      "id": "<entry_id>",
      "status": "<valid|stale|needs_review>",
      "reason": "<explanation if not valid>",
      "recommendation": "<suggested action if needed>"
    }
  ],
  "valid_count": <number>,
  "stale_count": <number>,
  "review_count": <number>
}`, context, formattedEntries, today)
}
