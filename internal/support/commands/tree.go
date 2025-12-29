package commands

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/samestrin/llm-tools/internal/support/gitignore"
	"github.com/samestrin/llm-tools/internal/support/utils"
	"github.com/samestrin/llm-tools/pkg/output"
	"github.com/spf13/cobra"
)

var (
	treeDepth       int
	treeSizes       bool
	treeNoGitignore bool
	treePath        string
	treeJSON        bool
	treeMinimal     bool
)

// TreeEntry represents a single entry in the tree
type TreeEntry struct {
	Name  string       `json:"name,omitempty"`
	Path  string       `json:"path,omitempty"`
	Type  string       `json:"type,omitempty"`
	Size  int64        `json:"size,omitempty"`
	Items []*TreeEntry `json:"items,omitempty"`
}

// TreeResult represents the complete tree output
type TreeResult struct {
	Root    string       `json:"root,omitempty"`
	Depth   int          `json:"depth,omitempty"`
	Entries []*TreeEntry `json:"entries,omitempty"`
}

// newTreeCmd creates the tree command
func newTreeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tree",
		Short: "Display directory tree structure",
		Long: `Display directory tree structure with optional file sizes.
Respects .gitignore patterns by default.`,
		Args: cobra.NoArgs,
		RunE: runTree,
	}
	cmd.Flags().StringVar(&treePath, "path", ".", "Directory path to display")
	cmd.Flags().IntVar(&treeDepth, "depth", 999, "Maximum depth to display")
	cmd.Flags().BoolVar(&treeSizes, "sizes", false, "Show file sizes")
	cmd.Flags().BoolVar(&treeNoGitignore, "no-gitignore", false, "Disable .gitignore filtering")
	cmd.Flags().BoolVar(&treeJSON, "json", false, "Output as JSON")
	cmd.Flags().BoolVar(&treeMinimal, "min", false, "Output in minimal/token-optimized format")
	return cmd
}

func runTree(cmd *cobra.Command, args []string) error {
	path, err := filepath.Abs(treePath)
	if err != nil {
		return fmt.Errorf("invalid path: %s", treePath)
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

	// Build tree structure
	entries := buildTreeEntries(path, 0, treeDepth, ignorer)
	result := TreeResult{
		Root:    path,
		Depth:   treeDepth,
		Entries: entries,
	}

	formatter := output.New(treeJSON, treeMinimal, cmd.OutOrStdout())
	return formatter.Print(result, func(w io.Writer, data interface{}) {
		r := data.(TreeResult)
		printTreeText(w, r.Root, r.Entries, "", r.Root)
	})
}

// buildTreeEntries builds a structured tree of entries
func buildTreeEntries(currentPath string, depth, maxDepth int, ignorer *gitignore.Parser) []*TreeEntry {
	if depth > maxDepth {
		return nil
	}

	entries, err := os.ReadDir(currentPath)
	if err != nil {
		return nil
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

	var result []*TreeEntry
	for _, entry := range items {
		fullPath := filepath.Join(currentPath, entry.Name())
		treeEntry := &TreeEntry{
			Name: entry.Name(),
			Path: fullPath,
		}

		if entry.IsDir() {
			treeEntry.Type = "dir"
			treeEntry.Items = buildTreeEntries(fullPath, depth+1, maxDepth, ignorer)
		} else {
			treeEntry.Type = "file"
			if treeSizes {
				if info, err := entry.Info(); err == nil {
					treeEntry.Size = info.Size()
				}
			}
		}

		result = append(result, treeEntry)
	}

	return result
}

// printTreeText prints the tree in traditional text format
func printTreeText(w io.Writer, rootPath string, entries []*TreeEntry, prefix string, basePath string) {
	if prefix == "" {
		// Print root
		fmt.Fprintf(w, "%s/\n", rootPath)
	}

	for i, entry := range entries {
		isLast := i == len(entries)-1
		connector := "├── "
		extension := "│   "
		if isLast {
			connector = "└── "
			extension = "    "
		}

		name := entry.Name
		if entry.Type == "dir" {
			name += "/"
		}

		// Print entry
		if treeSizes && entry.Type == "file" && entry.Size > 0 {
			fmt.Fprintf(w, "%s%s%s [%s]\n", prefix, connector, name, utils.FormatSize(entry.Size))
		} else {
			fmt.Fprintf(w, "%s%s%s\n", prefix, connector, name)
		}

		// Recurse into directories
		if entry.Type == "dir" && len(entry.Items) > 0 {
			printTreeText(w, rootPath, entry.Items, prefix+extension, basePath)
		}
	}
}

func init() {
	RootCmd.AddCommand(newTreeCmd())
}
