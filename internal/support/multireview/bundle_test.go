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
	// Exercises the full ship pipeline. Now 5 subprocess calls:
	//   1. ssh -- docker exec mkdir (container workdir)
	//   2. ssh -- mkdir (host staging dir)
	//   3. scp bundle to host staging
	//   4. ssh -- docker cp staging/bundle -> container workdir
	//   5. ssh -- docker exec git clone inside container
	repoPath, _, _ := initFixtureRepo(t)

	origExec := execCommandContext
	t.Cleanup(func() { execCommandContext = origExec })

	var calls []string
	execCommandContext = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		calls = append(calls, name+" "+strings.Join(args, " "))
		return exec.CommandContext(ctx, "/bin/sh", "-c", "echo ok; exit 0")
	}

	ctx := context.Background()
	res, err := ShipBundle(ctx, ShipBundleParams{
		LocalRepo:        repoPath,
		Host:             "user@example.lan",
		GatewayContainer: "openclaw-gateway",
		RemoteWorkdir:    "/tmp/reviewer-bench-test",
		HostStagingDir:   "/tmp/reviewer-bench-staging-test",
		RepoName:         "fixture-repo",
		Timeout:          30 * time.Second,
	})
	if err != nil {
		t.Fatalf("ShipBundle: %v", err)
	}
	// RemoteRepoPath must point inside the CONTAINER workdir — reviewers
	// run inside the container and must be able to cd to it.
	if res.RemoteRepoPath != "/tmp/reviewer-bench-test/fixture-repo" {
		t.Errorf("remote path=%q want /tmp/reviewer-bench-test/fixture-repo", res.RemoteRepoPath)
	}
	if res.BundleSize <= 0 {
		t.Errorf("bundle size %d should be > 0", res.BundleSize)
	}
	if len(calls) < 5 {
		t.Fatalf("expected >=5 calls (container-mkdir, host-mkdir, scp, docker-cp, container-clone), got %d: %v", len(calls), calls)
	}

	// 1. First call: ssh wrapping docker exec mkdir for the container workdir.
	if !strings.Contains(calls[0], "ssh") || !strings.Contains(calls[0], "docker exec") || !strings.Contains(calls[0], "base64 -d") {
		t.Errorf("call[0] should be ssh+docker-exec wrapping mkdir, got: %s", calls[0])
	}

	// 2. Second call: ssh mkdir for host staging dir (raw, no docker exec).
	if !strings.Contains(calls[1], "ssh") || strings.Contains(calls[1], "docker exec") || !strings.Contains(calls[1], "mkdir") {
		t.Errorf("call[1] should be raw ssh mkdir for host staging, got: %s", calls[1])
	}
	if !strings.Contains(calls[1], "/tmp/reviewer-bench-staging-test") {
		t.Errorf("call[1] should target host staging dir, got: %s", calls[1])
	}

	// 3. Third call: scp local bundle to host staging.
	if !strings.HasPrefix(calls[2], "scp ") {
		t.Errorf("call[2] should be scp, got: %s", calls[2])
	}
	if !strings.Contains(calls[2], "/tmp/reviewer-bench-staging-test") {
		t.Errorf("call[2] should scp to host staging dir, got: %s", calls[2])
	}

	// 4. Fourth call: ssh wrapping docker cp.
	if !strings.Contains(calls[3], "ssh") || !strings.Contains(calls[3], "docker cp") {
		t.Errorf("call[3] should be ssh+docker cp, got: %s", calls[3])
	}
	if !strings.Contains(calls[3], "openclaw-gateway") {
		t.Errorf("call[3] should mention container name, got: %s", calls[3])
	}

	// 5. Fifth call: ssh wrapping docker exec git clone (base64-encoded).
	if !strings.Contains(calls[4], "ssh") || !strings.Contains(calls[4], "docker exec") || !strings.Contains(calls[4], "base64 -d") {
		t.Errorf("call[4] should be ssh+docker-exec wrapping git clone, got: %s", calls[4])
	}
}

func TestShipBundle_DefaultsContainerName(t *testing.T) {
	repoPath, _, _ := initFixtureRepo(t)

	origExec := execCommandContext
	t.Cleanup(func() { execCommandContext = origExec })

	var calls []string
	execCommandContext = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		calls = append(calls, name+" "+strings.Join(args, " "))
		return exec.CommandContext(ctx, "/bin/sh", "-c", "exit 0")
	}

	_, err := ShipBundle(context.Background(), ShipBundleParams{
		LocalRepo:      repoPath,
		Host:           "user@example.lan",
		RemoteWorkdir:  "/tmp/c-work",
		HostStagingDir: "/tmp/h-stage",
		RepoName:       "r",
		Timeout:        30 * time.Second,
		// GatewayContainer intentionally empty
	})
	if err != nil {
		t.Fatalf("ShipBundle: %v", err)
	}
	// One of the calls must mention 'openclaw-gateway' (single-quoted).
	found := false
	for _, c := range calls {
		if strings.Contains(c, "'openclaw-gateway'") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected default container 'openclaw-gateway' in calls: %v", calls)
	}
}

