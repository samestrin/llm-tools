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
	catfilesMaxSize     int
	catfilesNoGitignore bool
	catfilesJSON        bool
	catfilesMinimal     bool
)

// CatFileEntry represents a single file in the catfiles result
type CatFileEntry struct {
	Path    string `json:"path,omitempty"`
	P       string `json:"p,omitempty"`
	Size    int64  `json:"size,omitempty"`
	S       *int64 `json:"s,omitempty"`
	Content string `json:"content,omitempty"`
	C       string `json:"c,omitempty"`
}

// CatfilesResult represents the catfiles result
type CatfilesResult struct {
	Files     []CatFileEntry `json:"files,omitempty"`
	F         []CatFileEntry `json:"f,omitempty"`
	FileCount int            `json:"file_count,omitempty"`
	FC        *int           `json:"fc,omitempty"`
	TotalSize int64          `json:"total_size,omitempty"`
	TS        *int64         `json:"ts,omitempty"`
}

// newCatfilesCmd creates the catfiles command
func newCatfilesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "catfiles [paths...]",
		Short: "Concatenate multiple files with headers",
		Long: `Concatenate multiple files or directory contents with headers.
Each file is prefixed with a header showing the file path and size.

Respects .gitignore patterns by default.`,
		Args: cobra.MinimumNArgs(1),
		RunE: runCatfiles,
	}
	cmd.Flags().IntVar(&catfilesMaxSize, "max-size", 10, "Maximum total size in MB")
	cmd.Flags().BoolVar(&catfilesNoGitignore, "no-gitignore", false, "Disable .gitignore filtering")
	cmd.Flags().BoolVar(&catfilesJSON, "json", false, "Output as JSON")
	cmd.Flags().BoolVar(&catfilesMinimal, "min", false, "Output in minimal/token-optimized format")
	return cmd
}

func runCatfiles(cmd *cobra.Command, args []string) error {
	maxSizeBytes := int64(catfilesMaxSize) * 1024 * 1024
	var totalSize int64

	// Collect files
	var files []string
	for _, pathStr := range args {
		absPath, err := filepath.Abs(pathStr)
		if err != nil {
			return fmt.Errorf("invalid path: %s", pathStr)
		}

		info, err := os.Stat(absPath)
		if err != nil {
			return fmt.Errorf("path not found: %s", pathStr)
		}

		if info.IsDir() {
			var ignorer *gitignore.Parser
			if !catfilesNoGitignore {
				ignorer, _ = gitignore.NewParser(absPath)
			}

			filepath.Walk(absPath, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return nil
				}
				if info.IsDir() {
					return nil
				}
				if !catfilesNoGitignore && strings.HasPrefix(info.Name(), ".") {
					return nil
				}
				if ignorer != nil && ignorer.IsIgnored(path) {
					return nil
				}
				files = append(files, path)
				return nil
			})
		} else {
			files = append(files, absPath)
		}
	}

	if len(files) == 0 {
		return fmt.Errorf("no files found")
	}

	// Deduplicate and sort
	seen := make(map[string]bool)
	uniqueFiles := []string{}
	for _, f := range files {
		if !seen[f] {
			seen[f] = true
			uniqueFiles = append(uniqueFiles, f)
		}
	}
	sort.Strings(uniqueFiles)

	// Process files and collect entries
	var entries []CatFileEntry
	for _, filePath := range uniqueFiles {
		info, err := os.Stat(filePath)
		if err != nil {
			continue
		}

		size := info.Size()
		if totalSize+size > maxSizeBytes {
			fmt.Fprintf(cmd.ErrOrStderr(), "WARNING: Size limit reached (%dMB), stopping\n", catfilesMaxSize)
			break
		}

		content, err := os.ReadFile(filePath)
		if err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "WARNING: Cannot read file: %s\n", filePath)
			continue
		}

		// Check if binary
		if !isTextFile(content) {
			fmt.Fprintf(cmd.ErrOrStderr(), "WARNING: Skipping binary file: %s\n", filePath)
			continue
		}

		entries = append(entries, CatFileEntry{
			Path:    filePath,
			Size:    size,
			Content: string(content),
		})

		totalSize += size
	}

	// Build result
	fileCount := len(entries)
	var result CatfilesResult
	if catfilesMinimal {
		minEntries := make([]CatFileEntry, len(entries))
		for i, e := range entries {
			size := e.Size
			minEntries[i] = CatFileEntry{
				P: e.Path,
				S: &size,
				C: e.Content,
			}
		}
		result = CatfilesResult{
			F:  minEntries,
			FC: &fileCount,
			TS: &totalSize,
		}
	} else {
		result = CatfilesResult{
			Files:     entries,
			FileCount: fileCount,
			TotalSize: totalSize,
		}
	}

	formatter := output.New(catfilesJSON, catfilesMinimal, cmd.OutOrStdout())
	return formatter.Print(result, func(w io.Writer, data interface{}) {
		for _, e := range entries {
			fmt.Fprintln(w, strings.Repeat("=", 60))
			fmt.Fprintf(w, "FILE: %s\n", e.Path)
			fmt.Fprintf(w, "SIZE: %s\n", utils.FormatSize(e.Size))
			fmt.Fprintln(w, strings.Repeat("=", 60))
			fmt.Fprintln(w, e.Content)
			fmt.Fprintln(w)
		}
	})
}

func isTextFile(content []byte) bool {
	// Check for null bytes (common in binary files)
	for i := 0; i < len(content) && i < 8000; i++ {
		if content[i] == 0 {
			return false
		}
	}
	return true
}

func init() {
	RootCmd.AddCommand(newCatfilesCmd())
}
