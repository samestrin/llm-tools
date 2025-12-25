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
	catfilesMaxSize     int
	catfilesNoGitignore bool
)

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

	// Process files
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

		// Output with header
		fmt.Fprintln(cmd.OutOrStdout(), strings.Repeat("=", 60))
		fmt.Fprintf(cmd.OutOrStdout(), "FILE: %s\n", filePath)
		fmt.Fprintf(cmd.OutOrStdout(), "SIZE: %s\n", utils.FormatSize(size))
		fmt.Fprintln(cmd.OutOrStdout(), strings.Repeat("=", 60))
		fmt.Fprintln(cmd.OutOrStdout(), string(content))
		fmt.Fprintln(cmd.OutOrStdout())

		totalSize += size
	}

	return nil
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
