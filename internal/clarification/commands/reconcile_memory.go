package commands

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/samestrin/llm-tools/internal/clarification/storage"
	"github.com/samestrin/llm-tools/internal/clarification/tracking"
	"github.com/samestrin/llm-tools/pkg/output"
	"github.com/spf13/cobra"
)

var (
	reconcileFile        string
	reconcileProjectRoot string
	reconcileDryRun      bool
	reconcileQuiet       bool
	reconcileJSON        bool
	reconcileMinimal     bool
)

// ReconcileMemoryResult holds the reconciliation result
type ReconcileMemoryResult struct {
	File          string   `json:"file,omitempty"`
	F             string   `json:"f,omitempty"`
	ProjectRoot   string   `json:"project_root,omitempty"`
	PR            string   `json:"pr,omitempty"`
	DryRun        bool     `json:"dry_run,omitempty"`
	DR            *bool    `json:"dr,omitempty"`
	TotalScanned  int      `json:"total_scanned,omitempty"`
	TS            *int     `json:"ts,omitempty"`
	StaleFound    int      `json:"stale_found,omitempty"`
	SF            *int     `json:"sf,omitempty"`
	StaleEntries  []string `json:"stale_entries,omitempty"`
	SE            []string `json:"se,omitempty"`
	MarkedAsStale int      `json:"marked_as_stale,omitempty"`
	MS            *int     `json:"ms,omitempty"`
}

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
	cmd.Flags().BoolVar(&reconcileJSON, "json", false, "Output as JSON")
	cmd.Flags().BoolVar(&reconcileMinimal, "min", false, "Output in minimal/token-optimized format")
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

	var staleEntries []tracking.Entry
	var staleDetails []string
	var staleIDs []string

	for _, entry := range entries {
		// Extract file references from entry
		refs := extractFileReferences(&entry)

		// Check if any referenced files don't exist
		for _, ref := range refs {
			fullPath := filepath.Join(reconcileProjectRoot, ref)
			if _, err := os.Stat(fullPath); os.IsNotExist(err) {
				staleEntries = append(staleEntries, entry)
				staleIDs = append(staleIDs, entry.ID)
				staleDetails = append(staleDetails, fmt.Sprintf("  - %s: references '%s' (not found)", entry.ID, ref))
				break // Only count entry once even if multiple stale refs
			}
		}
	}

	// Apply changes (unless dry-run)
	markedCount := 0
	if !reconcileDryRun && len(staleEntries) > 0 {
		for _, entry := range staleEntries {
			entry.Status = "stale"
			if err := store.Update(ctx, &entry); err == nil {
				markedCount++
			}
		}
	}

	// Build output result
	totalScanned := len(entries)
	staleFound := len(staleEntries)
	var result ReconcileMemoryResult
	if reconcileMinimal {
		result = ReconcileMemoryResult{
			F:  reconcileFile,
			PR: reconcileProjectRoot,
			DR: &reconcileDryRun,
			TS: &totalScanned,
			SF: &staleFound,
			SE: staleIDs,
			MS: &markedCount,
		}
	} else {
		result = ReconcileMemoryResult{
			File:          reconcileFile,
			ProjectRoot:   reconcileProjectRoot,
			DryRun:        reconcileDryRun,
			TotalScanned:  totalScanned,
			StaleFound:    staleFound,
			StaleEntries:  staleIDs,
			MarkedAsStale: markedCount,
		}
	}

	if reconcileQuiet && !reconcileJSON && !reconcileMinimal {
		return nil
	}

	formatter := output.New(reconcileJSON, reconcileMinimal, cmd.OutOrStdout())
	return formatter.Print(result, func(w io.Writer, data interface{}) {
		if !reconcileQuiet {
			if reconcileDryRun {
				fmt.Fprintln(w, "[DRY RUN] Scanning for stale references...")
			} else {
				fmt.Fprintln(w, "Scanning for stale references...")
			}

			if len(staleEntries) == 0 {
				fmt.Fprintln(w, "No stale references found")
			} else {
				fmt.Fprintf(w, "Found %d entries with stale references:\n", len(staleEntries))
				for _, detail := range staleDetails {
					fmt.Fprintln(w, detail)
				}

				if !reconcileDryRun {
					fmt.Fprintf(w, "\nMarked %d entries as stale\n", markedCount)
				} else {
					fmt.Fprintln(w, "\n[DRY RUN] No changes made")
				}
			}
		}
	})
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
