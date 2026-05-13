package multireview

import (
	"context"
	"encoding/base64"
	"os/exec"
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestPreComputeDiff_Success(t *testing.T) {
	// Mock the SSH call to return a clean diff output + wc results.
	// The actual command we run is one composite shell pipeline; we mock the
	// wrapping ssh call and return canned stdout that matches the format the
	// production code expects.
	origExec := execCommandContext
	t.Cleanup(func() { execCommandContext = origExec })
	execCommandContext = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		// Production runs: `wc -c <diff> ; wc -l <diff>` separated by newlines.
		// First line: byte count + path. Second line: line count + path.
		return exec.CommandContext(ctx, "/bin/sh", "-c",
			`echo "  12345 /tmp/workdir/diff.txt"; echo "    250 /tmp/workdir/diff.txt"`)
	}

	res, err := PreComputeDiff(context.Background(), PreComputeDiffParams{
		Host:           "user@example.lan",
		RemoteRepoPath: "/tmp/workdir/myrepo",
		RemoteWorkdir:  "/tmp/workdir",
		BaseRef:        "v1.0.0",
		HeadRef:        "HEAD",
		Timeout:        30 * time.Second,
	})
	if err != nil {
		t.Fatalf("PreComputeDiff: %v", err)
	}
	if res.DiffPath != "/tmp/workdir/diff.txt" {
		t.Errorf("DiffPath=%q want /tmp/workdir/diff.txt", res.DiffPath)
	}
	if res.SizeBytes != 12345 {
		t.Errorf("SizeBytes=%d want 12345", res.SizeBytes)
	}
	if res.LineCount != 250 {
		t.Errorf("LineCount=%d want 250", res.LineCount)
	}
	if res.Empty {
		t.Error("Empty should be false for non-zero diff")
	}
}

func TestPreComputeDiff_RequiresInputs(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		name string
		p    PreComputeDiffParams
	}{
		{"empty host", PreComputeDiffParams{RemoteRepoPath: "/r", RemoteWorkdir: "/w", BaseRef: "v1", HeadRef: "HEAD"}},
		{"empty repo path", PreComputeDiffParams{Host: "u@h", RemoteWorkdir: "/w", BaseRef: "v1", HeadRef: "HEAD"}},
		{"empty workdir", PreComputeDiffParams{Host: "u@h", RemoteRepoPath: "/r", BaseRef: "v1", HeadRef: "HEAD"}},
		{"empty base ref", PreComputeDiffParams{Host: "u@h", RemoteRepoPath: "/r", RemoteWorkdir: "/w", HeadRef: "HEAD"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := PreComputeDiff(ctx, c.p)
			if err == nil {
				t.Errorf("expected error for %s", c.name)
			}
		})
	}
}

func TestPreComputeDiff_RefNotFound(t *testing.T) {
	origExec := execCommandContext
	t.Cleanup(func() { execCommandContext = origExec })
	execCommandContext = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "/bin/sh", "-c",
			`echo "fatal: bad revision 'badref'" >&2; exit 128`)
	}

	_, err := PreComputeDiff(context.Background(), PreComputeDiffParams{
		Host:           "user@example.lan",
		RemoteRepoPath: "/tmp/w/repo",
		RemoteWorkdir:  "/tmp/w",
		BaseRef:        "badref",
		HeadRef:        "HEAD",
		Timeout:        10 * time.Second,
	})
	if err == nil {
		t.Fatal("expected error for bad ref")
	}
	if !strings.Contains(err.Error(), "bad revision") && !strings.Contains(err.Error(), "exit") {
		t.Errorf("error should reference git failure: %v", err)
	}
}

func TestPreComputeDiff_EmptyDiff(t *testing.T) {
	// git diff produces an empty file when base==head. wc -c returns 0.
	origExec := execCommandContext
	t.Cleanup(func() { execCommandContext = origExec })
	execCommandContext = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "/bin/sh", "-c",
			`echo "0 /tmp/w/diff.txt"; echo "0 /tmp/w/diff.txt"`)
	}

	res, err := PreComputeDiff(context.Background(), PreComputeDiffParams{
		Host:           "user@example.lan",
		RemoteRepoPath: "/tmp/w/repo",
		RemoteWorkdir:  "/tmp/w",
		BaseRef:        "HEAD",
		HeadRef:        "HEAD",
		Timeout:        10 * time.Second,
	})
	if err != nil {
		t.Fatalf("PreComputeDiff: %v", err)
	}
	if !res.Empty {
		t.Error("Empty should be true for zero-byte diff")
	}
	if res.SizeBytes != 0 {
		t.Errorf("SizeBytes=%d want 0", res.SizeBytes)
	}
}

