package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var (
	repoRootValidate bool
	repoRootPath     string
)

// newRepoRootCmd creates the repo-root command
func newRepoRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "repo-root",
		Short: "Find and output git repository root",
		Long: `Find the git repository root for a given path.

Returns the absolute path to the repository root, suitable for anchoring
all subsequent path operations in LLM prompts.

Output format:
  ROOT: /absolute/path/to/repo
  VALID: TRUE|FALSE (with --validate)

Examples:
  llm-support repo-root
  llm-support repo-root --path /path/to/subdir
  llm-support repo-root --validate`,
		Args: cobra.NoArgs,
		RunE: runRepoRoot,
	}

	cmd.Flags().StringVar(&repoRootPath, "path", ".", "Starting path to search from")
	cmd.Flags().BoolVar(&repoRootValidate, "validate", false, "Also verify .git directory exists")

	return cmd
}

func runRepoRoot(cmd *cobra.Command, args []string) error {
	// Use path from flag
	startPath := repoRootPath

	// Resolve to absolute path
	absPath, err := filepath.Abs(startPath)
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}

	// Verify path exists
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		return fmt.Errorf("path does not exist: %s", absPath)
	}

	// Get repository root using git rev-parse
	root, err := runGitOutput(absPath, "rev-parse", "--show-toplevel")
	if err != nil {
		fmt.Fprintf(cmd.OutOrStdout(), "ROOT: \n")
		fmt.Fprintf(cmd.OutOrStdout(), "ERROR: not a git repository\n")
		if repoRootValidate {
			fmt.Fprintf(cmd.OutOrStdout(), "VALID: FALSE\n")
		}
		return nil // Return nil so output is parseable, not an error
	}

	root = strings.TrimSpace(root)
	fmt.Fprintf(cmd.OutOrStdout(), "ROOT: %s\n", root)

	// Optionally validate .git exists
	if repoRootValidate {
		gitDir := filepath.Join(root, ".git")
		if _, err := os.Stat(gitDir); err == nil {
			fmt.Fprintf(cmd.OutOrStdout(), "VALID: TRUE\n")
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "VALID: FALSE\n")
		}
	}

	return nil
}

func init() {
	RootCmd.AddCommand(newRepoRootCmd())
}
