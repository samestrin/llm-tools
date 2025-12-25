package commands

import (
	"fmt"
	"os"
	"strings"

	"github.com/sergi/go-diff/diffmatchpatch"
	"github.com/spf13/cobra"
)

var diffUnified bool

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
		fmt.Fprintln(cmd.OutOrStdout(), "IDENTICAL: Files are identical")
		return nil
	}

	dmp := diffmatchpatch.New()
	diffs := dmp.DiffMain(text1, text2, true)

	if diffUnified {
		// Unified diff format
		fmt.Fprintf(cmd.OutOrStdout(), "--- %s\n", path1)
		fmt.Fprintf(cmd.OutOrStdout(), "+++ %s\n", path2)

		for _, diff := range diffs {
			switch diff.Type {
			case diffmatchpatch.DiffDelete:
				for _, line := range strings.Split(diff.Text, "\n") {
					if line != "" {
						fmt.Fprintf(cmd.OutOrStdout(), "-%s\n", line)
					}
				}
			case diffmatchpatch.DiffInsert:
				for _, line := range strings.Split(diff.Text, "\n") {
					if line != "" {
						fmt.Fprintf(cmd.OutOrStdout(), "+%s\n", line)
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
				fmt.Fprintf(cmd.OutOrStdout(), "- %s\n", strings.TrimSpace(diff.Text))
			case diffmatchpatch.DiffInsert:
				fmt.Fprintf(cmd.OutOrStdout(), "+ %s\n", strings.TrimSpace(diff.Text))
			}
		}
	}

	return nil
}

func init() {
	RootCmd.AddCommand(newDiffCmd())
}
