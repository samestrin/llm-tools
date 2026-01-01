package commands

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestInitTempCommand(t *testing.T) {
	// These tests need to run in a git repository
	// Use the actual repo we're in for testing

	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)

	// Navigate to repo root for consistent test environment
	repoRoot, err := getRepoRoot(origDir)
	if err != nil {
		t.Skip("skipping test: not in a git repository")
	}
	os.Chdir(repoRoot)

	// Use a unique test directory name to avoid conflicts
	testName := "init-temp-test-" + time.Now().Format("20060102150405")

	t.Run("create new temp dir with all fields", func(t *testing.T) {
		// Reset flags
		initTempName = ""
		initTempPreserve = false
		initTempClean = true
		initTempWithGit = true
		initTempSkipContext = false
		initTempJSON = false
		initTempMinimal = false

		cmd := newInitTempCmd()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetArgs([]string{"--name", testName, "--with-git"})

		err := cmd.Execute()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		output := buf.String()

		// Check core fields
		expectedFields := []string{
			"TEMP_DIR=",
			"REPO_ROOT=",
			"TODAY=",
			"TODAY_LONG=",
			"TIMESTAMP=",
			"EPOCH=",
			"STATUS=CREATED",
			"CONTEXT_FILE=",
			"BRANCH=",
			"COMMIT_SHORT=",
		}

		for _, exp := range expectedFields {
			if !strings.Contains(output, exp) {
				t.Errorf("output should contain %q, got:\n%s", exp, output)
			}
		}

		// Verify context.env was created
		tempDir := filepath.Join(repoRoot, ".planning", ".temp", testName)
		contextFile := filepath.Join(tempDir, "context.env")
		if _, err := os.Stat(contextFile); os.IsNotExist(err) {
			t.Error("context.env file was not created")
		}

		// Cleanup
		os.RemoveAll(tempDir)
	})

	t.Run("skip context.env creation", func(t *testing.T) {
		// Reset flags
		initTempName = ""
		initTempPreserve = false
		initTempClean = true
		initTempWithGit = false
		initTempSkipContext = false
		initTempJSON = false
		initTempMinimal = false

		cmd := newInitTempCmd()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetArgs([]string{"--name", testName + "-skip", "--skip-context"})

		err := cmd.Execute()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		output := buf.String()

		// Should not contain CONTEXT_FILE
		if strings.Contains(output, "CONTEXT_FILE=") {
			t.Error("output should not contain CONTEXT_FILE when --skip-context is used")
		}

		// Verify context.env was NOT created
		tempDir := filepath.Join(repoRoot, ".planning", ".temp", testName+"-skip")
		contextFile := filepath.Join(tempDir, "context.env")
		if _, err := os.Stat(contextFile); !os.IsNotExist(err) {
			t.Error("context.env file should not exist with --skip-context")
		}

		// Cleanup
		os.RemoveAll(tempDir)
	})

	t.Run("json output format", func(t *testing.T) {
		// Reset flags
		initTempName = ""
		initTempPreserve = false
		initTempClean = true
		initTempWithGit = true
		initTempSkipContext = false
		initTempJSON = false
		initTempMinimal = false

		cmd := newInitTempCmd()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetArgs([]string{"--name", testName + "-json", "--with-git", "--json"})

		err := cmd.Execute()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var result InitTempResult
		if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
			t.Fatalf("failed to parse JSON output: %v", err)
		}

		// Check fields are populated
		if result.TempDir == "" {
			t.Error("temp_dir should not be empty")
		}
		if result.RepoRoot == "" {
			t.Error("repo_root should not be empty")
		}
		if result.Today == "" {
			t.Error("today should not be empty")
		}
		if result.TodayLong == "" {
			t.Error("today_long should not be empty")
		}
		if result.Timestamp == "" {
			t.Error("timestamp should not be empty")
		}
		if result.Epoch == 0 {
			t.Error("epoch should not be zero")
		}
		// Branch may be empty in detached HEAD state (e.g., GitHub Actions)
		// CommitShort should always be available though
		if result.CommitShort == "" {
			t.Error("commit_short should not be empty with --with-git")
		}

		// Cleanup
		os.RemoveAll(result.TempDir)
	})

	t.Run("preserve existing files", func(t *testing.T) {
		// Reset flags
		initTempName = ""
		initTempPreserve = false
		initTempClean = true
		initTempWithGit = false
		initTempSkipContext = false
		initTempJSON = false
		initTempMinimal = false

		preserveTestName := testName + "-preserve"
		tempDir := filepath.Join(repoRoot, ".planning", ".temp", preserveTestName)

		// Create dir with a file first
		os.MkdirAll(tempDir, 0755)
		os.WriteFile(filepath.Join(tempDir, "keep.txt"), []byte("keep"), 0644)

		cmd := newInitTempCmd()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetArgs([]string{"--name", preserveTestName, "--preserve"})

		err := cmd.Execute()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		output := buf.String()
		if !strings.Contains(output, "STATUS=EXISTS") {
			t.Error("status should be EXISTS for preserved directory")
		}
		if !strings.Contains(output, "EXISTING_FILES=") {
			t.Error("should report existing files count")
		}

		// Verify file was kept
		if _, err := os.Stat(filepath.Join(tempDir, "keep.txt")); os.IsNotExist(err) {
			t.Error("existing file should be preserved")
		}

		// Cleanup
		os.RemoveAll(tempDir)
	})

	t.Run("clean existing files", func(t *testing.T) {
		// Reset flags
		initTempName = ""
		initTempPreserve = false
		initTempClean = true
		initTempWithGit = false
		initTempSkipContext = false
		initTempJSON = false
		initTempMinimal = false

		cleanTestName := testName + "-clean"
		tempDir := filepath.Join(repoRoot, ".planning", ".temp", cleanTestName)

		// Create dir with a file first
		os.MkdirAll(tempDir, 0755)
		os.WriteFile(filepath.Join(tempDir, "old.txt"), []byte("old"), 0644)

		cmd := newInitTempCmd()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetArgs([]string{"--name", cleanTestName, "--clean"})

		err := cmd.Execute()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		output := buf.String()
		if !strings.Contains(output, "STATUS=CREATED") {
			t.Error("status should be CREATED for cleaned directory")
		}
		if !strings.Contains(output, "CLEANED=") {
			t.Error("should report cleaned files count")
		}

		// Verify old file was removed
		if _, err := os.Stat(filepath.Join(tempDir, "old.txt")); !os.IsNotExist(err) {
			t.Error("old file should be cleaned")
		}

		// Cleanup
		os.RemoveAll(tempDir)
	})

	t.Run("minimal output format", func(t *testing.T) {
		// Reset flags
		initTempName = ""
		initTempPreserve = false
		initTempClean = true
		initTempWithGit = true
		initTempSkipContext = false
		initTempJSON = false
		initTempMinimal = false

		cmd := newInitTempCmd()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetArgs([]string{"--name", testName + "-min", "--with-git", "--min", "--json"})

		err := cmd.Execute()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var result InitTempResult
		if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
			t.Fatalf("failed to parse JSON output: %v", err)
		}

		// Check minimal fields are populated (uses short aliases)
		if result.TD == "" {
			t.Error("td (temp_dir) should not be empty in minimal mode")
		}
		if result.RR == "" {
			t.Error("rr (repo_root) should not be empty in minimal mode")
		}
		if result.TS == "" {
			t.Error("ts (timestamp) should not be empty in minimal mode")
		}
		if result.TSLong == "" {
			t.Error("ts_long (today_long) should not be empty in minimal mode")
		}
		// Branch may be empty in detached HEAD state (e.g., GitHub Actions)
		// CommitShort should always be available though
		if result.CS == "" {
			t.Error("cs (commit_short) should not be empty in minimal mode with --with-git")
		}

		// Cleanup
		os.RemoveAll(result.TD)
	})

	t.Run("clean with subdirectories", func(t *testing.T) {
		// Reset flags
		initTempName = ""
		initTempPreserve = false
		initTempClean = true
		initTempWithGit = false
		initTempSkipContext = false
		initTempJSON = false
		initTempMinimal = false

		subdirTestName := testName + "-subdir"
		tempDir := filepath.Join(repoRoot, ".planning", ".temp", subdirTestName)

		// Create dir with subdirectory containing files
		subDir := filepath.Join(tempDir, "nested", "deep")
		os.MkdirAll(subDir, 0755)
		os.WriteFile(filepath.Join(subDir, "nested.txt"), []byte("nested"), 0644)
		os.WriteFile(filepath.Join(tempDir, "root.txt"), []byte("root"), 0644)

		cmd := newInitTempCmd()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetArgs([]string{"--name", subdirTestName, "--clean"})

		err := cmd.Execute()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		output := buf.String()
		if !strings.Contains(output, "CLEANED=") {
			t.Error("should report cleaned files count")
		}

		// Verify nested files were removed
		if _, err := os.Stat(filepath.Join(subDir, "nested.txt")); !os.IsNotExist(err) {
			t.Error("nested file should be cleaned")
		}

		// Cleanup
		os.RemoveAll(tempDir)
	})

	t.Run("context.env already exists", func(t *testing.T) {
		// Reset flags
		initTempName = ""
		initTempPreserve = false
		initTempClean = true
		initTempWithGit = false
		initTempSkipContext = false
		initTempJSON = false
		initTempMinimal = false

		existsTestName := testName + "-ctx-exists"
		tempDir := filepath.Join(repoRoot, ".planning", ".temp", existsTestName)

		// Create dir with existing context.env
		os.MkdirAll(tempDir, 0755)
		existingContent := "EXISTING=value\n"
		os.WriteFile(filepath.Join(tempDir, "context.env"), []byte(existingContent), 0644)

		cmd := newInitTempCmd()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetArgs([]string{"--name", existsTestName, "--preserve"})

		err := cmd.Execute()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Verify existing context.env was NOT overwritten
		content, err := os.ReadFile(filepath.Join(tempDir, "context.env"))
		if err != nil {
			t.Fatalf("failed to read context.env: %v", err)
		}
		if string(content) != existingContent {
			t.Error("existing context.env should not be overwritten")
		}

		// Cleanup
		os.RemoveAll(tempDir)
	})

	t.Run("preserve with minimal output", func(t *testing.T) {
		// Reset flags
		initTempName = ""
		initTempPreserve = false
		initTempClean = true
		initTempWithGit = false
		initTempSkipContext = false
		initTempJSON = false
		initTempMinimal = false

		preserveMinTestName := testName + "-preserve-min"
		tempDir := filepath.Join(repoRoot, ".planning", ".temp", preserveMinTestName)

		// Create dir with a file first
		os.MkdirAll(tempDir, 0755)
		os.WriteFile(filepath.Join(tempDir, "keep.txt"), []byte("keep"), 0644)

		cmd := newInitTempCmd()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetArgs([]string{"--name", preserveMinTestName, "--preserve", "--min", "--json"})

		err := cmd.Execute()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var result InitTempResult
		if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
			t.Fatalf("failed to parse JSON output: %v", err)
		}

		// Check minimal mode uses EF (existing_files alias)
		if result.EF == nil {
			t.Error("ef (existing_files) should be set in minimal mode with --preserve")
		}
		if result.S != "EXISTS" {
			t.Errorf("s (status) should be EXISTS, got %s", result.S)
		}

		// Cleanup
		os.RemoveAll(tempDir)
	})
}

