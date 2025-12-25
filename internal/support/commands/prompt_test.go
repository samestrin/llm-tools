package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestPromptCommand_NoInput(t *testing.T) {
	cmd := newPromptCmd()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error when no input source provided")
	}
}

func TestPromptCommand_MultipleInputs(t *testing.T) {
	cmd := newPromptCmd()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"--prompt", "test", "--file", "test.txt"})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error when multiple input sources provided")
	}
}

func TestSubstituteTemplate(t *testing.T) {
	tests := []struct {
		template  string
		variables map[string]string
		expected  string
	}{
		{
			template:  "Hello [[name]]!",
			variables: map[string]string{"name": "World"},
			expected:  "Hello World!",
		},
		{
			template:  "[[a]] + [[b]] = [[c]]",
			variables: map[string]string{"a": "1", "b": "2", "c": "3"},
			expected:  "1 + 2 = 3",
		},
		{
			template:  "No variables here",
			variables: map[string]string{},
			expected:  "No variables here",
		},
		{
			template:  "[[missing]] stays",
			variables: map[string]string{"other": "value"},
			expected:  "[[missing]] stays",
		},
	}

	for _, tt := range tests {
		result := substituteTemplate(tt.template, tt.variables)
		if result != tt.expected {
			t.Errorf("substituteTemplate(%q, %v) = %q, want %q",
				tt.template, tt.variables, result, tt.expected)
		}
	}
}

func TestDetectLLMStyle(t *testing.T) {
	tests := []struct {
		binary   string
		expected string
	}{
		{"gemini", "gemini"},
		{"/usr/bin/gemini", "gemini"},
		{"claude", "claude"},
		{"gpt-cli", "openai"},
		{"openai", "openai"},
		{"llama", "llama"},
		{"ollama", "ollama"},
		{"unknown-tool", "generic"},
	}

	for _, tt := range tests {
		result := detectLLMStyle(tt.binary)
		if result != tt.expected {
			t.Errorf("detectLLMStyle(%q) = %q, want %q", tt.binary, result, tt.expected)
		}
	}
}

func TestValidateLLMResponse(t *testing.T) {
	tests := []struct {
		name         string
		response     string
		minLength    int
		mustContain  []string
		noErrorCheck bool
		wantValid    bool
	}{
		{
			name:      "valid response",
			response:  "This is a valid response",
			wantValid: true,
		},
		{
			name:      "too short",
			response:  "short",
			minLength: 100,
			wantValid: false,
		},
		{
			name:        "missing required text",
			response:    "some response",
			mustContain: []string{"required"},
			wantValid:   false,
		},
		{
			name:        "has required text",
			response:    "response with required text",
			mustContain: []string{"required"},
			wantValid:   true,
		},
		{
			name:      "contains error pattern",
			response:  "ERROR: something went wrong",
			wantValid: false,
		},
		{
			name:         "error pattern with check disabled",
			response:     "ERROR: something went wrong but we ignore it",
			noErrorCheck: true,
			wantValid:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid, _ := validateLLMResponse(tt.response, tt.minLength, tt.mustContain, tt.noErrorCheck)
			if valid != tt.wantValid {
				t.Errorf("validateLLMResponse() valid = %v, want %v", valid, tt.wantValid)
			}
		})
	}
}

func TestCheckErrorPatterns(t *testing.T) {
	tests := []struct {
		response string
		hasError bool
	}{
		{"Normal response", false},
		{"ERROR: something failed", true},
		{"FAILED: operation", true},
		{"Exception: null pointer", true},
		{"panic: runtime error", true},
		{"This response mentions error in content but not at start", false},
	}

	for _, tt := range tests {
		hasError, _ := checkErrorPatterns(tt.response)
		if hasError != tt.hasError {
			t.Errorf("checkErrorPatterns(%q) = %v, want %v", tt.response, hasError, tt.hasError)
		}
	}
}

func TestGenerateCacheKey(t *testing.T) {
	key1 := generateCacheKey("gemini", "prompt1", "instruction1")
	key2 := generateCacheKey("gemini", "prompt1", "instruction1")
	key3 := generateCacheKey("gemini", "prompt2", "instruction1")

	if key1 != key2 {
		t.Error("same inputs should generate same cache key")
	}
	if key1 == key3 {
		t.Error("different inputs should generate different cache keys")
	}
	if len(key1) != 64 {
		t.Errorf("cache key should be 64 chars (SHA256 hex), got %d", len(key1))
	}
}

func TestCacheOperations(t *testing.T) {
	key := "test-cache-key-12345"
	response := "cached response content"

	// Save to cache
	saveToCache(key, response)

	// Load from cache
	loaded, cached, age := loadFromCache(key, 3600)
	if !cached {
		t.Error("expected cache hit")
	}
	if loaded != response {
		t.Errorf("loaded = %q, want %q", loaded, response)
	}
	if age < 0 || age > 5 {
		t.Errorf("unexpected cache age: %d", age)
	}

	// Test expired cache (use -1 to ensure it's always expired)
	loaded, cached, _ = loadFromCache(key, -1)
	if cached {
		t.Error("expected cache miss with -1 TTL")
	}

	// Clean up
	cachePath := filepath.Join(getCacheDir(), key)
	os.Remove(cachePath)
}

func TestPromptCommand_TemplateFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "prompt-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create template file
	templateFile := filepath.Join(tmpDir, "template.txt")
	os.WriteFile(templateFile, []byte("Hello [[name]], your age is [[age]]"), 0644)

	cmd := newPromptCmd()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)

	// Missing variables should fail
	cmd.SetArgs([]string{"--template", templateFile, "--var", "name=World"})
	err = cmd.Execute()
	if err == nil {
		t.Error("expected error for missing template variable")
	}
}

func TestPromptCommand_PromptFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "prompt-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Test nonexistent file
	cmd := newPromptCmd()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"--file", "/nonexistent/file.txt"})

	err = cmd.Execute()
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestGetDefaultLLM(t *testing.T) {
	// Without any config, should return "gemini"
	oldEnv := os.Getenv("LLM_SUPPORT_LLM")
	os.Unsetenv("LLM_SUPPORT_LLM")
	defer os.Setenv("LLM_SUPPORT_LLM", oldEnv)

	result := getDefaultLLM()
	// Should return gemini or value from config file
	if result == "" {
		t.Error("getDefaultLLM should not return empty string")
	}
}

func TestGetDefaultLLM_FromEnv(t *testing.T) {
	oldEnv := os.Getenv("LLM_SUPPORT_LLM")
	os.Setenv("LLM_SUPPORT_LLM", "test-llm")
	defer os.Setenv("LLM_SUPPORT_LLM", oldEnv)

	result := getDefaultLLM()
	if result != "test-llm" {
		t.Errorf("getDefaultLLM() = %q, want %q", result, "test-llm")
	}
}
