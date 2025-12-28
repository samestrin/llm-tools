package commands

import (
	"bufio"
	"context"
	"fmt"
	"strings"

	"github.com/samestrin/llm-tools/internal/clarification/storage"
	"github.com/spf13/cobra"
)

var (
	deleteFile  string
	deleteID    string
	deleteForce bool
	deleteQuiet bool
)

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

	// Show entry details before deletion (unless quiet)
	if !deleteQuiet {
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

	// Confirm deletion (unless force)
	if !deleteForce {
		fmt.Fprintf(cmd.OutOrStdout(), "Are you sure you want to delete this entry? [y/N]: ")
		reader := bufio.NewReader(cmd.InOrStdin())
		response, _ := reader.ReadString('\n')
		response = strings.TrimSpace(strings.ToLower(response))

		if response != "y" && response != "yes" {
			if !deleteQuiet {
				fmt.Fprintln(cmd.OutOrStdout(), "Deletion cancelled.")
			}
			return nil
		}
	}

	// Delete entry
	if err := store.Delete(ctx, deleteID); err != nil {
		return fmt.Errorf("failed to delete entry: %w", err)
	}

	if !deleteQuiet {
		fmt.Fprintf(cmd.OutOrStdout(), "Entry %s deleted successfully.\n", deleteID)
	}

	return nil
}
