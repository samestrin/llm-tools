package commands

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func runDiffSmellCmd(t *testing.T, args ...string) (*SmellResult, error) {
	t.Helper()
	cmd := newDiffSmellCmd()
	var out, errb bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errb)
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		return nil, err
	}
	var res SmellResult
	if jerr := json.Unmarshal(out.Bytes(), &res); jerr != nil {
		t.Fatalf("decode: %v\nout: %s", jerr, out.String())
	}
	return &res, nil
}

func TestDiffSmellCmd_DiffFile(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "fix.diff")
	rewardHack := "diff --git a/foo_test.go b/foo_test.go\n--- a/foo_test.go\n+++ b/foo_test.go\n@@ -1,2 +1,3 @@\n package main\n+// relax\n"
	if err := os.WriteFile(p, []byte(rewardHack), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := runDiffSmellCmd(t, "--diff", p, "--json")
	if err != nil {
		t.Fatal(err)
	}
	if res.Summary.Verdict != "hard" {
		t.Errorf("verdict = %q, want hard", res.Summary.Verdict)
	}
}

func TestDiffSmellCmd_RepoRev(t *testing.T) {
	dir := t.TempDir()
	gitAt(t, dir, "2026-01-01T00:00:00", "init", "-q")
	if err := os.WriteFile(filepath.Join(dir, "svc.go"), []byte("package m\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitAt(t, dir, "2026-01-01T12:00:00", "add", "svc.go")
	gitAt(t, dir, "2026-01-01T12:00:00", "commit", "-q", "-m", "init")
	// A stub-body "fix".
	if err := os.WriteFile(filepath.Join(dir, "svc.go"), []byte("package m\n\nfunc Handle() error {\n\tpanic(\"todo\")\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitAt(t, dir, "2026-02-01T12:00:00", "add", "svc.go")
	gitAt(t, dir, "2026-02-01T12:00:00", "commit", "-q", "-m", "stub fix")

	res, err := runDiffSmellCmd(t, "--repo", dir, "--rev", "HEAD", "--json")
	if err != nil {
		t.Fatal(err)
	}
	if res.Summary.Verdict != "soft_only" {
		t.Errorf("verdict = %q, want soft_only; smells=%+v", res.Summary.Verdict, res.Smells)
	}
	if dsSmell(res, "stub_body") == nil {
		t.Errorf("expected stub_body from the committed fix")
	}
}

// Root commit (no parent) must not crash — git show handles it.
func TestDiffSmellCmd_RootCommit(t *testing.T) {
	dir := t.TempDir()
	gitAt(t, dir, "2026-01-01T00:00:00", "init", "-q")
	if err := os.WriteFile(filepath.Join(dir, "calc.go"), []byte("package m\nfunc Add(a,b int) int { return a+b }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitAt(t, dir, "2026-01-01T12:00:00", "add", "calc.go")
	gitAt(t, dir, "2026-01-01T12:00:00", "commit", "-q", "-m", "root")

	res, err := runDiffSmellCmd(t, "--repo", dir, "--rev", "HEAD", "--json")
	if err != nil {
		t.Fatalf("root commit should not error: %v", err)
	}
	if res.Summary.Verdict != "clean" {
		t.Errorf("verdict = %q, want clean", res.Summary.Verdict)
	}
}

func TestDiffSmellCmd_UnreadableDiff(t *testing.T) {
	cmd := newDiffSmellCmd()
	var out, errb bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errb)
	cmd.SetArgs([]string{"--diff", "/no/such/file.diff", "--json"})
	if err := cmd.Execute(); err == nil {
		t.Error("expected error for unreadable --diff path")
	}
}