func TestPreComputeDiff_LargeDiff(t *testing.T) {
	// A 2MB diff has SizeBytes ~ 2000000.
	origExec := execCommandContext
	t.Cleanup(func() { execCommandContext = origExec })
	execCommandContext = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "/bin/sh", "-c",
			`echo "2000000 /tmp/w/diff.txt"; echo "50000 /tmp/w/diff.txt"`)
	}

	res, err := PreComputeDiff(context.Background(), PreComputeDiffParams{
		Host:           "user@example.lan",
		RemoteRepoPath: "/tmp/w/repo",
		RemoteWorkdir:  "/tmp/w",
		BaseRef:        "v0",
		HeadRef:        "HEAD",
		Timeout:        10 * time.Second,
	})
	if err != nil {
		t.Fatalf("PreComputeDiff: %v", err)
	}
	if res.SizeBytes != 2000000 {
		t.Errorf("SizeBytes=%d want 2000000", res.SizeBytes)
	}
	if res.LineCount != 50000 {
		t.Errorf("LineCount=%d want 50000", res.LineCount)
	}
	if res.Empty {
		t.Error("Empty should be false for large diff")
	}
}

func TestPreComputeDiff_WcOutputBSDFormat(t *testing.T) {
	// BSD wc emits leading-space format: "  1234 path"
	origExec := execCommandContext
	t.Cleanup(func() { execCommandContext = origExec })
	execCommandContext = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "/bin/sh", "-c",
			`echo "    1234 /tmp/w/diff.txt"; echo "      50 /tmp/w/diff.txt"`)
	}

	res, err := PreComputeDiff(context.Background(), PreComputeDiffParams{
		Host:           "user@example.lan",
		RemoteRepoPath: "/tmp/w/repo",
		RemoteWorkdir:  "/tmp/w",
		BaseRef:        "v0",
		HeadRef:        "HEAD",
		Timeout:        10 * time.Second,
	})
	if err != nil {
		t.Fatalf("PreComputeDiff: %v", err)
	}
	if res.SizeBytes != 1234 {
		t.Errorf("BSD format: SizeBytes=%d want 1234", res.SizeBytes)
	}
	if res.LineCount != 50 {
		t.Errorf("BSD format: LineCount=%d want 50", res.LineCount)
	}
}

func TestPreComputeDiff_WcOutputGNUFormat(t *testing.T) {
	// GNU wc emits leading-digit format: "1234 path"
	origExec := execCommandContext
	t.Cleanup(func() { execCommandContext = origExec })
	execCommandContext = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "/bin/sh", "-c",
			`echo "1234 /tmp/w/diff.txt"; echo "50 /tmp/w/diff.txt"`)
	}

	res, err := PreComputeDiff(context.Background(), PreComputeDiffParams{
		Host:           "user@example.lan",
		RemoteRepoPath: "/tmp/w/repo",
		RemoteWorkdir:  "/tmp/w",
		BaseRef:        "v0",
		HeadRef:        "HEAD",
		Timeout:        10 * time.Second,
	})
	if err != nil {
		t.Fatalf("PreComputeDiff: %v", err)
	}
	if res.SizeBytes != 1234 {
		t.Errorf("GNU format: SizeBytes=%d want 1234", res.SizeBytes)
	}
	if res.LineCount != 50 {
		t.Errorf("GNU format: LineCount=%d want 50", res.LineCount)
	}
}

func TestPreComputeDiff_Timeout(t *testing.T) {
	origExec := execCommandContext
	t.Cleanup(func() { execCommandContext = origExec })
	execCommandContext = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "/bin/sh", "-c", `sleep 10`)
	}

	_, err := PreComputeDiff(context.Background(), PreComputeDiffParams{
		Host:           "user@example.lan",
		RemoteRepoPath: "/tmp/w/repo",
		RemoteWorkdir:  "/tmp/w",
		BaseRef:        "v0",
		HeadRef:        "HEAD",
		Timeout:        100 * time.Millisecond,
	})
	if err == nil {
		t.Fatal("expected timeout error")
	}
}

// decodeInnerCommand extracts the base64 token from the outer ssh wrapper
// and returns the decoded inner shell command. The wrapper shape is:
//
//	ssh ... -- docker exec '<container>' sh -c "$(echo '<b64>' | base64 -d)"
//
// We pull the first single-quoted base64-looking token after `echo `.
func decodeInnerCommand(t *testing.T, outer string) string {
	t.Helper()
	// Find "echo 'XXXX'" where XXXX is base64.
	re := regexp.MustCompile(`echo '([A-Za-z0-9+/=]+)'`)
	m := re.FindStringSubmatch(outer)
	if m == nil {
		t.Fatalf("could not find base64 token in: %s", outer)
	}
	dec, err := base64.StdEncoding.DecodeString(m[1])
	if err != nil {
		t.Fatalf("base64 decode of %q: %v", m[1], err)
	}
	return string(dec)
}

