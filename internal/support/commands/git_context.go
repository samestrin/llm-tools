package commands

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var (
	gitContextIncludeDiff bool
	gitContextSince       string
	gitContextMaxCommits  int
	gitContextJSON        bool
)

// GitContext holds the gathered git repository information
type GitContext struct {
	Repository GitRepository `json:"repository"`
	Branch     GitBranch     `json:"branch"`
	Status     GitStatus     `json:"status"`
	Commits    []GitCommit   `json:"commits,omitempty"`
	Remotes    []GitRemote   `json:"remotes,omitempty"`
	Diff       string        `json:"diff,omitempty"`
}

// GitRepository holds repository path info
type GitRepository struct {
	Path string `json:"path"`
	Root string `json:"root"`
}

// GitBranch holds branch information
type GitBranch struct {
	Current  string `json:"current"`
	Tracking string `json:"tracking,omitempty"`
}

// GitStatus holds working tree status
type GitStatus struct {
	Clean     bool `json:"clean"`
	Modified  int  `json:"modified"`
	Added     int  `json:"added"`
	Deleted   int  `json:"deleted"`
	Untracked int  `json:"untracked"`
	Conflict  bool `json:"conflict"`
}

// GitCommit holds commit information
type GitCommit struct {
	SHA     string `json:"sha"`
	Author  string `json:"author"`
	Date    string `json:"date"`
	Message string `json:"message"`
}

// GitRemote holds remote information
type GitRemote struct {
	Name     string `json:"name"`
	FetchURL string `json:"fetch_url"`
	PushURL  string `json:"push_url"`
}

// newGitContextCmd creates the git-context command
func newGitContextCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "git-context [path]",
		Short: "Gather git repository context information",
		Long: `Gather comprehensive git repository information including branch,
status, recent commits, and remotes.

Examples:
  llm-support git-context
  llm-support git-context /path/to/repo
  llm-support git-context --include-diff
  llm-support git-context --since 2025-12-01 --max-commits 10
  llm-support git-context --json`,
		Args: cobra.MaximumNArgs(1),
		RunE: runGitContext,
	}

	cmd.Flags().BoolVar(&gitContextIncludeDiff, "include-diff", false, "Include diff of uncommitted changes")
	cmd.Flags().StringVar(&gitContextSince, "since", "", "Only include commits since date (YYYY-MM-DD)")
	cmd.Flags().IntVar(&gitContextMaxCommits, "max-commits", 10, "Maximum number of commits to include")
	cmd.Flags().BoolVar(&gitContextJSON, "json", false, "Output as JSON")

	return cmd
}

func runGitContext(cmd *cobra.Command, args []string) error {
	// Determine repository path
	repoPath := "."
	if len(args) > 0 {
		repoPath = args[0]
	}

	absPath, err := filepath.Abs(repoPath)
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}

	// Check if git is available
	if _, err := exec.LookPath("git"); err != nil {
		return fmt.Errorf("git command not found")
	}

	// Check if path is a git repository
	if err := runGit(absPath, "rev-parse", "--git-dir"); err != nil {
		return fmt.Errorf("not a git repository: %s", absPath)
	}

	// Validate since date if provided
	if gitContextSince != "" {
		if _, err := time.Parse("2006-01-02", gitContextSince); err != nil {
			return fmt.Errorf("invalid date format: %s (expected YYYY-MM-DD)", gitContextSince)
		}
	}

	// Gather context
	ctx := GitContext{}

	// Get repository info
	root, _ := runGitOutput(absPath, "rev-parse", "--show-toplevel")
	ctx.Repository = GitRepository{
		Path: absPath,
		Root: strings.TrimSpace(root),
	}

	// Get branch info
	branch, _ := runGitOutput(absPath, "rev-parse", "--abbrev-ref", "HEAD")
	ctx.Branch.Current = strings.TrimSpace(branch)

	if ctx.Branch.Current == "HEAD" {
		// Detached HEAD - get commit SHA instead
		sha, _ := runGitOutput(absPath, "rev-parse", "--short", "HEAD")
		ctx.Branch.Current = "detached:" + strings.TrimSpace(sha)
	}

	// Get tracking branch
	tracking, err := runGitOutput(absPath, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}")
	if err == nil {
		ctx.Branch.Tracking = strings.TrimSpace(tracking)
	}

	// Get status
	ctx.Status = getGitStatus(absPath)

	// Get commits
	ctx.Commits = getGitCommits(absPath, gitContextMaxCommits, gitContextSince)

	// Get remotes
	ctx.Remotes = getGitRemotes(absPath)

	// Get diff if requested
	if gitContextIncludeDiff && !ctx.Status.Clean {
		diff, _ := runGitOutput(absPath, "diff")
		stagedDiff, _ := runGitOutput(absPath, "diff", "--cached")
		ctx.Diff = diff + stagedDiff

		// Truncate if too large
		if len(ctx.Diff) > 50000 {
			ctx.Diff = ctx.Diff[:50000] + "\n... (diff truncated, too large)"
		}
	}

	// Output
	if gitContextJSON {
		output, _ := json.MarshalIndent(ctx, "", "  ")
		fmt.Fprintln(cmd.OutOrStdout(), string(output))
	} else {
		printGitContext(cmd, ctx)
	}

	return nil
}

