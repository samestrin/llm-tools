package commands

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/samestrin/llm-tools/pkg/output"
	"github.com/samestrin/llm-tools/pkg/pathvalidation"
	"github.com/spf13/cobra"
)

var (
	initTempPreserve    bool
	initTempName        string
	initTempClean       bool
	initTempWithGit     bool
	initTempSkipContext bool
	initTempJSON        bool
	initTempMinimal     bool
)

// InitTempResult holds the init-temp command result
type InitTempResult struct {
	// Core fields (always present)
	TempDir   string `json:"temp_dir"`
	TD        string `json:"td,omitempty"` // minimal alias
	RepoRoot  string `json:"repo_root"`
	RR        string `json:"rr,omitempty"` // minimal alias
	Today     string `json:"today"`
	TodayLong string `json:"today_long,omitempty"`
	TSLong    string `json:"ts_long,omitempty"` // minimal alias
	Timestamp string `json:"timestamp"`
	TS        string `json:"ts,omitempty"` // minimal alias
	Epoch     int64  `json:"epoch"`

	// Git fields (with --with-git) - optional
	Branch      string `json:"branch,omitempty"`
	BR          string `json:"br,omitempty"` // minimal alias
	CommitShort string `json:"commit_short,omitempty"`
	CS          string `json:"cs,omitempty"` // minimal alias

	// Status fields
	Status        string `json:"status"`
	S             string `json:"s,omitempty"` // minimal alias
	Cleaned       int    `json:"cleaned,omitempty"`
	Cl            *int   `json:"cl,omitempty"` // minimal alias
	ExistingFiles int    `json:"existing_files,omitempty"`
	EF            *int   `json:"ef,omitempty"` // minimal alias
	ContextFile   string `json:"context_file"`
	CF            string `json:"cf,omitempty"` // minimal alias
}

// newInitTempCmd creates the init-temp command
func newInitTempCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init-temp",
		Short: "Initialize temp directory with common variables",
		Long: `Initialize and manage temp directories with consistent patterns.

Creates .planning/.temp/{name}/ directory for command-specific temp files.
Also returns common variables needed for LLM prompts (repo root, timestamps).

Modes:
  --clean (default): Remove existing files before creating
  --preserve: Keep existing files if directory exists

Output (always returned):
  TEMP_DIR: path to temp directory
  REPO_ROOT: git repository root
  TODAY: YYYY-MM-DD
  TODAY_LONG: January 02, 2006 03:04:05PM
  TIMESTAMP: YYYY-MM-DD HH:MM:SS
  EPOCH: Unix timestamp (seconds)
  STATUS: CREATED | EXISTS
  CONTEXT_FILE: path to context.env (unless --skip-context)

Output (with --with-git):
  BRANCH: current git branch
  COMMIT_SHORT: short commit hash`,
		RunE: runInitTemp,
	}

	cmd.Flags().StringVar(&initTempName, "name", "", "Name for temp directory (required)")
	cmd.Flags().BoolVar(&initTempPreserve, "preserve", false, "Keep existing files")
	cmd.Flags().BoolVar(&initTempClean, "clean", true, "Remove existing files (default)")
	cmd.Flags().BoolVar(&initTempWithGit, "with-git", false, "Include BRANCH and COMMIT_SHORT")
	cmd.Flags().BoolVar(&initTempSkipContext, "skip-context", false, "Don't create context.env file")
	cmd.Flags().BoolVar(&initTempJSON, "json", false, "Output as JSON")
	cmd.Flags().BoolVar(&initTempMinimal, "min", false, "Output in minimal/token-optimized format")
	cmd.MarkFlagRequired("name")

	return cmd
}

