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

// TestParseGitStatus tests the porcelain format parsing logic
func TestParseGitStatus(t *testing.T) {
	tests := []struct {
		name              string
		gitOutput         string
		includeUntracked  bool
		stagedOnly        bool
		expectedCount     int
		expectedFiles     []string
	}{
		{
			name:             "basic modified files",
			gitOutput:        " M file1.go\n M file2.go\n",
			includeUntracked: true,
			stagedOnly:       false,
			expectedCount:    2,
			expectedFiles:    []string{"file1.go", "file2.go"},
		},
		{
			name:             "include untracked by default",
			gitOutput:        " M file1.go\n?? file2.go\n",
			includeUntracked: true,
			stagedOnly:       false,
			expectedCount:    2,
			expectedFiles:    []string{"file1.go", "file2.go"},
		},
		{
			name:             "exclude untracked",
			gitOutput:        " M file1.go\n?? file2.go\n",
			includeUntracked: false,
			stagedOnly:       false,
			expectedCount:    1,
			expectedFiles:    []string{"file1.go"},
		},
		{
			name:             "staged only mode",
			gitOutput:        "M  file1.go\n M file2.go\n?? file3.go\n",
			includeUntracked: true,
			stagedOnly:       true,
			expectedCount:    1,
			expectedFiles:    []string{"file1.go"},
		},
		{
			name:             "newly added staged file",
			gitOutput:        "A  newfile.go\n",
			includeUntracked: true,
			stagedOnly:       true,
			expectedCount:    1,
			expectedFiles:    []string{"newfile.go"},
		},
		{
			name:             "file with both staged and unstaged changes",
			gitOutput:        "MM file.go\n",
			includeUntracked: true,
			stagedOnly:       true,
			expectedCount:    1,
			expectedFiles:    []string{"file.go"},
		},
		{
			name:             "deleted file",
			gitOutput:        " D deleted.go\nD  staged_delete.go\n",
			includeUntracked: true,
			stagedOnly:       false,
			expectedCount:    2,
			expectedFiles:    []string{"deleted.go", "staged_delete.go"},
		},
		{
			name:             "empty working tree",
			gitOutput:        "",
			includeUntracked: true,
			stagedOnly:       false,
			expectedCount:    0,
			expectedFiles:    []string{},
		},
		{
			name:             "empty lines ignored",
			gitOutput:        " M file1.go\n\n M file2.go\n",
			includeUntracked: true,
			stagedOnly:       false,
			expectedCount:    2,
			expectedFiles:    []string{"file1.go", "file2.go"},
		},
		{
			name:             "renamed file",
			gitOutput:        "R  old.go -> new.go\n",
			includeUntracked: true,
			stagedOnly:       true,
			expectedCount:    1,
			expectedFiles:    []string{"old.go -> new.go"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseGitStatus(tt.gitOutput, "", tt.includeUntracked, tt.stagedOnly)

			if result.Count != tt.expectedCount {
				t.Errorf("expected count %d, got %d", tt.expectedCount, result.Count)
			}

			if len(result.Files) != len(tt.expectedFiles) {
				t.Errorf("expected %d files, got %d: %v", len(tt.expectedFiles), len(result.Files), result.Files)
				return
			}

			for i, expectedFile := range tt.expectedFiles {
				if result.Files[i] != expectedFile {
					t.Errorf("expected file %q at index %d, got %q", expectedFile, i, result.Files[i])
				}
			}
		})
	}
}

// TestPathFiltering tests path prefix filtering
func TestPathFiltering(t *testing.T) {
	tests := []struct {
		name          string
		gitOutput     string
		pathFilter    string
		expectedCount int
		expectedFiles []string
	}{
		{
			name:          "filter planning directory",
			gitOutput:     " M .planning/task.md\n M src/main.go\n M .planning/spec.md\n",
			pathFilter:    ".planning/",
			expectedCount: 2,
			expectedFiles: []string{".planning/task.md", ".planning/spec.md"},
		},
		{
			name:          "no path filter returns all",
			gitOutput:     " M file1.go\n M file2.go\n M file3.go\n",
			pathFilter:    "",
			expectedCount: 3,
			expectedFiles: []string{"file1.go", "file2.go", "file3.go"},
		},
		{
			name:          "path filter with no matches",
			gitOutput:     " M src/main.go\n M docs/readme.md\n",
			pathFilter:    ".planning/",
			expectedCount: 0,
			expectedFiles: []string{},
		},
		{
			name:          "prefix matching without trailing slash",
			gitOutput:     " M .planning/task.md\n M .planning-backup/old.md\n",
			pathFilter:    ".planning",
			expectedCount: 2,
			expectedFiles: []string{".planning/task.md", ".planning-backup/old.md"},
		},
		{
			name:          "path filter with special characters",
			gitOutput:     " M my-project/files_[test]/data.json\n",
			pathFilter:    "my-project/files_[test]/",
			expectedCount: 1,
			expectedFiles: []string{"my-project/files_[test]/data.json"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseGitStatus(tt.gitOutput, tt.pathFilter, true, false)

			if result.Count != tt.expectedCount {
				t.Errorf("expected count %d, got %d", tt.expectedCount, result.Count)
			}

			if len(result.Files) != len(tt.expectedFiles) {
				t.Errorf("expected %d files, got %d: %v", len(tt.expectedFiles), len(result.Files), result.Files)
			}
		})
	}
}