func TestPreComputeDiff_HeadRefDefaultsToHEAD(t *testing.T) {
	// When HeadRef is empty, it should default to "HEAD" rather than passing
	// an empty string to git.
	origExec := execCommandContext
	t.Cleanup(func() { execCommandContext = origExec })
	var capturedCmd string
	execCommandContext = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		// Capture the outer ssh command so we can decode the inner payload.
		if len(args) > 0 {
			capturedCmd = strings.Join(append([]string{name}, args...), " ")
		}
		return exec.CommandContext(ctx, "/bin/sh", "-c",
			`echo "100 /tmp/w/diff.txt"; echo "5 /tmp/w/diff.txt"`)
	}

	_, err := PreComputeDiff(context.Background(), PreComputeDiffParams{
		Host:           "user@example.lan",
		RemoteRepoPath: "/tmp/w/repo",
		RemoteWorkdir:  "/tmp/w",
		BaseRef:        "v0",
		HeadRef:        "", // explicitly empty
		Timeout:        10 * time.Second,
	})
	if err != nil {
		t.Fatalf("PreComputeDiff: %v", err)
	}
	inner := decodeInnerCommand(t, capturedCmd)
	// The shell-quoted form is `'v0'..'HEAD'`.
	if !strings.Contains(inner, "'HEAD'") {
		t.Errorf("expected decoded inner command to contain 'HEAD' (single-quoted), got: %s", inner)
	}
}

func TestPreComputeDiff_RoutedThroughContainer(t *testing.T) {
	// The diff pipeline must run via `docker exec`, not raw ssh, so the file
	// lands inside the container's overlay /tmp where reviewers can see it.
	origExec := execCommandContext
	t.Cleanup(func() { execCommandContext = origExec })
	var capturedCmd string
	execCommandContext = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		if len(args) > 0 {
			capturedCmd = strings.Join(append([]string{name}, args...), " ")
		}
		return exec.CommandContext(ctx, "/bin/sh", "-c",
			`echo "100 /tmp/w/diff.txt"; echo "5 /tmp/w/diff.txt"`)
	}

	_, err := PreComputeDiff(context.Background(), PreComputeDiffParams{
		Host:             "user@example.lan",
		GatewayContainer: "openclaw-gateway",
		RemoteRepoPath:   "/tmp/w/repo",
		RemoteWorkdir:    "/tmp/w",
		BaseRef:          "v0",
		HeadRef:          "HEAD",
		Timeout:          10 * time.Second,
	})
	if err != nil {
		t.Fatalf("PreComputeDiff: %v", err)
	}
	// Outer must mention docker exec + base64 -d + the container name.
	if !strings.Contains(capturedCmd, "docker exec") {
		t.Errorf("outer command should route through docker exec, got: %s", capturedCmd)
	}
	if !strings.Contains(capturedCmd, "'openclaw-gateway'") {
		t.Errorf("outer command should mention container 'openclaw-gateway', got: %s", capturedCmd)
	}
	if !strings.Contains(capturedCmd, "base64 -d") {
		t.Errorf("outer command should base64-decode inner payload, got: %s", capturedCmd)
	}

	// Inner (decoded) must still be the original git diff > diff.txt && wc pipeline.
	inner := decodeInnerCommand(t, capturedCmd)
	if !strings.Contains(inner, "git -C '/tmp/w/repo' diff") {
		t.Errorf("decoded inner should run git diff with quoted repo path, got: %s", inner)
	}
	if !strings.Contains(inner, "> '/tmp/w/diff.txt'") {
		t.Errorf("decoded inner should redirect to quoted diff.txt, got: %s", inner)
	}
	if !strings.Contains(inner, "wc -c") || !strings.Contains(inner, "wc -l") {
		t.Errorf("decoded inner should include wc -c and wc -l, got: %s", inner)
	}
}

func TestPreComputeDiff_DefaultsContainerName(t *testing.T) {
	// Empty GatewayContainer defaults to openclaw-gateway via ContainerExec.
	origExec := execCommandContext
	t.Cleanup(func() { execCommandContext = origExec })
	var capturedCmd string
	execCommandContext = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		if len(args) > 0 {
			capturedCmd = strings.Join(append([]string{name}, args...), " ")
		}
		return exec.CommandContext(ctx, "/bin/sh", "-c",
			`echo "0 /tmp/w/diff.txt"; echo "0 /tmp/w/diff.txt"`)
	}

	_, err := PreComputeDiff(context.Background(), PreComputeDiffParams{
		Host:           "user@example.lan",
		RemoteRepoPath: "/tmp/w/r",
		RemoteWorkdir:  "/tmp/w",
		BaseRef:        "v0",
		HeadRef:        "HEAD",
		Timeout:        10 * time.Second,
		// GatewayContainer intentionally empty
	})
	if err != nil {
		t.Fatalf("PreComputeDiff: %v", err)
	}
	if !strings.Contains(capturedCmd, "'openclaw-gateway'") {
		t.Errorf("expected default container name in outer command, got: %s", capturedCmd)
	}
}
