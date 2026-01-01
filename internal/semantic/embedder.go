package semantic

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// EmbedderConfig holds configuration for the embedding client
type EmbedderConfig struct {
	APIURL     string        // Base URL for embedding API (OpenAI-compatible)
	Model      string        // Model name (e.g., "nomic-embed-text", "mxbai-embed-large", "text-embedding-ada-002")
	APIKey     string        // API key (optional for local models)
	Timeout    time.Duration // Request timeout
	MaxRetries int           // Maximum retry attempts
}

// DefaultEmbedderConfig returns sensible defaults
func DefaultEmbedderConfig() EmbedderConfig {
	return EmbedderConfig{
		APIURL:     "http://localhost:11434", // Ollama default
		Model:      "nomic-embed-text",       // 768 dims, 8192 context - best all-rounder for code
		Timeout:    30 * time.Second,
		MaxRetries: 3,
	}
}

// Embedder generates embeddings using an OpenAI-compatible API
type Embedder struct {
	config     EmbedderConfig
	client     *http.Client
	dimensions int
}

// NewEmbedder creates a new Embedder with the given configuration
func NewEmbedder(cfg EmbedderConfig) (*Embedder, error) {
	if cfg.APIURL == "" {
		cfg.APIURL = "http://localhost:11434"
	}
	if cfg.Model == "" {
		cfg.Model = "nomic-embed-text"
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}

	return &Embedder{
		config: cfg,
		client: &http.Client{
			Timeout: cfg.Timeout,
		},
	}, nil
}

// embeddingRequest represents the OpenAI embeddings API request format
type embeddingRequest struct {
	Input          interface{} `json:"input"` // string or []string
	Model          string      `json:"model"`
	EncodingFormat string      `json:"encoding_format,omitempty"`
}

// embeddingResponse represents the OpenAI embeddings API response format
type embeddingResponse struct {
	Object string `json:"object"`
	Data   []struct {
		Object    string    `json:"object"`
		Index     int       `json:"index"`
		Embedding []float64 `json:"embedding"`
	} `json:"data"`
	Model string `json:"model"`
	Usage struct {
		PromptTokens int `json:"prompt_tokens"`
		TotalTokens  int `json:"total_tokens"`
	} `json:"usage"`
}

// errorResponse represents an API error response
type errorResponse struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error"`
}

// Embed generates an embedding for a single text
func (e *Embedder) Embed(ctx context.Context, text string) ([]float32, error) {
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
func (e *Embedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	// Build request
	var input interface{}
	if len(texts) == 1 {
		input = texts[0]
	} else {
		input = texts
	}

	req := embeddingRequest{
		Input: input,
		Model: e.config.Model,
	}

	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Build HTTP request
	url := e.config.APIURL + "/v1/embeddings"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if e.config.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+e.config.APIKey)
	}

	// Execute with retries
	var resp *http.Response
	var lastErr error

	maxAttempts := e.config.MaxRetries + 1
	if maxAttempts < 1 {
		maxAttempts = 1
	}

	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			// Exponential backoff
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(time.Duration(attempt*attempt) * 100 * time.Millisecond):
			}
		}

		resp, lastErr = e.client.Do(httpReq)
		if lastErr == nil && resp.StatusCode < 500 {
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
		var errResp errorResponse
		json.Unmarshal(body, &errResp)
		if errResp.Error.Message != "" {
			return nil, fmt.Errorf("API error: %s", errResp.Error.Message)
		}
		return nil, fmt.Errorf("API error: status %d", resp.StatusCode)
	}

	// Parse successful response
	var embResp embeddingResponse
	if err := json.Unmarshal(body, &embResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Convert to float32 slices
	result := make([][]float32, len(embResp.Data))
	for _, data := range embResp.Data {
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
func (e *Embedder) Dimensions() int {
	return e.dimensions
}

// Model returns the model name
func (e *Embedder) Model() string {
	return e.config.Model
}
