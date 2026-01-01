package commands

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/samestrin/llm-tools/pkg/output"
	"github.com/spf13/cobra"
)

var (
	cleanTempName      string
	cleanTempAll       bool
	cleanTempOlderThan string
	cleanTempDryRun    bool
	cleanTempJSON      bool
	cleanTempMinimal   bool
)

// CleanTempResult holds the clean-temp command result
type CleanTempResult struct {
	// Standard output
	Removed      []string `json:"removed,omitempty"`
	RemovedCount int      `json:"removed_count,omitempty"`
	Status       string   `json:"status,omitempty"`
	DryRun       bool     `json:"dry_run,omitempty"`

	// Minimal output aliases
	R  []string `json:"r,omitempty"`  // removed
	RC *int     `json:"rc,omitempty"` // removed_count
	S  string   `json:"s,omitempty"`  // status
	DR *bool    `json:"dr,omitempty"` // dry_run
}

// newCleanTempCmd creates the clean-temp command
func newCleanTempCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "clean-temp",
		Short: "Clean up temp directories",
		Long: `Clean up temp directories created by init-temp.

Modes:
  --name <name>        Remove specific temp directory
  --all                Remove all temp directories
  --older-than <dur>   Remove directories older than duration (e.g., 7d, 24h)

Options:
  --dry-run            Show what would be removed without removing

Examples:
  llm-support clean-temp --name my-workflow
  llm-support clean-temp --all
  llm-support clean-temp --older-than 7d
  llm-support clean-temp --all --dry-run`,
		RunE: runCleanTemp,
	}

	cmd.Flags().StringVar(&cleanTempName, "name", "", "Name of temp directory to remove")
	cmd.Flags().BoolVar(&cleanTempAll, "all", false, "Remove all temp directories")
	cmd.Flags().StringVar(&cleanTempOlderThan, "older-than", "", "Remove directories older than duration (e.g., 7d, 24h, 1h30m)")
	cmd.Flags().BoolVar(&cleanTempDryRun, "dry-run", false, "Show what would be removed without removing")
	cmd.Flags().BoolVar(&cleanTempJSON, "json", false, "Output as JSON")
	cmd.Flags().BoolVar(&cleanTempMinimal, "min", false, "Output in minimal/token-optimized format")

	return cmd
}

func runCleanTemp(cmd *cobra.Command, args []string) error {
	// Validate flags
	if cleanTempName == "" && !cleanTempAll && cleanTempOlderThan == "" {
		return fmt.Errorf("must specify one of: --name, --all, or --older-than")
	}
	if cleanTempName != "" && cleanTempAll {
		return fmt.Errorf("cannot specify both --name and --all")
	}

	// Get repository root
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	repoRoot, err := getRepoRoot(cwd)
	if err != nil {
		return fmt.Errorf("not in a git repository: %w", err)
	}

	baseTemp := filepath.Join(repoRoot, ".planning", ".temp")

	// Check if base temp directory exists
	if _, err := os.Stat(baseTemp); os.IsNotExist(err) {
		return fmt.Errorf("temp directory does not exist: %s", baseTemp)
	}

	var removed []string
	var olderThanDuration time.Duration

	// Parse older-than duration if specified
	if cleanTempOlderThan != "" {
		olderThanDuration, err = parseDuration(cleanTempOlderThan)
		if err != nil {
			return fmt.Errorf("invalid duration %q: %w", cleanTempOlderThan, err)
		}
	}

	if cleanTempName != "" {
		// Remove specific directory
		targetDir := filepath.Join(baseTemp, cleanTempName)
		if _, err := os.Stat(targetDir); os.IsNotExist(err) {
			return fmt.Errorf("temp directory not found: %s", cleanTempName)
		}

		if !cleanTempDryRun {
			if err := os.RemoveAll(targetDir); err != nil {
				return fmt.Errorf("failed to remove %s: %w", cleanTempName, err)
			}
		}
		removed = append(removed, cleanTempName)
	} else {
		// Remove all or older-than
		entries, err := os.ReadDir(baseTemp)
		if err != nil {
			return fmt.Errorf("failed to read temp directory: %w", err)
		}

		cutoff := time.Now().Add(-olderThanDuration)

		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}

			shouldRemove := cleanTempAll

			// Check age if older-than specified
			if cleanTempOlderThan != "" {
				info, err := entry.Info()
				if err != nil {
					continue
				}
				if info.ModTime().Before(cutoff) {
					shouldRemove = true
				}
			}

			if shouldRemove {
				targetDir := filepath.Join(baseTemp, entry.Name())
				if !cleanTempDryRun {
					if err := os.RemoveAll(targetDir); err != nil {
						continue // Skip failed removals
					}
				}
				removed = append(removed, entry.Name())
			}
		}
	}

	// Build result
	status := "REMOVED"
	if cleanTempDryRun {
		status = "DRY_RUN"
	}
	if len(removed) == 0 {
		status = "NONE"
	}

	var result CleanTempResult
	if cleanTempMinimal {
		rc := len(removed)
		dr := cleanTempDryRun
		result = CleanTempResult{
			R:  removed,
			RC: &rc,
			S:  status,
		}
		if cleanTempDryRun {
			result.DR = &dr
		}
	} else {
		result = CleanTempResult{
			Removed:      removed,
			RemovedCount: len(removed),
			Status:       status,
			DryRun:       cleanTempDryRun,
		}
	}

	// Output
	formatter := output.New(cleanTempJSON, cleanTempMinimal, cmd.OutOrStdout())
	return formatter.Print(result, func(w io.Writer, data interface{}) {
		if cleanTempDryRun {
			fmt.Fprintln(w, "DRY RUN - no directories removed")
		}
		fmt.Fprintf(w, "STATUS=%s\n", status)
		fmt.Fprintf(w, "REMOVED_COUNT=%d\n", len(removed))
		for _, name := range removed {
			fmt.Fprintf(w, "REMOVED=%s\n", name)
		}
	})
}

// parseDuration parses duration strings like "7d", "24h", "1h30m"
func parseDuration(s string) (time.Duration, error) {
	// Check for day suffix
	if len(s) > 1 && s[len(s)-1] == 'd' {
		days := s[:len(s)-1]
		var d int
		if _, err := fmt.Sscanf(days, "%d", &d); err != nil {
			return 0, err
		}
		return time.Duration(d) * 24 * time.Hour, nil
	}

	// Standard Go duration parsing
	return time.ParseDuration(s)
}

func init() {
	RootCmd.AddCommand(newCleanTempCmd())
}
