package commands

import (
	"context"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/samestrin/llm-tools/internal/clarification/storage"
	"github.com/samestrin/llm-tools/pkg/output"
	"github.com/spf13/cobra"
)

var (
	optimizeFile       string
	optimizeVacuum     bool
	optimizePruneStale string
	optimizeStats      bool
	optimizeQuiet      bool
	optimizeJSON       bool
	optimizeMinimal    bool
)

// OptimizeMemoryResult holds the optimization result
type OptimizeMemoryResult struct {
	File          string            `json:"file,omitempty"`
	F             string            `json:"f,omitempty"`
	VacuumBytes   int64             `json:"vacuum_bytes,omitempty"`
	VB            *int64            `json:"vb,omitempty"`
	PrunedEntries int               `json:"pruned_entries,omitempty"`
	PE            *int              `json:"pe,omitempty"`
	TotalEntries  int               `json:"total_entries,omitempty"`
	TE            *int              `json:"te,omitempty"`
	TotalVariants int               `json:"total_variants,omitempty"`
	TV            *int              `json:"tv,omitempty"`
	TotalTags     int               `json:"total_tags,omitempty"`
	TT            *int              `json:"tt,omitempty"`
	TotalSprints  int               `json:"total_sprints,omitempty"`
	TS            *int              `json:"ts,omitempty"`
	StorageSize   int64             `json:"storage_size,omitempty"`
	SS            *int64            `json:"ss,omitempty"`
	LastModified  string            `json:"last_modified,omitempty"`
	LM            string            `json:"lm,omitempty"`
	ByStatus      map[string]int    `json:"by_status,omitempty"`
	BS            map[string]int    `json:"bs,omitempty"`
}

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
	cmd.Flags().BoolVar(&optimizeJSON, "json", false, "Output as JSON")
	cmd.Flags().BoolVar(&optimizeMinimal, "min", false, "Output in minimal/token-optimized format")
	cmd.MarkFlagRequired("file")

	return cmd
}

var optimizeMemoryCmd = NewOptimizeMemoryCmd()

func init() {
	rootCmd.AddCommand(optimizeMemoryCmd)
}

func runOptimizeMemory(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// If no operation specified, show help
	if !optimizeVacuum && optimizePruneStale == "" && !optimizeStats {
		return fmt.Errorf("specify at least one operation: --vacuum, --prune-stale, or --stats")
	}

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

	// Track results for output
	var vacuumBytes int64
	var prunedEntries int
	var stats *storage.StorageStats

	// Handle vacuum
	if optimizeVacuum {
		if storageType != storage.StorageTypeSQLite {
			return fmt.Errorf("vacuum is only supported for SQLite storage")
		}

		freed, err := store.Vacuum(ctx)
		if err != nil {
			return fmt.Errorf("vacuum failed: %w", err)
		}
		vacuumBytes = freed
	}

	// Handle prune-stale
	if optimizePruneStale != "" {
		duration, err := parseDuration(optimizePruneStale)
		if err != nil {
			return fmt.Errorf("invalid duration: %w", err)
		}

		cutoffDate := time.Now().Add(-duration).Format("2006-01-02")

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
			prunedEntries = result.Processed
		}
	}

	// Handle stats
	if optimizeStats {
		s, err := store.Stats(ctx)
		if err != nil {
			return fmt.Errorf("failed to get stats: %w", err)
		}
		stats = s
	}

	// Build output result
	var result OptimizeMemoryResult
	if optimizeMinimal {
		result = OptimizeMemoryResult{F: optimizeFile}
		if optimizeVacuum {
			result.VB = &vacuumBytes
		}
		if optimizePruneStale != "" {
			result.PE = &prunedEntries
		}
		if stats != nil {
			result.TE = &stats.TotalEntries
			result.TV = &stats.TotalVariants
			result.TT = &stats.TotalTags
			result.TS = &stats.TotalSprints
			result.SS = &stats.StorageSize
			result.LM = stats.LastModified
			result.BS = stats.EntriesByStatus
		}
	} else {
		result = OptimizeMemoryResult{File: optimizeFile}
		if optimizeVacuum {
			result.VacuumBytes = vacuumBytes
		}
		if optimizePruneStale != "" {
			result.PrunedEntries = prunedEntries
		}
		if stats != nil {
			result.TotalEntries = stats.TotalEntries
			result.TotalVariants = stats.TotalVariants
			result.TotalTags = stats.TotalTags
			result.TotalSprints = stats.TotalSprints
			result.StorageSize = stats.StorageSize
			result.LastModified = stats.LastModified
			result.ByStatus = stats.EntriesByStatus
		}
	}

	if optimizeQuiet && !optimizeJSON && !optimizeMinimal {
		return nil
	}

	formatter := output.New(optimizeJSON, optimizeMinimal, cmd.OutOrStdout())
	return formatter.Print(result, func(w io.Writer, data interface{}) {
		if !optimizeQuiet {
			if optimizeVacuum {
				fmt.Fprintf(w, "VACUUM complete. Space reclaimed: %d bytes\n", vacuumBytes)
			}
			if optimizePruneStale != "" {
				if prunedEntries > 0 {
					fmt.Fprintf(w, "Pruned %d stale entries\n", prunedEntries)
				} else {
					fmt.Fprintln(w, "No stale entries found")
				}
			}
			if stats != nil {
				fmt.Fprintln(w, "Storage Statistics:")
				fmt.Fprintf(w, "  Total Entries:  %d\n", stats.TotalEntries)
				fmt.Fprintf(w, "  Total Variants: %d\n", stats.TotalVariants)
				fmt.Fprintf(w, "  Total Tags:     %d\n", stats.TotalTags)
				fmt.Fprintf(w, "  Total Sprints:  %d\n", stats.TotalSprints)
				fmt.Fprintf(w, "  Storage Size:   %d bytes\n", stats.StorageSize)
				fmt.Fprintf(w, "  Last Modified:  %s\n", stats.LastModified)
				fmt.Fprintln(w, "  By Status:")
				for status, count := range stats.EntriesByStatus {
					fmt.Fprintf(w, "    %s: %d\n", status, count)
				}
			}
		}
	})
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
