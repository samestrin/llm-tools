package gitrange

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// mustGit runs a git command in dir, failing the test on error.
func mustGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func commitFile(t *testing.T, dir, name, content, msg string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	mustGit(t, dir, "add", name)
	mustGit(t, dir, "commit", "-q", "-m", msg)
}

func initRepo(t *testing.T, branch string) string {
	t.Helper()
	dir := t.TempDir()
	mustGit(t, dir, "init", "-q", "-b", branch)
	mustGit(t, dir, "config", "user.email", "test@example.com")
	mustGit(t, dir, "config", "user.name", "Test")
	mustGit(t, dir, "config", "commit.gpgsign", "false")
	return dir
}

// fixtureFeatureBranch: main (2 commits) + feature branch (2 commits), checked
// out on feature.
func fixtureFeatureBranch(t *testing.T) string {
	t.Helper()
	dir := initRepo(t, "main")
	commitFile(t, dir, "a.txt", "one", "c1")
	commitFile(t, dir, "b.txt", "two", "c2")
	mustGit(t, dir, "checkout", "-q", "-b", "feature")
	commitFile(t, dir, "c.txt", "three", "f1")
	commitFile(t, dir, "d.txt", "four", "f2")
	return dir
}

func TestResolve_MergeBase_FeatureBranch(t *testing.T) {
	dir := fixtureFeatureBranch(t)
	wantBase := mustGit(t, dir, "merge-base", "HEAD", "main")
	wantHead := mustGit(t, dir, "rev-parse", "HEAD")

	res, err := Resolve(Params{RepoPath: dir})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if res.Base != wantBase {
		t.Errorf("Base = %s, want %s", res.Base, wantBase)
	}
	if res.Head != wantHead {
		t.Errorf("Head = %s, want %s", res.Head, wantHead)
	}
	if res.Detection != "merge-base" {
		t.Errorf("Detection = %q, want merge-base", res.Detection)
	}
	if res.BaseSymbolic != "main" {
		t.Errorf("BaseSymbolic = %q, want main", res.BaseSymbolic)
	}
	if res.CommitCount != 2 {
		t.Errorf("CommitCount = %d, want 2", res.CommitCount)
	}
	if res.FilesChanged != 2 {
		t.Errorf("FilesChanged = %d, want 2", res.FilesChanged)
	}
	if res.Empty {
		t.Error("Empty = true, want false")
	}
}

func TestResolve_MergeBase_MasterDefault(t *testing.T) {
	dir := initRepo(t, "master")
	commitFile(t, dir, "a.txt", "one", "c1")
	mustGit(t, dir, "checkout", "-q", "-b", "feature")
	commitFile(t, dir, "b.txt", "two", "f1")

	res, err := Resolve(Params{RepoPath: dir})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if res.BaseSymbolic != "master" {
		t.Errorf("BaseSymbolic = %q, want master", res.BaseSymbolic)
	}
	if res.CommitCount != 1 {
		t.Errorf("CommitCount = %d, want 1", res.CommitCount)
	}
}

func TestResolve_MergeBase_OriginHEADNonStandardDefault(t *testing.T) {
	src := initRepo(t, "trunk")
	commitFile(t, src, "a.txt", "one", "c1")

	dst := filepath.Join(t.TempDir(), "clone")
	cmd := exec.Command("git", "clone", "-q", src, dst)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git clone: %v\n%s", err, out)
	}
	mustGit(t, dst, "config", "user.email", "test@example.com")
	mustGit(t, dst, "config", "user.name", "Test")
	mustGit(t, dst, "config", "commit.gpgsign", "false")
	mustGit(t, dst, "checkout", "-q", "-b", "feature")
	commitFile(t, dst, "b.txt", "two", "f1")

	res, err := Resolve(Params{RepoPath: dst})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if res.BaseSymbolic != "origin/trunk" && res.BaseSymbolic != "trunk" {
		t.Errorf("BaseSymbolic = %q, want origin/trunk (or trunk)", res.BaseSymbolic)
	}
	if res.CommitCount != 1 {
		t.Errorf("CommitCount = %d, want 1", res.CommitCount)
	}
}

func TestResolve_NoDefaultBranch(t *testing.T) {
	dir := initRepo(t, "develop")
	commitFile(t, dir, "a.txt", "one", "c1")

	_, err := Resolve(Params{RepoPath: dir})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "--base") {
		t.Errorf("error %q should mention --base", err)
	}
}

