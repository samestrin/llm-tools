package llmapi

import (
	"errors"
	"os"
)

// DefaultBaseURL is the default OpenAI API endpoint.
const DefaultBaseURL = "https://api.openai.com/v1"

// DefaultModel is the default model to use.
const DefaultModel = "gpt-4o-mini"

// APIConfig holds API configuration settings.
type APIConfig struct {
	APIKey  string
	BaseURL string
	Model   string
}

// GetAPIConfig loads API configuration from environment variables with defaults.
// Supported environment variables:
//   - OPENAI_API_KEY or ANTHROPIC_API_KEY for API key (required)
//   - OPENAI_BASE_URL for custom endpoint (default: https://api.openai.com/v1)
//   - OPENAI_MODEL for model selection (default: gpt-4o-mini)
func GetAPIConfig() *APIConfig {
	config := &APIConfig{
		APIKey:  getEnvWithFallbacks("OPENAI_API_KEY", "ANTHROPIC_API_KEY"),
		BaseURL: getEnvOrDefault("OPENAI_BASE_URL", DefaultBaseURL),
		Model:   getEnvOrDefault("OPENAI_MODEL", DefaultModel),
	}
	return config
}

// Validate checks that required configuration values are present.
func (c *APIConfig) Validate() error {
	if c.APIKey == "" {
		return errors.New("API key is required: set OPENAI_API_KEY or ANTHROPIC_API_KEY environment variable")
	}
	return nil
}

// getEnvOrDefault returns the environment variable value or a default.
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvWithFallbacks returns the first non-empty environment variable.
func getEnvWithFallbacks(keys ...string) string {
	for _, key := range keys {
		if value := os.Getenv(key); value != "" {
			return value
		}
	}
	return ""
}
