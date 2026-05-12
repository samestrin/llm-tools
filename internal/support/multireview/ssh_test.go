package multireview

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// helperExec returns a fake exec.Cmd that runs the given script via /bin/sh -c.
// Used to inject deterministic behavior into ssh tests without real SSH.
func helperExec(script string) func(ctx context.Context, name string, args ...string) *exec.Cmd {
	return func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "/bin/sh", "-c", script)
	}
}

func TestSSHRun_Success(t *testing.T) {
	origExec := execCommandContext
	execCommandContext = helperExec(`echo "hello world"`)
	t.Cleanup(func() { execCommandContext = origExec })

	res, err := SSHRun(context.Background(), SSHParams{
		Host:    "user@example.lan",
		Command: "echo hello world",
		Timeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("SSHRun: %v", err)
	}
	if !strings.Contains(res.Stdout, "hello world") {
		t.Errorf("stdout=%q want hello world", res.Stdout)
	}
	if res.ExitCode != 0 {
		t.Errorf("exit=%d want 0", res.ExitCode)
	}
}

func TestSSHRun_NonZeroExit(t *testing.T) {
	origExec := execCommandContext
	execCommandContext = helperExec(`echo "boom" >&2; exit 42`)
	t.Cleanup(func() { execCommandContext = origExec })

	res, err := SSHRun(context.Background(), SSHParams{
		Host:    "user@example.lan",
		Command: "false",
		Timeout: 5 * time.Second,
	})
	// SSHRun returns the result with non-zero ExitCode but no error for clean exit
	if err != nil {
		t.Fatalf("SSHRun: unexpected error: %v", err)
	}
	if res.ExitCode != 42 {
		t.Errorf("exit=%d want 42", res.ExitCode)
	}
	if !strings.Contains(res.Stderr, "boom") {
		t.Errorf("stderr=%q want boom", res.Stderr)
	}
}

func TestSSHRun_Timeout(t *testing.T) {
	origExec := execCommandContext
	execCommandContext = helperExec(`sleep 10`)
	t.Cleanup(func() { execCommandContext = origExec })

	_, err := SSHRun(context.Background(), SSHParams{
		Host:    "user@example.lan",
		Command: "sleep 10",
		Timeout: 100 * time.Millisecond,
	})
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	if !errors.Is(err, context.DeadlineExceeded) && !strings.Contains(err.Error(), "deadline") && !strings.Contains(err.Error(), "killed") {
		t.Errorf("error %q does not signal timeout", err.Error())
	}
}

func TestSSHRun_RequiresHost(t *testing.T) {
	_, err := SSHRun(context.Background(), SSHParams{
		Host:    "",
		Command: "echo hi",
		Timeout: time.Second,
	})
	if err == nil {
		t.Fatal("expected error for empty host")
	}
}

func TestSSHRun_RequiresCommand(t *testing.T) {
	_, err := SSHRun(context.Background(), SSHParams{
		Host:    "user@example.lan",
		Command: "",
		Timeout: time.Second,
	})
	if err == nil {
		t.Fatal("expected error for empty command")
	}
}

func TestSSHRun_StreamSeparation(t *testing.T) {
	// Stdout and stderr should not be intermixed.
	origExec := execCommandContext
	execCommandContext = helperExec(`echo "OUTPUT"; echo "ERROR" >&2`)
	t.Cleanup(func() { execCommandContext = origExec })

	res, err := SSHRun(context.Background(), SSHParams{
		Host:    "user@example.lan",
		Command: "anything",
		Timeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("SSHRun: %v", err)
	}
	if !strings.Contains(res.Stdout, "OUTPUT") || strings.Contains(res.Stdout, "ERROR") {
		t.Errorf("stdout=%q should have OUTPUT only", res.Stdout)
	}
	if !strings.Contains(res.Stderr, "ERROR") || strings.Contains(res.Stderr, "OUTPUT") {
		t.Errorf("stderr=%q should have ERROR only", res.Stderr)
	}
}

func TestSCPSend_RequiresPaths(t *testing.T) {
	_, err := SCPSend(context.Background(), SCPParams{
		Host:       "user@example.lan",
		LocalPath:  "",
		RemotePath: "/tmp/foo",
		Timeout:    time.Second,
	})
	if err == nil {
		t.Fatal("expected error for empty local path")
	}

	_, err = SCPSend(context.Background(), SCPParams{
		Host:       "user@example.lan",
		LocalPath:  "/tmp/foo",
		RemotePath: "",
		Timeout:    time.Second,
	})
	if err == nil {
		t.Fatal("expected error for empty remote path")
	}
}

func TestSCPSend_Success(t *testing.T) {
	origExec := execCommandContext
	execCommandContext = helperExec(`echo "transferred"`)
	t.Cleanup(func() { execCommandContext = origExec })

	res, err := SCPSend(context.Background(), SCPParams{
		Host:       "user@example.lan",
		LocalPath:  "/tmp/bundle.git",
		RemotePath: "/tmp/remote/bundle.git",
		Timeout:    5 * time.Second,
	})
	if err != nil {
		t.Fatalf("SCPSend: %v", err)
	}
	if res.ExitCode != 0 {
		t.Errorf("exit=%d want 0", res.ExitCode)
	}
}
