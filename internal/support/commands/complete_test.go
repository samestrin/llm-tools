package commands

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompleteCommand_NoInput(t *testing.T) {
	// Reset flags
	completePrompt = ""
	completeFile = ""
	completeTemplate = ""

	cmd := newCompleteCmd()
	cmd.SetArgs([]string{})

	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)

	err := cmd.Execute()
	if err == nil {
		t.Error("Expected error when no input provided")
	}
	if !strings.Contains(err.Error(), "must specify one of") {
		t.Errorf("Expected 'must specify one of' error, got: %v", err)
	}
}

func TestCompleteCommand_MultipleInputs(t *testing.T) {
	// Reset flags
	completePrompt = "test"
	completeFile = "test.txt"
	completeTemplate = ""

	cmd := newCompleteCmd()
	cmd.SetArgs([]string{"--prompt", "test", "--file", "test.txt"})

	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)

	err := cmd.Execute()
	if err == nil {
		t.Error("Expected error when multiple inputs provided")
	}
	if !strings.Contains(err.Error(), "cannot specify multiple prompt sources") {
		t.Errorf("Expected 'cannot specify multiple prompt sources' error, got: %v", err)
	}
}

func TestCompleteCommand_MissingAPIKey(t *testing.T) {
	// Save and clear env vars
	oldKey := os.Getenv("OPENAI_API_KEY")
	oldAnthropicKey := os.Getenv("ANTHROPIC_API_KEY")
	os.Unsetenv("OPENAI_API_KEY")
	os.Unsetenv("ANTHROPIC_API_KEY")
	defer func() {
		if oldKey != "" {
			os.Setenv("OPENAI_API_KEY", oldKey)
		}
		if oldAnthropicKey != "" {
			os.Setenv("ANTHROPIC_API_KEY", oldAnthropicKey)
		}
	}()

	// Reset flags
	completePrompt = ""
	completeFile = ""
	completeTemplate = ""

	cmd := newCompleteCmd()
	cmd.SetArgs([]string{"--prompt", "test"})

	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)

	err := cmd.Execute()
	if err == nil {
		t.Error("Expected error when API key is missing")
	}
	if !strings.Contains(err.Error(), "API key is required") {
		t.Errorf("Expected 'API key is required' error, got: %v", err)
	}
}

func TestCompleteCommand_FileInput(t *testing.T) {
	// Create temp file with prompt
	tmpDir := t.TempDir()
	promptFile := filepath.Join(tmpDir, "prompt.txt")
	if err := os.WriteFile(promptFile, []byte("Test prompt content"), 0644); err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	// Save and clear env vars
	oldKey := os.Getenv("OPENAI_API_KEY")
	os.Unsetenv("OPENAI_API_KEY")
	defer func() {
		if oldKey != "" {
			os.Setenv("OPENAI_API_KEY", oldKey)
		}
	}()

	// Reset flags
	completePrompt = ""
	completeFile = ""
	completeTemplate = ""

	cmd := newCompleteCmd()
	cmd.SetArgs([]string{"--file", promptFile})

	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)

	// Should fail due to missing API key, not file reading
	err := cmd.Execute()
	if err == nil {
		t.Error("Expected error (missing API key)")
	}
	if strings.Contains(err.Error(), "failed to read prompt file") {
		t.Errorf("File should have been read successfully, got: %v", err)
	}
}

func TestCompleteCommand_FileNotFound(t *testing.T) {
	// Reset flags
	completePrompt = ""
	completeFile = ""
	completeTemplate = ""

	cmd := newCompleteCmd()
	cmd.SetArgs([]string{"--file", "/nonexistent/path/file.txt"})

	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)

	err := cmd.Execute()
	if err == nil {
		t.Error("Expected error when file not found")
	}
	if !strings.Contains(err.Error(), "failed to read prompt file") {
		t.Errorf("Expected 'failed to read prompt file' error, got: %v", err)
	}
}

func TestCompleteCommand_TemplateSubstitution(t *testing.T) {
	// Create temp template file
	tmpDir := t.TempDir()
	templateFile := filepath.Join(tmpDir, "template.txt")
	templateContent := "Hello [[name]], please [[action]]"
	if err := os.WriteFile(templateFile, []byte(templateContent), 0644); err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	// Save and clear env vars
	oldKey := os.Getenv("OPENAI_API_KEY")
	os.Unsetenv("OPENAI_API_KEY")
	defer func() {
		if oldKey != "" {
			os.Setenv("OPENAI_API_KEY", oldKey)
		}
	}()

	// Reset flags
	completePrompt = ""
	completeFile = ""
	completeTemplate = ""
	completeVars = nil

	cmd := newCompleteCmd()
	cmd.SetArgs([]string{
		"--template", templateFile,
		"--var", "name=World",
		"--var", "action=test",
	})

	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)

	// Should fail due to missing API key, not template parsing
	err := cmd.Execute()
	if err == nil {
		t.Error("Expected error (missing API key)")
	}
	// Should not fail on template substitution
	if strings.Contains(err.Error(), "template variable") {
		t.Errorf("Template substitution should have succeeded, got: %v", err)
	}
}

func TestCompleteCommand_TemplateMissingVar(t *testing.T) {
	// Create temp template file
	tmpDir := t.TempDir()
	templateFile := filepath.Join(tmpDir, "template.txt")
	templateContent := "Hello [[name]], please [[action]]"
	if err := os.WriteFile(templateFile, []byte(templateContent), 0644); err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	// Reset flags
	completePrompt = ""
	completeFile = ""
	completeTemplate = ""
	completeVars = nil

	cmd := newCompleteCmd()
	cmd.SetArgs([]string{
		"--template", templateFile,
		"--var", "name=World",
		// Missing "action" variable
	})

	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)

	err := cmd.Execute()
	if err == nil {
		t.Error("Expected error when template variable is missing")
	}
	if !strings.Contains(err.Error(), "template variable(s) not provided") {
		t.Errorf("Expected 'template variable(s) not provided' error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "action") {
		t.Errorf("Error should mention missing variable 'action', got: %v", err)
	}
}

