package multireview

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// InvokeReviewerParams configures a single openclaw agent invocation.
type InvokeReviewerParams struct {
	// Host is the SSH target running openclaw-gateway.
	Host string
	// AgentName is the openclaw agent id (e.g. "bruce", "greta").
	AgentName string
	// TaskMessage is the prompt sent to the agent. The caller composes it
	// with whatever scope, repo path, and TD_STREAM format instructions are
	// appropriate for the run.
	TaskMessage string
	// Timeout caps the SSH/docker call. Reasoning models on complex reviews
	// can take 5-20 minutes; size accordingly.
	Timeout time.Duration
	// GatewayContainer is the docker container name. Defaults to
	// "openclaw-gateway".
	GatewayContainer string
}

// InvokeReviewerResult captures one reviewer's output.
type InvokeReviewerResult struct {
	// AgentName echoes the input for convenience.
	AgentName string
	// Status from the openclaw envelope ("ok", "error", etc.).
	Status string
	// Model from envelope.result.meta.agentMeta.model.
	Model string
	// DurationMS from envelope.result.meta.durationMs.
	DurationMS int64
	// Aborted from envelope.result.meta.aborted — true means openclaw
	// killed the call before completion (timeout, mid-stream error).
	Aborted bool
	// ReviewProse is all payload text concatenated in order. This is what a
	// human reads; the TD_STREAM lines are extracted from it downstream.
	ReviewProse string
	// RawJSON preserves the entire envelope for offline replay/diagnosis.
	RawJSON string
}

// openclawEnvelope mirrors the JSON shape returned by `openclaw agent --json`.
type openclawEnvelope struct {
	RunID   string `json:"runId"`
	Status  string `json:"status"`
	Summary string `json:"summary"`
	Result  struct {
		Payloads []struct {
			Text     string `json:"text"`
			MediaURL string `json:"mediaUrl"`
		} `json:"payloads"`
		Meta struct {
			DurationMS int64 `json:"durationMs"`
			Aborted    bool  `json:"aborted"`
			AgentMeta  struct {
				SessionID string `json:"sessionId"`
				Provider  string `json:"provider"`
				Model     string `json:"model"`
			} `json:"agentMeta"`
		} `json:"meta"`
	} `json:"result"`
}

// InvokeReviewer runs a single openclaw agent via SSH+docker exec and parses
// the JSON envelope into a structured result.
//
// The remote command is:
//
//	docker exec -i <container> openclaw agent --agent <name> --message <msg> --json
//
// The task message is passed via stdin to avoid shell-escaping headaches with
// large multi-line prompts. We use docker exec -i (interactive) for the stdin
// pipe to work.
func InvokeReviewer(parent context.Context, p InvokeReviewerParams) (InvokeReviewerResult, error) {
	if p.Host == "" {
		return InvokeReviewerResult{}, fmt.Errorf("invoke: host required")
	}
	if p.AgentName == "" {
		return InvokeReviewerResult{}, fmt.Errorf("invoke: agent name required")
	}
	if p.TaskMessage == "" {
		return InvokeReviewerResult{}, fmt.Errorf("invoke: task message required")
	}
	if p.Timeout <= 0 {
		p.Timeout = 20 * time.Minute
	}
	if p.GatewayContainer == "" {
		p.GatewayContainer = "openclaw-gateway"
	}

	// Build the remote command. The task message can contain anything
	// (quotes, dollar signs, newlines, backticks, even our own delimiter
	// strings) so we base64-encode it and decode on the remote. This
	// removes any quoting concerns at the SSH boundary.
	encoded := base64.StdEncoding.EncodeToString([]byte(p.TaskMessage))
	remoteCmd := fmt.Sprintf(
		`docker exec %s openclaw agent --agent %s --json --message "$(echo %s | base64 -d)"`,
		shellQuote(p.GatewayContainer),
		shellQuote(p.AgentName),
		shellQuote(encoded),
	)

	res, err := SSHRun(parent, SSHParams{
		Host:    p.Host,
		Command: remoteCmd,
		Timeout: p.Timeout,
	})
	if err != nil {
		return InvokeReviewerResult{AgentName: p.AgentName}, fmt.Errorf("invoke %s: %w", p.AgentName, err)
	}
	if res.ExitCode != 0 {
		return InvokeReviewerResult{AgentName: p.AgentName}, fmt.Errorf("invoke %s: ssh exit %d, stderr: %s", p.AgentName, res.ExitCode, res.Stderr)
	}
	if strings.TrimSpace(res.Stdout) == "" {
		return InvokeReviewerResult{AgentName: p.AgentName}, fmt.Errorf("invoke %s: empty response", p.AgentName)
	}

	var env openclawEnvelope
	if err := json.Unmarshal([]byte(res.Stdout), &env); err != nil {
		// Preview the start of stdout to help diagnose what came back.
		preview := res.Stdout
		if len(preview) > 200 {
			preview = preview[:200] + "..."
		}
		return InvokeReviewerResult{AgentName: p.AgentName}, fmt.Errorf("invoke %s: parse json: %w (got: %s)", p.AgentName, err, preview)
	}

	var prose strings.Builder
	for _, pl := range env.Result.Payloads {
		prose.WriteString(pl.Text)
	}

	return InvokeReviewerResult{
		AgentName:   p.AgentName,
		Status:      env.Status,
		Model:       env.Result.Meta.AgentMeta.Model,
		DurationMS:  env.Result.Meta.DurationMS,
		Aborted:     env.Result.Meta.Aborted,
		ReviewProse: prose.String(),
		RawJSON:     res.Stdout,
	}, nil
}
