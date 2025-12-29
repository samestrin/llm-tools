package commands

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/samestrin/llm-tools/pkg/output"
	"github.com/sergi/go-diff/diffmatchpatch"
	"github.com/spf13/cobra"
)

var (
	diffUnified bool
	diffJSON    bool
	diffMinimal bool
)

// DiffResult represents the result of a diff operation
type DiffResult struct {
	Identical bool     `json:"identical"`
	File1     string   `json:"file1,omitempty"`
	F1        string   `json:"f1,omitempty"`
	File2     string   `json:"file2,omitempty"`
	F2        string   `json:"f2,omitempty"`
	Additions []string `json:"additions,omitempty"`
	A         []string `json:"a,omitempty"`
	Deletions []string `json:"deletions,omitempty"`
	D         []string `json:"d,omitempty"`
}

// newDiffCmd creates the diff command
func newDiffCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "diff [path1] [path2]",
		Short: "Compare two files",
		Long:  `Compare two files and show the differences.`,
		Args:  cobra.ExactArgs(2),
		RunE:  runDiff,
	}
	cmd.Flags().BoolVarP(&diffUnified, "unified", "u", false, "Use unified diff format")
	cmd.Flags().BoolVar(&diffJSON, "json", false, "Output as JSON")
	cmd.Flags().BoolVar(&diffMinimal, "min", false, "Output in minimal/token-optimized format")
	return cmd
}

func runDiff(cmd *cobra.Command, args []string) error {
	path1, path2 := args[0], args[1]

	content1, err := os.ReadFile(path1)
	if err != nil {
		return fmt.Errorf("cannot read file: %s", path1)
	}

	content2, err := os.ReadFile(path2)
	if err != nil {
		return fmt.Errorf("cannot read file: %s", path2)
	}

	text1 := string(content1)
	text2 := string(content2)

	if text1 == text2 {
		result := DiffResult{Identical: true}
		if diffMinimal {
			result.F1 = path1
			result.F2 = path2
		} else {
			result.File1 = path1
			result.File2 = path2
		}
		formatter := output.New(diffJSON, diffMinimal, cmd.OutOrStdout())
		return formatter.Print(result, func(w io.Writer, data interface{}) {
			fmt.Fprintln(w, "IDENTICAL: Files are identical")
		})
	}

	dmp := diffmatchpatch.New()
	diffs := dmp.DiffMain(text1, text2, true)

	// Collect additions and deletions
	var additions, deletions []string
	for _, diff := range diffs {
		switch diff.Type {
		case diffmatchpatch.DiffDelete:
			for _, line := range strings.Split(diff.Text, "\n") {
				if line != "" {
					deletions = append(deletions, strings.TrimSpace(line))
				}
			}
		case diffmatchpatch.DiffInsert:
			for _, line := range strings.Split(diff.Text, "\n") {
				if line != "" {
					additions = append(additions, strings.TrimSpace(line))
				}
			}
		}
	}

	var result DiffResult
	if diffMinimal {
		result = DiffResult{
			Identical: false,
			F1:        path1,
			F2:        path2,
			A:         additions,
			D:         deletions,
		}
	} else {
		result = DiffResult{
			Identical: false,
			File1:     path1,
			File2:     path2,
			Additions: additions,
			Deletions: deletions,
		}
	}

	formatter := output.New(diffJSON, diffMinimal, cmd.OutOrStdout())
	return formatter.Print(result, func(w io.Writer, data interface{}) {
		if diffUnified {
			// Unified diff format
			fmt.Fprintf(w, "--- %s\n", path1)
			fmt.Fprintf(w, "+++ %s\n", path2)

			for _, diff := range diffs {
				switch diff.Type {
				case diffmatchpatch.DiffDelete:
					for _, line := range strings.Split(diff.Text, "\n") {
						if line != "" {
							fmt.Fprintf(w, "-%s\n", line)
						}
					}
				case diffmatchpatch.DiffInsert:
					for _, line := range strings.Split(diff.Text, "\n") {
						if line != "" {
							fmt.Fprintf(w, "+%s\n", line)
						}
					}
				case diffmatchpatch.DiffEqual:
					// Skip equal parts in unified diff for brevity
				}
			}
		} else {
			// Simple diff format
			for _, diff := range diffs {
				switch diff.Type {
				case diffmatchpatch.DiffDelete:
					fmt.Fprintf(w, "- %s\n", strings.TrimSpace(diff.Text))
				case diffmatchpatch.DiffInsert:
					fmt.Fprintf(w, "+ %s\n", strings.TrimSpace(diff.Text))
				}
			}
		}
	})
}

func init() {
	RootCmd.AddCommand(newDiffCmd())
}
