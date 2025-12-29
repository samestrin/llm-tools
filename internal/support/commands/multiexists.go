package commands

import (
	"fmt"
	"io"
	"os"

	"github.com/samestrin/llm-tools/pkg/output"
	"github.com/spf13/cobra"
)

var (
	multiexistsVerbose bool
	multiexistsNoFail  bool
	multiexistsJSON    bool
	multiexistsMinimal bool
)

// MultiexistsEntry represents a single path check result
type MultiexistsEntry struct {
	Path   string `json:"path,omitempty"`
	Exists bool   `json:"exists"`
	Type   string `json:"type,omitempty"`
}

// MultiexistsResult represents the complete check result
type MultiexistsResult struct {
	Entries      []MultiexistsEntry `json:"entries,omitempty"`
	AllExist     bool               `json:"all_exist"`
	ExistCount   int                `json:"exist_count"`
	MissingCount int                `json:"missing_count"`
}

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
	cmd.Flags().BoolVar(&multiexistsJSON, "json", false, "Output as JSON")
	cmd.Flags().BoolVar(&multiexistsMinimal, "min", false, "Output in minimal/token-optimized format")
	return cmd
}

func runMultiexists(cmd *cobra.Command, args []string) error {
	var entries []MultiexistsEntry
	for _, pathStr := range args {
		entry := MultiexistsEntry{Path: pathStr}

		info, err := os.Stat(pathStr)
		if err == nil {
			entry.Exists = true
			if info.IsDir() {
				entry.Type = "directory"
			} else if info.Mode()&os.ModeSymlink != 0 {
				entry.Type = "symlink"
			} else {
				entry.Type = "file"
			}
		}

		entries = append(entries, entry)
	}

	// Calculate summary
	existCount := 0
	for _, e := range entries {
		if e.Exists {
			existCount++
		}
	}
	missingCount := len(entries) - existCount
	allExist := missingCount == 0

	result := MultiexistsResult{
		Entries:      entries,
		AllExist:     allExist,
		ExistCount:   existCount,
		MissingCount: missingCount,
	}

	formatter := output.New(multiexistsJSON, multiexistsMinimal, cmd.OutOrStdout())
	if err := formatter.Print(result, printMultiexistsText); err != nil {
		return err
	}

	// Exit with error if files are missing (unless --no-fail)
	if !allExist && !multiexistsNoFail {
		return fmt.Errorf("some files are missing")
	}

	return nil
}

func printMultiexistsText(w io.Writer, data interface{}) {
	result := data.(MultiexistsResult)

	// Print entries
	for _, e := range result.Entries {
		check := "✓"
		status := "EXISTS"
		if !e.Exists {
			check = "✗"
			status = "MISSING"
		}

		if multiexistsVerbose && e.Type != "" {
			fmt.Fprintf(w, "%s %s: %s (%s)\n", check, e.Path, status, e.Type)
		} else if e.Type == "directory" {
			// Always show directories with type
			fmt.Fprintf(w, "%s %s/: %s (directory)\n", check, e.Path, status)
		} else {
			fmt.Fprintf(w, "%s %s: %s\n", check, e.Path, status)
		}
	}

	// Print summary
	fmt.Fprintln(w)
	if result.AllExist {
		fmt.Fprintln(w, "ALL_EXIST: TRUE")
	} else {
		fmt.Fprintln(w, "ALL_EXIST: FALSE")
	}
	fmt.Fprintf(w, "MISSING_COUNT: %d\n", result.MissingCount)
	fmt.Fprintf(w, "EXIST_COUNT: %d\n", result.ExistCount)
}

func init() {
	RootCmd.AddCommand(newMultiexistsCmd())
}
