package multireview

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// initFixtureRepo creates a small git repo with two commits and one tag so
// bundle tests can exercise real `git bundle` against real history.
func initFixtureRepo(t *testing.T) (repoPath, baseRef, headSHA string) {
	t.Helper()
	repoPath = t.TempDir()
	mustRun := func(args ...string) string {
		cmd := exec.Command("git", args...)
		cmd.Dir = repoPath
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	mustRun("init", "-q", "-b", "main")
	mustRun("config", "user.email", "test@example.com")
	mustRun("config", "user.name", "Test")
	mustRun("config", "commit.gpgsign", "false")

	if err := os.WriteFile(filepath.Join(repoPath, "a.txt"), []byte("first\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mustRun("add", "a.txt")
	mustRun("commit", "-q", "-m", "first")
	mustRun("tag", "v0.1.0")
	baseRef = "v0.1.0"

	if err := os.WriteFile(filepath.Join(repoPath, "b.txt"), []byte("second\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mustRun("add", "b.txt")
	mustRun("commit", "-q", "-m", "second")
	headSHA = mustRun("rev-parse", "HEAD")
	return
}

func TestCreateBundle_Success(t *testing.T) {
	repoPath, _, _ := initFixtureRepo(t)
	outBundle := filepath.Join(t.TempDir(), "out.bundle")

	if err := CreateBundle(repoPath, outBundle); err != nil {
		t.Fatalf("CreateBundle: %v", err)
	}
	info, err := os.Stat(outBundle)
	if err != nil {
		t.Fatalf("bundle file: %v", err)
	}
	if info.Size() < 100 {
		t.Errorf("bundle too small: %d bytes", info.Size())
	}
}

func TestCreateBundle_RequiresInputs(t *testing.T) {
	if err := CreateBundle("", "/tmp/x.bundle"); err == nil {
		t.Error("expected error for empty repo")
	}
	if err := CreateBundle("/tmp/repo", ""); err == nil {
		t.Error("expected error for empty bundle path")
	}
}

func TestCreateBundle_InvalidRepo(t *testing.T) {
	notARepo := t.TempDir() // empty dir, no .git
	outBundle := filepath.Join(t.TempDir(), "out.bundle")

	err := CreateBundle(notARepo, outBundle)
	if err == nil {
		t.Fatal("expected error for non-git directory")
	}
}

func TestBundleClonesCleanly(t *testing.T) {
	// End-to-end: create a bundle, clone from it, verify both commits exist
	// and the v0.1.0 tag resolves on the clone.
	repoPath, baseRef, headSHA := initFixtureRepo(t)
	outBundle := filepath.Join(t.TempDir(), "out.bundle")
	if err := CreateBundle(repoPath, outBundle); err != nil {
		t.Fatalf("CreateBundle: %v", err)
	}

	cloneTo := filepath.Join(t.TempDir(), "clone")
	clone := exec.Command("git", "clone", "-q", outBundle, cloneTo)
	if out, err := clone.CombinedOutput(); err != nil {
		t.Fatalf("git clone bundle: %v\n%s", err, out)
	}

	verifyHead := exec.Command("git", "rev-parse", "HEAD")
	verifyHead.Dir = cloneTo
	headOut, err := verifyHead.Output()
	if err != nil {
		t.Fatalf("git rev-parse HEAD: %v", err)
	}
	if got := strings.TrimSpace(string(headOut)); got != headSHA {
		t.Errorf("clone HEAD=%s want %s", got, headSHA)
	}

	verifyTag := exec.Command("git", "rev-parse", baseRef)
	verifyTag.Dir = cloneTo
	if out, err := verifyTag.CombinedOutput(); err != nil {
		t.Fatalf("git rev-parse %s on clone: %v\n%s", baseRef, err, out)
	}
}

func TestShipBundle_Pipeline(t *testing.T) {
	// Exercises the full ship pipeline: scp (mocked) + remote git clone
	// (mocked). Verifies the right SSH commands are issued in order.
	repoPath, _, _ := initFixtureRepo(t)

	origExec := execCommandContext
	t.Cleanup(func() { execCommandContext = origExec })

	var calls []string
	execCommandContext = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		calls = append(calls, name+" "+strings.Join(args, " "))
		// Echo success for any call.
		return exec.CommandContext(ctx, "/bin/sh", "-c", "echo ok; exit 0")
	}

	ctx := context.Background()
	res, err := ShipBundle(ctx, ShipBundleParams{
		LocalRepo:     repoPath,
		Host:          "user@example.lan",
		RemoteWorkdir: "/tmp/reviewer-bench-test",
		RepoName:      "fixture-repo",
		Timeout:       30 * time.Second,
	})
	if err != nil {
		t.Fatalf("ShipBundle: %v", err)
	}
	if res.RemoteRepoPath != "/tmp/reviewer-bench-test/fixture-repo" {
		t.Errorf("remote path=%q", res.RemoteRepoPath)
	}
	if res.BundleSize <= 0 {
		t.Errorf("bundle size %d should be > 0", res.BundleSize)
	}
	// Expect: ssh mkdir, scp, ssh clone — three subprocess calls minimum.
	if len(calls) < 3 {
		t.Errorf("expected >=3 calls, got %d: %v", len(calls), calls)
	}
	// First should be ssh mkdir.
	if !strings.Contains(calls[0], "ssh") || !strings.Contains(calls[0], "mkdir") {
		t.Errorf("first call should be ssh mkdir, got %q", calls[0])
	}
	// One of them should be scp.
	foundScp := false
	for _, c := range calls {
		if strings.HasPrefix(c, "scp ") {
			foundScp = true
			break
		}
	}
	if !foundScp {
		t.Errorf("no scp call observed: %v", calls)
	}
	// Last should be ssh clone.
	last := calls[len(calls)-1]
	if !strings.Contains(last, "ssh") || !strings.Contains(last, "git clone") {
		t.Errorf("last call should be ssh git clone, got %q", last)
	}
}

func TestShipBundle_RequiresInputs(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		name string
		p    ShipBundleParams
	}{
		{"empty repo", ShipBundleParams{Host: "u@h", RemoteWorkdir: "/tmp/x", RepoName: "r"}},
		{"empty host", ShipBundleParams{LocalRepo: "/tmp/r", RemoteWorkdir: "/tmp/x", RepoName: "r"}},
		{"empty workdir", ShipBundleParams{LocalRepo: "/tmp/r", Host: "u@h", RepoName: "r"}},
		{"empty reponame", ShipBundleParams{LocalRepo: "/tmp/r", Host: "u@h", RemoteWorkdir: "/tmp/x"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := ShipBundle(ctx, c.p)
			if err == nil {
				t.Errorf("expected error for %s", c.name)
			}
		})
	}
}

func TestShipBundle_RemoteCloneFailureSurfaces(t *testing.T) {
	repoPath, _, _ := initFixtureRepo(t)

	origExec := execCommandContext
	t.Cleanup(func() { execCommandContext = origExec })

	// Mock fails on the third call (the clone) with a specific error message.
	callIdx := 0
	execCommandContext = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		callIdx++
		joined := strings.Join(args, " ")
		if strings.Contains(joined, "git clone") {
			return exec.CommandContext(ctx, "/bin/sh", "-c", `echo "fatal: bad bundle" >&2; exit 128`)
		}
		return exec.CommandContext(ctx, "/bin/sh", "-c", "exit 0")
	}

	_, err := ShipBundle(context.Background(), ShipBundleParams{
		LocalRepo:     repoPath,
		Host:          "user@example.lan",
		RemoteWorkdir: "/tmp/reviewer-bench-test",
		RepoName:      "fixture",
		Timeout:       10 * time.Second,
	})
	if err == nil {
		t.Fatal("expected error from failing clone")
	}
	if !strings.Contains(err.Error(), "clone") {
		t.Errorf("error should mention clone: %v", err)
	}
}
