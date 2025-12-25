// Package llmapi provides an OpenAI-compatible API client with retry logic,
// system message support, and concurrent request handling.
package llmapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"time"
)

// LLMClient is an OpenAI-compatible API client with retry support.
type LLMClient struct {
	APIKey     string
	BaseURL    string
	Model      string
	HTTPClient *http.Client

	// Retry configuration
	MaxRetries   int
	RetryDelay   time.Duration
	RetryBackoff float64 // Multiplier for exponential backoff
}

// NewLLMClient creates a new LLM client with the given configuration.
func NewLLMClient(apiKey, baseURL, model string) *LLMClient {
	return &LLMClient{
		APIKey:  apiKey,
		BaseURL: baseURL,
		Model:   model,
		HTTPClient: &http.Client{
			Timeout: 120 * time.Second,
		},
		MaxRetries:   3,
		RetryDelay:   2 * time.Second,
		RetryBackoff: 2.0,
	}
}

// NewLLMClientFromConfig creates a new LLM client from APIConfig.
func NewLLMClientFromConfig(config *APIConfig) *LLMClient {
	return NewLLMClient(config.APIKey, config.BaseURL, config.Model)
}

// Message represents a chat message.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatRequest represents a chat completion request.
type ChatRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature float64   `json:"temperature,omitempty"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
}

// Choice represents a response choice.
type Choice struct {
	Index        int     `json:"index"`
	Message      Message `json:"message"`
	FinishReason string  `json:"finish_reason"`
}

// ChatResponse represents a chat completion response.
type ChatResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
}

// APIError represents an API error response.
type APIError struct {
	ErrorInfo struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error"`
	StatusCode int `json:"-"` // HTTP status code
}

// Complete sends a chat completion request and returns the response content.
// This is a convenience method that uses only a user message.
func (c *LLMClient) Complete(prompt string, timeout time.Duration) (string, error) {
	return c.CompleteWithSystem("", prompt, timeout)
}

// CompleteWithSystem sends a chat completion request with both system and user messages.
func (c *LLMClient) CompleteWithSystem(systemPrompt, userPrompt string, timeout time.Duration) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return c.CompleteWithContext(ctx, systemPrompt, userPrompt)
}

// CompleteWithContext sends a chat completion request using the provided context.
func (c *LLMClient) CompleteWithContext(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	// Build messages
	var messages []Message
	if systemPrompt != "" {
		messages = append(messages, Message{Role: "system", Content: systemPrompt})
	}
	messages = append(messages, Message{Role: "user", Content: userPrompt})

	return c.CompleteMessages(ctx, messages)
}

// CompleteMessages sends a chat completion request with custom messages.
func (c *LLMClient) CompleteMessages(ctx context.Context, messages []Message) (string, error) {
	req := ChatRequest{
		Model:       c.Model,
		Messages:    messages,
		Temperature: 0.7,
	}

	return c.doRequestWithRetry(ctx, req)
}

// doRequestWithRetry executes the API request with retry logic for transient errors.
func (c *LLMClient) doRequestWithRetry(ctx context.Context, req ChatRequest) (string, error) {
	var lastErr error

	for attempt := 0; attempt <= c.MaxRetries; attempt++ {
		if attempt > 0 {
			// Calculate backoff delay
			delay := time.Duration(float64(c.RetryDelay) * math.Pow(c.RetryBackoff, float64(attempt-1)))

			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(delay):
				// Continue with retry
			}
		}

		result, err := c.doRequest(ctx, req)
		if err == nil {
			return result, nil
		}

		// Check if error is retryable
		var apiErr *APIError
		if errors.As(err, &apiErr) {
			if !isRetryable(apiErr.StatusCode) {
				return "", err
			}
			lastErr = err
			continue
		}

		// Network errors are retryable
		lastErr = err
	}

	return "", fmt.Errorf("max retries exceeded: %w", lastErr)
}

// isRetryable returns true if the HTTP status code indicates a retryable error.
func isRetryable(statusCode int) bool {
	switch statusCode {
	case http.StatusTooManyRequests, // 429
		http.StatusInternalServerError, // 500
		http.StatusBadGateway,          // 502
		http.StatusServiceUnavailable,  // 503
		http.StatusGatewayTimeout:      // 504
		return true
	default:
		return false
	}
}

// doRequest executes a single API request.
func (c *LLMClient) doRequest(ctx context.Context, req ChatRequest) (string, error) {
	reqBody, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	url := c.BaseURL + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(reqBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.APIKey)

	// Send request
	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	// Check for HTTP errors
	if resp.StatusCode != http.StatusOK {
		apiErr := &APIError{StatusCode: resp.StatusCode}
		if json.Unmarshal(body, apiErr) == nil && apiErr.ErrorInfo.Message != "" {
			return "", fmt.Errorf("API error (%d): %s: %w", resp.StatusCode, apiErr.ErrorInfo.Message, apiErr)
		}
		apiErr.ErrorInfo.Message = fmt.Sprintf("status %d", resp.StatusCode)
		return "", fmt.Errorf("API error: status %d: %w", resp.StatusCode, apiErr)
	}

	// Parse response
	var chatResp ChatResponse
	if err := json.Unmarshal(body, &chatResp); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	// Extract content
	if len(chatResp.Choices) == 0 {
		return "", errors.New("no choices in response")
	}

	return chatResp.Choices[0].Message.Content, nil
}

// Error implements the error interface for APIError.
func (e *APIError) Error() string {
	return fmt.Sprintf("API error (%d): %s", e.StatusCode, e.ErrorInfo.Message)
}
