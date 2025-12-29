package commands

import (
	"fmt"
	"strings"

	"github.com/samestrin/llm-tools/internal/filesystem/core"
	"github.com/spf13/cobra"
)

func addSearchCommands(rootCmd *cobra.Command) {
	rootCmd.AddCommand(searchFilesCmd())
	rootCmd.AddCommand(searchCodeCmd())
}

func searchFilesCmd() *cobra.Command {
	var path, pattern string
	var recursive, showHidden bool
	var maxResults int

	cmd := &cobra.Command{
		Use:   "search-files",
		Short: "Search for files by name",
		Long:  "Searches for files matching a pattern within a directory",
		Run: func(cmd *cobra.Command, args []string) {
			result, err := core.SearchFiles(core.SearchFilesOptions{
				Path:        path,
				Pattern:     pattern,
				Recursive:   recursive,
				ShowHidden:  showHidden,
				MaxResults:  maxResults,
				AllowedDirs: GetAllowedDirs(),
			})
			if err != nil {
				OutputError(err)
			}
			OutputResult(result, func() string {
				var sb strings.Builder
				sb.WriteString(fmt.Sprintf("Found %d files matching '%s' in %s\n\n",
					result.Total, result.Pattern, result.Path))
				for _, m := range result.Matches {
					typeIndicator := ""
					if m.IsDir {
						typeIndicator = " (dir)"
					}
					sb.WriteString(fmt.Sprintf("%s%s  %d bytes\n", m.Path, typeIndicator, m.Size))
				}
				return sb.String()
			})
		},
	}

	cmd.Flags().StringVar(&path, "path", "", "Directory to search in (required)")
	cmd.Flags().StringVar(&pattern, "pattern", "", "Search pattern (required)")
	cmd.Flags().BoolVar(&recursive, "recursive", true, "Search recursively")
	cmd.Flags().BoolVar(&showHidden, "show-hidden", false, "Include hidden files")
	cmd.Flags().IntVar(&maxResults, "max-results", 1000, "Maximum results to return")
	cmd.MarkFlagRequired("path")
	cmd.MarkFlagRequired("pattern")

	return cmd
}

func searchCodeCmd() *cobra.Command {
	var path, pattern string
	var caseInsensitive, regex, showHidden bool
	var contextLines, maxResults int
	var fileTypes []string

	cmd := &cobra.Command{
		Use:   "search-code",
		Short: "Search for patterns in file contents",
		Long:  "Searches for patterns in file contents with optional context lines",
		Run: func(cmd *cobra.Command, args []string) {
			result, err := core.SearchCode(core.SearchCodeOptions{
				Path:            path,
				Pattern:         pattern,
				CaseInsensitive: caseInsensitive,
				Regex:           regex,
				ContextLines:    contextLines,
				FileTypes:       fileTypes,
				MaxResults:      maxResults,
				ShowHidden:      showHidden,
				AllowedDirs:     GetAllowedDirs(),
			})
			if err != nil {
				OutputError(err)
			}
			OutputResult(result, func() string {
				var sb strings.Builder
				sb.WriteString(fmt.Sprintf("Found %d matches in %d files for '%s'\n\n",
					result.TotalMatches, result.TotalFiles, result.Pattern))
				for _, m := range result.Matches {
					sb.WriteString(fmt.Sprintf("%s:%d: %s\n", m.File, m.Line, m.Content))
					if len(m.Context) > 0 {
						sb.WriteString("  Context:\n")
						for _, c := range m.Context {
							sb.WriteString(fmt.Sprintf("    %s\n", c))
						}
					}
				}
				return sb.String()
			})
		},
	}

	cmd.Flags().StringVar(&path, "path", "", "Directory to search in (required)")
	cmd.Flags().StringVar(&pattern, "pattern", "", "Search pattern (required)")
	cmd.Flags().BoolVar(&caseInsensitive, "ignore-case", false, "Case insensitive search")
	cmd.Flags().BoolVar(&regex, "regex", false, "Use regex pattern")
	cmd.Flags().IntVar(&contextLines, "context", 0, "Lines of context around matches")
	cmd.Flags().StringSliceVar(&fileTypes, "file-types", nil, "File extensions to include")
	cmd.Flags().IntVar(&maxResults, "max-results", 1000, "Maximum results")
	cmd.Flags().BoolVar(&showHidden, "show-hidden", false, "Include hidden files")
	cmd.MarkFlagRequired("path")
	cmd.MarkFlagRequired("pattern")

	return cmd
}