func TestShipBundle_RequiresInputs(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		name string
		p    ShipBundleParams
	}{
		{"empty repo", ShipBundleParams{Host: "u@h", RemoteWorkdir: "/tmp/x", HostStagingDir: "/tmp/s", RepoName: "r"}},
		{"empty host", ShipBundleParams{LocalRepo: "/tmp/r", RemoteWorkdir: "/tmp/x", HostStagingDir: "/tmp/s", RepoName: "r"}},
		{"empty workdir", ShipBundleParams{LocalRepo: "/tmp/r", Host: "u@h", HostStagingDir: "/tmp/s", RepoName: "r"}},
		{"empty staging", ShipBundleParams{LocalRepo: "/tmp/r", Host: "u@h", RemoteWorkdir: "/tmp/x", RepoName: "r"}},
		{"empty reponame", ShipBundleParams{LocalRepo: "/tmp/r", Host: "u@h", RemoteWorkdir: "/tmp/x", HostStagingDir: "/tmp/s"}},
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

func TestShipBundle_ContainerMkdirFails(t *testing.T) {
	// First call (container mkdir) fails -> hard-stop, no scp attempted.
	repoPath, _, _ := initFixtureRepo(t)

	origExec := execCommandContext
	t.Cleanup(func() { execCommandContext = origExec })

	var calls []string
	callIdx := 0
	execCommandContext = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		callIdx++
		calls = append(calls, name+" "+strings.Join(args, " "))
		if callIdx == 1 {
			return exec.CommandContext(ctx, "/bin/sh", "-c", `echo "Error: No such container: openclaw-gateway" >&2; exit 1`)
		}
		return exec.CommandContext(ctx, "/bin/sh", "-c", "exit 0")
	}

	_, err := ShipBundle(context.Background(), ShipBundleParams{
		LocalRepo:      repoPath,
		Host:           "user@example.lan",
		RemoteWorkdir:  "/tmp/c-work",
		HostStagingDir: "/tmp/h-stage",
		RepoName:       "r",
		Timeout:        10 * time.Second,
	})
	if err == nil {
		t.Fatal("expected error from failing container mkdir")
	}
	// Must NOT have attempted scp after the failure.
	for i, c := range calls {
		if strings.HasPrefix(c, "scp ") {
			t.Errorf("scp should not be attempted after container mkdir failure (call[%d]=%s)", i, c)
		}
	}
}

func TestShipBundle_DockerCpFails(t *testing.T) {
	// The docker cp call (call #4) fails -> error wraps stderr.
	repoPath, _, _ := initFixtureRepo(t)

	origExec := execCommandContext
	t.Cleanup(func() { execCommandContext = origExec })

	callIdx := 0
	execCommandContext = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		callIdx++
		joined := strings.Join(args, " ")
		if strings.Contains(joined, "docker cp") {
			return exec.CommandContext(ctx, "/bin/sh", "-c", `echo "Error response from daemon: no such container" >&2; exit 1`)
		}
		return exec.CommandContext(ctx, "/bin/sh", "-c", "exit 0")
	}

	_, err := ShipBundle(context.Background(), ShipBundleParams{
		LocalRepo:      repoPath,
		Host:           "user@example.lan",
		RemoteWorkdir:  "/tmp/c-work",
		HostStagingDir: "/tmp/h-stage",
		RepoName:       "r",
		Timeout:        10 * time.Second,
	})
	if err == nil {
		t.Fatal("expected error from failing docker cp")
	}
	if !strings.Contains(err.Error(), "docker cp") && !strings.Contains(err.Error(), "no such container") {
		t.Errorf("error should reference docker cp failure: %v", err)
	}
}

func TestShipBundle_ContainerCloneFails(t *testing.T) {
	// Two docker exec calls happen in ShipBundle: the first is the workdir
	// mkdir, the second is the clone. Fail the second.
	repoPath, _, _ := initFixtureRepo(t)

	origExec := execCommandContext
	t.Cleanup(func() { execCommandContext = origExec })

	dockerExecCount := 0
	execCommandContext = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		joined := strings.Join(args, " ")
		if strings.Contains(joined, "docker exec") {
			dockerExecCount++
			if dockerExecCount == 2 {
				return exec.CommandContext(ctx, "/bin/sh", "-c", `echo "fatal: clone destination exists" >&2; exit 128`)
			}
		}
		return exec.CommandContext(ctx, "/bin/sh", "-c", "exit 0")
	}

	_, err := ShipBundle(context.Background(), ShipBundleParams{
		LocalRepo:      repoPath,
		Host:           "user@example.lan",
		RemoteWorkdir:  "/tmp/c-work",
		HostStagingDir: "/tmp/h-stage",
		RepoName:       "r",
		Timeout:        10 * time.Second,
	})
	if err == nil {
		t.Fatal("expected error from failing clone")
	}
	if !strings.Contains(err.Error(), "clone") {
		t.Errorf("error should mention clone: %v", err)
	}
}

func TestShipBundle_ReturnsContainerPath(t *testing.T) {
	// The returned RemoteRepoPath must be a CONTAINER path, never a host
	// staging path. Reviewers run inside the container; if we return a
	// host path the task message will point at something they can't see.
	repoPath, _, _ := initFixtureRepo(t)

	origExec := execCommandContext
	t.Cleanup(func() { execCommandContext = origExec })
	execCommandContext = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "/bin/sh", "-c", "exit 0")
	}

	res, err := ShipBundle(context.Background(), ShipBundleParams{
		LocalRepo:      repoPath,
		Host:           "user@example.lan",
		RemoteWorkdir:  "/tmp/container-work-9999",
		HostStagingDir: "/tmp/host-stage-9999",
		RepoName:       "myrepo",
		Timeout:        10 * time.Second,
	})
	if err != nil {
		t.Fatalf("ShipBundle: %v", err)
	}
	if !strings.HasPrefix(res.RemoteRepoPath, "/tmp/container-work-9999/") {
		t.Errorf("RemoteRepoPath should be inside container workdir, got %q", res.RemoteRepoPath)
	}
	if strings.Contains(res.RemoteRepoPath, "host-stage") {
		t.Errorf("RemoteRepoPath leaked host staging dir: %q", res.RemoteRepoPath)
	}
}
