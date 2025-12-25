package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/samestrin/llm-tools/internal/support/gitignore"
	"github.com/samestrin/llm-tools/internal/support/utils"
	"github.com/spf13/cobra"
)

var (
	statsNoGitignore bool
	statsPath        string
)

// newStatsCmd creates the stats command
func newStatsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stats",
		Short: "Display directory statistics",
		Long: `Display statistics about a directory including file counts,
total size, and breakdown by file extension.`,
		Args: cobra.NoArgs,
		RunE: runStats,
	}
	cmd.Flags().StringVar(&statsPath, "path", ".", "Directory path to analyze")
	cmd.Flags().BoolVar(&statsNoGitignore, "no-gitignore", false, "Disable .gitignore filtering")
	return cmd
}

func runStats(cmd *cobra.Command, args []string) error {
	path, err := filepath.Abs(statsPath)
	if err != nil {
		return fmt.Errorf("invalid path: %s", statsPath)
	}

	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("path does not exist: %s", path)
	}

	if !info.IsDir() {
		return fmt.Errorf("path is not a directory: %s", path)
	}

	var ignorer *gitignore.Parser
	if !statsNoGitignore {
		ignorer, _ = gitignore.NewParser(path)
	}

	var totalFiles int
	var totalDirs int
	var totalSize int64
	extCounts := make(map[string]int)
	extSizes := make(map[string]int64)

	filepath.Walk(path, func(filePath string, fileInfo os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		// Skip hidden files unless --no-gitignore
		if !statsNoGitignore && strings.HasPrefix(fileInfo.Name(), ".") {
			if fileInfo.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Check gitignore
		if ignorer != nil && ignorer.IsIgnored(filePath) {
			if fileInfo.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if fileInfo.IsDir() {
			if filePath != path {
				totalDirs++
			}
		} else {
			totalFiles++
			size := fileInfo.Size()
			totalSize += size

			ext := strings.ToLower(filepath.Ext(fileInfo.Name()))
			if ext == "" {
				ext = "(no extension)"
			}
			extCounts[ext]++
			extSizes[ext] += size
		}

		return nil
	})

	// Print summary
	fmt.Fprintf(cmd.OutOrStdout(), "PATH: %s\n", path)
	fmt.Fprintf(cmd.OutOrStdout(), "FILES: %d\n", totalFiles)
	fmt.Fprintf(cmd.OutOrStdout(), "DIRECTORIES: %d\n", totalDirs)
	fmt.Fprintf(cmd.OutOrStdout(), "TOTAL_SIZE: %s\n", utils.FormatSize(totalSize))
	fmt.Fprintln(cmd.OutOrStdout())

	// Print extension breakdown (sorted by count)
	if len(extCounts) > 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "BY_EXTENSION:")

		type extInfo struct {
			ext   string
			count int
			size  int64
		}
		var exts []extInfo
		for ext, count := range extCounts {
			exts = append(exts, extInfo{ext, count, extSizes[ext]})
		}
		sort.Slice(exts, func(i, j int) bool {
			return exts[i].count > exts[j].count
		})

		for _, e := range exts {
			fmt.Fprintf(cmd.OutOrStdout(), "  %s: %d files (%s)\n", e.ext, e.count, utils.FormatSize(e.size))
		}
	}

	return nil
}

func init() {
	RootCmd.AddCommand(newStatsCmd())
}
