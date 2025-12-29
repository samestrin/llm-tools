package commands

import (
	"context"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/samestrin/llm-tools/internal/clarification/storage"
	"github.com/samestrin/llm-tools/internal/clarification/tracking"
	"github.com/samestrin/llm-tools/pkg/output"

	"github.com/spf13/cobra"
)

var listEntriesCmd = &cobra.Command{
	Use:   "list-entries",
	Short: "List clarification entries",
	Long:  `List all clarification entries from a tracking file with optional filters.`,
	RunE:  runListEntries,
}

var (
	listFile           string
	listStatus         string
	listMinOccurrences int
	listJSON           bool
	listMinimal        bool
)

func init() {
	rootCmd.AddCommand(listEntriesCmd)
	listEntriesCmd.Flags().StringVarP(&listFile, "file", "f", "", "Tracking file path (required)")
	listEntriesCmd.Flags().StringVar(&listStatus, "status", "", "Filter by status (pending, promoted, dismissed)")
	listEntriesCmd.Flags().IntVar(&listMinOccurrences, "min-occurrences", 0, "Filter by minimum occurrences")
	listEntriesCmd.Flags().BoolVar(&listJSON, "json", false, "Output as JSON")
	listEntriesCmd.Flags().BoolVar(&listMinimal, "min", false, "Output in minimal/token-optimized format")
	listEntriesCmd.MarkFlagRequired("file")
}

// ListEntriesResult represents the JSON output of the list-entries command.
type ListEntriesResult struct {
	Count   int              `json:"count"`
	Entries []tracking.Entry `json:"entries"`
}

func runListEntries(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Get storage instance
	store, err := GetStorageOrError(ctx, listFile)
	if err != nil {
		return err
	}
	defer store.Close()

	// Build filter
	filter := storage.ListFilter{
		Status:         listStatus,
		MinOccurrences: listMinOccurrences,
	}

	// Get entries from storage
	entries, err := store.List(ctx, filter)
	if err != nil {
		return fmt.Errorf("failed to list entries: %w", err)
	}

	// Apply any additional filtering (for backward compatibility)
	filtered := filterEntries(entries, listStatus, listMinOccurrences)

	result := ListEntriesResult{
		Count:   len(filtered),
		Entries: filtered,
	}

	formatter := output.New(listJSON, listMinimal, cmd.OutOrStdout())
	return formatter.Print(result, func(w io.Writer, data interface{}) {
		r := data.(ListEntriesResult)
		printEntriesTableToWriter(w, r.Entries)
	})
}

// filterEntries applies status and occurrence filters to entries.
func filterEntries(entries []tracking.Entry, status string, minOccurrences int) []tracking.Entry {
	var filtered []tracking.Entry

	for _, entry := range entries {
		// Apply status filter
		if status != "" && entry.Status != status {
			continue
		}

		// Apply occurrences filter
		if minOccurrences > 0 && entry.Occurrences < minOccurrences {
			continue
		}

		filtered = append(filtered, entry)
	}

	// Return empty slice instead of nil
	if filtered == nil {
		filtered = []tracking.Entry{}
	}

	return filtered
}

// printEntriesTableToWriter prints entries in a formatted table to an io.Writer.
func printEntriesTableToWriter(out io.Writer, entries []tracking.Entry) {
	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	defer w.Flush()

	// Print header
	fmt.Fprintln(w, "ID\tQuestion\tStatus\tOccurrences\tConfidence")
	fmt.Fprintln(w, "--\t--------\t------\t-----------\t----------")

	// Print entries
	for _, entry := range entries {
		question := truncateString(entry.CanonicalQuestion, 40)
		fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%s\n",
			entry.ID,
			question,
			entry.Status,
			entry.Occurrences,
			entry.Confidence,
		)
	}

	fmt.Fprintf(w, "\nTotal: %d entries\n", len(entries))
}

// truncateString truncates a string to maxLen with ellipsis if needed.
func truncateString(s string, maxLen int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
