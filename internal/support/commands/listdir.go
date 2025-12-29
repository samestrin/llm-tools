package commands

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/samestrin/llm-tools/internal/support/gitignore"
	"github.com/samestrin/llm-tools/internal/support/utils"
	"github.com/samestrin/llm-tools/pkg/output"
	"github.com/spf13/cobra"
)

var (
	listdirDates       bool
	listdirSizes       bool
	listdirNoGitignore bool
	listdirPath        string
	listdirJSON        bool
	listdirMinimal     bool
)

// ListdirEntry represents a single directory entry
type ListdirEntry struct {
	Name     string `json:"name,omitempty"`
	Type     string `json:"type,omitempty"`
	Size     int64  `json:"size,omitempty"`
	Modified string `json:"modified,omitempty"`
}

// ListdirResult represents the directory listing result
type ListdirResult struct {
	Path    string         `json:"path,omitempty"`
	Entries []ListdirEntry `json:"entries,omitempty"`
	Empty   bool           `json:"empty,omitempty"`
}

// newListdirCmd creates the listdir command
func newListdirCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "listdir",
		Short: "List directory contents with optional metadata",
		Long: `List directory contents with optional file sizes and dates.
Respects .gitignore patterns by default.

Output format:
  [type] name [size] [date]

Types: file, dir`,
		Args: cobra.NoArgs,
		RunE: runListdir,
	}
	cmd.Flags().StringVar(&listdirPath, "path", ".", "Directory path to list")
	cmd.Flags().BoolVar(&listdirDates, "dates", false, "Show modification dates")
	cmd.Flags().BoolVar(&listdirSizes, "sizes", false, "Show file sizes")
	cmd.Flags().BoolVar(&listdirNoGitignore, "no-gitignore", false, "Disable .gitignore filtering")
	cmd.Flags().BoolVar(&listdirJSON, "json", false, "Output as JSON")
	cmd.Flags().BoolVar(&listdirMinimal, "min", false, "Output in minimal/token-optimized format")
	return cmd
}

func runListdir(cmd *cobra.Command, args []string) error {
	path, err := filepath.Abs(listdirPath)
	if err != nil {
		return fmt.Errorf("invalid path: %s", listdirPath)
	}

	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("path does not exist: %s", path)
	}

	if !info.IsDir() {
		return fmt.Errorf("path is not a directory: %s", path)
	}

	// Setup gitignore if needed
	var ignorer *gitignore.Parser
	if !listdirNoGitignore {
		ignorer, _ = gitignore.NewParser(path)
	}

	// Read directory entries
	entries, err := os.ReadDir(path)
	if err != nil {
		return fmt.Errorf("permission denied: %s", path)
	}

	// Collect and sort entries
	type entry struct {
		name     string
		isDir    bool
		size     int64
		modified time.Time
	}

	var results []entry
	for _, e := range entries {
		name := e.Name()

		// Skip hidden files unless --no-gitignore
		if !listdirNoGitignore && strings.HasPrefix(name, ".") {
			continue
		}

		// Skip if gitignored
		fullPath := filepath.Join(path, name)
		if ignorer != nil && ignorer.IsIgnored(fullPath) {
			continue
		}

		info, err := e.Info()
		if err != nil {
			continue
		}

		results = append(results, entry{
			name:     name,
			isDir:    e.IsDir(),
			size:     info.Size(),
			modified: info.ModTime(),
		})
	}

	// Sort by name
	sort.Slice(results, func(i, j int) bool {
		return results[i].name < results[j].name
	})

	// Build structured result
	listdirResult := ListdirResult{
		Path:  path,
		Empty: len(results) == 0,
	}

	for _, e := range results {
		entry := ListdirEntry{
			Name: e.name,
		}
		if e.isDir {
			entry.Type = "dir"
		} else {
			entry.Type = "file"
			if listdirSizes {
				entry.Size = e.size
			}
		}
		if listdirDates {
			entry.Modified = e.modified.Format("2006-01-02 15:04:05")
		}
		listdirResult.Entries = append(listdirResult.Entries, entry)
	}

	formatter := output.New(listdirJSON, listdirMinimal, cmd.OutOrStdout())
	return formatter.Print(listdirResult, printListdirText)
}

func printListdirText(w io.Writer, data interface{}) {
	result := data.(ListdirResult)

	if result.Empty {
		fmt.Fprintln(w, "EMPTY_DIRECTORY")
		return
	}

	for _, e := range result.Entries {
		var parts []string

		// Type indicator
		if e.Type == "dir" {
			parts = append(parts, "[dir]")
		} else {
			parts = append(parts, "[file]")
		}

		// Name
		parts = append(parts, e.Name)

		// Size (for files only)
		if listdirSizes && e.Type == "file" && e.Size > 0 {
			parts = append(parts, utils.FormatSize(e.Size))
		}

		// Date
		if e.Modified != "" {
			parts = append(parts, e.Modified)
		}

		fmt.Fprintln(w, strings.Join(parts, " "))
	}
}

func init() {
	RootCmd.AddCommand(newListdirCmd())
}
