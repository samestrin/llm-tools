package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/samestrin/llm-tools/internal/semantic"
)

func TestErrConfigNotFound(t *testing.T) {
	path := "/path/to/missing.yaml"
	err := ErrConfigNotFound(path)

	if err.Type != semantic.ErrTypeNotFound {
		t.Errorf("expected ErrTypeNotFound, got %v", err.Type)
	}
	if !strings.Contains(err.Message, path) {
		t.Errorf("expected message to contain path %q, got %q", path, err.Message)
	}
	if err.Hint == "" {
		t.Error("expected non-empty hint")
	}
}

func TestErrConfigPermissionDenied(t *testing.T) {
	path := "/path/to/protected.yaml"
	cause := errors.New("permission denied")
	err := ErrConfigPermissionDenied(path, cause)

	if err.Type != semantic.ErrTypeNotFound {
		t.Errorf("expected ErrTypeNotFound, got %v", err.Type)
	}
	if !strings.Contains(err.Message, path) {
		t.Errorf("expected message to contain path %q, got %q", path, err.Message)
	}
	if err.Cause != cause {
		t.Errorf("expected cause to be wrapped")
	}
	if !strings.Contains(err.Hint, "permission") {
		t.Errorf("expected hint to mention permissions, got %q", err.Hint)
	}
}

func TestErrConfigEmpty(t *testing.T) {
	path := "/path/to/empty.yaml"
	err := ErrConfigEmpty(path)

	if err.Type != semantic.ErrTypeConfiguration {
		t.Errorf("expected ErrTypeConfiguration, got %v", err.Type)
	}
	if !strings.Contains(err.Message, "empty") {
		t.Errorf("expected message to mention 'empty', got %q", err.Message)
	}
	if !strings.Contains(err.Message, path) {
		t.Errorf("expected message to contain path %q, got %q", path, err.Message)
	}
}

func TestErrConfigInvalidYAML(t *testing.T) {
	path := "/path/to/invalid.yaml"
	cause := errors.New("[5:3] unexpected mapping key")
	err := ErrConfigInvalidYAML(path, cause)

	if err.Type != semantic.ErrTypeInvalidInput {
		t.Errorf("expected ErrTypeInvalidInput, got %v", err.Type)
	}
	if !strings.Contains(err.Message, path) {
		t.Errorf("expected message to contain path %q, got %q", path, err.Message)
	}
	// Should extract line/column from goccy format
	if !strings.Contains(err.Message, "line 5") || !strings.Contains(err.Message, "column 3") {
		t.Errorf("expected message to contain extracted line/column, got %q", err.Message)
	}
	if !strings.Contains(err.Hint, "indentation") {
		t.Errorf("expected hint to mention indentation, got %q", err.Hint)
	}
}

func TestErrConfigInvalidYAML_NoLineInfo(t *testing.T) {
	path := "/path/to/invalid.yaml"
	cause := errors.New("some generic error")
	err := ErrConfigInvalidYAML(path, cause)

	if err.Type != semantic.ErrTypeInvalidInput {
		t.Errorf("expected ErrTypeInvalidInput, got %v", err.Type)
	}
	// Without line info, should still have path
	if !strings.Contains(err.Message, path) {
		t.Errorf("expected message to contain path %q, got %q", path, err.Message)
	}
}

func TestErrConfigMissingSemantic(t *testing.T) {
	path := "/path/to/config.yaml"
	err := ErrConfigMissingSemantic(path)

	if err.Type != semantic.ErrTypeConfiguration {
		t.Errorf("expected ErrTypeConfiguration, got %v", err.Type)
	}
	if !strings.Contains(err.Message, "semantic:") {
		t.Errorf("expected message to mention 'semantic:', got %q", err.Message)
	}
	if !strings.Contains(err.Hint, "semantic:") {
		t.Errorf("expected hint to include example, got %q", err.Hint)
	}
}

func TestErrConfigPathEmpty(t *testing.T) {
	err := ErrConfigPathEmpty()

	if err.Type != semantic.ErrTypeInvalidInput {
		t.Errorf("expected ErrTypeInvalidInput, got %v", err.Type)
	}
	if !strings.Contains(err.Message, "empty") || !strings.Contains(err.Message, "whitespace") {
		t.Errorf("expected message to mention empty/whitespace, got %q", err.Message)
	}
}