func runInitTemp(cmd *cobra.Command, args []string) error {
	name := initTempName

	// Check for unresolved template variables in name
	if err := pathvalidation.ValidatePathForCreation(name); err != nil {
		return err
	}

	// Get current working directory for git operations
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	// Get repository root (required - planning system needs git)
	repoRoot, err := getRepoRoot(cwd)
	if err != nil {
		return fmt.Errorf("not in a git repository: %w", err)
	}

	// Build paths relative to repo root
	baseTemp := filepath.Join(repoRoot, ".planning", ".temp")
	tempDir := filepath.Join(baseTemp, name)

	cleanedCount := 0
	existingFiles := 0
	status := "CREATED"

	// Handle existing directory
	if info, err := os.Stat(tempDir); err == nil && info.IsDir() {
		if initTempPreserve {
			// Count existing files
			existingFiles = countFilesRecursive(tempDir)
			status = "EXISTS"
		} else {
			// Clean mode - remove existing files
			cleanedCount = cleanDirectory(tempDir)
			status = "CREATED"
		}
	}

	// Ensure directory exists
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}

	// Generate timestamps
	now := time.Now()
	today := now.Format("2006-01-02")
	todayLong := now.Format("January 02, 2006 03:04:05PM")
	timestamp := now.Format("2006-01-02 15:04:05")
	epoch := now.Unix()

	// Create context.env file (unless skip-context)
	contextFile := ""
	if !initTempSkipContext {
		contextFile = filepath.Join(tempDir, "context.env")
		if err := createContextEnv(contextFile); err != nil {
			return fmt.Errorf("failed to create context.env: %w", err)
		}
	}

	// Get git info if requested
	branch := ""
	commitShort := ""
	if initTempWithGit {
		branch, _ = getGitBranch(repoRoot)
		commitShort, _ = getGitCommitShort(repoRoot)
	}

	// Build result
	var result InitTempResult
	if initTempMinimal {
		result = InitTempResult{
			TD:     tempDir,
			RR:     repoRoot,
			Today:  today,
			TSLong: todayLong,
			TS:     timestamp,
			Epoch:  epoch,
			S:      status,
		}
		if contextFile != "" {
			result.CF = contextFile
		}
		if initTempWithGit {
			result.BR = branch
			result.CS = commitShort
		}
		if initTempPreserve && status == "EXISTS" {
			result.EF = &existingFiles
		} else if !initTempPreserve {
			result.Cl = &cleanedCount
		}
	} else {
		result = InitTempResult{
			TempDir:   tempDir,
			RepoRoot:  repoRoot,
			Today:     today,
			TodayLong: todayLong,
			Timestamp: timestamp,
			Epoch:     epoch,
			Status:    status,
		}
		if contextFile != "" {
			result.ContextFile = contextFile
		}
		if initTempWithGit {
			result.Branch = branch
			result.CommitShort = commitShort
		}
		if initTempPreserve && status == "EXISTS" {
			result.ExistingFiles = existingFiles
		} else if !initTempPreserve {
			result.Cleaned = cleanedCount
		}
	}

	// Output
	formatter := output.New(initTempJSON, initTempMinimal, cmd.OutOrStdout())
	return formatter.Print(result, func(w io.Writer, data interface{}) {
		fmt.Fprintf(w, "TEMP_DIR=%s\n", tempDir)
		fmt.Fprintf(w, "REPO_ROOT=%s\n", repoRoot)
		fmt.Fprintf(w, "TODAY=%s\n", today)
		fmt.Fprintf(w, "TODAY_LONG=%s\n", todayLong)
		fmt.Fprintf(w, "TIMESTAMP=%s\n", timestamp)
		fmt.Fprintf(w, "EPOCH=%d\n", epoch)
		fmt.Fprintf(w, "STATUS=%s\n", status)
		if contextFile != "" {
			fmt.Fprintf(w, "CONTEXT_FILE=%s\n", contextFile)
		}
		if initTempWithGit {
			fmt.Fprintf(w, "BRANCH=%s\n", branch)
			fmt.Fprintf(w, "COMMIT_SHORT=%s\n", commitShort)
		}
		if initTempPreserve && status == "EXISTS" {
			fmt.Fprintf(w, "EXISTING_FILES=%d\n", existingFiles)
		} else if !initTempPreserve {
			fmt.Fprintf(w, "CLEANED=%d\n", cleanedCount)
		}
	})
}

// getRepoRoot returns the git repository root for the given path
func getRepoRoot(path string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	cmd.Dir = path
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// getGitBranch returns the current git branch
func getGitBranch(path string) (string, error) {
	cmd := exec.Command("git", "branch", "--show-current")
	cmd.Dir = path
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// getGitCommitShort returns the short commit hash
func getGitCommitShort(path string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "--short", "HEAD")
	cmd.Dir = path
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// createContextEnv creates an empty context.env file if it doesn't exist
func createContextEnv(path string) error {
	// Check if file already exists
	if _, err := os.Stat(path); err == nil {
		return nil // File exists, don't overwrite
	}

	// Create new file with header
	content := `# Context variables for LLM prompt execution
# Auto-generated by init-temp
# Use 'llm-support context set --dir <dir> KEY VALUE' to add values
`
	return os.WriteFile(path, []byte(content), 0644)
}

func countFilesRecursive(dir string) int {
	count := 0
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			count++
		}
		return nil
	})
	return count
}

func cleanDirectory(dir string) int {
	count := 0
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}

	for _, entry := range entries {
		path := filepath.Join(dir, entry.Name())
		if entry.IsDir() {
			count += countFilesRecursive(path)
			os.RemoveAll(path)
		} else {
			os.Remove(path)
			count++
		}
	}
	return count
}

func init() {
	RootCmd.AddCommand(newInitTempCmd())
}
