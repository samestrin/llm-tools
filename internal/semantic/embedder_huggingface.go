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

// HuggingFaceConfig holds configuration for the HuggingFace embedding client
type HuggingFaceConfig struct {
	APIKey       string        // HuggingFace API key (required)
	Model        string        // Model name (default: sentence-transformers/all-MiniLM-L6-v2)
	BaseURL      string        // Base URL (default: https://api-inference.huggingface.co)
	Timeout      time.Duration // Request timeout
	WaitForModel bool          // Wait for model to load if not ready
}

// Validate checks if the config is valid
func (c *HuggingFaceConfig) Validate() error {
	if c.APIKey == "" {
		return fmt.Errorf("HUGGING_FACE_API_KEY is required")
	}
	return nil
}

// HuggingFaceEmbedder generates embeddings using the HuggingFace Inference API
type HuggingFaceEmbedder struct {
	config     HuggingFaceConfig
	client     *http.Client
	dimensions int
}

// NewHuggingFaceEmbedder creates a new HuggingFace embedding client
func NewHuggingFaceEmbedder(cfg HuggingFaceConfig) (*HuggingFaceEmbedder, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	// Apply defaults
	if cfg.Model == "" {
		cfg.Model = "sentence-transformers/all-MiniLM-L6-v2"
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://router.huggingface.co"
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 60 * time.Second // HF models may need time to load
	}

	return &HuggingFaceEmbedder{
		config: cfg,
		client: &http.Client{
			Timeout: cfg.Timeout,
		},
	}, nil
}

// NewHuggingFaceEmbedderFromEnv creates a HuggingFaceEmbedder from environment variables
func NewHuggingFaceEmbedderFromEnv() (*HuggingFaceEmbedder, error) {
	apiKey := os.Getenv("HUGGING_FACE_API_KEY")
	if apiKey == "" {
		apiKey = os.Getenv("HUGGINGFACE_API_KEY") // Alternative without underscore
	}
	if apiKey == "" {
		apiKey = os.Getenv("HF_TOKEN") // Alternative env var
	}

	config := HuggingFaceConfig{
		APIKey:       apiKey,
		Model:        os.Getenv("HF_MODEL"),
		WaitForModel: os.Getenv("HF_WAIT_FOR_MODEL") == "true",
	}
	return NewHuggingFaceEmbedder(config)
}

// huggingFaceRequest represents the HuggingFace feature-extraction request
type huggingFaceRequest struct {
	Inputs  interface{}            `json:"inputs"`
	Options map[string]interface{} `json:"options,omitempty"`
}

// huggingFaceErrorResponse represents an error from the HuggingFace API
type huggingFaceErrorResponse struct {
	Error         string  `json:"error"`
	EstimatedTime float64 `json:"estimated_time,omitempty"`
}

// Embed generates an embedding for a single text
func (e *HuggingFaceEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
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
func (e *HuggingFaceEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
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

	req := huggingFaceRequest{
		Inputs: input,
	}

	if e.config.WaitForModel {
		req.Options = map[string]interface{}{
			"wait_for_model": true,
		}
	}

	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Build HTTP request - use feature-extraction pipeline
	// Format: https://router.huggingface.co/hf-inference/models/{model}/pipeline/feature-extraction
	url := fmt.Sprintf("%s/hf-inference/models/%s/pipeline/feature-extraction", e.config.BaseURL, e.config.Model)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+e.config.APIKey)

	// Execute with retry for model loading
	var resp *http.Response
	maxRetries := 3
	if e.config.WaitForModel {
		maxRetries = 5
	}

	for attempt := 0; attempt < maxRetries; attempt++ {
		resp, err = e.client.Do(httpReq)
		if err != nil {
			return nil, fmt.Errorf("failed to send request: %w", err)
		}

		// Check if model is loading
		if resp.StatusCode == http.StatusServiceUnavailable {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()

			var errResp huggingFaceErrorResponse
			json.Unmarshal(body, &errResp)

			if e.config.WaitForModel && attempt < maxRetries-1 {
				// Wait and retry
				waitTime := time.Duration(errResp.EstimatedTime) * time.Second
				if waitTime < time.Second {
					waitTime = 2 * time.Second
				}
				if waitTime > 30*time.Second {
					waitTime = 30 * time.Second
				}

				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case <-time.After(waitTime):
					continue
				}
			}
		}
		break
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Check for error response
	if resp.StatusCode != http.StatusOK {
		var errResp huggingFaceErrorResponse
		json.Unmarshal(body, &errResp)
		if errResp.Error != "" {
			return nil, fmt.Errorf("HuggingFace API error: %s", errResp.Error)
		}
		return nil, fmt.Errorf("HuggingFace API error: status %d", resp.StatusCode)
	}

	// Parse response - HF can return various formats
	return e.parseEmbeddings(body, len(texts))
}

