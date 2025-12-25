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
	treeDepth       int
	treeSizes       bool
	treeNoGitignore bool
)

// newTreeCmd creates the tree command
func newTreeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tree [path]",
		Short: "Display directory tree structure",
		Long: `Display directory tree structure with optional file sizes.
Respects .gitignore patterns by default.`,
		Args: cobra.ExactArgs(1),
		RunE: runTree,
	}
	cmd.Flags().IntVar(&treeDepth, "depth", 999, "Maximum depth to display")
	cmd.Flags().BoolVar(&treeSizes, "sizes", false, "Show file sizes")
	cmd.Flags().BoolVar(&treeNoGitignore, "no-gitignore", false, "Disable .gitignore filtering")
	return cmd
}

func runTree(cmd *cobra.Command, args []string) error {
	path, err := filepath.Abs(args[0])
	if err != nil {
		return fmt.Errorf("invalid path: %s", args[0])
	}

	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("path does not exist: %s", path)
	}

	if !info.IsDir() {
		return fmt.Errorf("path is not a directory: %s", path)
	}

	var ignorer *gitignore.Parser
	if !treeNoGitignore {
		ignorer, _ = gitignore.NewParser(path)
	}

	// Print root
	fmt.Fprintf(cmd.OutOrStdout(), "%s/\n", path)

	// Build tree
	buildTree(cmd, path, "", 0, treeDepth, ignorer)
	return nil
}

func buildTree(cmd *cobra.Command, currentPath, prefix string, depth, maxDepth int, ignorer *gitignore.Parser) {
	if depth > maxDepth {
		return
	}

	entries, err := os.ReadDir(currentPath)
	if err != nil {
		return
	}

	// Filter and sort entries
	var items []os.DirEntry
	for _, entry := range entries {
		name := entry.Name()

		// Skip hidden files unless --no-gitignore
		if !treeNoGitignore && strings.HasPrefix(name, ".") {
			continue
		}

		// Check gitignore
		fullPath := filepath.Join(currentPath, name)
		if ignorer != nil && ignorer.IsIgnored(fullPath) {
			continue
		}

		items = append(items, entry)
	}

	// Sort: directories first, then by name
	sort.Slice(items, func(i, j int) bool {
		iDir := items[i].IsDir()
		jDir := items[j].IsDir()
		if iDir != jDir {
			return iDir
		}
		return items[i].Name() < items[j].Name()
	})

	for i, entry := range items {
		isLast := i == len(items)-1
		connector := "├── "
		extension := "│   "
		if isLast {
			connector = "└── "
			extension = "    "
		}

		name := entry.Name()
		if entry.IsDir() {
			name += "/"
		}

		// Print entry
		if treeSizes && !entry.IsDir() {
			info, err := entry.Info()
			if err == nil {
				fmt.Fprintf(cmd.OutOrStdout(), "%s%s%s [%s]\n", prefix, connector, name, utils.FormatSize(info.Size()))
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "%s%s%s\n", prefix, connector, name)
			}
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "%s%s%s\n", prefix, connector, name)
		}

		// Recurse into directories
		if entry.IsDir() {
			buildTree(cmd, filepath.Join(currentPath, entry.Name()), prefix+extension, depth+1, maxDepth, ignorer)
		}
	}
}

func init() {
	RootCmd.AddCommand(newTreeCmd())
}