func getGitStatus(path string) GitStatus {
	status := GitStatus{Clean: true}

	output, err := runGitOutput(path, "status", "--porcelain")
	if err != nil || output == "" {
		return status
	}

	lines := strings.Split(strings.TrimSpace(output), "\n")
	for _, line := range lines {
		if len(line) < 2 {
			continue
		}

		status.Clean = false
		indexStatus := line[0]
		workStatus := line[1]

		// Check for conflicts
		if indexStatus == 'U' || workStatus == 'U' ||
			(indexStatus == 'A' && workStatus == 'A') ||
			(indexStatus == 'D' && workStatus == 'D') {
			status.Conflict = true
		}

		// Count by type
		switch {
		case indexStatus == 'M' || workStatus == 'M':
			status.Modified++
		case indexStatus == 'A':
			status.Added++
		case indexStatus == 'D' || workStatus == 'D':
			status.Deleted++
		case indexStatus == '?' && workStatus == '?':
			status.Untracked++
		}
	}

	return status
}

func getGitCommits(path string, maxCommits int, since string) []GitCommit {
	args := []string{"log",
		fmt.Sprintf("--max-count=%d", maxCommits),
		"--pretty=format:%H|%an <%ae>|%aI|%s",
	}

	if since != "" {
		args = append(args, "--since="+since)
	}

	output, err := runGitOutput(path, args...)
	if err != nil || output == "" {
		return nil
	}

	var commits []GitCommit
	lines := strings.Split(strings.TrimSpace(output), "\n")
	for _, line := range lines {
		parts := strings.SplitN(line, "|", 4)
		if len(parts) == 4 {
			commits = append(commits, GitCommit{
				SHA:     parts[0][:12], // Short SHA
				Author:  parts[1],
				Date:    parts[2],
				Message: parts[3],
			})
		}
	}

	return commits
}

func getGitRemotes(path string) []GitRemote {
	output, err := runGitOutput(path, "remote", "-v")
	if err != nil || output == "" {
		return nil
	}

	remoteMap := make(map[string]*GitRemote)
	lines := strings.Split(strings.TrimSpace(output), "\n")

	// Parse: origin	git@github.com:user/repo.git (fetch)
	re := regexp.MustCompile(`^(\S+)\s+(\S+)\s+\((\w+)\)$`)

	for _, line := range lines {
		matches := re.FindStringSubmatch(line)
		if len(matches) == 4 {
			name := matches[1]
			url := matches[2]
			typ := matches[3]

			if _, exists := remoteMap[name]; !exists {
				remoteMap[name] = &GitRemote{Name: name}
			}

			if typ == "fetch" {
				remoteMap[name].FetchURL = url
			} else if typ == "push" {
				remoteMap[name].PushURL = url
			}
		}
	}

	remotes := make([]GitRemote, 0, len(remoteMap))
	for _, r := range remoteMap {
		remotes = append(remotes, *r)
	}
	return remotes
}

func printGitContext(cmd *cobra.Command, ctx GitContext) {
	out := cmd.OutOrStdout()

	fmt.Fprintf(out, "REPOSITORY: %s\n", ctx.Repository.Root)
	fmt.Fprintf(out, "BRANCH: %s\n", ctx.Branch.Current)
	if ctx.Branch.Tracking != "" {
		fmt.Fprintf(out, "TRACKING: %s\n", ctx.Branch.Tracking)
	}
	fmt.Fprintln(out, "---")

	// Status
	if ctx.Status.Clean {
		fmt.Fprintln(out, "STATUS: clean")
	} else {
		fmt.Fprintln(out, "STATUS: dirty")
		if ctx.Status.Conflict {
			fmt.Fprintln(out, "CONFLICT: true")
		}
		fmt.Fprintf(out, "MODIFIED: %d\n", ctx.Status.Modified)
		fmt.Fprintf(out, "ADDED: %d\n", ctx.Status.Added)
		fmt.Fprintf(out, "DELETED: %d\n", ctx.Status.Deleted)
		fmt.Fprintf(out, "UNTRACKED: %d\n", ctx.Status.Untracked)
	}
	fmt.Fprintln(out, "---")

	// Commits
	if len(ctx.Commits) > 0 {
		fmt.Fprintf(out, "COMMITS: %d\n", len(ctx.Commits))
		for _, commit := range ctx.Commits {
			fmt.Fprintf(out, "  %s %s\n", commit.SHA, commit.Message)
		}
	} else {
		fmt.Fprintln(out, "COMMITS: 0")
	}
	fmt.Fprintln(out, "---")

	// Remotes
	if len(ctx.Remotes) > 0 {
		fmt.Fprintf(out, "REMOTES: %d\n", len(ctx.Remotes))
		for _, remote := range ctx.Remotes {
			fmt.Fprintf(out, "  %s: %s\n", remote.Name, remote.FetchURL)
		}
	} else {
		fmt.Fprintln(out, "REMOTES: no remotes configured")
	}

	// Diff
	if ctx.Diff != "" {
		fmt.Fprintln(out, "---")
		fmt.Fprintln(out, "DIFF:")
		fmt.Fprintln(out, ctx.Diff)
	}
}

func runGit(path string, args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = path
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run()
}

func runGitOutput(path string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = path
	output, err := cmd.Output()
	return string(output), err
}

// parseMaxCommits helper for testing
func parseMaxCommits(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}

func init() {
	RootCmd.AddCommand(newGitContextCmd())
}
