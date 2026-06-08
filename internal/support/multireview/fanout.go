package multireview

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// FanoutParams configures the multi-agent review fan-out.
type FanoutParams struct {
	// Registry provides agent and provider configurations.
	Registry *Registry
	// ParallelAgents run concurrently (non-rate-limited providers).
	ParallelAgents []string
	// SerialAgents run sequentially (rate-limited providers like o3).
	SerialAgents []string
	// TaskMessage is the prompt sent to each agent.
	TaskMessage string
	// GlobalTimeout caps the entire fan-out operation.
	GlobalTimeout time.Duration
	// OutputDir is where per-agent outputs are written.
	OutputDir string
}

// FanoutResult captures the aggregated fan-out results.
type FanoutResult struct {
	// Results contains per-agent invocation results.
	Results []InvokeDirectResult
	// SuccessCount is the number of successful invocations.
	SuccessCount int
	// FailedCount is the number of failed invocations.
	FailedCount int
	// TotalFindings is the count of TD_STREAM entries across all agents.
	TotalFindings int
	// TotalDurationMS is the wall-clock time for the entire fan-out.
	TotalDurationMS int64
}

// AgentStatus is written to status.json for each agent.
type AgentStatus struct {
	AgentName  string `json:"agent_name"`
	Model      string `json:"model"`
	Status     string `json:"status"`
	DurationMS int64  `json:"duration_ms"`
	Error      string `json:"error,omitempty"`
}

// Fanout invokes multiple agents and collects their results.
// Parallel agents run concurrently; serial agents run one at a time after parallel.
func Fanout(ctx context.Context, p FanoutParams) (FanoutResult, error) {
	start := time.Now()

	result := FanoutResult{
		Results: make([]InvokeDirectResult, 0, len(p.ParallelAgents)+len(p.SerialAgents)),
	}

	totalAgents := len(p.ParallelAgents) + len(p.SerialAgents)
	if totalAgents == 0 {
		return result, errors.New("no agents specified")
	}

	// Apply global timeout
	if p.GlobalTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, p.GlobalTimeout)
		defer cancel()
	}

	// Create output directory structure
	if p.OutputDir != "" {
		if err := os.MkdirAll(filepath.Join(p.OutputDir, "raw"), 0755); err != nil {
			return result, fmt.Errorf("failed to create output dir: %w", err)
		}
	}

	var mu sync.Mutex

	// Helper to invoke a single agent
	invokeAgent := func(agentName string) InvokeDirectResult {
		agent, err := p.Registry.GetAgent(agentName)
		if err != nil {
			return InvokeDirectResult{
				AgentName: agentName,
				Status:    "failed",
				Error:     fmt.Errorf("agent not found: %w", err),
			}
		}

		apiConfig, err := p.Registry.ResolveProvider(agent.Provider)
		if err != nil {
			return InvokeDirectResult{
				AgentName: agentName,
				Model:     agent.Model,
				Status:    "failed",
				Error:     fmt.Errorf("provider error: %w", err),
			}
		}

		timeout := time.Duration(agent.TimeoutSecs) * time.Second
		if timeout == 0 {
			timeout = 10 * time.Minute // default
		}

		res := InvokeDirect(ctx, InvokeDirectParams{
			AgentName:   agentName,
			AgentConfig: agent,
			APIConfig:   apiConfig,
			TaskMessage: p.TaskMessage,
			Timeout:     timeout,
		})

		// Write output files
		if p.OutputDir != "" {
			writeAgentOutput(p.OutputDir, res)
		}

		return res
	}

	// Run parallel agents concurrently
	if len(p.ParallelAgents) > 0 {
		var wg sync.WaitGroup
		for _, agentName := range p.ParallelAgents {
			wg.Add(1)
			go func(name string) {
				defer wg.Done()
				res := invokeAgent(name)
				mu.Lock()
				result.Results = append(result.Results, res)
				mu.Unlock()
			}(agentName)
		}
		wg.Wait()
	}

	// Run serial agents sequentially
	for _, agentName := range p.SerialAgents {
		// Check for context cancellation before each serial agent
		select {
		case <-ctx.Done():
			res := InvokeDirectResult{
				AgentName: agentName,
				Status:    "timeout",
				Error:     ctx.Err(),
			}
			mu.Lock()
			result.Results = append(result.Results, res)
			mu.Unlock()
			continue
		default:
		}

		res := invokeAgent(agentName)
		mu.Lock()
		result.Results = append(result.Results, res)
		mu.Unlock()
	}

	// Calculate aggregates
	result.TotalDurationMS = time.Since(start).Milliseconds()
	for _, r := range result.Results {
		if r.Status == "ok" {
			result.SuccessCount++
		} else {
			result.FailedCount++
		}
	}

	// Return error if all agents failed
	if result.SuccessCount == 0 && totalAgents > 0 {
		return result, errors.New("all agents failed")
	}

	return result, nil
}

// writeAgentOutput writes the agent's review and status to disk.
func writeAgentOutput(outputDir string, res InvokeDirectResult) error {
	agentDir := filepath.Join(outputDir, "raw", res.AgentName)
	if err := os.MkdirAll(agentDir, 0755); err != nil {
		return err
	}

	// Write review.md
	reviewPath := filepath.Join(agentDir, "review.md")
	if err := os.WriteFile(reviewPath, []byte(res.ReviewProse), 0644); err != nil {
		return err
	}

	// Write status.json
	status := AgentStatus{
		AgentName:  res.AgentName,
		Model:      res.Model,
		Status:     res.Status,
		DurationMS: res.DurationMS,
	}
	if res.Error != nil {
		status.Error = res.Error.Error()
	}

	statusJSON, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		return err
	}

	statusPath := filepath.Join(agentDir, "status.json")
	return os.WriteFile(statusPath, statusJSON, 0644)
}
