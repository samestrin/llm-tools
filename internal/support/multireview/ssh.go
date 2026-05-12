// Package multireview orchestrates code reviews across multiple external
// openclaw reviewer agents. It bundles a diff range, ships it to a remote
// host running openclaw-gateway, invokes each reviewer agent in parallel
// (or serially when rate-limit constraints require), and collects the
// pipe-delimited TD findings each reviewer produces.
//
// The package is split into focused helpers:
//
//   - ssh.go     — SSH and SCP wrappers with timeout + stream separation
//   - bundle.go  — git bundle + scp + remote clone pipeline
//   - openclaw.go — invoke a reviewer agent and parse the JSON envelope
//   - stream.go  — extract TD lines from a review and merge across reviewers
//
// External commands are invoked through execCommandContext, a package
// variable that tests replace to inject deterministic behavior.
package multireview

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"time"
)

// execCommandContext is the exec.CommandContext indirection used by all
// subprocess calls in this package. Tests override it to inject scripted
// behavior without depending on real SSH/scp/docker.
var execCommandContext = exec.CommandContext

// SSHParams configures a single remote command invocation.
type SSHParams struct {
	// Host is the SSH target ("user@host" or just "host").
	Host string
	// Command is the remote shell command to execute.
	Command string
	// Timeout caps the wall-clock time before the call is killed.
	Timeout time.Duration
	// ExtraSSHArgs are flags inserted before the host (e.g. "-i", "/path/key").
	ExtraSSHArgs []string
}

// SSHResult captures the outcome of an SSH command.
type SSHResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// SSHRun executes a single remote command via /usr/bin/ssh. It separates
// stdout and stderr (do not interleave — callers parse stdout JSON and rely
// on stderr being free of payload data), enforces a timeout, and reports a
// non-zero ExitCode without converting that into a Go error.
//
// A returned error means the call could not be executed or was killed
// (context deadline, signal). A normal non-zero remote exit is signaled
// through SSHResult.ExitCode only.
func SSHRun(parent context.Context, p SSHParams) (SSHResult, error) {
	if p.Host == "" {
		return SSHResult{}, fmt.Errorf("ssh: host required")
	}
	if p.Command == "" {
		return SSHResult{}, fmt.Errorf("ssh: command required")
	}
	if p.Timeout <= 0 {
		p.Timeout = 60 * time.Second
	}

	ctx, cancel := context.WithTimeout(parent, p.Timeout)
	defer cancel()

	args := []string{"-o", "BatchMode=yes", "-o", "StrictHostKeyChecking=accept-new"}
	args = append(args, p.ExtraSSHArgs...)
	args = append(args, p.Host, "--", p.Command)

	cmd := execCommandContext(ctx, "ssh", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	res := SSHResult{
		Stdout: stdout.String(),
		Stderr: stderr.String(),
	}
	if ctx.Err() == context.DeadlineExceeded {
		return res, fmt.Errorf("ssh %s: deadline exceeded after %s", p.Host, p.Timeout)
	}
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			res.ExitCode = exitErr.ExitCode()
			// Non-zero remote exit is data, not failure.
			return res, nil
		}
		return res, fmt.Errorf("ssh %s: %w", p.Host, err)
	}
	return res, nil
}

// SCPParams configures a single scp transfer.
type SCPParams struct {
	Host       string
	LocalPath  string
	RemotePath string
	Timeout    time.Duration
	// Reverse=true pulls FROM remote TO local instead of pushing.
	Reverse bool
}

// SCPSend transfers a single file via scp. Errors are returned for missing
// inputs or non-zero scp exit (transfers are atomic — partial success is
// not a thing scp reports without -p, so non-zero is a real failure).
func SCPSend(parent context.Context, p SCPParams) (SSHResult, error) {
	if p.Host == "" {
		return SSHResult{}, fmt.Errorf("scp: host required")
	}
	if p.LocalPath == "" {
		return SSHResult{}, fmt.Errorf("scp: local path required")
	}
	if p.RemotePath == "" {
		return SSHResult{}, fmt.Errorf("scp: remote path required")
	}
	if p.Timeout <= 0 {
		p.Timeout = 300 * time.Second
	}

	ctx, cancel := context.WithTimeout(parent, p.Timeout)
	defer cancel()

	remoteSpec := fmt.Sprintf("%s:%s", p.Host, p.RemotePath)
	var src, dst string
	if p.Reverse {
		src, dst = remoteSpec, p.LocalPath
	} else {
		src, dst = p.LocalPath, remoteSpec
	}

	args := []string{"-q", "-o", "BatchMode=yes", "-o", "StrictHostKeyChecking=accept-new", src, dst}
	cmd := execCommandContext(ctx, "scp", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	res := SSHResult{Stdout: stdout.String(), Stderr: stderr.String()}
	if ctx.Err() == context.DeadlineExceeded {
		return res, fmt.Errorf("scp %s -> %s: deadline exceeded after %s", src, dst, p.Timeout)
	}
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			res.ExitCode = exitErr.ExitCode()
			return res, fmt.Errorf("scp %s -> %s: exit %d, stderr: %s", src, dst, exitErr.ExitCode(), stderr.String())
		}
		return res, fmt.Errorf("scp %s -> %s: %w", src, dst, err)
	}
	return res, nil
}
