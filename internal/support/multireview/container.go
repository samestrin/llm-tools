package multireview

import (
	"context"
	"encoding/base64"
	"fmt"
	"time"
)

// ContainerExecParams configures a command that runs inside a docker container
// on the SSH target host. The Command is base64-encoded for transport so it
// can carry arbitrary quotes, redirects, pipes, and newlines without quoting
// concerns at the SSH/shell boundary.
type ContainerExecParams struct {
	// Host is the SSH target ("user@host" or just "host").
	Host string
	// GatewayContainer is the docker container name. Defaults to
	// "openclaw-gateway" when empty.
	GatewayContainer string
	// Command is the shell command to execute inside the container.
	Command string
	// Timeout caps the wall-clock time before the call is killed.
	Timeout time.Duration
	// ExtraSSHArgs are inserted before the host (e.g. "-i", "/path/key").
	ExtraSSHArgs []string
}

// ContainerExec runs a single command inside a docker container on the SSH
// target host. The wire command is shaped as:
//
//	ssh <host> -- docker exec <container> sh -c "$(echo <b64> | base64 -d)"
//
// The container name and base64 blob are shell-quoted so the outer ssh shell
// never reinterprets them. The inner `sh -c "$(...)"` evaluates the
// `base64 -d` substitution to produce the raw command bytes for the inner
// shell — bypassing all quoting concerns for the caller's Command.
//
// Mirrors SSHRun semantics: a returned error means the call could not be
// executed or was killed (context deadline, signal). A normal non-zero exit
// inside the container is signaled through SSHResult.ExitCode only.
func ContainerExec(parent context.Context, p ContainerExecParams) (SSHResult, error) {
	if p.Host == "" {
		return SSHResult{}, fmt.Errorf("container exec: host required")
	}
	if p.Command == "" {
		return SSHResult{}, fmt.Errorf("container exec: command required")
	}
	if p.GatewayContainer == "" {
		p.GatewayContainer = "openclaw-gateway"
	}

	encoded := base64.StdEncoding.EncodeToString([]byte(p.Command))
	remoteCmd := fmt.Sprintf(
		`docker exec %s sh -c "$(echo %s | base64 -d)"`,
		shellQuote(p.GatewayContainer),
		shellQuote(encoded),
	)

	return SSHRun(parent, SSHParams{
		Host:         p.Host,
		Command:      remoteCmd,
		Timeout:      p.Timeout,
		ExtraSSHArgs: p.ExtraSSHArgs,
	})
}
