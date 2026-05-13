package multireview

import (
	"context"
	"os/exec"
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

func TestPreComputeDiff_HeadRefDefaultsToHEAD(t *testing.T) {
	// When HeadRef is empty, it should default to "HEAD" rather than passing
	// an empty string to git.
	origExec := execCommandContext
	t.Cleanup(func() { execCommandContext = origExec })
	var capturedCmd string
	execCommandContext = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		// Capture the remote command we sent (last arg) so we can assert HEAD.
		if len(args) > 0 {
			capturedCmd = args[len(args)-1]
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
	// The refs are shell-quoted, so look for the HEAD token in context:
	// `'v0'..'HEAD'` should appear in the command.
	if !strings.Contains(capturedCmd, "'HEAD'") {
		t.Errorf("expected remote command to default HeadRef to HEAD (look for 'HEAD' in quoted form), got: %s", capturedCmd)
	}
}
