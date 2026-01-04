package commands

import (
	"fmt"
	"strings"

	"github.com/samestrin/llm-tools/internal/filesystem/core"
	"github.com/spf13/cobra"
)

func addReadCommands(rootCmd *cobra.Command) {
	rootCmd.AddCommand(readFileCmd())
	rootCmd.AddCommand(readMultipleFilesCmd())
	rootCmd.AddCommand(extractLinesCmd())
}

func readFileCmd() *cobra.Command {
	var path string
	var startOffset, lineStart, lineCount int
	var maxSize int64

	cmd := &cobra.Command{
		Use:   "read-file",
		Short: "Read a file",
		Long:  "Reads a file with optional line range or byte offset",
		Run: func(cmd *cobra.Command, args []string) {
			// Size limit: 0 = use default, -1 = no limit, >0 = custom
			sizeLimit := maxSize
			if !cmd.Flags().Changed("max-size") {
				sizeLimit = 0 // Use default
			}

			result, err := core.ReadFile(core.ReadFileOptions{
				Path:             path,
				StartOffset:      startOffset,
				LineStart:        lineStart,
				LineCount:        lineCount,
				AllowedDirs:      GetAllowedDirs(),
				SizeCheckMaxSize: sizeLimit,
			})
			if err != nil {
				// Check for size exceeded error and output as JSON
				if sizeErr, ok := err.(*core.SizeExceededError); ok {
					if jsonOutput {
						fmt.Println(sizeErr.ToJSON())
						return
					}
				}
				OutputError(err)
				return
			}
			OutputResult(result, func() string {
				return result.Content
			})
		},
	}

	cmd.Flags().StringVar(&path, "path", "", "File path to read (required)")
	cmd.Flags().IntVar(&startOffset, "start-offset", 0, "Starting byte offset")
	cmd.Flags().Int64Var(&maxSize, "max-size", 0, "Maximum JSON output size in chars (0 = default 70000, -1 = no limit)")
	cmd.Flags().IntVar(&lineStart, "line-start", 0, "Starting line number")
	cmd.Flags().IntVar(&lineCount, "line-count", 0, "Number of lines to read")
	cmd.MarkFlagRequired("path")

	return cmd
}

func readMultipleFilesCmd() *cobra.Command {
	var paths []string
	var maxTotalSize int64

	cmd := &cobra.Command{
		Use:   "read-multiple-files",
		Short: "Read multiple files simultaneously",
		Long:  "Reads multiple files concurrently and returns their contents",
		Run: func(cmd *cobra.Command, args []string) {
			// Size limit: 0 = use default, -1 = no limit, >0 = custom
			sizeLimit := maxTotalSize
			if !cmd.Flags().Changed("max-total-size") {
				sizeLimit = 0 // Use default
			}

			result, err := core.ReadMultipleFiles(core.ReadMultipleFilesOptions{
				Paths:                 paths,
				AllowedDirs:           GetAllowedDirs(),
				SizeCheckMaxTotalSize: sizeLimit,
			})
			if err != nil {
				// Check for size exceeded error and output as JSON
				if sizeErr, ok := err.(*core.TotalSizeExceededError); ok {
					if jsonOutput {
						fmt.Println(sizeErr.ToJSON())
						return
					}
				}
				OutputError(err)
				return
			}
			OutputResult(result, func() string {
				var sb strings.Builder
				sb.WriteString(fmt.Sprintf("Read %d files (%d success, %d failed)\n",
					len(result.Files), result.Success, result.Failed))
				for _, f := range result.Files {
					if f.Error != "" {
						sb.WriteString(fmt.Sprintf("\n--- %s (ERROR: %s) ---\n", f.Path, f.Error))
					} else {
						sb.WriteString(fmt.Sprintf("\n--- %s (%d bytes, %d lines) ---\n%s",
							f.Path, f.Size, f.Lines, f.Content))
					}
				}
				return sb.String()
			})
		},
	}

	cmd.Flags().StringSliceVar(&paths, "paths", nil, "File paths to read (comma-separated)")
	cmd.Flags().Int64Var(&maxTotalSize, "max-total-size", 0, "Maximum combined JSON output size in chars (0 = default 70000, -1 = no limit)")
	cmd.MarkFlagRequired("paths")

	return cmd
}

func extractLinesCmd() *cobra.Command {
	var path, pattern string
	var lineNumbers []int
	var startLine, endLine, contextLines int

	cmd := &cobra.Command{
		Use:   "extract-lines",
		Short: "Extract specific lines from a file",
		Long:  "Extracts lines by number, range, or pattern from a file",
		Run: func(cmd *cobra.Command, args []string) {
			result, err := core.ExtractLines(core.ExtractLinesOptions{
				Path:         path,
				LineNumbers:  lineNumbers,
				StartLine:    startLine,
				EndLine:      endLine,
				Pattern:      pattern,
				ContextLines: contextLines,
				AllowedDirs:  GetAllowedDirs(),
			})
			if err != nil {
				OutputError(err)
			}
			OutputResult(result, func() string {
				return result.Content
			})
		},
	}

	cmd.Flags().StringVar(&path, "path", "", "File path (required)")
	cmd.Flags().IntSliceVar(&lineNumbers, "lines", nil, "Specific line numbers to extract")
	cmd.Flags().IntVar(&startLine, "start", 0, "Start line for range extraction")
	cmd.Flags().IntVar(&endLine, "end", 0, "End line for range extraction")
	cmd.Flags().StringVar(&pattern, "pattern", "", "Pattern to match for extraction")
	cmd.Flags().IntVar(&contextLines, "context", 0, "Context lines around pattern matches")
	cmd.MarkFlagRequired("path")

	return cmd
}
