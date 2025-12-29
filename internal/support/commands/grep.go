package commands

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/samestrin/llm-tools/internal/support/gitignore"
	"github.com/samestrin/llm-tools/pkg/output"
	"github.com/spf13/cobra"
)

var (
	grepIgnoreCase  bool
	grepLineNumbers bool
	grepFilesOnly   bool
	grepNoGitignore bool
	grepJSON        bool
	grepMinimal     bool
)

// GrepMatch represents a single grep match
type GrepMatch struct {
	File string `json:"file,omitempty"`
	Line int    `json:"line,omitempty"`
	Text string `json:"text,omitempty"`
}

// GrepResult represents the complete grep result
type GrepResult struct {
	Pattern string      `json:"pattern,omitempty"`
	Matches []GrepMatch `json:"matches,omitempty"`
	Files   []string    `json:"files,omitempty"`
}

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
	cmd.Flags().BoolVar(&grepJSON, "json", false, "Output as JSON")
	cmd.Flags().BoolVar(&grepMinimal, "min", false, "Output in minimal/token-optimized format")
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
	result := GrepResult{
		Pattern: pattern,
	}
	matchedFiles := make(map[string]bool)

	for _, filePath := range files {
		file, err := os.Open(filePath)
		if err != nil {
			continue
		}

		scanner := bufio.NewScanner(file)
		lineNum := 0

		for scanner.Scan() {
			lineNum++
			line := scanner.Text()

			if re.MatchString(line) {
				if grepFilesOnly {
					if !matchedFiles[filePath] {
						result.Files = append(result.Files, filePath)
						matchedFiles[filePath] = true
					}
					break
				}

				match := GrepMatch{
					File: filePath,
					Text: line,
				}
				if grepLineNumbers {
					match.Line = lineNum
				}
				result.Matches = append(result.Matches, match)
			}
		}

		file.Close()
	}

	formatter := output.New(grepJSON, grepMinimal, cmd.OutOrStdout())
	return formatter.Print(result, printGrepText)
}

func printGrepText(w io.Writer, data interface{}) {
	r := data.(GrepResult)

	if grepFilesOnly {
		for _, f := range r.Files {
			fmt.Fprintln(w, f)
		}
		return
	}

	for _, m := range r.Matches {
		if grepLineNumbers && m.Line > 0 {
			fmt.Fprintf(w, "%s:%d:%s\n", m.File, m.Line, m.Text)
		} else {
			fmt.Fprintf(w, "%s:%s\n", m.File, m.Text)
		}
	}
}

func init() {
	RootCmd.AddCommand(newGrepCmd())
}