func TestCompleteCommand_TemplateFileVar(t *testing.T) {
	// Create temp files
	tmpDir := t.TempDir()
	templateFile := filepath.Join(tmpDir, "template.txt")
	varFile := filepath.Join(tmpDir, "var.txt")

	templateContent := "Code: [[code]]"
	varContent := "function test() {}"

	if err := os.WriteFile(templateFile, []byte(templateContent), 0644); err != nil {
		t.Fatalf("Failed to create template file: %v", err)
	}
	if err := os.WriteFile(varFile, []byte(varContent), 0644); err != nil {
		t.Fatalf("Failed to create var file: %v", err)
	}

	// Save and clear env vars
	oldKey := os.Getenv("OPENAI_API_KEY")
	os.Unsetenv("OPENAI_API_KEY")
	defer func() {
		if oldKey != "" {
			os.Setenv("OPENAI_API_KEY", oldKey)
		}
	}()

	// Reset flags
	completePrompt = ""
	completeFile = ""
	completeTemplate = ""
	completeVars = nil

	cmd := newCompleteCmd()
	cmd.SetArgs([]string{
		"--template", templateFile,
		"--var", "code=@" + varFile,
	})

	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)

	// Should fail due to missing API key, not file reading
	err := cmd.Execute()
	if err == nil {
		t.Error("Expected error (missing API key)")
	}
	if strings.Contains(err.Error(), "failed to read variable file") {
		t.Errorf("Variable file should have been read successfully, got: %v", err)
	}
}

func TestSubstituteCompleteTemplate(t *testing.T) {
	tests := []struct {
		name      string
		template  string
		variables map[string]string
		expected  string
	}{
		{
			name:      "simple substitution",
			template:  "Hello [[name]]!",
			variables: map[string]string{"name": "World"},
			expected:  "Hello World!",
		},
		{
			name:      "multiple variables",
			template:  "[[greeting]] [[name]], [[message]]",
			variables: map[string]string{"greeting": "Hi", "name": "Alice", "message": "welcome!"},
			expected:  "Hi Alice, welcome!",
		},
		{
			name:      "repeated variable",
			template:  "[[x]] + [[x]] = 2[[x]]",
			variables: map[string]string{"x": "a"},
			expected:  "a + a = 2a",
		},
		{
			name:      "no variables",
			template:  "Plain text",
			variables: map[string]string{},
			expected:  "Plain text",
		},
		{
			name:      "multiline content",
			template:  "Code:\n[[code]]\nEnd",
			variables: map[string]string{"code": "line1\nline2"},
			expected:  "Code:\nline1\nline2\nEnd",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := substituteCompleteTemplate(tt.template, tt.variables)
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestCompleteResult_JSONOutput(t *testing.T) {
	result := CompleteResult{
		Status:       "SUCCESS",
		Attempts:     1,
		Model:        "gpt-4o-mini",
		OutputLength: 42,
		Response:     "Hello!",
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("Failed to marshal result: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	if parsed["status"] != "SUCCESS" {
		t.Errorf("Expected status SUCCESS, got %v", parsed["status"])
	}
	if parsed["model"] != "gpt-4o-mini" {
		t.Errorf("Expected model gpt-4o-mini, got %v", parsed["model"])
	}
	if parsed["response"] != "Hello!" {
		t.Errorf("Expected response 'Hello!', got %v", parsed["response"])
	}
}

func TestCompleteResult_MinimalOutput(t *testing.T) {
	attempts := 2
	outputLen := 100
	result := CompleteResult{
		S:  "SUCCESS",
		A:  &attempts,
		M:  "gpt-4o",
		OL: &outputLen,
		R:  "Response text",
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("Failed to marshal result: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	if parsed["s"] != "SUCCESS" {
		t.Errorf("Expected s SUCCESS, got %v", parsed["s"])
	}
	if parsed["m"] != "gpt-4o" {
		t.Errorf("Expected m gpt-4o, got %v", parsed["m"])
	}
}

func TestCompleteCommand_Flags(t *testing.T) {
	cmd := newCompleteCmd()

	// Check flag definitions
	flags := []string{
		"prompt", "file", "template", "var", "system",
		"model", "temperature", "max-tokens", "timeout",
		"retries", "retry-delay", "output", "strip", "json", "min",
	}

	for _, flag := range flags {
		if cmd.Flags().Lookup(flag) == nil {
			t.Errorf("Expected flag --%s to be defined", flag)
		}
	}
}

func TestCompleteCommand_DefaultValues(t *testing.T) {
	cmd := newCompleteCmd()

	// Check default values
	tempFlag := cmd.Flags().Lookup("temperature")
	if tempFlag.DefValue != "0.7" {
		t.Errorf("Expected temperature default 0.7, got %s", tempFlag.DefValue)
	}

	timeoutFlag := cmd.Flags().Lookup("timeout")
	if timeoutFlag.DefValue != "120" {
		t.Errorf("Expected timeout default 120, got %s", timeoutFlag.DefValue)
	}

	retriesFlag := cmd.Flags().Lookup("retries")
	if retriesFlag.DefValue != "3" {
		t.Errorf("Expected retries default 3, got %s", retriesFlag.DefValue)
	}

	retryDelayFlag := cmd.Flags().Lookup("retry-delay")
	if retryDelayFlag.DefValue != "2" {
		t.Errorf("Expected retry-delay default 2, got %s", retryDelayFlag.DefValue)
	}
}
