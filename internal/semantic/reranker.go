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

// RerankerInterface defines the interface for reranking search results
type RerankerInterface interface {
	// Rerank takes a query and candidate documents, returns relevance scores
	// Scores are typically 0-1 where higher means more relevant
	Rerank(ctx context.Context, query string, documents []string) ([]float32, error)

	// Model returns the reranker model name
	Model() string
}

// RerankerConfig holds configuration for the reranker client
type RerankerConfig struct {
	APIURL      string        // Base URL for reranker API (Cohere-compatible)
	Model       string        // Model name (e.g., "Qwen/Qwen3-Reranker-0.6B")
	APIKey      string        // API key (optional for local models)
	Timeout     time.Duration // Request timeout
	MaxRetries  int           // Maximum retry attempts
	Instruction string        // Custom instruction for reranking (optional)
}

// DefaultRerankerConfig returns sensible defaults
func DefaultRerankerConfig() RerankerConfig {
	return RerankerConfig{
		APIURL:      "", // Empty means reranking disabled
		Model:       "Qwen/Qwen3-Reranker-0.6B",
		Timeout:     60 * time.Second, // Reranking can be slower than embedding
		MaxRetries:  3,
		Instruction: "Given a query, retrieve relevant code or documentation that answers the query",
	}
}

// Reranker reranks search results using a Cohere-compatible API
type Reranker struct {
	config RerankerConfig
	client *http.Client
}

// NewReranker creates a new Reranker with the given configuration
func NewReranker(cfg RerankerConfig) (*Reranker, error) {
	if cfg.APIURL == "" {
		return nil, fmt.Errorf("reranker API URL is required")
	}
	if cfg.Model == "" {
		cfg.Model = "Qwen/Qwen3-Reranker-0.6B"
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 60 * time.Second
	}
	if cfg.Instruction == "" {
		cfg.Instruction = "Given a query, retrieve relevant code or documentation that answers the query"
	}

	return &Reranker{
		config: cfg,
		client: &http.Client{
			Timeout: cfg.Timeout,
		},
	}, nil
}

// rerankRequest is the Cohere-compatible request format
type rerankRequest struct {
	Query       string   `json:"query"`
	Documents   []string `json:"documents"`
	Model       string   `json:"model,omitempty"`
	TopN        int      `json:"top_n,omitempty"`
	Instruction string   `json:"instruction,omitempty"`
}

// rerankResponse is the Cohere-compatible response format
type rerankResponse struct {
	Results []rerankResult `json:"results"`
	Model   string         `json:"model,omitempty"`
}

type rerankResult struct {
	Index          int     `json:"index"`
	RelevanceScore float64 `json:"relevance_score"`
	Document       string  `json:"document,omitempty"`
}

// Rerank reranks documents by relevance to the query
func (r *Reranker) Rerank(ctx context.Context, query string, documents []string) ([]float32, error) {
	if len(documents) == 0 {
		return nil, nil
	}

	// Build request
	req := rerankRequest{
		Query:       query,
		Documents:   documents,
		Model:       r.config.Model,
		Instruction: r.config.Instruction,
	}

	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Build HTTP request
	url := r.config.APIURL + "/v1/rerank"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if r.config.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+r.config.APIKey)
	}

	// Execute with retries
	var resp *http.Response
	var lastErr error

	maxAttempts := r.config.MaxRetries + 1
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

		resp, lastErr = r.client.Do(httpReq)
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
		return nil, fmt.Errorf("reranker API error: status %d, body: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var rerankResp rerankResponse
	if err := json.Unmarshal(body, &rerankResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Convert to scores array (preserving original order)
	// The API returns results sorted by score, but we need them in original document order
	scores := make([]float32, len(documents))
	for _, result := range rerankResp.Results {
		if result.Index >= 0 && result.Index < len(documents) {
			scores[result.Index] = float32(result.RelevanceScore)
		}
	}

	return scores, nil
}

// Model returns the reranker model name
func (r *Reranker) Model() string {
	return r.config.Model
}
