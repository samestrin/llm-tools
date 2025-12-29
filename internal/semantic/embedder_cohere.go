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

// CohereConfig holds configuration for the Cohere embedding client
type CohereConfig struct {
	APIKey    string        // Cohere API key (required)
	Model     string        // Model name (default: embed-english-v3.0)
	BaseURL   string        // Base URL (default: https://api.cohere.com)
	InputType string        // Input type: search_document, search_query, classification, clustering
	Timeout   time.Duration // Request timeout
}

// Validate checks if the config is valid
func (c *CohereConfig) Validate() error {
	if c.APIKey == "" {
		return fmt.Errorf("COHERE_API_KEY is required")
	}
	return nil
}

// CohereEmbedder generates embeddings using the Cohere API
type CohereEmbedder struct {
	config     CohereConfig
	client     *http.Client
	dimensions int
}

// NewCohereEmbedder creates a new Cohere embedding client
func NewCohereEmbedder(cfg CohereConfig) (*CohereEmbedder, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	// Apply defaults
	if cfg.Model == "" {
		cfg.Model = "embed-english-v3.0"
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.cohere.com"
	}
	if cfg.InputType == "" {
		cfg.InputType = "search_document"
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}

	return &CohereEmbedder{
		config: cfg,
		client: &http.Client{
			Timeout: cfg.Timeout,
		},
	}, nil
}

// NewCohereEmbedderFromEnv creates a CohereEmbedder from environment variables
func NewCohereEmbedderFromEnv() (*CohereEmbedder, error) {
	config := CohereConfig{
		APIKey:    os.Getenv("COHERE_API_KEY"),
		Model:     os.Getenv("COHERE_MODEL"),
		InputType: os.Getenv("COHERE_INPUT_TYPE"),
	}
	return NewCohereEmbedder(config)
}

// cohereEmbedRequest represents the Cohere embed API request
type cohereEmbedRequest struct {
	Model     string   `json:"model"`
	Texts     []string `json:"texts"`
	InputType string   `json:"input_type"`
	Truncate  string   `json:"truncate,omitempty"`
}

// cohereEmbedResponse represents the Cohere embed API response
type cohereEmbedResponse struct {
	ID           string      `json:"id"`
	Embeddings   [][]float64 `json:"embeddings"`
	Texts        []string    `json:"texts"`
	ResponseType string      `json:"response_type"`
	Meta         struct {
		APIVersion struct {
			Version string `json:"version"`
		} `json:"api_version"`
		BilledUnits struct {
			InputTokens int `json:"input_tokens"`
		} `json:"billed_units"`
	} `json:"meta"`
}

// cohereErrorResponse represents an error from the Cohere API
type cohereErrorResponse struct {
	Message string `json:"message"`
}

// Embed generates an embedding for a single text
func (e *CohereEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
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
func (e *CohereEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	// Build request
	req := cohereEmbedRequest{
		Model:     e.config.Model,
		Texts:     texts,
		InputType: e.config.InputType,
		Truncate:  "END", // Truncate from end if too long
	}

	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Build HTTP request
	url := e.config.BaseURL + "/v1/embed"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+e.config.APIKey)

	// Execute request
	resp, err := e.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Check for error response
	if resp.StatusCode != http.StatusOK {
		var errResp cohereErrorResponse
		json.Unmarshal(body, &errResp)
		if errResp.Message != "" {
			return nil, fmt.Errorf("Cohere API error: %s", errResp.Message)
		}
		return nil, fmt.Errorf("Cohere API error: status %d", resp.StatusCode)
	}

	// Parse successful response
	var embedResp cohereEmbedResponse
	if err := json.Unmarshal(body, &embedResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Convert to float32 slices
	result := make([][]float32, len(embedResp.Embeddings))
	for i, embedding := range embedResp.Embeddings {
		result[i] = make([]float32, len(embedding))
		for j, v := range embedding {
			result[i][j] = float32(v)
		}

		// Update dimensions if not set
		if e.dimensions == 0 && len(embedding) > 0 {
			e.dimensions = len(embedding)
		}
	}

	return result, nil
}

// Dimensions returns the embedding dimension (0 if unknown)
func (e *CohereEmbedder) Dimensions() int {
	return e.dimensions
}

// Model returns the model name
func (e *CohereEmbedder) Model() string {
	return e.config.Model
}

// SetInputType allows changing the input type for different use cases
// Use "search_query" for queries, "search_document" for documents to be searched
func (e *CohereEmbedder) SetInputType(inputType string) {
	e.config.InputType = inputType
}
