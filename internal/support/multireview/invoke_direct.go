package multireview

import (
	"context"
	"errors"
	"time"

	"github.com/samestrin/llm-tools/pkg/llmapi"
)

// InvokeDirectParams configures a direct LLM provider invocation.
type InvokeDirectParams struct {
	// AgentName identifies the agent (e.g., "bruce", "greta").
	AgentName string
	// AgentConfig contains the agent's model, temperature, and system prompt.
	AgentConfig AgentConfig
	// APIConfig contains the provider's API key and base URL.
	APIConfig *llmapi.APIConfig
	// TaskMessage is the user prompt sent to the agent.
	TaskMessage string
	// Timeout caps the API call duration.
	Timeout time.Duration
}

// InvokeDirectResult captures one direct invocation's output.
type InvokeDirectResult struct {
	// AgentName echoes the input for convenience.
	AgentName string
	// Model from the agent configuration.
	Model string
	// Status: "ok", "failed", or "timeout".
	Status string
	// DurationMS is the elapsed time in milliseconds.
	DurationMS int64
	// ReviewProse is the full response content.
	ReviewProse string
	// Error captures any failure.
	Error error
	// FallbackUsed indicates a fallback agent was used after primary failed.
	FallbackUsed bool
	// FallbackFrom is the original agent name that failed (when FallbackUsed=true).
	FallbackFrom string
	// OriginalError is the error from the primary agent (when FallbackUsed=true).
	OriginalError error
}

// InvokeDirect calls an LLM provider directly without SSH/Docker.
func InvokeDirect(ctx context.Context, p InvokeDirectParams) InvokeDirectResult {
	start := time.Now()

	result := InvokeDirectResult{
		AgentName: p.AgentName,
		Model:     p.AgentConfig.Model,
	}

	// Create LLM client
	client := llmapi.NewLLMClient(p.APIConfig.APIKey, p.APIConfig.BaseURL, p.AgentConfig.Model)
	client.Temperature = p.AgentConfig.Temperature
	// Reduce retry delays to prevent timeout exhaustion on non-retryable errors
	client.RetryDelay = 500 * time.Millisecond
	client.RetryBackoff = 1.5

	// Apply timeout to context if not already set
	if p.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, p.Timeout)
		defer cancel()
	}

	// Make the API call
	content, err := client.CompleteWithContext(ctx, p.AgentConfig.SystemPrompt, p.TaskMessage)

	result.DurationMS = time.Since(start).Milliseconds()

	if err != nil {
		// Check for context cancellation (timeout or explicit cancel)
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) ||
			errors.Is(ctx.Err(), context.DeadlineExceeded) || errors.Is(ctx.Err(), context.Canceled) {
			result.Status = "timeout"
			result.Error = err
			return result
		}

		result.Status = "failed"
		result.Error = err
		return result
	}

	result.Status = "ok"
	result.ReviewProse = content
	return result
}

// ToInvokeReviewerResult converts to the openclaw-compatible result format.
func (r InvokeDirectResult) ToInvokeReviewerResult() InvokeReviewerResult {
	return InvokeReviewerResult{
		AgentName:   r.AgentName,
		Model:       r.Model,
		Status:      r.Status,
		DurationMS:  r.DurationMS,
		Aborted:     r.Status == "timeout" || r.Status == "failed",
		ReviewProse: r.ReviewProse,
	}
}
