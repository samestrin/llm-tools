package commands

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestGitContextBasic(t *testing.T) {
	// Skip if git not available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	// Create temp git repo
	tmpDir := t.TempDir()
	initGitRepo(t, tmpDir)

	cmd := newGitContextCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--path", tmpDir})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "REPOSITORY:") {
		t.Error("expected REPOSITORY in output")
	}
	if !strings.Contains(output, "BRANCH:") {
		t.Error("expected BRANCH in output")
	}
	if !strings.Contains(output, "STATUS:") {
		t.Error("expected STATUS in output")
	}
}

func TestGitContextJSON(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	tmpDir := t.TempDir()
	initGitRepo(t, tmpDir)

	cmd := newGitContextCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--path", tmpDir, "--json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var ctx GitContext
	if err := json.Unmarshal(buf.Bytes(), &ctx); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if ctx.Repository.Root == "" {
		t.Error("expected repository root in JSON output")
	}
	if ctx.Branch.Current == "" {
		t.Error("expected branch in JSON output")
	}
}

func TestGitContextWithChanges(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	tmpDir := t.TempDir()
	initGitRepo(t, tmpDir)

	// Create uncommitted file
	testFile := filepath.Join(tmpDir, "new-file.txt")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatal(err)
	}

	cmd := newGitContextCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--path", tmpDir, "--json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var ctx GitContext
	if err := json.Unmarshal(buf.Bytes(), &ctx); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if ctx.Status.Clean {
		t.Error("expected status to be dirty with new file")
	}
	if ctx.Status.Untracked != 1 {
		t.Errorf("expected 1 untracked file, got: %d", ctx.Status.Untracked)
	}
}

func TestGitContextNotARepo(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	tmpDir := t.TempDir()

	cmd := newGitContextCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--path", tmpDir})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for non-git directory")
	}
	if !strings.Contains(err.Error(), "not a git repository") {
		t.Errorf("expected 'not a git repository' error, got: %v", err)
	}
}

func TestGitContextInvalidDate(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	tmpDir := t.TempDir()
	initGitRepo(t, tmpDir)

	cmd := newGitContextCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--path", tmpDir, "--since", "invalid-date"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for invalid date format")
	}
	if !strings.Contains(err.Error(), "invalid date format") {
		t.Errorf("expected 'invalid date format' error, got: %v", err)
	}
}

func TestGitContextMaxCommits(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	tmpDir := t.TempDir()
	initGitRepoWithCommits(t, tmpDir, 5)

	cmd := newGitContextCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--path", tmpDir, "--max-commits", "3", "--json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var ctx GitContext
	if err := json.Unmarshal(buf.Bytes(), &ctx); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if len(ctx.Commits) > 3 {
		t.Errorf("expected max 3 commits, got: %d", len(ctx.Commits))
	}
}

func TestGitContextIncludeDiff(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	tmpDir := t.TempDir()
	initGitRepo(t, tmpDir)

	// Create and modify a tracked file
	testFile := filepath.Join(tmpDir, "tracked.txt")
	if err := os.WriteFile(testFile, []byte("initial"), 0644); err != nil {
		t.Fatal(err)
	}
	runGitCmd(t, tmpDir, "add", "tracked.txt")
	runGitCmd(t, tmpDir, "commit", "-m", "add tracked file")

	// Modify the file
	if err := os.WriteFile(testFile, []byte("modified"), 0644); err != nil {
		t.Fatal(err)
	}

	cmd := newGitContextCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--path", tmpDir, "--include-diff", "--json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var ctx GitContext
	if err := json.Unmarshal(buf.Bytes(), &ctx); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if ctx.Diff == "" {
		t.Error("expected diff in output with --include-diff")
	}
}

func TestGitStatusParsing(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(t *testing.T, dir string)
		expected GitStatus
	}{
		{
			name:  "clean repo",
			setup: func(t *testing.T, dir string) {},
			expected: GitStatus{
				Clean: true,
			},
		},
		{
			name: "modified file",
			setup: func(t *testing.T, dir string) {
				// Create and commit a file
				f := filepath.Join(dir, "test.txt")
				os.WriteFile(f, []byte("initial"), 0644)
				runGitCmd(t, dir, "add", "test.txt")
				runGitCmd(t, dir, "commit", "-m", "add test")
				// Modify it
				os.WriteFile(f, []byte("modified"), 0644)
			},
			expected: GitStatus{
				Clean:    false,
				Modified: 1,
			},
		},
		{
			name: "untracked file",
			setup: func(t *testing.T, dir string) {
				f := filepath.Join(dir, "untracked.txt")
				os.WriteFile(f, []byte("content"), 0644)
			},
			expected: GitStatus{
				Clean:     false,
				Untracked: 1,
			},
		},
	}

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			initGitRepo(t, tmpDir)
			tt.setup(t, tmpDir)

			status := getGitStatus(tmpDir)

			if status.Clean != tt.expected.Clean {
				t.Errorf("expected clean=%v, got clean=%v", tt.expected.Clean, status.Clean)
			}
			if status.Modified != tt.expected.Modified {
				t.Errorf("expected modified=%d, got modified=%d", tt.expected.Modified, status.Modified)
			}
			if status.Untracked != tt.expected.Untracked {
				t.Errorf("expected untracked=%d, got untracked=%d", tt.expected.Untracked, status.Untracked)
			}
		})
	}
}

