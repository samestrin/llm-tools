package llmapi

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// DefaultBaseURL is the default OpenAI API endpoint.
const DefaultBaseURL = "https://api.openai.com/v1"

// DefaultModel is the default model to use.
const DefaultModel = "gpt-4o-mini"

// DefaultAPIKeyFile is the default location for file-based API key.
const DefaultAPIKeyFile = ".planning/.config/openai_api_key"

// APIConfig holds API configuration settings.
type APIConfig struct {
	APIKey  string
	BaseURL string
	Model   string
}

// GetAPIConfig loads API configuration from environment variables and files with defaults.
// Priority order for API key:
//  1. OPENAI_API_KEY environment variable
//  2. ANTHROPIC_API_KEY environment variable
//  3. File at .planning/.config/openai_api_key (relative to working directory)
//  4. File at ~/.config/llm-tools/api_key (user home directory)
func GetAPIConfig() *APIConfig {
	config := &APIConfig{
		APIKey:  loadAPIKey(),
		BaseURL: getEnvOrDefault("OPENAI_BASE_URL", DefaultBaseURL),
		Model:   getEnvOrDefault("OPENAI_MODEL", DefaultModel),
	}
	return config
}

// Validate checks that required configuration values are present.
func (c *APIConfig) Validate() error {
	if c.APIKey == "" {
		return errors.New("API key is required: set OPENAI_API_KEY environment variable or create .planning/.config/openai_api_key file")
	}
	return nil
}

// loadAPIKey attempts to load API key from environment or file.
func loadAPIKey() string {
	// Try environment variables first
	if key := getEnvWithFallbacks("OPENAI_API_KEY", "ANTHROPIC_API_KEY"); key != "" {
		return key
	}

	// Try file-based key (relative to current directory)
	if key := loadAPIKeyFromFile(DefaultAPIKeyFile); key != "" {
		return key
	}

	// Try file in user home directory
	if home, err := os.UserHomeDir(); err == nil {
		homeKeyFile := filepath.Join(home, ".config", "llm-tools", "api_key")
		if key := loadAPIKeyFromFile(homeKeyFile); key != "" {
			return key
		}
	}

	return ""
}

// loadAPIKeyFromFile reads an API key from a file, trimming whitespace.
func loadAPIKeyFromFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
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
