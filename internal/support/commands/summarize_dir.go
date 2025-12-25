package commands

import (
	"bufio"
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
	summarizeDirFormat      string
	summarizeDirRecursive   bool
	summarizeDirGlob        string
	summarizeDirMaxTokens   int
	summarizeDirMaxLines    int
	summarizeDirNoGitignore bool
)

// newSummarizeDirCmd creates the summarize-dir command
func newSummarizeDirCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "summarize-dir [path]",
		Short: "Summarize directory contents",
		Long: `Generate a summary of directory contents.
Useful for providing context to LLMs about a codebase.

Formats:
  tree    - Directory tree with file types
  outline - Brief outline of each file
  full    - Full content (truncated)`,
		Args: cobra.ExactArgs(1),
		RunE: runSummarizeDir,
	}
	cmd.Flags().StringVar(&summarizeDirFormat, "format", "tree", "Output format: tree, outline, full")
	cmd.Flags().BoolVarP(&summarizeDirRecursive, "recursive", "r", true, "Recursive scan")
	cmd.Flags().StringVar(&summarizeDirGlob, "glob", "", "File glob pattern")
	cmd.Flags().IntVar(&summarizeDirMaxTokens, "max-tokens", 4000, "Approximate max tokens")
	cmd.Flags().IntVar(&summarizeDirMaxLines, "lines", 10, "Max lines per file in outline mode")
	cmd.Flags().BoolVar(&summarizeDirNoGitignore, "no-gitignore", false, "Disable .gitignore filtering")
	return cmd
}

func runSummarizeDir(cmd *cobra.Command, args []string) error {
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
	if !summarizeDirNoGitignore {
		ignorer, _ = gitignore.NewParser(path)
	}

	switch summarizeDirFormat {
	case "tree":
		return summarizeTree(cmd, path, ignorer)
	case "outline":
		return summarizeOutline(cmd, path, ignorer)
	case "full":
		return summarizeFull(cmd, path, ignorer)
	default:
		return fmt.Errorf("unknown format: %s (supported: tree, outline, full)", summarizeDirFormat)
	}
}

func summarizeTree(cmd *cobra.Command, path string, ignorer *gitignore.Parser) error {
	fmt.Fprintf(cmd.OutOrStdout(), "DIRECTORY: %s\n\n", path)

	type fileInfo struct {
		path string
		size int64
		ext  string
	}

	var files []fileInfo
	var dirs []string

	walkFn := func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		// Skip hidden files
		if !summarizeDirNoGitignore && strings.HasPrefix(info.Name(), ".") {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Check gitignore
		if ignorer != nil && ignorer.IsIgnored(filePath) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		relPath, _ := filepath.Rel(path, filePath)
		if relPath == "." {
			return nil
		}

		if info.IsDir() {
			dirs = append(dirs, relPath)
			if !summarizeDirRecursive {
				return filepath.SkipDir
			}
		} else {
			files = append(files, fileInfo{
				path: relPath,
				size: info.Size(),
				ext:  filepath.Ext(info.Name()),
			})
		}
		return nil
	}

	filepath.Walk(path, walkFn)

	// Print directories
	fmt.Fprintln(cmd.OutOrStdout(), "DIRECTORIES:")
	sort.Strings(dirs)
	for _, d := range dirs {
		fmt.Fprintf(cmd.OutOrStdout(), "  %s/\n", d)
	}

	// Print files by extension
	fmt.Fprintln(cmd.OutOrStdout(), "\nFILES:")
	sort.Slice(files, func(i, j int) bool {
		return files[i].path < files[j].path
	})

	for _, f := range files {
		fmt.Fprintf(cmd.OutOrStdout(), "  %s (%s)\n", f.path, utils.FormatSize(f.size))
	}

	// Summary
	fmt.Fprintf(cmd.OutOrStdout(), "\nSUMMARY: %d directories, %d files\n", len(dirs), len(files))

	return nil
}

func summarizeOutline(cmd *cobra.Command, path string, ignorer *gitignore.Parser) error {
	fmt.Fprintf(cmd.OutOrStdout(), "DIRECTORY: %s\n", path)
	fmt.Fprintf(cmd.OutOrStdout(), "FORMAT: outline (first %d lines per file)\n\n", summarizeDirMaxLines)

	var totalChars int
	maxChars := summarizeDirMaxTokens * 4 // Rough token-to-char ratio

	walkFn := func(filePath string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

		if totalChars >= maxChars {
			return filepath.SkipAll
		}

		// Skip hidden files
		if !summarizeDirNoGitignore && strings.HasPrefix(info.Name(), ".") {
			return nil
		}

		// Check gitignore
		if ignorer != nil && ignorer.IsIgnored(filePath) {
			return nil
		}

		relPath, _ := filepath.Rel(path, filePath)

		file, err := os.Open(filePath)
		if err != nil {
			return nil
		}
		defer file.Close()

		// Read first N lines
		scanner := bufio.NewScanner(file)
		var lines []string
		for scanner.Scan() && len(lines) < summarizeDirMaxLines {
			lines = append(lines, scanner.Text())
		}

		if len(lines) > 0 {
			fmt.Fprintf(cmd.OutOrStdout(), "--- %s ---\n", relPath)
			content := strings.Join(lines, "\n")
			fmt.Fprintln(cmd.OutOrStdout(), content)
			fmt.Fprintln(cmd.OutOrStdout())
			totalChars += len(content)
		}

		return nil
	}

	filepath.Walk(path, walkFn)
	return nil
}

func summarizeFull(cmd *cobra.Command, path string, ignorer *gitignore.Parser) error {
	fmt.Fprintf(cmd.OutOrStdout(), "DIRECTORY: %s\n", path)
	fmt.Fprintln(cmd.OutOrStdout(), "FORMAT: full (truncated to max tokens)")

	var totalChars int
	maxChars := summarizeDirMaxTokens * 4

	walkFn := func(filePath string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

		if totalChars >= maxChars {
			return filepath.SkipAll
		}

		// Skip hidden files
		if !summarizeDirNoGitignore && strings.HasPrefix(info.Name(), ".") {
			return nil
		}

		// Check gitignore
		if ignorer != nil && ignorer.IsIgnored(filePath) {
			return nil
		}

		relPath, _ := filepath.Rel(path, filePath)

		content, err := os.ReadFile(filePath)
		if err != nil {
			return nil
		}

		// Skip binary files
		if !isTextFile(content) {
			return nil
		}

		remaining := maxChars - totalChars
		text := string(content)
		if len(text) > remaining {
			text = text[:remaining] + "\n... (truncated)"
		}

		fmt.Fprintf(cmd.OutOrStdout(), "=== %s ===\n", relPath)
		fmt.Fprintln(cmd.OutOrStdout(), text)
		fmt.Fprintln(cmd.OutOrStdout())

		totalChars += len(text)
		return nil
	}

	filepath.Walk(path, walkFn)
	return nil
}

func init() {
	RootCmd.AddCommand(newSummarizeDirCmd())
}
