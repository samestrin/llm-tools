package commands

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/samestrin/llm-tools/internal/clarification/storage"
	"github.com/samestrin/llm-tools/pkg/output"
	"github.com/spf13/cobra"
)

var (
	deleteFile    string
	deleteID      string
	deleteForce   bool
	deleteQuiet   bool
	deleteJSON    bool
	deleteMinimal bool
)

// DeleteClarificationResult holds the deletion result
type DeleteClarificationResult struct {
	File      string `json:"file,omitempty"`
	F         string `json:"f,omitempty"`
	ID        string `json:"id,omitempty"`
	I         string `json:"i,omitempty"`
	Question  string `json:"question,omitempty"`
	Q         string `json:"q,omitempty"`
	Status    string `json:"status,omitempty"`
	S         string `json:"s,omitempty"`
	Deleted   bool   `json:"deleted,omitempty"`
	D         *bool  `json:"d,omitempty"`
	Cancelled bool   `json:"cancelled,omitempty"`
	C         *bool  `json:"c,omitempty"`
}

// NewDeleteClarificationCmd creates a new delete-clarification command.
func NewDeleteClarificationCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete-clarification",
		Short: "Delete a clarification entry",
		Long:  `Delete a clarification entry by ID from the storage file.`,
		RunE:  runDeleteClarification,
	}

	cmd.Flags().StringVarP(&deleteFile, "file", "f", "", "Storage file path (required)")
	cmd.Flags().StringVar(&deleteID, "id", "", "Entry ID to delete (required)")
	cmd.Flags().BoolVar(&deleteForce, "force", false, "Skip confirmation prompt")
	cmd.Flags().BoolVarP(&deleteQuiet, "quiet", "q", false, "Suppress output")
	cmd.Flags().BoolVar(&deleteJSON, "json", false, "Output as JSON")
	cmd.Flags().BoolVar(&deleteMinimal, "min", false, "Output in minimal/token-optimized format")
	cmd.MarkFlagRequired("file")
	cmd.MarkFlagRequired("id")

	return cmd
}

var deleteClarificationCmd = NewDeleteClarificationCmd()

func init() {
	rootCmd.AddCommand(deleteClarificationCmd)
}

func runDeleteClarification(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Open storage
	store, err := storage.NewStorage(ctx, deleteFile)
	if err != nil {
		return fmt.Errorf("failed to open storage: %w", err)
	}
	defer store.Close()

	// Read entry to show details
	entry, err := store.Read(ctx, deleteID)
	if err != nil {
		return fmt.Errorf("entry not found: %s", deleteID)
	}

	// Show entry details before deletion (unless quiet or JSON mode)
	if !deleteQuiet && !deleteJSON {
		fmt.Fprintln(cmd.OutOrStdout(), "Entry to delete:")
		fmt.Fprintf(cmd.OutOrStdout(), "  ID:          %s\n", entry.ID)
		fmt.Fprintf(cmd.OutOrStdout(), "  Question:    %s\n", truncateString(entry.CanonicalQuestion, 60))
		fmt.Fprintf(cmd.OutOrStdout(), "  Answer:      %s\n", truncateString(entry.CurrentAnswer, 60))
		fmt.Fprintf(cmd.OutOrStdout(), "  Status:      %s\n", entry.Status)
		fmt.Fprintf(cmd.OutOrStdout(), "  Occurrences: %d\n", entry.Occurrences)
		fmt.Fprintf(cmd.OutOrStdout(), "  First Seen:  %s\n", entry.FirstSeen)
		fmt.Fprintf(cmd.OutOrStdout(), "  Last Seen:   %s\n", entry.LastSeen)
		if len(entry.ContextTags) > 0 {
			fmt.Fprintf(cmd.OutOrStdout(), "  Tags:        %s\n", strings.Join(entry.ContextTags, ", "))
		}
		fmt.Fprintln(cmd.OutOrStdout())
	}

	// Confirm deletion (unless force or JSON mode)
	cancelled := false
	if !deleteForce && !deleteJSON {
		fmt.Fprintf(cmd.OutOrStdout(), "Are you sure you want to delete this entry? [y/N]: ")
		reader := bufio.NewReader(cmd.InOrStdin())
		response, _ := reader.ReadString('\n')
		response = strings.TrimSpace(strings.ToLower(response))

		if response != "y" && response != "yes" {
			cancelled = true
		}
	}

	// Build result
	deleted := false
	if !cancelled {
		// Delete entry
		if err := store.Delete(ctx, deleteID); err != nil {
			return fmt.Errorf("failed to delete entry: %w", err)
		}
		deleted = true
	}

	// Build output result
	var result DeleteClarificationResult
	if deleteMinimal {
		result = DeleteClarificationResult{
			F: deleteFile,
			I: deleteID,
			Q: truncateString(entry.CanonicalQuestion, 40),
			D: &deleted,
			C: &cancelled,
		}
	} else {
		result = DeleteClarificationResult{
			File:      deleteFile,
			ID:        deleteID,
			Question:  entry.CanonicalQuestion,
			Status:    entry.Status,
			Deleted:   deleted,
			Cancelled: cancelled,
		}
	}

	if deleteQuiet && !deleteJSON && !deleteMinimal {
		return nil
	}

	formatter := output.New(deleteJSON, deleteMinimal, cmd.OutOrStdout())
	return formatter.Print(result, func(w io.Writer, data interface{}) {
		if !deleteQuiet {
			if cancelled {
				fmt.Fprintln(w, "Deletion cancelled.")
			} else {
				fmt.Fprintf(w, "Entry %s deleted successfully.\n", deleteID)
			}
		}
	})
}
