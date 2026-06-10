package commands

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/samestrin/llm-tools/internal/support/gitrange"
)

// initRangeFixtureRepo builds main (2 commits) + feature (2 commits), checked
// out on feature.
func initRangeFixtureRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	mustRun := func(args ...string) {
		c := exec.Command("git", args...)
		c.Dir = dir
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	write := func(name, content string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mustRun("init", "-q", "-b", "main")
	mustRun("config", "user.email", "test@example.com")
	mustRun("config", "user.name", "Test")
	mustRun("config", "commit.gpgsign", "false")
	write("a.txt", "one")
	mustRun("add", "a.txt")
	mustRun("commit", "-q", "-m", "c1")
	write("b.txt", "two")
	mustRun("add", "b.txt")
	mustRun("commit", "-q", "-m", "c2")
	mustRun("checkout", "-q", "-b", "feature")
	write("c.txt", "three")
	mustRun("add", "c.txt")
	mustRun("commit", "-q", "-m", "f1")
	write("d.txt", "four")
	mustRun("add", "d.txt")
	mustRun("commit", "-q", "-m", "f2")
	return dir
}

func gitInDir(t *testing.T, dir string, args ...string) string {
	t.Helper()
	c := exec.Command("git", args...)
	c.Dir = dir
	out, err := c.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func runReviewRangeCmd(t *testing.T, args ...string) (string, error) {
	t.Helper()
	cmd := newReviewRangeCmd()
	cmd.SetArgs(args)
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	err := cmd.Execute()
	return stdout.String(), err
}

func TestReviewRangeCmd_JSON(t *testing.T) {
	dir := initRangeFixtureRepo(t)

	out, err := runReviewRangeCmd(t, "--repo", dir, "--json")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var res gitrange.Result
	if jsonErr := json.Unmarshal([]byte(out), &res); jsonErr != nil {
		t.Fatalf("invalid JSON output %q: %v", out, jsonErr)
	}
	if len(res.Base) != 40 || len(res.Head) != 40 {
		t.Errorf("Base/Head should be full SHAs: %q / %q", res.Base, res.Head)
	}
	if res.Detection != "merge-base" {
		t.Errorf("Detection = %q, want merge-base", res.Detection)
	}
	if res.CommitCount != 2 {
		t.Errorf("CommitCount = %d, want 2", res.CommitCount)
	}
	if res.Empty {
		t.Error("Empty = true, want false")
	}
}

func TestReviewRangeCmd_JSONMin(t *testing.T) {
	dir := initRangeFixtureRepo(t)

	// The MCP handler appends --json --min to every invocation.
	out, err := runReviewRangeCmd(t, "--repo", dir, "--json", "--min")
	if err != nil {
		t.Fatalf("Execute with --json --min: %v", err)
	}
	var res gitrange.Result
	if jsonErr := json.Unmarshal([]byte(out), &res); jsonErr != nil {
		t.Fatalf("invalid JSON output %q: %v", out, jsonErr)
	}
}

func TestReviewRangeCmd_Text(t *testing.T) {
	dir := initRangeFixtureRepo(t)

	out, err := runReviewRangeCmd(t, "--repo", dir)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	for _, want := range []string{"BASE:", "HEAD:", "DETECTION: merge-base", "COMMITS: 2", "EMPTY: FALSE"} {
		if !strings.Contains(out, want) {
			t.Errorf("text output missing %q:\n%s", want, out)
		}
	}
}

func TestReviewRangeCmd_EmptyRange_ExitZero(t *testing.T) {
	dir := initRangeFixtureRepo(t)
	gitInDir(t, dir, "checkout", "-q", "main")

	out, err := runReviewRangeCmd(t, "--repo", dir, "--json")
	if err != nil {
		t.Fatalf("empty range should exit 0, got: %v", err)
	}
	var res gitrange.Result
	if jsonErr := json.Unmarshal([]byte(out), &res); jsonErr != nil {
		t.Fatalf("invalid JSON: %v", jsonErr)
	}
	if !res.Empty {
		t.Error("Empty = false, want true")
	}
	if res.Message == "" {
		t.Error("Message should be populated when empty")
	}
}

func TestReviewRangeCmd_FailOnEmpty(t *testing.T) {
	dir := initRangeFixtureRepo(t)
	gitInDir(t, dir, "checkout", "-q", "main")

	out, err := runReviewRangeCmd(t, "--repo", dir, "--json", "--fail-on-empty")
	if err == nil {
		t.Fatal("expected error with --fail-on-empty on empty range")
	}
	if !strings.Contains(err.Error(), "empty") {
		t.Errorf("error %q should mention empty", err)
	}
	// In JSON mode the result print is suppressed so the root error handler
	// emits exactly one JSON document, not two concatenated ones.
	if strings.TrimSpace(out) != "" {
		t.Errorf("JSON --fail-on-empty should not print a result doc before the error, got %q", out)
	}

	// Text mode still prints the human-readable result before the error.
	outText, errText := runReviewRangeCmd(t, "--repo", dir, "--fail-on-empty")
	if errText == nil {
		t.Fatal("expected error in text mode too")
	}
	if !strings.Contains(outText, "EMPTY: TRUE") {
		t.Errorf("text mode should still print the result, got %q", outText)
	}
}

func TestReviewRangeCmd_MergeCommit(t *testing.T) {
	dir := initRangeFixtureRepo(t)
	gitInDir(t, dir, "checkout", "-q", "main")
	gitInDir(t, dir, "merge", "--squash", "-q", "feature")
	gitInDir(t, dir, "commit", "-q", "-m", "squash feature")
	sha := gitInDir(t, dir, "rev-parse", "HEAD")
	wantBase := gitInDir(t, dir, "rev-parse", "HEAD^")

	out, err := runReviewRangeCmd(t, "--repo", dir, "--merge-commit", sha, "--json")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	var res gitrange.Result
	if jsonErr := json.Unmarshal([]byte(out), &res); jsonErr != nil {
		t.Fatalf("invalid JSON: %v", jsonErr)
	}
	if res.Base != wantBase || res.Head != sha {
		t.Errorf("range = %s..%s, want %s..%s", res.Base, res.Head, wantBase, sha)
	}
	if res.Detection != "merge-commit" {
		t.Errorf("Detection = %q, want merge-commit", res.Detection)
	}
}

func TestReviewRangeCmd_ExplicitRefs(t *testing.T) {
	dir := initRangeFixtureRepo(t)

	out, err := runReviewRangeCmd(t, "--repo", dir, "--base", "main", "--head", "feature", "--json")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	var res gitrange.Result
	if jsonErr := json.Unmarshal([]byte(out), &res); jsonErr != nil {
		t.Fatalf("invalid JSON: %v", jsonErr)
	}
	if res.Detection != "explicit" {
		t.Errorf("Detection = %q, want explicit", res.Detection)
	}
}

func TestReviewRangeCmd_NotARepo(t *testing.T) {
	_, err := runReviewRangeCmd(t, "--repo", t.TempDir(), "--json")
	if err == nil {
		t.Fatal("expected error for non-repo")
	}
}
