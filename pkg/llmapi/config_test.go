package llmapi

import (
	"os"
	"testing"
)

func TestGetAPIConfig_EnvVars(t *testing.T) {
	// Save original env vars
	origKey := os.Getenv("OPENAI_API_KEY")
	origURL := os.Getenv("OPENAI_BASE_URL")
	origModel := os.Getenv("OPENAI_MODEL")
	defer func() {
		os.Setenv("OPENAI_API_KEY", origKey)
		os.Setenv("OPENAI_BASE_URL", origURL)
		os.Setenv("OPENAI_MODEL", origModel)
	}()

	// Set test env vars
	os.Setenv("OPENAI_API_KEY", "test-api-key")
	os.Setenv("OPENAI_BASE_URL", "https://test.api.com/v1")
	os.Setenv("OPENAI_MODEL", "test-model")

	config := GetAPIConfig()

	if config.APIKey != "test-api-key" {
		t.Errorf("expected APIKey 'test-api-key', got %s", config.APIKey)
	}
	if config.BaseURL != "https://test.api.com/v1" {
		t.Errorf("expected BaseURL 'https://test.api.com/v1', got %s", config.BaseURL)
	}
	if config.Model != "test-model" {
		t.Errorf("expected Model 'test-model', got %s", config.Model)
	}
}

func TestGetAPIConfig_Defaults(t *testing.T) {
	// Save and clear env vars
	origKey := os.Getenv("OPENAI_API_KEY")
	origURL := os.Getenv("OPENAI_BASE_URL")
	origModel := os.Getenv("OPENAI_MODEL")
	defer func() {
		os.Setenv("OPENAI_API_KEY", origKey)
		os.Setenv("OPENAI_BASE_URL", origURL)
		os.Setenv("OPENAI_MODEL", origModel)
	}()

	os.Unsetenv("OPENAI_API_KEY")
	os.Unsetenv("OPENAI_BASE_URL")
	os.Unsetenv("OPENAI_MODEL")

	config := GetAPIConfig()

	// Default values
	if config.BaseURL != "https://api.openai.com/v1" {
		t.Errorf("expected default BaseURL 'https://api.openai.com/v1', got %s", config.BaseURL)
	}
	if config.Model != "gpt-4o-mini" {
		t.Errorf("expected default Model 'gpt-4o-mini', got %s", config.Model)
	}
}

func TestAPIConfig_Validate_Missing(t *testing.T) {
	config := &APIConfig{
		APIKey:  "",
		BaseURL: "https://api.openai.com/v1",
		Model:   "gpt-4o-mini",
	}

	err := config.Validate()
	if err == nil {
		t.Error("expected error for missing API key")
	}
}

func TestAPIConfig_Validate_Valid(t *testing.T) {
	config := &APIConfig{
		APIKey:  "test-key",
		BaseURL: "https://api.openai.com/v1",
		Model:   "gpt-4o-mini",
	}

	err := config.Validate()
	if err != nil {
		t.Errorf("expected no error for valid config, got: %v", err)
	}
}

func TestGetAPIConfig_AnthropicEnvVars(t *testing.T) {
	// Save original env vars
	origKey := os.Getenv("OPENAI_API_KEY")
	origAnthropicKey := os.Getenv("ANTHROPIC_API_KEY")
	defer func() {
		os.Setenv("OPENAI_API_KEY", origKey)
		os.Setenv("ANTHROPIC_API_KEY", origAnthropicKey)
	}()

	// Clear OpenAI key, set Anthropic key
	os.Unsetenv("OPENAI_API_KEY")
	os.Setenv("ANTHROPIC_API_KEY", "anthropic-test-key")

	config := GetAPIConfig()

	// Should fallback to Anthropic key
	if config.APIKey != "anthropic-test-key" {
		t.Errorf("expected APIKey to fallback to ANTHROPIC_API_KEY, got %s", config.APIKey)
	}
}