// Helper functions

func initGitRepo(t *testing.T, dir string) {
	t.Helper()
	runGitCmd(t, dir, "init")
	runGitCmd(t, dir, "config", "user.email", "test@example.com")
	runGitCmd(t, dir, "config", "user.name", "Test User")

	// Create initial commit
	f := filepath.Join(dir, "README.md")
	os.WriteFile(f, []byte("# Test"), 0644)
	runGitCmd(t, dir, "add", "README.md")
	runGitCmd(t, dir, "commit", "-m", "Initial commit")
}

func initGitRepoWithCommits(t *testing.T, dir string, numCommits int) {
	t.Helper()
	runGitCmd(t, dir, "init")
	runGitCmd(t, dir, "config", "user.email", "test@example.com")
	runGitCmd(t, dir, "config", "user.name", "Test User")

	for i := 1; i <= numCommits; i++ {
		f := filepath.Join(dir, "file"+string(rune('0'+i))+".txt")
		os.WriteFile(f, []byte("content"), 0644)
		runGitCmd(t, dir, "add", ".")
		runGitCmd(t, dir, "commit", "-m", "Commit "+string(rune('0'+i)))
	}
}

func runGitCmd(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_DATE=2025-12-24T12:00:00", "GIT_COMMITTER_DATE=2025-12-24T12:00:00")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, output)
	}
}

// TestGitContextMinimalOutput tests minimal output mode
func TestGitContextMinimalOutput(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	tmpDir := t.TempDir()
	initGitRepo(t, tmpDir)

	cmd := newGitContextCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--path", tmpDir, "--min"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	// Minimal output should still have key information
	if !strings.Contains(output, "BRANCH:") {
		t.Errorf("minimal output should contain BRANCH:")
	}
}

// TestGitContextWithStagedChanges tests staged changes detection
func TestGitContextWithStagedChanges(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	tmpDir := t.TempDir()
	initGitRepo(t, tmpDir)

	// Create and stage a file
	testFile := filepath.Join(tmpDir, "staged.txt")
	os.WriteFile(testFile, []byte("staged content"), 0644)
	runGitCmd(t, tmpDir, "add", "staged.txt")

	cmd := newGitContextCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--path", tmpDir, "--json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var ctx GitContext
	if err := json.Unmarshal(buf.Bytes(), &ctx); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if ctx.Status.Clean {
		t.Error("expected status to be dirty with staged file")
	}
	// Added files are detected (new staged files)
	if ctx.Status.Added != 1 {
		t.Errorf("expected 1 added file, got: %d", ctx.Status.Added)
	}
}

// TestGitContextWithDeletedFile tests deleted file detection
func TestGitContextWithDeletedFile(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	tmpDir := t.TempDir()
	initGitRepo(t, tmpDir)

	// Create, commit, and delete a file
	testFile := filepath.Join(tmpDir, "to_delete.txt")
	os.WriteFile(testFile, []byte("will be deleted"), 0644)
	runGitCmd(t, tmpDir, "add", "to_delete.txt")
	runGitCmd(t, tmpDir, "commit", "-m", "add file to delete")
	os.Remove(testFile)

	cmd := newGitContextCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--path", tmpDir, "--json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var ctx GitContext
	if err := json.Unmarshal(buf.Bytes(), &ctx); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if ctx.Status.Clean {
		t.Error("expected status to be dirty with deleted file")
	}
	if ctx.Status.Deleted != 1 {
		t.Errorf("expected 1 deleted file, got: %d", ctx.Status.Deleted)
	}
}

// TestGitContextSinceDate tests --since flag with valid date
func TestGitContextSinceDate(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	tmpDir := t.TempDir()
	initGitRepoWithCommits(t, tmpDir, 3)

	cmd := newGitContextCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	// Use a date in the past to get all commits
	cmd.SetArgs([]string{"--path", tmpDir, "--since", "2020-01-01", "--json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var ctx GitContext
	if err := json.Unmarshal(buf.Bytes(), &ctx); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	// Should have commits since the date
	if len(ctx.Commits) == 0 {
		t.Error("expected commits since 2020-01-01")
	}
}
