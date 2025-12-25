package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/samestrin/llm-tools/internal/clarification/tracking"

	"github.com/spf13/cobra"
)

var promoteClarificationCmd = &cobra.Command{
	Use:   "promote-clarification",
	Short: "Promote a clarification to CLAUDE.md",
	Long:  `Promote a clarification entry by appending it to a target file (default: CLAUDE.md) and updating its status.`,
	RunE:  runPromoteClarification,
}

var (
	promoteFile   string
	promoteID     string
	promoteTarget string
	promoteForce  bool
)

func init() {
	rootCmd.AddCommand(promoteClarificationCmd)
	promoteClarificationCmd.Flags().StringVarP(&promoteFile, "file", "f", "", "Tracking file path (required)")
	promoteClarificationCmd.Flags().StringVar(&promoteID, "id", "", "Entry ID to promote (required)")
	promoteClarificationCmd.Flags().StringVar(&promoteTarget, "target", "CLAUDE.md", "Target file for promotion")
	promoteClarificationCmd.Flags().BoolVar(&promoteForce, "force", false, "Force re-promotion of already promoted entry")
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
	// Load tracking file
	if !tracking.FileExists(promoteFile) {
		return fmt.Errorf("tracking file not found: %s", promoteFile)
	}

	tf, err := tracking.LoadTrackingFile(promoteFile)
	if err != nil {
		return fmt.Errorf("failed to load tracking file: %w", err)
	}

	// Find entry by ID
	var entryIndex = -1
	for i, entry := range tf.Entries {
		if entry.ID == promoteID {
			entryIndex = i
			break
		}
	}

	if entryIndex == -1 {
		return fmt.Errorf("entry not found: %s", promoteID)
	}

	entry := &tf.Entries[entryIndex]

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
	tf.LastUpdated = today

	// Save tracking file
	if err := tracking.SaveTrackingFile(tf, promoteFile); err != nil {
		return fmt.Errorf("failed to save tracking file: %w", err)
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

	output, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal result: %w", err)
	}

	fmt.Fprintln(cmd.OutOrStdout(), string(output))
	return nil
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