// TestGitChangesOutputModes tests all output mode combinations
func TestGitChangesOutputModes(t *testing.T) {
	// Skip if not in a git repository
	if _, err := exec.Command("git", "rev-parse", "--git-dir").Output(); err != nil {
		t.Skip("not in a git repository, skipping integration test")
	}

	tests := []struct {
		name           string
		jsonFlag       bool
		minFlag        bool
		validateOutput func(t *testing.T, output string)
	}{
		{
			name:     "default text output",
			jsonFlag: false,
			minFlag:  false,
			validateOutput: func(t *testing.T, output string) {
				if !strings.Contains(output, "COUNT:") {
					t.Errorf("expected 'COUNT:' in output, got: %s", output)
				}
				if !strings.Contains(output, "FILES:") {
					t.Errorf("expected 'FILES:' in output, got: %s", output)
				}
			},
		},
		{
			name:     "json output",
			jsonFlag: true,
			minFlag:  false,
			validateOutput: func(t *testing.T, output string) {
				var result GitChangesResult
				if err := json.Unmarshal([]byte(output), &result); err != nil {
					t.Errorf("invalid JSON output: %v", err)
				}
			},
		},
		{
			name:     "minimal output",
			jsonFlag: false,
			minFlag:  true,
			validateOutput: func(t *testing.T, output string) {
				trimmed := strings.TrimSpace(output)
				// Should be just a number
				if _, err := parseInt(trimmed); err != nil && trimmed != "0" {
					// Allow zero or a valid integer
					if len(trimmed) > 0 {
						for _, c := range trimmed {
							if c < '0' || c > '9' {
								t.Errorf("expected numeric output, got: %s", trimmed)
								break
							}
						}
					}
				}
			},
		},
		{
			name:     "json minimal output",
			jsonFlag: true,
			minFlag:  true,
			validateOutput: func(t *testing.T, output string) {
				trimmed := strings.TrimSpace(output)
				if !strings.HasPrefix(trimmed, "{\"count\":") {
					t.Errorf("expected '{\"count\":...' output, got: %s", trimmed)
				}
				// Should not contain "files" key
				if strings.Contains(trimmed, "files") {
					t.Errorf("minimal JSON should not contain files, got: %s", trimmed)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout bytes.Buffer
			cmd := newGitChangesCmd()
			cmd.SetOut(&stdout)

			args := []string{}
			if tt.jsonFlag {
				args = append(args, "--json")
			}
			if tt.minFlag {
				args = append(args, "--min")
			}
			cmd.SetArgs(args)

			err := cmd.Execute()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			tt.validateOutput(t, stdout.String())
		})
	}
}

// TestGitChangesErrorHandling tests error conditions
func TestGitChangesErrorHandling(t *testing.T) {
	t.Run("non-git directory", func(t *testing.T) {
		tmpDir := t.TempDir()

		var stdout, stderr bytes.Buffer
		cmd := newGitChangesCmd()
		cmd.SetOut(&stdout)
		cmd.SetErr(&stderr)

		// Save current dir and change to temp dir
		origDir, _ := os.Getwd()
		os.Chdir(tmpDir)
		defer os.Chdir(origDir)

		// Reset the path flag for this test
		gitChangesPath = ""

		err := cmd.Execute()
		if err == nil {
			t.Error("expected error for non-git directory")
			return
		}

		if !strings.Contains(err.Error(), "not a git repository") && !strings.Contains(strings.ToLower(err.Error()), "git") {
			t.Errorf("expected git-related error, got: %v", err)
		}
	})
}

// TestGitChangesWithRealRepo tests in the actual repository
func TestGitChangesWithRealRepo(t *testing.T) {
	// Skip if not in a git repository
	if _, err := exec.Command("git", "rev-parse", "--git-dir").Output(); err != nil {
		t.Skip("not in a git repository, skipping integration test")
	}

	t.Run("basic execution succeeds", func(t *testing.T) {
		var stdout bytes.Buffer
		cmd := newGitChangesCmd()
		cmd.SetOut(&stdout)
		cmd.SetArgs([]string{"--min"})

		err := cmd.Execute()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Should produce a number
		output := strings.TrimSpace(stdout.String())
		for _, c := range output {
			if c < '0' || c > '9' {
				t.Errorf("expected numeric output, got: %s", output)
				break
			}
		}
	})

	t.Run("path filter execution", func(t *testing.T) {
		var stdout bytes.Buffer
		cmd := newGitChangesCmd()
		cmd.SetOut(&stdout)
		cmd.SetArgs([]string{"--path", ".planning/", "--json"})

		err := cmd.Execute()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var result GitChangesResult
		if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
			t.Errorf("invalid JSON output: %v", err)
		}
	})
}

