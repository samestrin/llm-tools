package commands

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/samestrin/llm-tools/internal/support/gitignore"
	"github.com/samestrin/llm-tools/internal/support/utils"
	"github.com/samestrin/llm-tools/pkg/output"
	"github.com/spf13/cobra"
)

// defaultExcludes contains directory names that are excluded by default
// These are common build/dependency directories that bloat tree output
var defaultExcludes = []string{
	"node_modules",
	".git",
	"vendor",
	"__pycache__",
	".venv",
	"venv",
	".next",
	".nuxt",
	"target", // Rust/Java
}

var (
	treeDepth             int
	treeSizes             bool
	treeNoGitignore       bool
	treeNoDefaultExcludes bool
	treePath              string
	treeJSON              bool
	treeMinimal           bool
	treeMaxEntries        int
	treeExcludePatterns   []string
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
	Root      string       `json:"root,omitempty"`
	Depth     int          `json:"depth,omitempty"`
	Entries   []*TreeEntry `json:"entries,omitempty"`
	Total     int          `json:"total,omitempty"`
	Truncated bool         `json:"truncated,omitempty"`
	Message   string       `json:"message,omitempty"`
}

// newTreeCmd creates the tree command
func newTreeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tree",
		Short: "Display directory tree structure",
		Long: `Display directory tree structure with optional file sizes.

Filtering (all enabled by default):
  - Respects .gitignore patterns
  - Excludes common build/dependency directories:
    node_modules, .git, vendor, __pycache__, .venv, venv, .next, .nuxt, target

Use --no-gitignore and --no-default-excludes to disable filtering.
Use --exclude to add custom patterns.

Examples:
  llm-support tree --path ./src
  llm-support tree --max-entries 1000
  llm-support tree --exclude "\.test\." --exclude "fixtures"
  llm-support tree --no-default-excludes  # include node_modules etc`,
		Args: cobra.NoArgs,
		RunE: runTree,
	}
	cmd.Flags().StringVar(&treePath, "path", ".", "Directory path to display")
	cmd.Flags().IntVar(&treeDepth, "depth", 999, "Maximum depth to display")
	cmd.Flags().IntVar(&treeMaxEntries, "max-entries", 500, "Maximum entries to display (0 = unlimited)")
	cmd.Flags().BoolVar(&treeSizes, "sizes", false, "Show file sizes")
	cmd.Flags().BoolVar(&treeNoGitignore, "no-gitignore", false, "Disable .gitignore filtering")
	cmd.Flags().BoolVar(&treeNoDefaultExcludes, "no-default-excludes", false, "Disable default directory excludes")
	cmd.Flags().StringArrayVar(&treeExcludePatterns, "exclude", nil, "Regex patterns to exclude (can be repeated)")
	cmd.Flags().BoolVar(&treeJSON, "json", false, "Output as JSON")
	cmd.Flags().BoolVar(&treeMinimal, "min", false, "Output in minimal/token-optimized format")
	return cmd
}

// treeBuilder holds state for building the tree
type treeBuilder struct {
	ignorer         *gitignore.Parser
	excludePatterns []*regexp.Regexp
	maxEntries      int
	entryCount      int
	truncated       bool
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

	// Initialize builder
	builder := &treeBuilder{
		maxEntries: treeMaxEntries,
	}

	// Set up gitignore parser
	if !treeNoGitignore {
		builder.ignorer, _ = gitignore.NewParser(path)
	}

	// Compile exclude patterns
	for _, pattern := range treeExcludePatterns {
		re, err := regexp.Compile(pattern)
		if err != nil {
			return fmt.Errorf("invalid exclude pattern %q: %w", pattern, err)
		}
		builder.excludePatterns = append(builder.excludePatterns, re)
	}

	// Build tree structure
	entries := builder.buildTreeEntries(path, 0, treeDepth)

	result := TreeResult{
		Root:      path,
		Depth:     treeDepth,
		Entries:   entries,
		Total:     builder.entryCount,
		Truncated: builder.truncated,
	}

	if builder.truncated {
		result.Message = fmt.Sprintf("Output truncated at %d entries (use --max-entries to adjust)", treeMaxEntries)
	}

	formatter := output.New(treeJSON, treeMinimal, cmd.OutOrStdout())
	return formatter.Print(result, func(w io.Writer, data interface{}) {
		r := data.(TreeResult)
		printTreeText(w, r.Root, r.Entries, "", r.Root)
		if r.Truncated {
			fmt.Fprintf(w, "\n... truncated at %d entries (use --max-entries to adjust)\n", r.Total)
		}
	})
}

// buildTreeEntries builds a structured tree of entries
func (b *treeBuilder) buildTreeEntries(currentPath string, depth, maxDepth int) []*TreeEntry {
	if depth > maxDepth {
		return nil
	}

	// Check if we've hit the entry limit
	if b.maxEntries > 0 && b.entryCount >= b.maxEntries {
		b.truncated = true
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

		// Check default excludes (directory names only)
		if !treeNoDefaultExcludes && entry.IsDir() {
			if b.isDefaultExcluded(name) {
				continue
			}
		}

		// Check gitignore
		fullPath := filepath.Join(currentPath, name)
		if b.ignorer != nil && b.ignorer.IsIgnored(fullPath) {
			continue
		}

		// Check custom exclude patterns
		if b.matchesExcludePattern(name) {
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
		// Check entry limit before adding each entry
		if b.maxEntries > 0 && b.entryCount >= b.maxEntries {
			b.truncated = true
			break
		}

		fullPath := filepath.Join(currentPath, entry.Name())
		treeEntry := &TreeEntry{
			Name: entry.Name(),
			Path: fullPath,
		}
		b.entryCount++

		if entry.IsDir() {
			treeEntry.Type = "dir"
			treeEntry.Items = b.buildTreeEntries(fullPath, depth+1, maxDepth)
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

// isDefaultExcluded checks if a directory name is in the default excludes list
func (b *treeBuilder) isDefaultExcluded(name string) bool {
	for _, excluded := range defaultExcludes {
		if name == excluded {
			return true
		}
	}
	return false
}

// matchesExcludePattern checks if a name matches any custom exclude pattern
func (b *treeBuilder) matchesExcludePattern(name string) bool {
	for _, re := range b.excludePatterns {
		if re.MatchString(name) {
			return true
		}
	}
	return false
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
