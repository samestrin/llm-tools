package commands

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/samestrin/llm-tools/pkg/output"
	"github.com/spf13/cobra"
)

var (
	repoRootValidate bool
	repoRootPath     string
	repoRootJSON     bool
	repoRootMinimal  bool
)

// RepoRootResult represents the repo root result
type RepoRootResult struct {
	Root  string `json:"root,omitempty"`
	Valid bool   `json:"valid,omitempty"`
	Error string `json:"error,omitempty"`
}

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
	cmd.Flags().BoolVar(&repoRootJSON, "json", false, "Output as JSON")
	cmd.Flags().BoolVar(&repoRootMinimal, "min", false, "Output in minimal/token-optimized format")

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
	result := RepoRootResult{}

	if err != nil {
		result.Error = "not a git repository"
		if repoRootValidate {
			result.Valid = false
		}
	} else {
		result.Root = strings.TrimSpace(root)

		// Optionally validate .git exists
		if repoRootValidate {
			gitDir := filepath.Join(result.Root, ".git")
			if _, err := os.Stat(gitDir); err == nil {
				result.Valid = true
			} else {
				result.Valid = false
			}
		}
	}

	formatter := output.New(repoRootJSON, repoRootMinimal, cmd.OutOrStdout())
	return formatter.Print(result, printRepoRootText)
}

func printRepoRootText(w io.Writer, data interface{}) {
	r := data.(RepoRootResult)
	fmt.Fprintf(w, "ROOT: %s\n", r.Root)
	if r.Error != "" {
		fmt.Fprintf(w, "ERROR: %s\n", r.Error)
	}
	if repoRootValidate {
		if r.Valid {
			fmt.Fprintf(w, "VALID: TRUE\n")
		} else {
			fmt.Fprintf(w, "VALID: FALSE\n")
		}
	}
}

func init() {
	RootCmd.AddCommand(newRepoRootCmd())
}