// TestGitChangesResultStruct tests JSON marshaling of GitChangesResult
func TestGitChangesResultStruct(t *testing.T) {
	tests := []struct {
		name     string
		result   GitChangesResult
		expected string
	}{
		{
			name:     "empty result omits files",
			result:   GitChangesResult{Count: 0, Files: []string{}},
			expected: `{"count":0}`, // omitempty removes empty files array
		},
		{
			name:     "nil files omitted",
			result:   GitChangesResult{Count: 0, Files: nil},
			expected: `{"count":0}`, // omitempty removes nil files
		},
		{
			name:     "single file",
			result:   GitChangesResult{Count: 1, Files: []string{"file.go"}},
			expected: `{"count":1,"files":["file.go"]}`,
		},
		{
			name:     "multiple files",
			result:   GitChangesResult{Count: 2, Files: []string{"a.go", "b.go"}},
			expected: `{"count":2,"files":["a.go","b.go"]}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output, err := json.Marshal(tt.result)
			if err != nil {
				t.Fatalf("failed to marshal: %v", err)
			}

			if string(output) != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, string(output))
			}
		})
	}
}

// TestCombinedFilters tests combining path filter with change type filters
func TestCombinedFilters(t *testing.T) {
	tests := []struct {
		name             string
		gitOutput        string
		pathFilter       string
		includeUntracked bool
		stagedOnly       bool
		expectedCount    int
	}{
		{
			name:             "path filter with staged only",
			gitOutput:        "M  .planning/task.md\n M .planning/spec.md\nM  src/main.go\n",
			pathFilter:       ".planning/",
			includeUntracked: true,
			stagedOnly:       true,
			expectedCount:    1,
		},
		{
			name:             "path filter excluding untracked",
			gitOutput:        " M .planning/task.md\n?? .planning/new.md\n?? src/temp.go\n",
			pathFilter:       ".planning/",
			includeUntracked: false,
			stagedOnly:       false,
			expectedCount:    1,
		},
		{
			name:             "all filters combined",
			gitOutput:        "M  .planning/task.md\n M .planning/spec.md\n?? .planning/new.md\nM  src/main.go\n",
			pathFilter:       ".planning/",
			includeUntracked: false,
			stagedOnly:       true,
			expectedCount:    1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseGitStatus(tt.gitOutput, tt.pathFilter, tt.includeUntracked, tt.stagedOnly)

			if result.Count != tt.expectedCount {
				t.Errorf("expected count %d, got %d", tt.expectedCount, result.Count)
			}
		})
	}
}

// parseInt is a simple helper for testing
func parseInt(s string) (int, error) {
	var result int
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, os.ErrInvalid
		}
		result = result*10 + int(c-'0')
	}
	return result, nil
}

// TestGitChangesMinimalModeCount tests that minimal mode outputs just the count
func TestGitChangesMinimalModeCount(t *testing.T) {
	// Create a mock test using parsed output
	result := parseGitStatus(" M file1.go\n M file2.go\n M file3.go\n", "", true, false)
	if result.Count != 3 {
		t.Errorf("expected count 3, got %d", result.Count)
	}
}

// TestFilesWithSpacesInNames tests handling of files with spaces
func TestFilesWithSpacesInNames(t *testing.T) {
	gitOutput := " M my file.txt\n M another file.md\n"
	result := parseGitStatus(gitOutput, "", true, false)

	if result.Count != 2 {
		t.Errorf("expected count 2, got %d", result.Count)
	}

	if len(result.Files) > 0 && result.Files[0] != "my file.txt" {
		t.Errorf("expected 'my file.txt', got %q", result.Files[0])
	}
}

// Placeholder for testing renaming detection
func TestRenamedFiles(t *testing.T) {
	// Git shows renamed files as "R  old -> new"
	gitOutput := "R  old.go -> new.go\n"
	result := parseGitStatus(gitOutput, "", true, true)

	if result.Count != 1 {
		t.Errorf("expected count 1, got %d", result.Count)
	}
}

// TestEmptyPathFilterEquivalentToNoFilter ensures empty string behaves like no filter
func TestEmptyPathFilterEquivalentToNoFilter(t *testing.T) {
	gitOutput := " M file1.go\n M file2.go\n"

	resultEmpty := parseGitStatus(gitOutput, "", true, false)

	// Empty filter should return all files
	if resultEmpty.Count != 2 {
		t.Errorf("empty filter: expected count 2, got %d", resultEmpty.Count)
	}
}

// TestGitChangesJSONValid tests that JSON output is valid
func TestGitChangesJSONValid(t *testing.T) {
	// Skip if not in a git repository
	if _, err := exec.Command("git", "rev-parse", "--git-dir").Output(); err != nil {
		t.Skip("not in a git repository, skipping integration test")
	}

	var stdout bytes.Buffer
	cmd := newGitChangesCmd()
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"--json"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := stdout.Bytes()
	if !json.Valid(output) {
		t.Errorf("output is not valid JSON: %s", output)
	}
}

// Ensure test file imports are valid
var _ = filepath.Join
