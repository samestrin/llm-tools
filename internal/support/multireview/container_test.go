package multireview

import (
	"context"
	"encoding/base64"
	"errors"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// captureExec returns an execCommandContext stand-in that records the args
// passed to it and returns a scripted /bin/sh fake. The caller reads back the
// captured args to assert on the command the wrapper built.
func captureExec(script string, captured *[]string) func(ctx context.Context, name string, args ...string) *exec.Cmd {
	return func(ctx context.Context, name string, args ...string) *exec.Cmd {
		// Store the full argv so tests can assert the shape of the
		// outer ssh invocation.
		all := append([]string{name}, args...)
		*captured = append(*captured, strings.Join(all, " "))
		return exec.CommandContext(ctx, "/bin/sh", "-c", script)
	}
}

func TestContainerExec_BuildsCommand(t *testing.T) {
	origExec := execCommandContext
	t.Cleanup(func() { execCommandContext = origExec })

	var captured []string
	execCommandContext = captureExec(`echo ok`, &captured)

	_, err := ContainerExec(context.Background(), ContainerExecParams{
		Host:             "user@example.lan",
		GatewayContainer: "openclaw-gateway",
		Command:          "ls /tmp",
		Timeout:          5 * time.Second,
	})
	if err != nil {
		t.Fatalf("ContainerExec: %v", err)
	}
	if len(captured) != 1 {
		t.Fatalf("expected 1 exec call, got %d", len(captured))
	}
	cmd := captured[0]

	// Outer wrapper: ssh ... user@example.lan -- docker exec ...
	if !strings.HasPrefix(cmd, "ssh ") {
		t.Errorf("expected ssh prefix, got: %s", cmd)
	}
	if !strings.Contains(cmd, "user@example.lan") {
		t.Errorf("expected host in command, got: %s", cmd)
	}
	if !strings.Contains(cmd, "docker exec") {
		t.Errorf("expected docker exec in command, got: %s", cmd)
	}
	if !strings.Contains(cmd, "'openclaw-gateway'") {
		t.Errorf("expected single-quoted container name, got: %s", cmd)
	}
	if !strings.Contains(cmd, "base64 -d") {
		t.Errorf("expected base64 -d decoder in command, got: %s", cmd)
	}

	// The encoded inner command must round-trip back to "ls /tmp".
	expected := base64.StdEncoding.EncodeToString([]byte("ls /tmp"))
	if !strings.Contains(cmd, expected) {
		t.Errorf("expected base64-encoded command %q in: %s", expected, cmd)
	}
}

func TestContainerExec_DefaultsContainer(t *testing.T) {
	origExec := execCommandContext
	t.Cleanup(func() { execCommandContext = origExec })

	var captured []string
	execCommandContext = captureExec(`echo ok`, &captured)

	_, err := ContainerExec(context.Background(), ContainerExecParams{
		Host:    "user@example.lan",
		Command: "true",
		// GatewayContainer intentionally empty
		Timeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("ContainerExec: %v", err)
	}
	if !strings.Contains(captured[0], "'openclaw-gateway'") {
		t.Errorf("expected default container 'openclaw-gateway', got: %s", captured[0])
	}
}

func TestContainerExec_PreservesStdoutStderr(t *testing.T) {
	origExec := execCommandContext
	execCommandContext = helperExec(`echo OUTPUT; echo ERROR >&2`)
	t.Cleanup(func() { execCommandContext = origExec })

	res, err := ContainerExec(context.Background(), ContainerExecParams{
		Host:    "user@example.lan",
		Command: "anything",
		Timeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("ContainerExec: %v", err)
	}
	if !strings.Contains(res.Stdout, "OUTPUT") || strings.Contains(res.Stdout, "ERROR") {
		t.Errorf("stdout=%q should have OUTPUT only", res.Stdout)
	}
	if !strings.Contains(res.Stderr, "ERROR") || strings.Contains(res.Stderr, "OUTPUT") {
		t.Errorf("stderr=%q should have ERROR only", res.Stderr)
	}
}

func TestContainerExec_NonZeroExitNotError(t *testing.T) {
	origExec := execCommandContext
	execCommandContext = helperExec(`echo "missing container" >&2; exit 1`)
	t.Cleanup(func() { execCommandContext = origExec })

	res, err := ContainerExec(context.Background(), ContainerExecParams{
		Host:    "user@example.lan",
		Command: "true",
		Timeout: 5 * time.Second,
	})
	// Mirrors SSHRun: non-zero remote exit is data, not error.
	if err != nil {
		t.Fatalf("ContainerExec: unexpected error: %v", err)
	}
	if res.ExitCode != 1 {
		t.Errorf("ExitCode=%d want 1", res.ExitCode)
	}
	if !strings.Contains(res.Stderr, "missing container") {
		t.Errorf("stderr=%q want missing container", res.Stderr)
	}
}

func TestContainerExec_HandlesSpecialChars(t *testing.T) {
	// The Command field carries arbitrary shell content: pipes, redirects,
	// nested quotes, dollar-signs, newlines. Base64 encoding must round-trip
	// it unchanged so the inner shell sees the literal bytes.
	origExec := execCommandContext
	t.Cleanup(func() { execCommandContext = origExec })

	var captured []string
	execCommandContext = captureExec(`echo ok`, &captured)

	weird := `git -C '/tmp/r' diff 'v1'..'HEAD' > '/tmp/d.txt' && wc -c '/tmp/d.txt'`
	_, err := ContainerExec(context.Background(), ContainerExecParams{
		Host:    "user@example.lan",
		Command: weird,
		Timeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("ContainerExec: %v", err)
	}
	expected := base64.StdEncoding.EncodeToString([]byte(weird))
	if !strings.Contains(captured[0], expected) {
		t.Errorf("expected base64 of weird command in: %s", captured[0])
	}

	// Also test multi-line and embedded newlines.
	multiline := "line1\nline2\nline3 with 'quotes' and $dollars"
	captured = nil
	_, err = ContainerExec(context.Background(), ContainerExecParams{
		Host:    "user@example.lan",
		Command: multiline,
		Timeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("ContainerExec multiline: %v", err)
	}
	expectedMulti := base64.StdEncoding.EncodeToString([]byte(multiline))
	if !strings.Contains(captured[0], expectedMulti) {
		t.Errorf("expected base64 of multiline in: %s", captured[0])
	}
}

func TestContainerExec_TimeoutPropagates(t *testing.T) {
	origExec := execCommandContext
	execCommandContext = helperExec(`sleep 10`)
	t.Cleanup(func() { execCommandContext = origExec })

	_, err := ContainerExec(context.Background(), ContainerExecParams{
		Host:    "user@example.lan",
		Command: "sleep 10",
		Timeout: 100 * time.Millisecond,
	})
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !errors.Is(err, context.DeadlineExceeded) && !strings.Contains(err.Error(), "deadline") && !strings.Contains(err.Error(), "killed") {
		t.Errorf("error %q does not signal timeout", err.Error())
	}
}

func TestContainerExec_RequiresHost(t *testing.T) {
	_, err := ContainerExec(context.Background(), ContainerExecParams{
		Host:    "",
		Command: "true",
		Timeout: time.Second,
	})
	if err == nil {
		t.Fatal("expected error for empty host")
	}
}

func TestContainerExec_RequiresCommand(t *testing.T) {
	_, err := ContainerExec(context.Background(), ContainerExecParams{
		Host:    "user@example.lan",
		Command: "",
		Timeout: time.Second,
	})
	if err == nil {
		t.Fatal("expected error for empty command")
	}
}
