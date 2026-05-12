package multireview

import (
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestInvokeReviewer_ParsesEnvelope(t *testing.T) {
	// Real openclaw `agent --json` envelope: {runId, status, summary, result: {payloads, meta}}
	mockResponse := `{
		"runId": "abc-123",
		"status": "ok",
		"summary": "completed",
		"result": {
			"payloads": [
				{"text": "# Review\n\nVerdict: ship-ready\n\nMEDIUM|src/foo.go:42|missing nil check|add guard|robustness\n", "mediaUrl": null}
			],
			"meta": {
				"durationMs": 123456,
				"aborted": false,
				"agentMeta": {
					"sessionId": "sess-456",
					"provider": "litellm",
					"model": "qwen-3.6-plus"
				}
			}
		}
	}`

	origExec := execCommandContext
	t.Cleanup(func() { execCommandContext = origExec })
	execCommandContext = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		// docker exec command should be invoked via ssh
		return exec.CommandContext(ctx, "/bin/sh", "-c", "cat <<'EOF'\n"+mockResponse+"\nEOF")
	}

	res, err := InvokeReviewer(context.Background(), InvokeReviewerParams{
		Host:        "user@example.lan",
		AgentName:   "bruce",
		TaskMessage: "Review the diff at /tmp/bench/repo",
		Timeout:     30 * time.Second,
	})
	if err != nil {
		t.Fatalf("InvokeReviewer: %v", err)
	}
	if res.Status != "ok" {
		t.Errorf("status=%q want ok", res.Status)
	}
	if res.Model != "qwen-3.6-plus" {
		t.Errorf("model=%q want qwen-3.6-plus", res.Model)
	}
	if res.DurationMS != 123456 {
		t.Errorf("durationMs=%d want 123456", res.DurationMS)
	}
	if !strings.Contains(res.ReviewProse, "ship-ready") {
		t.Errorf("review prose missing verdict: %q", res.ReviewProse)
	}
	if !strings.Contains(res.ReviewProse, "MEDIUM|src/foo.go:42|") {
		t.Errorf("review prose missing TD line: %q", res.ReviewProse)
	}
	if res.RawJSON == "" {
		t.Error("raw JSON should be preserved")
	}
}

func TestInvokeReviewer_RequiresInputs(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		name string
		p    InvokeReviewerParams
	}{
		{"empty host", InvokeReviewerParams{AgentName: "bruce", TaskMessage: "m"}},
		{"empty agent", InvokeReviewerParams{Host: "u@h", TaskMessage: "m"}},
		{"empty task", InvokeReviewerParams{Host: "u@h", AgentName: "bruce"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := InvokeReviewer(ctx, c.p)
			if err == nil {
				t.Errorf("expected error")
			}
		})
	}
}

func TestInvokeReviewer_HandlesMalformedJSON(t *testing.T) {
	origExec := execCommandContext
	t.Cleanup(func() { execCommandContext = origExec })
	execCommandContext = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "/bin/sh", "-c", `echo "not json at all"`)
	}

	_, err := InvokeReviewer(context.Background(), InvokeReviewerParams{
		Host:        "user@example.lan",
		AgentName:   "bruce",
		TaskMessage: "test",
		Timeout:     5 * time.Second,
	})
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
	if !strings.Contains(err.Error(), "parse") && !strings.Contains(err.Error(), "json") {
		t.Errorf("error %q should mention parsing", err.Error())
	}
}

func TestInvokeReviewer_HandlesSSHFailure(t *testing.T) {
	origExec := execCommandContext
	t.Cleanup(func() { execCommandContext = origExec })
	execCommandContext = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "/bin/sh", "-c", `echo "Permission denied (publickey)" >&2; exit 255`)
	}

	_, err := InvokeReviewer(context.Background(), InvokeReviewerParams{
		Host:        "user@example.lan",
		AgentName:   "bruce",
		TaskMessage: "test",
		Timeout:     5 * time.Second,
	})
	if err == nil {
		t.Fatal("expected error for SSH failure")
	}
}

func TestInvokeReviewer_HandlesEmptyResponse(t *testing.T) {
	origExec := execCommandContext
	t.Cleanup(func() { execCommandContext = origExec })
	execCommandContext = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "/bin/sh", "-c", "")
	}

	_, err := InvokeReviewer(context.Background(), InvokeReviewerParams{
		Host:        "user@example.lan",
		AgentName:   "bruce",
		TaskMessage: "test",
		Timeout:     5 * time.Second,
	})
	if err == nil {
		t.Fatal("expected error for empty response")
	}
}

func TestInvokeReviewer_ConcatenatesMultiplePayloads(t *testing.T) {
	// Some openclaw responses have multiple payload entries (preamble + body)
	mockResponse := `{
		"runId": "x",
		"status": "ok",
		"summary": "done",
		"result": {
			"payloads": [
				{"text": "preamble: starting review", "mediaUrl": null},
				{"text": "the actual review body", "mediaUrl": null}
			],
			"meta": {"durationMs": 1000, "aborted": false, "agentMeta": {"model": "test"}}
		}
	}`
	origExec := execCommandContext
	t.Cleanup(func() { execCommandContext = origExec })
	execCommandContext = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "/bin/sh", "-c", "cat <<'EOF'\n"+mockResponse+"\nEOF")
	}

	res, err := InvokeReviewer(context.Background(), InvokeReviewerParams{
		Host:        "user@example.lan",
		AgentName:   "bruce",
		TaskMessage: "test",
		Timeout:     5 * time.Second,
	})
	if err != nil {
		t.Fatalf("InvokeReviewer: %v", err)
	}
	if !strings.Contains(res.ReviewProse, "preamble") {
		t.Error("payload 1 missing")
	}
	if !strings.Contains(res.ReviewProse, "actual review body") {
		t.Error("payload 2 missing")
	}
}