func TestResolve_OnDefaultBranch_Empty(t *testing.T) {
	dir := initRepo(t, "main")
	commitFile(t, dir, "a.txt", "one", "c1")
	commitFile(t, dir, "b.txt", "two", "c2")

	res, err := Resolve(Params{RepoPath: dir})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if !res.Empty {
		t.Fatal("Empty = false, want true (HEAD == main)")
	}
	if res.Base != res.Head {
		t.Errorf("Base %s != Head %s, expected identical", res.Base, res.Head)
	}
	if res.Message == "" {
		t.Error("Message should be populated when Empty")
	}
	if !strings.Contains(res.Message, "--merge-commit") {
		t.Errorf("Message %q should steer to --merge-commit", res.Message)
	}
}

func TestResolve_Explicit(t *testing.T) {
	dir := fixtureFeatureBranch(t)

	res, err := Resolve(Params{RepoPath: dir, BaseRef: "main", HeadRef: "feature"})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if res.Detection != "explicit" {
		t.Errorf("Detection = %q, want explicit", res.Detection)
	}
	if res.CommitCount != 2 {
		t.Errorf("CommitCount = %d, want 2", res.CommitCount)
	}
	if res.Base == "" || res.Head == "" || len(res.Base) != 40 || len(res.Head) != 40 {
		t.Errorf("Base/Head should be full SHAs, got %q/%q", res.Base, res.Head)
	}
}

func TestResolve_ExplicitBadRef(t *testing.T) {
	dir := fixtureFeatureBranch(t)

	_, err := Resolve(Params{RepoPath: dir, BaseRef: "does-not-exist"})
	if err == nil {
		t.Fatal("expected error for bad base ref")
	}
}

func TestResolve_MergeCommit_TrueMerge(t *testing.T) {
	dir := fixtureFeatureBranch(t)
	mustGit(t, dir, "checkout", "-q", "main")
	mustGit(t, dir, "merge", "-q", "--no-ff", "feature", "-m", "merge feature")
	sha := mustGit(t, dir, "rev-parse", "HEAD")
	wantBase := mustGit(t, dir, "rev-parse", "HEAD^")

	res, err := Resolve(Params{RepoPath: dir, MergeCommit: sha})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if res.Detection != "merge-commit" {
		t.Errorf("Detection = %q, want merge-commit", res.Detection)
	}
	if res.Base != wantBase {
		t.Errorf("Base = %s, want first parent %s", res.Base, wantBase)
	}
	if res.Head != sha {
		t.Errorf("Head = %s, want %s", res.Head, sha)
	}
	if res.FilesChanged != 2 {
		t.Errorf("FilesChanged = %d, want 2 (c.txt, d.txt)", res.FilesChanged)
	}
	if res.Empty {
		t.Error("Empty = true, want false")
	}
}

func TestResolve_MergeCommit_SquashCommit(t *testing.T) {
	dir := fixtureFeatureBranch(t)
	mustGit(t, dir, "checkout", "-q", "main")
	mustGit(t, dir, "merge", "--squash", "-q", "feature")
	mustGit(t, dir, "commit", "-q", "-m", "squash feature")
	sha := mustGit(t, dir, "rev-parse", "HEAD")
	wantBase := mustGit(t, dir, "rev-parse", "HEAD^")

	res, err := Resolve(Params{RepoPath: dir, MergeCommit: sha})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if res.Base != wantBase {
		t.Errorf("Base = %s, want %s", res.Base, wantBase)
	}
	if res.FilesChanged != 2 {
		t.Errorf("FilesChanged = %d, want 2", res.FilesChanged)
	}
	if res.CommitCount != 1 {
		t.Errorf("CommitCount = %d, want 1", res.CommitCount)
	}
}

func TestResolve_MergeCommit_RootCommit(t *testing.T) {
	dir := initRepo(t, "main")
	commitFile(t, dir, "a.txt", "one", "c1")
	sha := mustGit(t, dir, "rev-parse", "HEAD")

	_, err := Resolve(Params{RepoPath: dir, MergeCommit: sha})
	if err == nil {
		t.Fatal("expected error for root commit")
	}
	if !strings.Contains(err.Error(), "parent") {
		t.Errorf("error %q should mention parent", err)
	}
}

