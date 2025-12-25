package commands

import (
	"encoding/json"
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/samestrin/llm-tools/internal/clarification/tracking"

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
)

func init() {
	rootCmd.AddCommand(listEntriesCmd)
	listEntriesCmd.Flags().StringVarP(&listFile, "file", "f", "", "Tracking file path (required)")
	listEntriesCmd.Flags().StringVar(&listStatus, "status", "", "Filter by status (pending, promoted, dismissed)")
	listEntriesCmd.Flags().IntVar(&listMinOccurrences, "min-occurrences", 0, "Filter by minimum occurrences")
	listEntriesCmd.Flags().BoolVar(&listJSON, "json", false, "Output as JSON")
	listEntriesCmd.MarkFlagRequired("file")
}

// ListEntriesResult represents the JSON output of the list-entries command.
type ListEntriesResult struct {
	Count   int              `json:"count"`
	Entries []tracking.Entry `json:"entries"`
}

func runListEntries(cmd *cobra.Command, args []string) error {
	// Load tracking file
	if !tracking.FileExists(listFile) {
		return fmt.Errorf("tracking file not found: %s", listFile)
	}

	tf, err := tracking.LoadTrackingFile(listFile)
	if err != nil {
		return fmt.Errorf("failed to load tracking file: %w", err)
	}

	// Apply filters
	filtered := filterEntries(tf.Entries, listStatus, listMinOccurrences)

	if listJSON {
		// JSON output
		result := ListEntriesResult{
			Count:   len(filtered),
			Entries: filtered,
		}
		output, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal result: %w", err)
		}
		fmt.Fprintln(cmd.OutOrStdout(), string(output))
	} else {
		// Table output
		printEntriesTable(cmd, filtered)
	}

	return nil
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

// printEntriesTable prints entries in a formatted table.
func printEntriesTable(cmd *cobra.Command, entries []tracking.Entry) {
	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
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
