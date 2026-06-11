package llmapi

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"sync/atomic"
	"time"
)

// DefaultStreamIdleTimeout is the no-activity window applied to streaming
// requests when the caller does not specify one.
const DefaultStreamIdleTimeout = 120 * time.Second

// maxStreamLineSize bounds a single SSE line; review responses can carry
// large chunks but a single data line should never approach this.
const maxStreamLineSize = 4 * 1024 * 1024

// streamChunk is the SSE payload shape for chat completion deltas.
type streamChunk struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
}

// CompleteMessagesStream sends a streaming chat completion request and
// assembles the response content from SSE chunks.
//
// idleTimeout bounds the gap between stream activity (any received bytes
// count as liveness, not just content tokens); the caller's ctx governs the
// total budget. idleTimeout <= 0 uses DefaultStreamIdleTimeout.
//
// Retry semantics match doRequestWithRetry with two additions: a timed-out
// or stalled stream is never retried, and no retry happens after partial
// content has been received (re-sending would force the backend to repeat
// the full prefill).
func (c *LLMClient) CompleteMessagesStream(ctx context.Context, messages []Message, idleTimeout time.Duration) (string, error) {
	if idleTimeout <= 0 {
		idleTimeout = DefaultStreamIdleTimeout
	}

	req := ChatRequest{
		Model:       c.Model,
		Messages:    messages,
		Temperature: c.Temperature,
		Stream:      true,
	}

	var lastErr error
	for attempt := 0; attempt <= c.MaxRetries; attempt++ {
		if attempt > 0 {
			delay := time.Duration(float64(c.RetryDelay) * math.Pow(c.RetryBackoff, float64(attempt-1)))
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(delay):
			}
		}

		content, receivedAny, err := c.doStreamRequest(ctx, req, idleTimeout)
		if err == nil {
			return content, nil
		}

		var apiErr *APIError
		if errors.As(err, &apiErr) {
			if !isRetryable(apiErr.StatusCode) {
				return "", err
			}
			lastErr = err
			continue
		}

		if isTimeoutErr(err) {
			return "", err
		}

		if receivedAny {
			return "", err
		}

		lastErr = err
	}

	return "", fmt.Errorf("max retries exceeded: %w", lastErr)
}

// doStreamRequest executes a single streaming attempt. receivedAny reports
// whether any response bytes arrived, so the caller can avoid retrying a
// partially-consumed stream.
func (c *LLMClient) doStreamRequest(ctx context.Context, req ChatRequest, idleTimeout time.Duration) (content string, receivedAny bool, err error) {
	reqBody, err := json.Marshal(req)
	if err != nil {
		return "", false, fmt.Errorf("failed to marshal request: %w", err)
	}

	// The watchdog cancels the request when the stream goes quiet for
	// idleTimeout; it is re-armed on every received line.
	streamCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	var idleFired atomic.Bool
	watchdog := time.AfterFunc(idleTimeout, func() {
		idleFired.Store(true)
		cancel()
	})
	defer watchdog.Stop()

	wrapIdle := func(err error) error {
		if idleFired.Load() {
			return fmt.Errorf("stream idle timeout: no activity for %s: %w", idleTimeout, context.DeadlineExceeded)
		}
		return err
	}

	url := c.BaseURL + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(streamCtx, "POST", url, bytes.NewReader(reqBody))
	if err != nil {
		return "", false, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	httpReq.Header.Set("Authorization", "Bearer "+c.APIKey)

	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return "", false, wrapIdle(fmt.Errorf("request failed: %w", err))
	}
	defer resp.Body.Close()
	watchdog.Reset(idleTimeout)

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		apiErr := &APIError{StatusCode: resp.StatusCode}
		if json.Unmarshal(body, apiErr) == nil && apiErr.ErrorInfo.Message != "" {
			return "", false, fmt.Errorf("API error (%d): %s: %w", resp.StatusCode, apiErr.ErrorInfo.Message, apiErr)
		}
		apiErr.ErrorInfo.Message = fmt.Sprintf("status %d", resp.StatusCode)
		return "", false, fmt.Errorf("API error: status %d: %w", resp.StatusCode, apiErr)
	}

	// Some OpenAI-compatible gateways ignore stream:true and reply with a
	// plain JSON completion; fall back to non-streaming parsing for those.
	if !strings.Contains(resp.Header.Get("Content-Type"), "text/event-stream") {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", true, wrapIdle(fmt.Errorf("failed to read response: %w", err))
		}
		var chatResp ChatResponse
		if err := json.Unmarshal(body, &chatResp); err != nil {
			return "", true, fmt.Errorf("failed to parse response: %w", err)
		}
		if len(chatResp.Choices) == 0 {
			return "", true, errors.New("no choices in response")
		}
		return chatResp.Choices[0].Message.Content, true, nil
	}

	var sb strings.Builder
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), maxStreamLineSize)

	for scanner.Scan() {
		// Any bytes count as liveness: empty keep-alive lines and SSE
		// comments re-arm the watchdog just like content chunks.
		watchdog.Reset(idleTimeout)
		receivedAny = true

		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, ":") {
			continue
		}
		data, ok := strings.CutPrefix(line, "data:")
		if !ok {
			continue
		}
		data = strings.TrimSpace(data)
		if data == "[DONE]" {
			return sb.String(), true, nil
		}

		var chunk streamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			return "", receivedAny, fmt.Errorf("failed to parse stream chunk: %w", err)
		}
		if len(chunk.Choices) > 0 {
			sb.WriteString(chunk.Choices[0].Delta.Content)
		}
	}

	if err := scanner.Err(); err != nil {
		return "", receivedAny, wrapIdle(fmt.Errorf("stream read failed: %w", err))
	}

	// Stream closed without [DONE]; accept assembled content if any arrived.
	if sb.Len() == 0 {
		return "", receivedAny, errors.New("no content in stream")
	}
	return sb.String(), true, nil
}