func TestErrProfileNotFound(t *testing.T) {
	name := "staging"
	available := []string{"code", "docs", "memory"}
	err := ErrProfileNotFound(name, available)

	if err.Type != semantic.ErrTypeNotFound {
		t.Errorf("expected ErrTypeNotFound, got %v", err.Type)
	}
	if !strings.Contains(err.Message, name) {
		t.Errorf("expected message to contain profile name %q, got %q", name, err.Message)
	}
	// Hint should list available profiles
	for _, p := range available {
		if !strings.Contains(err.Hint, p) {
			t.Errorf("expected hint to list available profile %q, got %q", p, err.Hint)
		}
	}
}

func TestErrProfileNotFound_EmptyList(t *testing.T) {
	name := "myprofile"
	err := ErrProfileNotFound(name, []string{})

	if err.Type != semantic.ErrTypeNotFound {
		t.Errorf("expected ErrTypeNotFound, got %v", err.Type)
	}
	// Hint should indicate no profiles defined
	if !strings.Contains(err.Hint, "No profiles") || !strings.Contains(err.Hint, "built-in") {
		t.Errorf("expected hint to indicate no profiles and suggest built-ins, got %q", err.Hint)
	}
}

func TestErrProfileInvalidValue(t *testing.T) {
	err := ErrProfileInvalidValue("dev", "top_k", "integer", "string")

	if err.Type != semantic.ErrTypeInvalidInput {
		t.Errorf("expected ErrTypeInvalidInput, got %v", err.Type)
	}
	if !strings.Contains(err.Message, "top_k") {
		t.Errorf("expected message to contain field name, got %q", err.Message)
	}
	if !strings.Contains(err.Message, "dev") {
		t.Errorf("expected message to contain profile name, got %q", err.Message)
	}
	if !strings.Contains(err.Message, "integer") || !strings.Contains(err.Message, "string") {
		t.Errorf("expected message to contain types, got %q", err.Message)
	}
}

func TestErrNoProfilesDefined(t *testing.T) {
	err := ErrNoProfilesDefined()

	if err.Type != semantic.ErrTypeConfiguration {
		t.Errorf("expected ErrTypeConfiguration, got %v", err.Type)
	}
	if !strings.Contains(err.Message, "no profiles") {
		t.Errorf("expected message to mention 'no profiles', got %q", err.Message)
	}
	if !strings.Contains(err.Hint, "built-in") {
		t.Errorf("expected hint to mention built-in profiles, got %q", err.Hint)
	}
}

func TestExtractLineColumn(t *testing.T) {
	tests := []struct {
		errMsg   string
		expected string
	}{
		{"[5:3] unexpected mapping key", "line 5, column 3"},
		{"[12:25] sequence end token", "line 12, column 25"},
		{"some other error", ""},
		{"", ""},
	}

	for _, tt := range tests {
		var err error
		if tt.errMsg != "" {
			err = errors.New(tt.errMsg)
		}
		result := extractLineColumn(err)
		if result != tt.expected {
			t.Errorf("extractLineColumn(%q) = %q, want %q", tt.errMsg, result, tt.expected)
		}
	}
}

func TestWrapReadError_NotExist(t *testing.T) {
	path := "/nonexistent/file.yaml"
	_, osErr := os.ReadFile(path)
	err := WrapReadError(path, osErr)

	if err.Type != semantic.ErrTypeNotFound {
		t.Errorf("expected ErrTypeNotFound, got %v", err.Type)
	}
	if !strings.Contains(err.Message, "not found") {
		t.Errorf("expected 'not found' in message, got %q", err.Message)
	}
}

