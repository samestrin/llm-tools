package commands

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/samestrin/llm-tools/internal/clarification/storage"
	"github.com/samestrin/llm-tools/internal/clarification/tracking"
	"github.com/samestrin/llm-tools/pkg/output"

	"github.com/spf13/cobra"
)

var promoteClarificationCmd = &cobra.Command{
	Use:   "promote-clarification",
	Short: "Promote a clarification to CLAUDE.md",
	Long:  `Promote a clarification entry by appending it to a target file (default: CLAUDE.md) and updating its status.`,
	RunE:  runPromoteClarification,
}

var (
	promoteFile    string
	promoteID      string
	promoteTarget  string
	promoteForce   bool
	promoteJSON    bool
	promoteMinimal bool
)

func init() {
	rootCmd.AddCommand(promoteClarificationCmd)
	promoteClarificationCmd.Flags().StringVarP(&promoteFile, "file", "f", "", "Tracking file path (required)")
	promoteClarificationCmd.Flags().StringVar(&promoteID, "id", "", "Entry ID to promote (required)")
	promoteClarificationCmd.Flags().StringVar(&promoteTarget, "target", "CLAUDE.md", "Target file for promotion")
	promoteClarificationCmd.Flags().BoolVar(&promoteForce, "force", false, "Force re-promotion of already promoted entry")
	promoteClarificationCmd.Flags().BoolVar(&promoteJSON, "json", false, "Output as JSON")
	promoteClarificationCmd.Flags().BoolVar(&promoteMinimal, "min", false, "Output in minimal/token-optimized format")
	promoteClarificationCmd.MarkFlagRequired("file")
	promoteClarificationCmd.MarkFlagRequired("id")
}

// PromoteResult represents the JSON output of the promote-clarification command.
type PromoteResult struct {
	Status  string `json:"status"`
	ID      string `json:"id"`
	Target  string `json:"target"`
	Message string `json:"message"`
}

func runPromoteClarification(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Get storage instance
	store, err := GetStorageOrError(ctx, promoteFile)
	if err != nil {
		return err
	}
	defer store.Close()

	// Find entry by ID
	entry, err := store.Read(ctx, promoteID)
	if err != nil {
		if err == storage.ErrNotFound {
			return fmt.Errorf("entry not found: %s", promoteID)
		}
		return fmt.Errorf("failed to read entry: %w", err)
	}

	// Check if already promoted
	if entry.Status == "promoted" && !promoteForce {
		return fmt.Errorf("entry already promoted to %s on %s (use --force to re-promote)", entry.PromotedTo, entry.PromotedDate)
	}

	// Format entry for CLAUDE.md
	formattedContent := formatForClaudeMD(entry)

	// Append to target file
	targetPath := promoteTarget
	if err := appendToFile(targetPath, formattedContent); err != nil {
		return fmt.Errorf("failed to append to target file: %w", err)
	}

	// Update entry status
	today := time.Now().Format("2006-01-02")
	wasAlreadyPromoted := entry.Status == "promoted"
	entry.Status = "promoted"
	entry.PromotedTo = filepath.Base(targetPath)
	entry.PromotedDate = today

	// Save updated entry
	if err := store.Update(ctx, entry); err != nil {
		return fmt.Errorf("failed to update entry: %w", err)
	}

	// Prepare result
	status := "promoted"
	message := "Entry promoted successfully"
	if wasAlreadyPromoted {
		status = "re-promoted"
		message = "Entry re-promoted successfully"
	}

	result := PromoteResult{
		Status:  status,
		ID:      promoteID,
		Target:  targetPath,
		Message: message,
	}

	formatter := output.New(promoteJSON, promoteMinimal, cmd.OutOrStdout())
	return formatter.Print(result, printPromoteText)
}

func printPromoteText(w io.Writer, data interface{}) {
	r := data.(PromoteResult)
	fmt.Fprintf(w, "STATUS: %s\n", r.Status)
	fmt.Fprintf(w, "ID: %s\n", r.ID)
	fmt.Fprintf(w, "TARGET: %s\n", r.Target)
	fmt.Fprintf(w, "MESSAGE: %s\n", r.Message)
}

// formatForClaudeMD formats a clarification entry for CLAUDE.md.
func formatForClaudeMD(entry *tracking.Entry) string {
	return fmt.Sprintf(`
## %s

**Question:** %s

**Answer:** %s

*Source: Clarification Learning System (ID: %s, Occurrences: %d)*

---
`, entry.CanonicalQuestion, entry.CanonicalQuestion, entry.CurrentAnswer, entry.ID, entry.Occurrences)
}

// appendToFile appends content to a file, creating it if necessary.
func appendToFile(path string, content string) error {
	// Ensure parent directory exists
	dir := filepath.Dir(path)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}
	}

	// Open file for appending (create if not exists)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.WriteString(content)
	return err
}
