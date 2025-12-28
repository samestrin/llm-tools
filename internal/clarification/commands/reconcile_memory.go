package commands

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/samestrin/llm-tools/internal/clarification/storage"
	"github.com/samestrin/llm-tools/internal/clarification/tracking"
	"github.com/spf13/cobra"
)

var (
	reconcileFile        string
	reconcileProjectRoot string
	reconcileDryRun      bool
	reconcileQuiet       bool
)

// NewReconcileMemoryCmd creates a new reconcile-memory command.
func NewReconcileMemoryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "reconcile-memory",
		Short: "Reconcile clarification references with codebase",
		Long: `Scan clarification entries for file path references and identify
references to files that no longer exist in the codebase.

Stale references are marked with status 'stale' for review.
Use --dry-run to preview changes without modifying the database.`,
		RunE: runReconcileMemory,
	}

	cmd.Flags().StringVarP(&reconcileFile, "file", "f", "", "Storage file path (required)")
	cmd.Flags().StringVarP(&reconcileProjectRoot, "project-root", "p", "", "Project root directory (required)")
	cmd.Flags().BoolVar(&reconcileDryRun, "dry-run", false, "Show changes without applying")
	cmd.Flags().BoolVarP(&reconcileQuiet, "quiet", "q", false, "Suppress output")
	cmd.MarkFlagRequired("file")
	cmd.MarkFlagRequired("project-root")

	return cmd
}

var reconcileMemoryCmd = NewReconcileMemoryCmd()

func init() {
	rootCmd.AddCommand(reconcileMemoryCmd)
}

func runReconcileMemory(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Verify project root exists
	if _, err := os.Stat(reconcileProjectRoot); os.IsNotExist(err) {
		return fmt.Errorf("project root not found: %s", reconcileProjectRoot)
	}

	// Open storage
	store, err := storage.NewStorage(ctx, reconcileFile)
	if err != nil {
		return fmt.Errorf("failed to open storage: %w", err)
	}
	defer store.Close()

	// Get all entries
	entries, err := store.List(ctx, storage.ListFilter{})
	if err != nil {
		return fmt.Errorf("failed to list entries: %w", err)
	}

	if !reconcileQuiet {
		if reconcileDryRun {
			fmt.Fprintln(cmd.OutOrStdout(), "[DRY RUN] Scanning for stale references...")
		} else {
			fmt.Fprintln(cmd.OutOrStdout(), "Scanning for stale references...")
		}
	}

	var staleEntries []tracking.Entry
	var staleDetails []string

	for _, entry := range entries {
		// Extract file references from entry
		refs := extractFileReferences(&entry)

		// Check if any referenced files don't exist
		for _, ref := range refs {
			fullPath := filepath.Join(reconcileProjectRoot, ref)
			if _, err := os.Stat(fullPath); os.IsNotExist(err) {
				staleEntries = append(staleEntries, entry)
				staleDetails = append(staleDetails, fmt.Sprintf("  - %s: references '%s' (not found)", entry.ID, ref))
				break // Only count entry once even if multiple stale refs
			}
		}
	}

	if !reconcileQuiet {
		if len(staleEntries) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "No stale references found")
			return nil
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Found %d entries with stale references:\n", len(staleEntries))
		for _, detail := range staleDetails {
			fmt.Fprintln(cmd.OutOrStdout(), detail)
		}
	}

	// Apply changes (unless dry-run)
	if !reconcileDryRun {
		for _, entry := range staleEntries {
			entry.Status = "stale"
			if err := store.Update(ctx, &entry); err != nil {
				if !reconcileQuiet {
					fmt.Fprintf(cmd.OutOrStdout(), "Warning: failed to mark entry %s as stale: %v\n", entry.ID, err)
				}
			}
		}

		if !reconcileQuiet {
			fmt.Fprintf(cmd.OutOrStdout(), "\nMarked %d entries as stale\n", len(staleEntries))
		}
	} else {
		if !reconcileQuiet {
			fmt.Fprintln(cmd.OutOrStdout(), "\n[DRY RUN] No changes made")
		}
	}

	return nil
}

// extractFileReferences extracts potential file path references from an entry.
// It checks context_tags and scans the answer text for file patterns.
func extractFileReferences(entry *tracking.Entry) []string {
	var refs []string
	seen := make(map[string]bool)

	// Check context_tags for file-like patterns
	filePattern := regexp.MustCompile(`\.(go|ts|tsx|js|jsx|py|rs|java|rb|php|md|yaml|yml|json|toml|sql)$`)
	for _, tag := range entry.ContextTags {
		if filePattern.MatchString(tag) && !seen[tag] {
			refs = append(refs, tag)
			seen[tag] = true
		}
	}

	// Scan answer text for file path patterns (conservative)
	// Look for patterns like: path/to/file.ext or ./file.ext
	pathPattern := regexp.MustCompile("(?:^|[\\s])([a-zA-Z0-9_./\\-]+\\.(go|ts|tsx|js|jsx|py|rs|java|rb|php|md|yaml|yml|json|toml|sql))(?:[\\s,)]|$)")
	matches := pathPattern.FindAllStringSubmatch(entry.CurrentAnswer, -1)
	for _, match := range matches {
		if len(match) > 1 {
			filePath := strings.TrimPrefix(match[1], "./")
			if !seen[filePath] {
				refs = append(refs, filePath)
				seen[filePath] = true
			}
		}
	}

	return refs
}
