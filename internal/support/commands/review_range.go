package commands

import (
	"fmt"
	"io"

	"github.com/samestrin/llm-tools/internal/support/gitrange"
	"github.com/samestrin/llm-tools/pkg/output"
	"github.com/spf13/cobra"
)

// Flag variables for the review_range command.
var (
	rrRepo        string
	rrBase        string
	rrHead        string
	rrMergeCommit string
	rrFailOnEmpty bool
	rrJSON        bool
	rrMinimal     bool
)

func newReviewRangeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "review_range",
		Short: "Deterministically resolve a git review range (base/head SHAs)",
		Long: `Resolve the base..head range a code review should cover, and detect
empty ranges (e.g. a branch that is already merged into the default branch).

Resolution modes:
  --merge-commit <sha>   base = sha^, head = sha. Use after a squash merge
                         when HEAD is back on the default branch.
  --base [--head]        explicit refs (head defaults to HEAD).
  (no flags)             merge-base of HEAD against the default branch
                         (origin/HEAD, then main/master/origin variants).

An empty range exits 0 with "empty": true and a guidance message — pass
--fail-on-empty to exit non-zero instead. Resolution errors (bad ref, not a
repository, shallow clone) always exit non-zero.

Examples:
  llm-support review_range --repo .
  llm-support review_range --repo . --merge-commit 9e013e7
  llm-support review_range --repo . --base main --head feature --fail-on-empty`,
		Args: cobra.NoArgs,
		RunE: runReviewRange,
	}

	cmd.Flags().StringVar(&rrRepo, "repo", ".", "Repository path")
	cmd.Flags().StringVar(&rrBase, "base", "", "Explicit base ref")
	cmd.Flags().StringVar(&rrHead, "head", "", "Explicit head ref (default HEAD)")
	cmd.Flags().StringVar(&rrMergeCommit, "merge-commit", "", "Merge/squash commit SHA; resolves to sha^..sha (mutually exclusive with --base/--head)")
	cmd.Flags().BoolVar(&rrFailOnEmpty, "fail-on-empty", false, "Exit non-zero when the resolved range is empty")
	cmd.Flags().BoolVar(&rrJSON, "json", false, "Output as JSON")
	cmd.Flags().BoolVar(&rrMinimal, "min", false, "Output in minimal/token-optimized format")

	return cmd
}

func runReviewRange(cmd *cobra.Command, _ []string) error {
	res, err := gitrange.Resolve(gitrange.Params{
		RepoPath:    rrRepo,
		BaseRef:     rrBase,
		HeadRef:     rrHead,
		MergeCommit: rrMergeCommit,
	})
	if err != nil {
		return err
	}

	formatter := output.New(rrJSON, rrMinimal, cmd.OutOrStdout())
	if printErr := formatter.Print(res, printReviewRangeText); printErr != nil {
		return printErr
	}

	if rrFailOnEmpty && res.Empty {
		return fmt.Errorf("range %s..%s is empty — nothing to review", res.Base[:7], res.Head[:7])
	}
	return nil
}

func printReviewRangeText(w io.Writer, data interface{}) {
	r := data.(gitrange.Result)
	fmt.Fprintf(w, "BASE: %s\n", r.Base)
	fmt.Fprintf(w, "HEAD: %s\n", r.Head)
	if r.BaseSymbolic != "" {
		fmt.Fprintf(w, "BASE_REF: %s\n", r.BaseSymbolic)
	}
	fmt.Fprintf(w, "DETECTION: %s\n", r.Detection)
	fmt.Fprintf(w, "COMMITS: %d\n", r.CommitCount)
	fmt.Fprintf(w, "FILES_CHANGED: %d\n", r.FilesChanged)
	if r.Empty {
		fmt.Fprintf(w, "EMPTY: TRUE\n")
	} else {
		fmt.Fprintf(w, "EMPTY: FALSE\n")
	}
	if r.Message != "" {
		fmt.Fprintf(w, "MESSAGE: %s\n", r.Message)
	}
}

func init() {
	RootCmd.AddCommand(newReviewRangeCmd())
}
