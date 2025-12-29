package semantic

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// OpenRouterConfig holds configuration for the OpenRouter embedding client
type OpenRouterConfig struct {
	APIKey     string        // OpenRouter API key (required)
	Model      string        // Model name (default: mistralai/codestral-embed-2505)
	BaseURL    string        // Base URL (default: https://openrouter.ai)
	Timeout    time.Duration // Request timeout
	MaxRetries int           // Maximum retry attempts for rate limiting
}

// Validate checks if the config is valid
func (c *OpenRouterConfig) Validate() error {
	if c.APIKey == "" {
		return fmt.Errorf("OPENROUTER_API_KEY is required")
	}
	return nil
}

// OpenRouterEmbedder generates embeddings using the OpenRouter API
type OpenRouterEmbedder struct {
	config     OpenRouterConfig
	client     *http.Client
	dimensions int
}

// NewOpenRouterEmbedder creates a new OpenRouter embedding client
func NewOpenRouterEmbedder(cfg OpenRouterConfig) (*OpenRouterEmbedder, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	// Apply defaults
	if cfg.Model == "" {
		cfg.Model = "mistralai/codestral-embed-2505"
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://openrouter.ai"
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}
	if cfg.MaxRetries == 0 {
		cfg.MaxRetries = 3
	}

	return &OpenRouterEmbedder{
		config: cfg,
		client: &http.Client{
			Timeout: cfg.Timeout,
		},
	}, nil
}

// NewOpenRouterEmbedderFromEnv creates an OpenRouterEmbedder from environment variables
func NewOpenRouterEmbedderFromEnv() (*OpenRouterEmbedder, error) {
	config := OpenRouterConfig{
		APIKey: os.Getenv("OPENROUTER_API_KEY"),
		Model:  os.Getenv("OPENROUTER_MODEL"),
	}
	return NewOpenRouterEmbedder(config)
}

// openRouterEmbedRequest represents the OpenRouter embeddings API request
type openRouterEmbedRequest struct {
	Model          string      `json:"model"`
	Input          interface{} `json:"input"` // string or []string
	EncodingFormat string      `json:"encoding_format"`
}

// openRouterEmbedData represents a single embedding in the response
type openRouterEmbedData struct {
	Object    string    `json:"object"`
	Index     int       `json:"index"`
	Embedding []float64 `json:"embedding"`
}

// openRouterEmbedResponse represents the OpenRouter embeddings API response
type openRouterEmbedResponse struct {
	Object string                `json:"object"`
	Data   []openRouterEmbedData `json:"data"`
	Model  string                `json:"model"`
	Usage  struct {
		PromptTokens int `json:"prompt_tokens"`
		TotalTokens  int `json:"total_tokens"`
	} `json:"usage"`
}

// openRouterErrorResponse represents an error from the OpenRouter API
type openRouterErrorResponse struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error"`
}

// Embed generates an embedding for a single text
func (e *OpenRouterEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	embeddings, err := e.EmbedBatch(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	if len(embeddings) == 0 {
		return nil, fmt.Errorf("no embedding returned")
	}
	return embeddings[0], nil
}

// EmbedBatch generates embeddings for multiple texts
func (e *OpenRouterEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	// Build request with encoding_format: float
	var input interface{}
	if len(texts) == 1 {
		input = texts[0]
	} else {
		input = texts
	}

	req := openRouterEmbedRequest{
		Model:          e.config.Model,
		Input:          input,
		EncodingFormat: "float",
	}

	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Build HTTP request - OpenRouter uses /api/v1/embeddings
	url := e.config.BaseURL + "/api/v1/embeddings"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+e.config.APIKey)

	// Execute with retries for rate limiting
	var resp *http.Response
	var lastErr error

	maxAttempts := e.config.MaxRetries + 1
	if maxAttempts < 1 {
		maxAttempts = 1
	}

	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			// Exponential backoff for retries
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(time.Duration(attempt*attempt) * 100 * time.Millisecond):
			}

			// Recreate request body for retry
			httpReq, err = http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(reqBody))
			if err != nil {
				return nil, fmt.Errorf("failed to create request: %w", err)
			}
			httpReq.Header.Set("Content-Type", "application/json")
			httpReq.Header.Set("Authorization", "Bearer "+e.config.APIKey)
		}

		resp, lastErr = e.client.Do(httpReq)
		if lastErr == nil && resp.StatusCode != http.StatusTooManyRequests && resp.StatusCode < 500 {
			break
		}
		if resp != nil {
			resp.Body.Close()
		}
	}

	if lastErr != nil {
		return nil, fmt.Errorf("failed to send request: %w", lastErr)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Check for error response
	if resp.StatusCode != http.StatusOK {
		var errResp openRouterErrorResponse
		json.Unmarshal(body, &errResp)
		if errResp.Error.Message != "" {
			return nil, fmt.Errorf("OpenRouter API error: %s", errResp.Error.Message)
		}
		return nil, fmt.Errorf("OpenRouter API error: status %d", resp.StatusCode)
	}

	// Parse successful response
	var embedResp openRouterEmbedResponse
	if err := json.Unmarshal(body, &embedResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Convert to float32 slices
	result := make([][]float32, len(embedResp.Data))
	for _, data := range embedResp.Data {
		embedding := make([]float32, len(data.Embedding))
		for j, v := range data.Embedding {
			embedding[j] = float32(v)
		}
		result[data.Index] = embedding

		// Update dimensions if not set
		if e.dimensions == 0 && len(embedding) > 0 {
			e.dimensions = len(embedding)
		}
	}

	return result, nil
}

// Dimensions returns the embedding dimension (0 if unknown)
func (e *OpenRouterEmbedder) Dimensions() int {
	return e.dimensions
}

// Model returns the model name
func (e *OpenRouterEmbedder) Model() string {
	return e.config.Model
}
