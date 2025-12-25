package commands

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	multiexistsVerbose bool
	multiexistsNoFail  bool
)

// newMultiexistsCmd creates the multiexists command
func newMultiexistsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "multiexists [paths...]",
		Short: "Check if multiple files/directories exist",
		Long: `Check if multiple files or directories exist.

Output format:
  ✓ path: EXISTS [type]
  ✗ path: MISSING

Summary:
  ALL_EXIST: TRUE/FALSE
  MISSING_COUNT: N
  EXIST_COUNT: N`,
		Args: cobra.MinimumNArgs(1),
		RunE: runMultiexists,
	}
	cmd.Flags().BoolVarP(&multiexistsVerbose, "verbose", "v", false, "Show file types")
	cmd.Flags().BoolVar(&multiexistsNoFail, "no-fail", false, "Don't exit with error if files are missing")
	return cmd
}

func runMultiexists(cmd *cobra.Command, args []string) error {
	type result struct {
		path     string
		exists   bool
		fileType string
	}

	var results []result
	for _, pathStr := range args {
		r := result{path: pathStr}

		info, err := os.Stat(pathStr)
		if err == nil {
			r.exists = true
			if info.IsDir() {
				r.fileType = "directory"
			} else if info.Mode()&os.ModeSymlink != 0 {
				r.fileType = "symlink"
			} else {
				r.fileType = "file"
			}
		}

		results = append(results, r)
	}

	// Print results
	for _, r := range results {
		check := "✓"
		status := "EXISTS"
		if !r.exists {
			check = "✗"
			status = "MISSING"
		}

		if multiexistsVerbose && r.fileType != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "%s %s: %s (%s)\n", check, r.path, status, r.fileType)
		} else if r.fileType == "directory" {
			// Always show directories with type
			fmt.Fprintf(cmd.OutOrStdout(), "%s %s/: %s (directory)\n", check, r.path, status)
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "%s %s: %s\n", check, r.path, status)
		}
	}

	// Print summary
	existCount := 0
	for _, r := range results {
		if r.exists {
			existCount++
		}
	}
	missingCount := len(results) - existCount
	allExist := missingCount == 0

	fmt.Fprintln(cmd.OutOrStdout())
	if allExist {
		fmt.Fprintln(cmd.OutOrStdout(), "ALL_EXIST: TRUE")
	} else {
		fmt.Fprintln(cmd.OutOrStdout(), "ALL_EXIST: FALSE")
	}
	fmt.Fprintf(cmd.OutOrStdout(), "MISSING_COUNT: %d\n", missingCount)
	fmt.Fprintf(cmd.OutOrStdout(), "EXIST_COUNT: %d\n", existCount)

	// Exit with error if files are missing (unless --no-fail)
	if !allExist && !multiexistsNoFail {
		return fmt.Errorf("some files are missing")
	}

	return nil
}

func init() {
	RootCmd.AddCommand(newMultiexistsCmd())
}
