package commands

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/samestrin/llm-tools/pkg/output"
	"github.com/spf13/cobra"
)

var (
	countMode      string
	countPath      string
	countRecursive bool
	countPattern   string
	countStyle     string
	countJSON      bool
	countMinimal   bool
	// Legacy flag aliases for backwards compatibility
	countCheckboxes bool
	countLines      bool
	countFiles      bool
)

// CountCheckboxResult holds checkbox count results
type CountCheckboxResult struct {
	Count     int     `json:"count"`
	Checked   int     `json:"checked"`
	Unchecked int     `json:"unchecked"`
	Percent   float64 `json:"percent"`
}

// CountLinesResult holds line count results
type CountLinesResult struct {
	Count int `json:"count"`
}

// CountFilesResult holds file count results
type CountFilesResult struct {
	Count int `json:"count"`
}

// newCountCmd creates the count command
func newCountCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "count",
		Short: "Count checkboxes, lines, or files",
		Long: `Count checkboxes, lines, or files in a path.

Modes (use --mode or legacy flags):
  --mode checkboxes  or  --checkboxes  - Count [ ] and [x] checkboxes in markdown files
  --mode lines       or  --lines       - Count lines in files
  --mode files       or  --files       - Count files matching pattern

Output format:
  TOTAL: N
  CHECKED: N (for checkboxes mode)
  UNCHECKED: N (for checkboxes mode)
  PERCENT: N% (for checkboxes mode)`,
		RunE: runCount,
	}
	cmd.Flags().StringVar(&countPath, "path", "", "Path to count in (required)")
	cmd.Flags().StringVar(&countMode, "mode", "", "Count mode: checkboxes, lines, files")
	cmd.Flags().BoolVarP(&countRecursive, "recursive", "r", false, "Recursive search")
	cmd.Flags().StringVar(&countPattern, "pattern", "", "Glob pattern for files mode")
	cmd.Flags().StringVar(&countStyle, "style", "all", "Checkbox style: all, list, heading")
	// Legacy flags for backwards compatibility with Python version
	cmd.Flags().BoolVar(&countCheckboxes, "checkboxes", false, "Count checkboxes (legacy, use --mode checkboxes)")
	cmd.Flags().BoolVar(&countLines, "lines", false, "Count lines (legacy, use --mode lines)")
	cmd.Flags().BoolVar(&countFiles, "files", false, "Count files (legacy, use --mode files)")
	// Output format flags
	cmd.Flags().BoolVar(&countJSON, "json", false, "Output as JSON")
	cmd.Flags().BoolVar(&countMinimal, "min", false, "Output in minimal/token-optimized format")
	cmd.MarkFlagRequired("path")
	return cmd
}

func runCount(cmd *cobra.Command, args []string) error {
	target, err := filepath.Abs(countPath)
	if err != nil {
		return fmt.Errorf("invalid path: %s", countPath)
	}

	info, err := os.Stat(target)
	if err != nil {
		return fmt.Errorf("path does not exist: %s", target)
	}

	// Handle legacy flags
	mode := countMode
	if mode == "" {
		if countCheckboxes {
			mode = "checkboxes"
		} else if countLines {
			mode = "lines"
		} else if countFiles {
			mode = "files"
		}
	}

	if mode == "" {
		return fmt.Errorf("must specify --mode or one of --checkboxes, --lines, --files")
	}

	switch mode {
	case "checkboxes":
		return runCountCheckboxes(cmd, target, info)
	case "lines":
		return runCountLines(cmd, target, info)
	case "files":
		return runCountFiles(cmd, target, info)
	default:
		return fmt.Errorf("unknown mode: %s (supported: checkboxes, lines, files)", mode)
	}
}

