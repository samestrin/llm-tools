package commands

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/samestrin/llm-tools/internal/clarification/storage"
	"github.com/samestrin/llm-tools/internal/clarification/tracking"
	"github.com/samestrin/llm-tools/internal/clarification/utils"
	"github.com/samestrin/llm-tools/pkg/output"

	"github.com/spf13/cobra"
)

var addClarificationCmd = &cobra.Command{
	Use:   "add-clarification",
	Short: "Add or update a clarification entry",
	Long:  `Add a new clarification entry or update an existing one in the tracking file.`,
	RunE:  runAddClarification,
}

var (
	addFile       string
	addQuestion   string
	addAnswer     string
	addID         string
	addSprint     string
	addTags       []string
	addCheckMatch bool
	addJSON       bool
	addMinimal    bool
)

func init() {
	rootCmd.AddCommand(addClarificationCmd)
	addClarificationCmd.Flags().StringVarP(&addFile, "file", "f", "", "Tracking file path (required)")
	addClarificationCmd.Flags().StringVarP(&addQuestion, "question", "q", "", "Question text")
	addClarificationCmd.Flags().StringVarP(&addAnswer, "answer", "a", "", "Answer text")
	addClarificationCmd.Flags().StringVar(&addID, "id", "", "Entry ID (for updates)")
	addClarificationCmd.Flags().StringVarP(&addSprint, "sprint", "s", "", "Sprint name")
	addClarificationCmd.Flags().StringSliceVarP(&addTags, "tag", "t", nil, "Context tags")
	addClarificationCmd.Flags().BoolVar(&addCheckMatch, "check-match", false, "Check for similar questions")
	addClarificationCmd.Flags().BoolVar(&addJSON, "json", false, "Output as JSON")
	addClarificationCmd.Flags().BoolVar(&addMinimal, "min", false, "Output in minimal/token-optimized format")
	addClarificationCmd.MarkFlagRequired("file")
}

// AddClarificationResult represents the JSON output of the add-clarification command.
type AddClarificationResult struct {
	Status           string   `json:"status"`
	ID               string   `json:"id"`
	Message          string   `json:"message"`
	PotentialMatches []string `json:"potential_matches,omitempty"`
}

// PotentialMatch represents a similar existing question.
type PotentialMatch struct {
	ID       string `json:"id"`
	Question string `json:"question"`
}

func runAddClarification(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Get storage instance
	store, err := GetStorageOrError(ctx, addFile)
	if err != nil {
		return err
	}
	defer store.Close()

	today := time.Now().Format("2006-01-02")
	var result AddClarificationResult
	var potentialMatches []string

	// Check for similar questions if requested
	if addCheckMatch && addQuestion != "" {
		entries, err := store.List(ctx, storage.ListFilter{})
		if err != nil {
			return fmt.Errorf("failed to list entries: %w", err)
		}
		for _, entry := range entries {
			if isSimilarQuestion(entry.CanonicalQuestion, addQuestion) {
				potentialMatches = append(potentialMatches, entry.ID)
			}
		}
	}

	// Update existing entry or create new
	if addID != "" {
		// Update existing entry
		entry, err := store.Read(ctx, addID)
		if err != nil {
			if err == storage.ErrNotFound {
				return fmt.Errorf("entry not found: %s", addID)
			}
			return fmt.Errorf("failed to read entry: %w", err)
		}

		// Update the entry
		if addAnswer != "" {
			entry.CurrentAnswer = addAnswer
		}
		entry.Occurrences++
		entry.LastSeen = today
		if addSprint != "" {
			entry.SprintsSeen = appendUnique(entry.SprintsSeen, addSprint)
		}
		if len(addTags) > 0 {
			entry.ContextTags = mergeUnique(entry.ContextTags, addTags)
		}

		if err := store.Update(ctx, entry); err != nil {
			return fmt.Errorf("failed to update entry: %w", err)
		}

		result = AddClarificationResult{
			Status:  "updated",
			ID:      addID,
			Message: "Entry updated successfully",
		}
	} else {
		// Create new entry
		if addQuestion == "" {
			return fmt.Errorf("question is required for new entries")
		}

		newID := utils.GenerateID(addQuestion)
		entry := &tracking.Entry{
			ID:                newID,
			CanonicalQuestion: addQuestion,
			CurrentAnswer:     addAnswer,
			Occurrences:       1,
			FirstSeen:         today,
			LastSeen:          today,
			Status:            "pending",
			Confidence:        "medium",
		}
		if addSprint != "" {
			entry.SprintsSeen = []string{addSprint}
		}
		if len(addTags) > 0 {
			entry.ContextTags = addTags
		}

		if err := store.Create(ctx, entry); err != nil {
			return fmt.Errorf("failed to create entry: %w", err)
		}

		result = AddClarificationResult{
			Status:  "created",
			ID:      newID,
			Message: "New entry created successfully",
		}
	}

	// Add potential matches to result
	if len(potentialMatches) > 0 {
		result.PotentialMatches = potentialMatches
	}

	// Output result using Formatter
	formatter := output.New(addJSON, addMinimal, cmd.OutOrStdout())
	return formatter.Print(result, func(w io.Writer, data interface{}) {
		r := data.(AddClarificationResult)
		fmt.Fprintf(w, "STATUS: %s\n", r.Status)
		fmt.Fprintf(w, "ID: %s\n", r.ID)
		fmt.Fprintf(w, "MESSAGE: %s\n", r.Message)
		if len(r.PotentialMatches) > 0 {
			fmt.Fprintf(w, "POTENTIAL_MATCHES: %s\n", strings.Join(r.PotentialMatches, ", "))
		}
	})
}

// isSimilarQuestion checks if two questions are similar using simple keyword matching.
func isSimilarQuestion(q1, q2 string) bool {
	// Normalize and tokenize
	words1 := normalizeAndTokenize(q1)
	words2 := normalizeAndTokenize(q2)

	// Check for significant word overlap
	commonWords := 0
	for _, w1 := range words1 {
		for _, w2 := range words2 {
			if w1 == w2 && len(w1) > 3 { // Only count words longer than 3 chars
				commonWords++
				break
			}
		}
	}

	// Consider similar if 50% or more words match
	minWords := len(words1)
	if len(words2) < minWords {
		minWords = len(words2)
	}
	if minWords == 0 {
		return false
	}

	return float64(commonWords)/float64(minWords) >= 0.5
}

// normalizeAndTokenize converts a string to lowercase and splits into words.
func normalizeAndTokenize(s string) []string {
	s = strings.ToLower(s)
	// Remove common punctuation
	s = strings.ReplaceAll(s, "?", "")
	s = strings.ReplaceAll(s, ".", "")
	s = strings.ReplaceAll(s, ",", "")
	return strings.Fields(s)
}

// appendUnique adds an item to a slice if not already present.
func appendUnique(slice []string, item string) []string {
	for _, s := range slice {
		if s == item {
			return slice
		}
	}
	return append(slice, item)
}

// mergeUnique merges two slices, keeping only unique items.
func mergeUnique(slice1, slice2 []string) []string {
	seen := make(map[string]bool)
	for _, s := range slice1 {
		seen[s] = true
	}
	result := slice1
	for _, s := range slice2 {
		if !seen[s] {
			result = append(result, s)
			seen[s] = true
		}
	}
	return result
}