func TestGetRepoRoot(t *testing.T) {
	cwd, _ := os.Getwd()

	root, err := getRepoRoot(cwd)
	if err != nil {
		t.Skip("skipping: not in a git repository")
	}

	// Should be an absolute path
	if !filepath.IsAbs(root) {
		t.Error("repo root should be an absolute path")
	}

	// Should contain a .git directory
	gitDir := filepath.Join(root, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		t.Error("repo root should contain .git directory")
	}
}

func TestGetGitBranch(t *testing.T) {
	cwd, _ := os.Getwd()
	root, err := getRepoRoot(cwd)
	if err != nil {
		t.Skip("skipping: not in a git repository")
	}

	branch, err := getGitBranch(root)
	if err != nil {
		t.Fatalf("failed to get branch: %v", err)
	}

	// Branch may be empty in detached HEAD state (e.g., GitHub Actions checkout)
	// Just verify the function doesn't error - empty branch is valid
	t.Logf("branch: %q (may be empty in CI)", branch)
}

func TestGetGitCommitShort(t *testing.T) {
	cwd, _ := os.Getwd()
	root, err := getRepoRoot(cwd)
	if err != nil {
		t.Skip("skipping: not in a git repository")
	}

	commit, err := getGitCommitShort(root)
	if err != nil {
		t.Fatalf("failed to get commit: %v", err)
	}

	if commit == "" {
		t.Error("commit should not be empty")
	}

	// Short commit should be 7-8 characters
	if len(commit) < 7 || len(commit) > 8 {
		t.Errorf("short commit should be 7-8 chars, got %d: %s", len(commit), commit)
	}
}