// parseEmbeddings handles the various response formats from HuggingFace
func (e *HuggingFaceEmbedder) parseEmbeddings(body []byte, expectedCount int) ([][]float32, error) {
	// Try parsing as 1D array first (single embedding): [0.1, 0.2, ...]
	var embedding1D []float64
	if err := json.Unmarshal(body, &embedding1D); err == nil && len(embedding1D) > 0 {
		// Check if values look like embedding floats (not indices)
		if embedding1D[0] >= -10 && embedding1D[0] <= 10 {
			result := make([]float32, len(embedding1D))
			for i, v := range embedding1D {
				result[i] = float32(v)
			}
			if e.dimensions == 0 {
				e.dimensions = len(result)
			}
			return [][]float32{result}, nil
		}
	}

	// Try parsing as 2D array: [[embedding1], [embedding2], ...]
	var embeddings2D [][]float64
	if err := json.Unmarshal(body, &embeddings2D); err == nil && len(embeddings2D) > 0 {
		// Check if this is actually a 2D embedding (tokens x dims) for single input
		if expectedCount == 1 && len(embeddings2D) > 1 && len(embeddings2D[0]) > 0 {
			// This might be token embeddings - we need to mean pool
			// But first check if it's a proper batch
			isFloat := true
			for _, v := range embeddings2D[0] {
				if v < -10 || v > 10 {
					isFloat = false
					break
				}
			}
			if isFloat && len(embeddings2D[0]) > 100 {
				// Looks like token embeddings, mean pool them
				embedding := e.meanPool(embeddings2D)
				return [][]float32{embedding}, nil
			}
		}

		// Normal batch response
		result := make([][]float32, len(embeddings2D))
		for i, emb := range embeddings2D {
			result[i] = make([]float32, len(emb))
			for j, v := range emb {
				result[i][j] = float32(v)
			}
			if e.dimensions == 0 && len(emb) > 0 {
				e.dimensions = len(emb)
			}
		}
		return result, nil
	}

	// Try parsing as 3D array: [[[token1], [token2], ...]] (per-token embeddings)
	var embeddings3D [][][]float64
	if err := json.Unmarshal(body, &embeddings3D); err == nil && len(embeddings3D) > 0 {
		result := make([][]float32, len(embeddings3D))
		for i, tokens := range embeddings3D {
			// Mean pool across tokens
			result[i] = e.meanPool(tokens)
			if e.dimensions == 0 && len(result[i]) > 0 {
				e.dimensions = len(result[i])
			}
		}
		return result, nil
	}

	return nil, fmt.Errorf("failed to parse HuggingFace response")
}

// meanPool performs mean pooling across token embeddings
func (e *HuggingFaceEmbedder) meanPool(tokens [][]float64) []float32 {
	if len(tokens) == 0 {
		return nil
	}

	dims := len(tokens[0])
	result := make([]float32, dims)

	for _, token := range tokens {
		for i, v := range token {
			if i < dims {
				result[i] += float32(v)
			}
		}
	}

	numTokens := float32(len(tokens))
	for i := range result {
		result[i] /= numTokens
	}

	return result
}

// Dimensions returns the embedding dimension (0 if unknown)
func (e *HuggingFaceEmbedder) Dimensions() int {
	return e.dimensions
}

// Model returns the model name
func (e *HuggingFaceEmbedder) Model() string {
	return e.config.Model
}
