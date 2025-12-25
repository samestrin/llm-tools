package commands

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/samestrin/llm-tools/internal/support/gitignore"
	"github.com/spf13/cobra"
)

var (
	grepIgnoreCase  bool
	grepLineNumbers bool
	grepFilesOnly   bool
	grepNoGitignore bool
)

// newGrepCmd creates the grep command
func newGrepCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "grep [pattern] [paths...]",
		Short: "Search for pattern in files",
		Long: `Search for a pattern in files using regular expressions.
Respects .gitignore patterns by default.`,
		Args: cobra.MinimumNArgs(2),
		RunE: runGrep,
	}
	cmd.Flags().BoolVarP(&grepIgnoreCase, "ignore-case", "i", false, "Case insensitive search")
	cmd.Flags().BoolVarP(&grepLineNumbers, "line-number", "n", false, "Show line numbers")
	cmd.Flags().BoolVarP(&grepFilesOnly, "files-with-matches", "l", false, "Only show file names")
	cmd.Flags().BoolVar(&grepNoGitignore, "no-gitignore", false, "Disable .gitignore filtering")
	return cmd
}

func runGrep(cmd *cobra.Command, args []string) error {
	pattern := args[0]
	paths := args[1:]

	// Compile regex
	flags := ""
	if grepIgnoreCase {
		flags = "(?i)"
	}
	re, err := regexp.Compile(flags + pattern)
	if err != nil {
		return fmt.Errorf("invalid pattern: %v", err)
	}

	// Collect files
	var files []string
	for _, pathStr := range paths {
		absPath, err := filepath.Abs(pathStr)
		if err != nil {
			continue
		}

		info, err := os.Stat(absPath)
		if err != nil {
			continue
		}

		if info.IsDir() {
			var ignorer *gitignore.Parser
			if !grepNoGitignore {
				ignorer, _ = gitignore.NewParser(absPath)
			}

			filepath.Walk(absPath, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return nil
				}
				if info.IsDir() {
					return nil
				}
				if !grepNoGitignore && strings.HasPrefix(info.Name(), ".") {
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

	// Search files
	matchedFiles := make(map[string]bool)

	for _, filePath := range files {
		file, err := os.Open(filePath)
		if err != nil {
			continue
		}

		scanner := bufio.NewScanner(file)
		lineNum := 0
		fileHasMatch := false

		for scanner.Scan() {
			lineNum++
			line := scanner.Text()

			if re.MatchString(line) {
				fileHasMatch = true

				if grepFilesOnly {
					if !matchedFiles[filePath] {
						fmt.Fprintln(cmd.OutOrStdout(), filePath)
						matchedFiles[filePath] = true
					}
					break
				}

				// Print match
				if grepLineNumbers {
					fmt.Fprintf(cmd.OutOrStdout(), "%s:%d:%s\n", filePath, lineNum, line)
				} else {
					fmt.Fprintf(cmd.OutOrStdout(), "%s:%s\n", filePath, line)
				}
			}
		}

		file.Close()
		_ = fileHasMatch // for future use
	}

	return nil
}

func init() {
	RootCmd.AddCommand(newGrepCmd())
}
