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

func TestCleanTempCommand(t *testing.T) {
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)

	repoRoot, err := getRepoRoot(origDir)
	if err != nil {
		t.Skip("skipping test: not in a git repository")
	}
	os.Chdir(repoRoot)

	baseTemp := filepath.Join(repoRoot, ".planning", ".temp")
	testPrefix := "clean-temp-test-" + time.Now().Format("20060102150405")

	t.Run("remove specific directory", func(t *testing.T) {
		// Reset flags
		cleanTempName = ""
		cleanTempAll = false
		cleanTempOlderThan = ""
		cleanTempDryRun = false
		cleanTempJSON = false
		cleanTempMinimal = false

		// Create test directory
		testDir := filepath.Join(baseTemp, testPrefix+"-specific")
		os.MkdirAll(testDir, 0755)
		os.WriteFile(filepath.Join(testDir, "test.txt"), []byte("test"), 0644)

		cmd := newCleanTempCmd()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetArgs([]string{"--name", testPrefix + "-specific"})

		err := cmd.Execute()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		output := buf.String()
		if !strings.Contains(output, "STATUS=REMOVED") {
			t.Errorf("expected STATUS=REMOVED, got: %s", output)
		}

		// Verify directory was removed
		if _, err := os.Stat(testDir); !os.IsNotExist(err) {
			t.Error("directory should have been removed")
			os.RemoveAll(testDir) // Cleanup
		}
	})

	t.Run("dry run", func(t *testing.T) {
		// Reset flags
		cleanTempName = ""
		cleanTempAll = false
		cleanTempOlderThan = ""
		cleanTempDryRun = false
		cleanTempJSON = false
		cleanTempMinimal = false

		// Create test directory
		testDir := filepath.Join(baseTemp, testPrefix+"-dryrun")
		os.MkdirAll(testDir, 0755)

		cmd := newCleanTempCmd()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetArgs([]string{"--name", testPrefix + "-dryrun", "--dry-run"})

		err := cmd.Execute()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		output := buf.String()
		if !strings.Contains(output, "STATUS=DRY_RUN") {
			t.Errorf("expected STATUS=DRY_RUN, got: %s", output)
		}

		// Verify directory was NOT removed
		if _, err := os.Stat(testDir); os.IsNotExist(err) {
			t.Error("directory should NOT have been removed in dry run")
		}

		// Cleanup
		os.RemoveAll(testDir)
	})

	t.Run("remove all", func(t *testing.T) {
		// Reset flags
		cleanTempName = ""
		cleanTempAll = false
		cleanTempOlderThan = ""
		cleanTempDryRun = false
		cleanTempJSON = false
		cleanTempMinimal = false

		// Create multiple test directories
		testDir1 := filepath.Join(baseTemp, testPrefix+"-all1")
		testDir2 := filepath.Join(baseTemp, testPrefix+"-all2")
		os.MkdirAll(testDir1, 0755)
		os.MkdirAll(testDir2, 0755)

		cmd := newCleanTempCmd()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetArgs([]string{"--all"})

		err := cmd.Execute()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		output := buf.String()
		if !strings.Contains(output, "STATUS=REMOVED") {
			t.Errorf("expected STATUS=REMOVED, got: %s", output)
		}

		// Verify directories were removed
		if _, err := os.Stat(testDir1); !os.IsNotExist(err) {
			t.Error("testDir1 should have been removed")
			os.RemoveAll(testDir1)
		}
		if _, err := os.Stat(testDir2); !os.IsNotExist(err) {
			t.Error("testDir2 should have been removed")
			os.RemoveAll(testDir2)
		}
	})

	t.Run("json output", func(t *testing.T) {
		// Reset flags
		cleanTempName = ""
		cleanTempAll = false
		cleanTempOlderThan = ""
		cleanTempDryRun = false
		cleanTempJSON = false
		cleanTempMinimal = false

		// Create test directory
		testDir := filepath.Join(baseTemp, testPrefix+"-json")
		os.MkdirAll(testDir, 0755)

		cmd := newCleanTempCmd()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetArgs([]string{"--name", testPrefix + "-json", "--json"})

		err := cmd.Execute()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var result CleanTempResult
		if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
			t.Fatalf("failed to parse JSON: %v", err)
		}

		if result.Status != "REMOVED" {
			t.Errorf("expected status REMOVED, got %s", result.Status)
		}
		if result.RemovedCount != 1 {
			t.Errorf("expected removed_count 1, got %d", result.RemovedCount)
		}
	})

	t.Run("minimal output", func(t *testing.T) {
		// Reset flags
		cleanTempName = ""
		cleanTempAll = false
		cleanTempOlderThan = ""
		cleanTempDryRun = false
		cleanTempJSON = false
		cleanTempMinimal = false

		// Create test directory
		testDir := filepath.Join(baseTemp, testPrefix+"-min")
		os.MkdirAll(testDir, 0755)

		cmd := newCleanTempCmd()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetArgs([]string{"--name", testPrefix + "-min", "--min", "--json"})

		err := cmd.Execute()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var result CleanTempResult
		if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
			t.Fatalf("failed to parse JSON: %v", err)
		}

		if result.S != "REMOVED" {
			t.Errorf("expected s=REMOVED, got %s", result.S)
		}
		if result.RC == nil || *result.RC != 1 {
			t.Error("expected rc=1")
		}
	})

	t.Run("older than duration", func(t *testing.T) {
		// Reset flags
		cleanTempName = ""
		cleanTempAll = false
		cleanTempOlderThan = ""
		cleanTempDryRun = false
		cleanTempJSON = false
		cleanTempMinimal = false

		// Create test directories - one old, one new
		oldDir := filepath.Join(baseTemp, testPrefix+"-old")
		newDir := filepath.Join(baseTemp, testPrefix+"-new")
		os.MkdirAll(oldDir, 0755)
		os.MkdirAll(newDir, 0755)

		// Make old dir actually old by setting mod time
		oldTime := time.Now().Add(-48 * time.Hour)
		os.Chtimes(oldDir, oldTime, oldTime)

		cmd := newCleanTempCmd()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetArgs([]string{"--older-than", "24h"})

		err := cmd.Execute()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		output := buf.String()
		if !strings.Contains(output, testPrefix+"-old") {
			t.Errorf("expected old dir in output, got: %s", output)
		}

		// Verify old dir was removed, new dir kept
		if _, err := os.Stat(oldDir); !os.IsNotExist(err) {
			t.Error("old directory should have been removed")
			os.RemoveAll(oldDir)
		}
		if _, err := os.Stat(newDir); os.IsNotExist(err) {
			t.Error("new directory should NOT have been removed")
		}

		// Cleanup
		os.RemoveAll(newDir)
	})

	t.Run("invalid duration", func(t *testing.T) {
		// Reset flags
		cleanTempName = ""
		cleanTempAll = false
		cleanTempOlderThan = ""
		cleanTempDryRun = false
		cleanTempJSON = false
		cleanTempMinimal = false

		cmd := newCleanTempCmd()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetErr(buf)
		cmd.SetArgs([]string{"--older-than", "invalid"})

		err := cmd.Execute()
		if err == nil {
			t.Error("expected error for invalid duration")
		}
	})

	t.Run("error without flags", func(t *testing.T) {
		// Reset flags
		cleanTempName = ""
		cleanTempAll = false
		cleanTempOlderThan = ""
		cleanTempDryRun = false
		cleanTempJSON = false
		cleanTempMinimal = false

		cmd := newCleanTempCmd()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetErr(buf)
		cmd.SetArgs([]string{})

		err := cmd.Execute()
		if err == nil {
			t.Error("expected error when no flags provided")
		}
	})

	t.Run("error with conflicting flags", func(t *testing.T) {
		// Reset flags
		cleanTempName = ""
		cleanTempAll = false
		cleanTempOlderThan = ""
		cleanTempDryRun = false
		cleanTempJSON = false
		cleanTempMinimal = false

		cmd := newCleanTempCmd()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetErr(buf)
		cmd.SetArgs([]string{"--name", "foo", "--all"})

		err := cmd.Execute()
		if err == nil {
			t.Error("expected error with conflicting flags")
		}
	})

	t.Run("nonexistent directory", func(t *testing.T) {
		// Reset flags
		cleanTempName = ""
		cleanTempAll = false
		cleanTempOlderThan = ""
		cleanTempDryRun = false
		cleanTempJSON = false
		cleanTempMinimal = false

		cmd := newCleanTempCmd()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetErr(buf)
		cmd.SetArgs([]string{"--name", "nonexistent-dir-12345"})

		err := cmd.Execute()
		if err == nil {
			t.Error("expected error for nonexistent directory")
		}
	})
}

func TestParseDuration(t *testing.T) {
	tests := []struct {
		input    string
		expected time.Duration
		wantErr  bool
	}{
		{"7d", 7 * 24 * time.Hour, false},
		{"1d", 24 * time.Hour, false},
		{"24h", 24 * time.Hour, false},
		{"1h30m", 90 * time.Minute, false},
		{"30m", 30 * time.Minute, false},
		{"invalid", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := parseDuration(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error for %q", tt.input)
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error for %q: %v", tt.input, err)
				return
			}
			if got != tt.expected {
				t.Errorf("parseDuration(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}