func TestWrapReadError_PermissionDenied(t *testing.T) {
	// Skip on systems where we can't reliably test permission errors
	if os.Getuid() == 0 {
		t.Skip("cannot test permission denied as root")
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "protected.yaml")

	// Create file and remove read permission
	if err := os.WriteFile(path, []byte("test"), 0000); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	defer os.Chmod(path, 0644) // Cleanup

	_, osErr := os.ReadFile(path)
	if osErr == nil {
		t.Skip("system did not produce permission error")
	}

	err := WrapReadError(path, osErr)
	if err.Type != semantic.ErrTypeNotFound {
		t.Errorf("expected ErrTypeNotFound for permission denied, got %v", err.Type)
	}
	if !strings.Contains(err.Message, "cannot read") {
		t.Errorf("expected 'cannot read' in message, got %q", err.Message)
	}
}

func TestLoadConfig_EmptyPath(t *testing.T) {
	_, err := LoadConfig("")
	if err == nil {
		t.Error("expected error for empty path")
	}

	semErr, ok := err.(*semantic.SemanticError)
	if !ok {
		t.Errorf("expected *SemanticError, got %T", err)
		return
	}

	if semErr.Type != semantic.ErrTypeInvalidInput {
		t.Errorf("expected ErrTypeInvalidInput, got %v", semErr.Type)
	}
}

func TestLoadConfig_WhitespacePath(t *testing.T) {
	_, err := LoadConfig("   ")
	if err == nil {
		t.Error("expected error for whitespace-only path")
	}

	semErr, ok := err.(*semantic.SemanticError)
	if !ok {
		t.Errorf("expected *SemanticError, got %T", err)
		return
	}

	if semErr.Type != semantic.ErrTypeInvalidInput {
		t.Errorf("expected ErrTypeInvalidInput, got %v", semErr.Type)
	}
}

func TestLoadConfig_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "empty.yaml")

	// Create empty file
	if err := os.WriteFile(configPath, []byte{}, 0644); err != nil {
		t.Fatalf("failed to create empty file: %v", err)
	}

	_, err := LoadConfig(configPath)
	if err == nil {
		t.Error("expected error for empty file")
	}

	semErr, ok := err.(*semantic.SemanticError)
	if !ok {
		t.Errorf("expected *SemanticError, got %T", err)
		return
	}

	if semErr.Type != semantic.ErrTypeConfiguration {
		t.Errorf("expected ErrTypeConfiguration, got %v", semErr.Type)
	}
	if !strings.Contains(semErr.Message, "empty") {
		t.Errorf("expected 'empty' in message, got %q", semErr.Message)
	}
}

func TestLoadConfig_InvalidYAML_ReturnsSemanticError(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "invalid.yaml")
	content := `
semantic:
  enabled: true
  invalid: [broken
`
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	_, err := LoadConfig(configPath)
	if err == nil {
		t.Error("expected error for invalid YAML")
	}

	semErr, ok := err.(*semantic.SemanticError)
	if !ok {
		t.Errorf("expected *SemanticError, got %T", err)
		return
	}

	if semErr.Type != semantic.ErrTypeInvalidInput {
		t.Errorf("expected ErrTypeInvalidInput, got %v", semErr.Type)
	}
}

func TestLoadConfig_NotFound_ReturnsSemanticError(t *testing.T) {
	_, err := LoadConfig("/definitely/not/a/real/path/config.yaml")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}

	semErr, ok := err.(*semantic.SemanticError)
	if !ok {
		t.Errorf("expected *SemanticError, got %T", err)
		return
	}

	if semErr.Type != semantic.ErrTypeNotFound {
		t.Errorf("expected ErrTypeNotFound, got %v", semErr.Type)
	}
}

func TestSemanticError_FormatWithHint(t *testing.T) {
	err := ErrConfigNotFound("/path/to/config.yaml")
	formatted := err.FormatWithHint()

	if !strings.Contains(formatted, err.Message) {
		t.Errorf("formatted output should contain message")
	}
	if !strings.Contains(formatted, err.Hint) {
		t.Errorf("formatted output should contain hint")
	}
	if !strings.Contains(formatted, "Hint:") {
		t.Errorf("formatted output should have 'Hint:' prefix")
	}
}

func TestSemanticError_Unwrap(t *testing.T) {
	cause := errors.New("underlying error")
	err := ErrConfigPermissionDenied("/path", cause)

	unwrapped := err.Unwrap()
	if unwrapped != cause {
		t.Errorf("Unwrap() should return the cause error")
	}
}