func TestResolve_MergeCommit_MutualExclusion(t *testing.T) {
	dir := fixtureFeatureBranch(t)
	sha := mustGit(t, dir, "rev-parse", "HEAD")

	if _, err := Resolve(Params{RepoPath: dir, MergeCommit: sha, BaseRef: "main"}); err == nil {
		t.Error("expected mutual-exclusion error for merge-commit + base")
	}
	if _, err := Resolve(Params{RepoPath: dir, MergeCommit: sha, HeadRef: "feature"}); err == nil {
		t.Error("expected mutual-exclusion error for merge-commit + head")
	}
}

func TestResolve_DetachedHEAD(t *testing.T) {
	dir := fixtureFeatureBranch(t)
	mustGit(t, dir, "checkout", "-q", "--detach", "HEAD")

	res, err := Resolve(Params{RepoPath: dir})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if res.CommitCount != 2 {
		t.Errorf("CommitCount = %d, want 2", res.CommitCount)
	}
}

func TestResolve_ShallowClone(t *testing.T) {
	src := fixtureFeatureBranch(t)
	mustGit(t, src, "checkout", "-q", "main")

	dst := filepath.Join(t.TempDir(), "shallow")
	cmd := exec.Command("git", "clone", "-q", "--depth", "1", "--branch", "feature", "file://"+src, dst)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git clone --depth 1: %v\n%s", err, out)
	}
	// Fetch a grafted, history-disjoint origin/main so default-branch detection
	// succeeds but merge-base cannot find a common ancestor.
	mustGit(t, dst, "fetch", "-q", "--depth", "1", "origin", "main:refs/remotes/origin/main")
	// Remove origin/HEAD so detection falls through to origin/main.
	cmd = exec.Command("git", "symbolic-ref", "--delete", "refs/remotes/origin/HEAD")
	cmd.Dir = dst
	_ = cmd.Run() // may not exist; ignore

	_, err := Resolve(Params{RepoPath: dst})
	if err == nil {
		t.Fatal("expected error in shallow clone with disjoint history")
	}
	if !strings.Contains(err.Error(), "shallow") {
		t.Errorf("error %q should mention shallow clone", err)
	}
}

func TestResolve_HeadIsTag(t *testing.T) {
	dir := fixtureFeatureBranch(t)
	mustGit(t, dir, "tag", "v1.0.0")
	mustGit(t, dir, "checkout", "-q", "main")

	res, err := Resolve(Params{RepoPath: dir, HeadRef: "v1.0.0"})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if res.CommitCount != 2 {
		t.Errorf("CommitCount = %d, want 2", res.CommitCount)
	}
}

func TestResolve_NotARepo(t *testing.T) {
	_, err := Resolve(Params{RepoPath: t.TempDir()})
	if err == nil {
		t.Fatal("expected error for non-repo")
	}
	if !strings.Contains(err.Error(), "not a git repository") {
		t.Errorf("error %q should say not a git repository", err)
	}
}

func TestResolve_HeadBehindBase(t *testing.T) {
	dir := fixtureFeatureBranch(t)

	res, err := Resolve(Params{RepoPath: dir, BaseRef: "feature", HeadRef: "main"})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if !res.Empty {
		t.Error("Empty = false, want true (head behind base)")
	}
	if res.Message == "" {
		t.Error("Message should be populated when Empty")
	}
}

func TestResolve_DefaultRepoPathAndHead(t *testing.T) {
	dir := fixtureFeatureBranch(t)
	// RepoPath defaults to "." — run from inside the repo.
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })

	res, err := Resolve(Params{})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if res.CommitCount != 2 {
		t.Errorf("CommitCount = %d, want 2", res.CommitCount)
	}
}

func TestDiff(t *testing.T) {
	dir := fixtureFeatureBranch(t)

	diff, err := Diff(dir, "main", "feature")
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if !strings.Contains(diff, "c.txt") || !strings.Contains(diff, "d.txt") {
		t.Errorf("diff should mention c.txt and d.txt:\n%s", diff)
	}

	empty, err := Diff(dir, "main", "main")
	if err != nil {
		t.Fatalf("Diff empty: %v", err)
	}
	if strings.TrimSpace(empty) != "" {
		t.Errorf("expected empty diff, got %q", empty)
	}
}

func TestDiff_BadRef(t *testing.T) {
	dir := fixtureFeatureBranch(t)
	if _, err := Diff(dir, "nope", "feature"); err == nil {
		t.Fatal("expected error for bad ref")
	}
}
