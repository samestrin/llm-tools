package commands

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var (
	gitChangesPath            string
	gitChangesIncludeUntracked bool
	gitChangesStagedOnly      bool
	gitChangesJSON            bool
	gitChangesMin             bool
)

// GitChangesResult holds the git changes detection result
type GitChangesResult struct {
	Count int      `json:"count"`
	Files []string `json:"files,omitempty"`
}

// newGitChangesCmd creates the git-changes command
func newGitChangesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "git-changes",
		Short: "Count and list git working tree changes",
		Long: `Count and list git working tree changes with optional path filtering.

This command runs 'git status --porcelain' and parses the output to count
and optionally list modified, staged, and untracked files. Path filtering
allows focusing on specific directories like '.planning/'.

Output modes:
  Default:    Multi-line text with COUNT and FILES sections
  --json:     JSON object with count and files array
  --min:      Just the count number
  --json --min: Minimal JSON with count only

Filtering options:
  --path:             Filter to files matching this path prefix
  --include-untracked: Include untracked files (default: true)
  --staged-only:      Only show staged changes

Examples:
  llm-support git-changes
  llm-support git-changes --path .planning/
  llm-support git-changes --staged-only
  llm-support git-changes --include-untracked=false
  llm-support git-changes --min
  llm-support git-changes --json --min`,
		Args: cobra.NoArgs,
		RunE: runGitChanges,
	}

	cmd.Flags().StringVar(&gitChangesPath, "path", "", "Filter to files matching this path prefix")
	cmd.Flags().BoolVar(&gitChangesIncludeUntracked, "include-untracked", true, "Include untracked files")
	cmd.Flags().BoolVar(&gitChangesStagedOnly, "staged-only", false, "Only show staged changes")
	cmd.Flags().BoolVar(&gitChangesJSON, "json", false, "Output as JSON")
	cmd.Flags().BoolVar(&gitChangesMin, "min", false, "Minimal output (count only)")

	return cmd
}

func runGitChanges(cmd *cobra.Command, args []string) error {
	// Check if git is available
	if _, err := exec.LookPath("git"); err != nil {
		return fmt.Errorf("git command not found. Please ensure git is installed and in your PATH")
	}

	// Check if we're in a git repository
	checkCmd := exec.Command("git", "rev-parse", "--git-dir")
	if err := checkCmd.Run(); err != nil {
		return fmt.Errorf("not a git repository (or any parent up to mount point)")
	}

	// Run git status --porcelain
	output, err := runGitStatusPorcelain()
	if err != nil {
		return fmt.Errorf("Error executing git status: %w", err)
	}

	// Normalize path filter
	pathFilter := gitChangesPath
	if pathFilter != "" {
		// Convert to repo-relative if needed
		pathFilter = normalizePathFilter(pathFilter)
	}

	// Parse the output
	result := parseGitStatus(output, pathFilter, gitChangesIncludeUntracked, gitChangesStagedOnly)

	// Output based on flags
	outputGitChanges(cmd, result)

	return nil
}

// runGitStatusPorcelain runs git status --porcelain and returns the output
func runGitStatusPorcelain() (string, error) {
	cmd := exec.Command("git", "status", "--porcelain")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(output), nil
}

// normalizePathFilter normalizes the path filter for matching
func normalizePathFilter(path string) string {
	// Clean the path to handle ../ and ./ components
	cleaned := filepath.Clean(path)

	// Handle special case of "." meaning root
	if cleaned == "." {
		return ""
	}

	return cleaned
}

// parseGitStatus parses git status --porcelain output and applies filters
func parseGitStatus(output string, pathFilter string, includeUntracked bool, stagedOnly bool) GitChangesResult {
	result := GitChangesResult{
		Count: 0,
		Files: []string{},
	}

	if output == "" {
		return result
	}

	lines := strings.Split(strings.TrimSuffix(output, "\n"), "\n")

	for _, line := range lines {
		// Skip empty lines
		if len(line) < 3 {
			continue
		}

		// Parse porcelain format: XY filename
		// X = status in index, Y = status in work tree
		indexStatus := line[0]
		workTreeStatus := line[1]
		filename := strings.TrimSpace(line[3:])

		// Handle renamed files: "R  old -> new"
		// Keep the full string for now

		// Filter: untracked files
		if indexStatus == '?' && workTreeStatus == '?' {
			if !includeUntracked {
				continue
			}
		}

		// Filter: staged-only mode
		if stagedOnly {
			// In staged-only mode, only include files with changes in the index (column 1)
			// A space or '?' in column 1 means no staged changes
			if indexStatus == ' ' || indexStatus == '?' {
				continue
			}
		}

		// Filter: path prefix
		if pathFilter != "" {
			if !strings.HasPrefix(filename, pathFilter) {
				continue
			}
		}

		// File passed all filters
		result.Files = append(result.Files, filename)
		result.Count++
	}

	return result
}

// outputGitChanges outputs the result in the requested format
func outputGitChanges(cmd *cobra.Command, result GitChangesResult) {
	out := cmd.OutOrStdout()

	if gitChangesJSON && gitChangesMin {
		// Compact JSON with only count
		fmt.Fprintf(out, "{\"count\":%d}\n", result.Count)
	} else if gitChangesJSON {
		// Full JSON output
		// Ensure files is never null
		if result.Files == nil {
			result.Files = []string{}
		}
		output, _ := json.MarshalIndent(result, "", "  ")
		fmt.Fprintln(out, string(output))
	} else if gitChangesMin {
		// Just the count
		fmt.Fprintln(out, result.Count)
	} else {
		// Human-readable text format
		fmt.Fprintf(out, "COUNT: %d\n", result.Count)
		fmt.Fprintln(out, "FILES:")
		for _, file := range result.Files {
			fmt.Fprintln(out, file)
		}
	}
}

func init() {
	RootCmd.AddCommand(newGitChangesCmd())
}