func runCountCheckboxes(cmd *cobra.Command, target string, info os.FileInfo) error {
	var filesToCheck []string

	if info.IsDir() {
		pattern := "*.md"
		if countRecursive {
			pattern = "**/*.md"
		}
		matches, _ := filepath.Glob(filepath.Join(target, pattern))
		filesToCheck = matches

		// For recursive, use Walk
		if countRecursive {
			filesToCheck = []string{}
			filepath.Walk(target, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return nil
				}
				if !info.IsDir() && strings.HasSuffix(path, ".md") {
					filesToCheck = append(filesToCheck, path)
				}
				return nil
			})
		}
	} else {
		filesToCheck = []string{target}
	}

	checked := 0
	unchecked := 0

	// Regex patterns for checkboxes
	listCheckedRe := regexp.MustCompile(`- \[[xX]\]`)
	listUncheckedRe := regexp.MustCompile(`- \[ \]`)
	headingCheckedRe := regexp.MustCompile(`(?m)^#{1,6}\s+.*\[[xX]\]`)
	headingUncheckedRe := regexp.MustCompile(`(?m)^#{1,6}\s+.*\[ \]`)

	for _, filePath := range filesToCheck {
		content, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}

		contentStr := string(content)

		style := countStyle
		if style == "" {
			style = "all"
		}

		if style == "all" || style == "list" {
			checked += len(listCheckedRe.FindAllString(contentStr, -1))
			unchecked += len(listUncheckedRe.FindAllString(contentStr, -1))
		}

		if style == "all" || style == "heading" {
			checked += len(headingCheckedRe.FindAllString(contentStr, -1))
			unchecked += len(headingUncheckedRe.FindAllString(contentStr, -1))
		}
	}

	total := checked + unchecked
	percent := 0.0
	if total > 0 {
		percent = float64(checked) / float64(total) * 100
	}

	result := CountCheckboxResult{
		Count:     total,
		Checked:   checked,
		Unchecked: unchecked,
		Percent:   percent,
	}

	formatter := output.New(countJSON, countMinimal, cmd.OutOrStdout())
	return formatter.Print(result, func(w io.Writer, data interface{}) {
		r := data.(CountCheckboxResult)
		fmt.Fprintf(w, "COUNT: %d\n", r.Count)
		fmt.Fprintf(w, "CHECKED: %d\n", r.Checked)
		fmt.Fprintf(w, "UNCHECKED: %d\n", r.Unchecked)
		fmt.Fprintf(w, "PERCENT: %.0f%%\n", r.Percent)
	})
}

func runCountLines(cmd *cobra.Command, target string, info os.FileInfo) error {
	var filesToCheck []string

	if info.IsDir() {
		if countRecursive {
			filepath.Walk(target, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return nil
				}
				if !info.IsDir() {
					filesToCheck = append(filesToCheck, path)
				}
				return nil
			})
		} else {
			entries, _ := os.ReadDir(target)
			for _, e := range entries {
				if !e.IsDir() {
					filesToCheck = append(filesToCheck, filepath.Join(target, e.Name()))
				}
			}
		}
	} else {
		filesToCheck = []string{target}
	}

	totalLines := 0
	for _, filePath := range filesToCheck {
		content, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}
		lines := strings.Count(string(content), "\n")
		// If file doesn't end with newline but has content, count as 1 line
		if len(content) > 0 && content[len(content)-1] != '\n' {
			lines++
		}
		totalLines += lines
	}

	result := CountLinesResult{Count: totalLines}
	formatter := output.New(countJSON, countMinimal, cmd.OutOrStdout())
	return formatter.Print(result, func(w io.Writer, data interface{}) {
		r := data.(CountLinesResult)
		fmt.Fprintf(w, "COUNT: %d\n", r.Count)
	})
}

func runCountFiles(cmd *cobra.Command, target string, info os.FileInfo) error {
	if !info.IsDir() {
		result := CountFilesResult{Count: 1}
		formatter := output.New(countJSON, countMinimal, cmd.OutOrStdout())
		return formatter.Print(result, func(w io.Writer, data interface{}) {
			r := data.(CountFilesResult)
			fmt.Fprintf(w, "COUNT: %d\n", r.Count)
		})
	}

	pattern := countPattern
	if pattern == "" {
		pattern = "*"
	}

	count := 0
	if countRecursive || strings.Contains(pattern, "**") {
		filepath.Walk(target, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if !info.IsDir() {
				if pattern == "*" || pattern == "**/*" {
					count++
				} else {
					matched, _ := filepath.Match(filepath.Base(pattern), filepath.Base(path))
					if matched {
						count++
					}
				}
			}
			return nil
		})
		_ = pattern // silence unused variable warning in cases where we don't use it
	} else {
		matches, _ := filepath.Glob(filepath.Join(target, pattern))
		for _, m := range matches {
			info, err := os.Stat(m)
			if err == nil && !info.IsDir() {
				count++
			}
		}
	}

	result := CountFilesResult{Count: count}
	formatter := output.New(countJSON, countMinimal, cmd.OutOrStdout())
	return formatter.Print(result, func(w io.Writer, data interface{}) {
		r := data.(CountFilesResult)
		fmt.Fprintf(w, "COUNT: %d\n", r.Count)
	})
}

func init() {
	RootCmd.AddCommand(newCountCmd())
}
