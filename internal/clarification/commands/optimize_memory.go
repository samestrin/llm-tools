package commands

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/samestrin/llm-tools/internal/clarification/storage"
	"github.com/spf13/cobra"
)

var (
	optimizeFile       string
	optimizeVacuum     bool
	optimizePruneStale string
	optimizeStats      bool
	optimizeQuiet      bool
)

// NewOptimizeMemoryCmd creates a new optimize-memory command.
func NewOptimizeMemoryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "optimize-memory",
		Short: "Optimize clarification storage",
		Long: `Optimize clarification storage for better performance and reduced size.

Operations:
  --vacuum       Run SQLite VACUUM to reclaim space (SQLite only)
  --prune-stale  Remove entries older than specified duration (e.g., 30d, 90d)
  --stats        Show storage statistics`,
		RunE: runOptimizeMemory,
	}

	cmd.Flags().StringVarP(&optimizeFile, "file", "f", "", "Storage file path (required)")
	cmd.Flags().BoolVar(&optimizeVacuum, "vacuum", false, "Run SQLite VACUUM")
	cmd.Flags().StringVar(&optimizePruneStale, "prune-stale", "", "Remove entries older than duration (e.g., 30d)")
	cmd.Flags().BoolVar(&optimizeStats, "stats", false, "Show storage statistics")
	cmd.Flags().BoolVarP(&optimizeQuiet, "quiet", "q", false, "Suppress output")
	cmd.MarkFlagRequired("file")

	return cmd
}

var optimizeMemoryCmd = NewOptimizeMemoryCmd()

func init() {
	rootCmd.AddCommand(optimizeMemoryCmd)
}

func runOptimizeMemory(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Detect storage type
	storageType, err := storage.DetectStorageType(optimizeFile)
	if err != nil {
		return err
	}

	// Open storage
	store, err := storage.NewStorage(ctx, optimizeFile)
	if err != nil {
		return fmt.Errorf("failed to open storage: %w", err)
	}
	defer store.Close()

	// Handle vacuum
	if optimizeVacuum {
		if storageType != storage.StorageTypeSQLite {
			return fmt.Errorf("vacuum is only supported for SQLite storage")
		}

		if !optimizeQuiet {
			fmt.Fprintln(cmd.OutOrStdout(), "Running VACUUM...")
		}

		freed, err := store.Vacuum(ctx)
		if err != nil {
			return fmt.Errorf("vacuum failed: %w", err)
		}

		if !optimizeQuiet {
			fmt.Fprintf(cmd.OutOrStdout(), "VACUUM complete. Space reclaimed: %d bytes\n", freed)
		}
	}

	// Handle prune-stale
	if optimizePruneStale != "" {
		duration, err := parseDuration(optimizePruneStale)
		if err != nil {
			return fmt.Errorf("invalid duration: %w", err)
		}

		cutoffDate := time.Now().Add(-duration).Format("2006-01-02")

		if !optimizeQuiet {
			fmt.Fprintf(cmd.OutOrStdout(), "Pruning entries older than %s (last seen before %s)...\n",
				optimizePruneStale, cutoffDate)
		}

		// Find entries to prune
		entries, err := store.List(ctx, storage.ListFilter{})
		if err != nil {
			return fmt.Errorf("failed to list entries: %w", err)
		}

		var toDelete []string
		for _, entry := range entries {
			if entry.LastSeen < cutoffDate {
				toDelete = append(toDelete, entry.ID)
			}
		}

		if len(toDelete) > 0 {
			result, err := store.BulkDelete(ctx, toDelete)
			if err != nil {
				return fmt.Errorf("failed to delete stale entries: %w", err)
			}

			if !optimizeQuiet {
				fmt.Fprintf(cmd.OutOrStdout(), "Pruned %d stale entries\n", result.Processed)
			}
		} else {
			if !optimizeQuiet {
				fmt.Fprintln(cmd.OutOrStdout(), "No stale entries found")
			}
		}
	}

	// Handle stats
	if optimizeStats {
		stats, err := store.Stats(ctx)
		if err != nil {
			return fmt.Errorf("failed to get stats: %w", err)
		}

		if !optimizeQuiet {
			fmt.Fprintln(cmd.OutOrStdout(), "Storage Statistics:")
			fmt.Fprintf(cmd.OutOrStdout(), "  Total Entries:  %d\n", stats.TotalEntries)
			fmt.Fprintf(cmd.OutOrStdout(), "  Total Variants: %d\n", stats.TotalVariants)
			fmt.Fprintf(cmd.OutOrStdout(), "  Total Tags:     %d\n", stats.TotalTags)
			fmt.Fprintf(cmd.OutOrStdout(), "  Total Sprints:  %d\n", stats.TotalSprints)
			fmt.Fprintf(cmd.OutOrStdout(), "  Storage Size:   %d bytes\n", stats.StorageSize)
			fmt.Fprintf(cmd.OutOrStdout(), "  Last Modified:  %s\n", stats.LastModified)
			fmt.Fprintln(cmd.OutOrStdout(), "  By Status:")
			for status, count := range stats.EntriesByStatus {
				fmt.Fprintf(cmd.OutOrStdout(), "    %s: %d\n", status, count)
			}
		}
	}

	// If no operation specified, show help
	if !optimizeVacuum && optimizePruneStale == "" && !optimizeStats {
		return fmt.Errorf("specify at least one operation: --vacuum, --prune-stale, or --stats")
	}

	return nil
}

// parseDuration parses a duration string like "30d", "90d", "1y"
func parseDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(strings.ToLower(s))

	re := regexp.MustCompile(`^(\d+)([dwmy])$`)
	matches := re.FindStringSubmatch(s)
	if matches == nil {
		return 0, fmt.Errorf("invalid format: use format like 30d, 90d, 1y")
	}

	value, _ := strconv.Atoi(matches[1])
	unit := matches[2]

	switch unit {
	case "d":
		return time.Duration(value) * 24 * time.Hour, nil
	case "w":
		return time.Duration(value) * 7 * 24 * time.Hour, nil
	case "m":
		return time.Duration(value) * 30 * 24 * time.Hour, nil
	case "y":
		return time.Duration(value) * 365 * 24 * time.Hour, nil
	default:
		return 0, fmt.Errorf("unknown unit: %s", unit)
	}
}
