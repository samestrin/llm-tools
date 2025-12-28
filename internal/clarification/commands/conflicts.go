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

var detectConflictsCmd = &cobra.Command{
	Use:   "detect-conflicts",
	Short: "Find clarification entries with conflicting answers",
	Long:  `Analyzes a tracking file to find entries that may have the same underlying question but different answers.`,
	RunE:  runDetectConflicts,
}

var (
	conflictsFile string
)

func init() {
	rootCmd.AddCommand(detectConflictsCmd)
	detectConflictsCmd.Flags().StringVarP(&conflictsFile, "file", "f", "", "Tracking file path (required)")
	detectConflictsCmd.MarkFlagRequired("file")
}

// ConflictEntry represents an entry involved in a conflict.
type ConflictEntry struct {
	ID       string `json:"id"`
	Question string `json:"question"`
	Answer   string `json:"answer"`
}

// Conflict represents a detected conflict between entries.
type Conflict struct {
	EntryIDs   []string        `json:"entry_ids"`
	Reason     string          `json:"reason"`
	Severity   string          `json:"severity"`
	Suggestion string          `json:"suggestion"`
	Entries    []ConflictEntry `json:"entries,omitempty"`
}

// ConflictsResult represents the JSON output of the detect-conflicts command.
type ConflictsResult struct {
	Status        string     `json:"status"`
	Conflicts     []Conflict `json:"conflicts"`
	ConflictCount int        `json:"conflict_count"`
	Note          string     `json:"note,omitempty"`
}

func runDetectConflicts(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Get storage instance
	store, err := GetStorageOrError(ctx, conflictsFile)
	if err != nil {
		return err
	}
	defer store.Close()

	// Get all entries
	entries, err := store.List(ctx, storage.ListFilter{})
	if err != nil {
		return fmt.Errorf("failed to list entries: %w", err)
	}

	// Need at least 2 entries to detect conflicts
	if len(entries) < 2 {
		result := ConflictsResult{
			Status:        "no_conflicts",
			Conflicts:     []Conflict{},
			ConflictCount: 0,
			Note:          "Not enough entries to detect conflicts",
		}
		return outputJSON(cmd, result)
	}

	// Get LLM client
	client, err := getLLMClient()
	if err != nil {
		return fmt.Errorf("failed to initialize LLM client: %w", err)
	}

	// Build prompt
	prompt := buildConflictsPrompt(entries)

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
		Conflicts     []Conflict `json:"conflicts"`
		ConflictCount int        `json:"conflict_count"`
	}
	if err := json.Unmarshal([]byte(cleanedResponse), &llmResult); err != nil {
		return fmt.Errorf("failed to parse LLM response JSON: %w", err)
	}

	// Build entry map for enrichment
	entryMap := make(map[string]tracking.Entry)
	for _, e := range entries {
		entryMap[e.ID] = e
	}

	// Enrich conflicts with entry details
	for i := range llmResult.Conflicts {
		llmResult.Conflicts[i].Entries = make([]ConflictEntry, 0, len(llmResult.Conflicts[i].EntryIDs))
		for _, eid := range llmResult.Conflicts[i].EntryIDs {
			if entry, ok := entryMap[eid]; ok {
				llmResult.Conflicts[i].Entries = append(llmResult.Conflicts[i].Entries, ConflictEntry{
					ID:       eid,
					Question: entry.CanonicalQuestion,
					Answer:   entry.CurrentAnswer,
				})
			}
		}
	}

	status := "conflicts_found"
	if len(llmResult.Conflicts) == 0 {
		status = "no_conflicts"
	}

	result := ConflictsResult{
		Status:        status,
		Conflicts:     llmResult.Conflicts,
		ConflictCount: llmResult.ConflictCount,
	}

	return outputJSON(cmd, result)
}

func buildConflictsPrompt(entries []tracking.Entry) string {
	// Format entries
	formattedEntries := ""
	for _, entry := range entries {
		answer := entry.CurrentAnswer
		if answer == "" {
			answer = "N/A"
		}
		formattedEntries += fmt.Sprintf("- ID: %s\n  Question: \"%s\"\n  Answer: \"%s\"\n\n", entry.ID, entry.CanonicalQuestion, answer)
	}

	return fmt.Sprintf(`Analyze these clarification entries for potential conflicts.
A conflict occurs when entries ask about the same underlying question but have different answers.

ENTRIES:
%s
INSTRUCTIONS:
- Find pairs of entries that are semantically asking the same question
- A conflict exists if the same question has different answers
- Minor wording differences are NOT conflicts (e.g., "Use Vitest" vs "Vitest is preferred")
- Different contexts may justify different answers (note this in reasoning)

Return ONLY valid JSON in this exact format (no markdown, no explanation):
{
  "conflicts": [
    {
      "entry_ids": ["<id1>", "<id2>"],
      "reason": "<why these conflict>",
      "severity": "<high|medium|low>",
      "suggestion": "<how to resolve>"
    }
  ],
  "conflict_count": <number>
}`, formattedEntries)
}
