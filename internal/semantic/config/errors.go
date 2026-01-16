// Package config provides configuration file support for llm-semantic commands.
// This file contains config-specific error constructors following the SemanticError pattern.
package config

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/samestrin/llm-tools/internal/semantic"
)

// ErrConfigNotFound creates an error for when the config file doesn't exist
func ErrConfigNotFound(path string) *semantic.SemanticError {
	return &semantic.SemanticError{
		Type:    semantic.ErrTypeNotFound,
		Message: fmt.Sprintf("config file not found: %s", path),
		Hint:    "Create a config file or specify an existing file path with --config.",
	}
}

// ErrConfigPermissionDenied creates an error for when the config file cannot be read
func ErrConfigPermissionDenied(path string, cause error) *semantic.SemanticError {
	return &semantic.SemanticError{
		Type:    semantic.ErrTypeNotFound,
		Message: fmt.Sprintf("cannot read config file: %s", path),
		Cause:   cause,
		Hint:    "Check file permissions with 'ls -la' and ensure the file is readable.",
	}
}

// ErrConfigEmpty creates an error for when the config file is empty
func ErrConfigEmpty(path string) *semantic.SemanticError {
	return &semantic.SemanticError{
		Type:    semantic.ErrTypeConfiguration,
		Message: fmt.Sprintf("config file is empty: %s", path),
		Hint:    "Add configuration content or remove the --config flag to use defaults.",
	}
}

// ErrConfigInvalidYAML creates an error for invalid YAML syntax
// It extracts line/column information from goccy/go-yaml errors when available
func ErrConfigInvalidYAML(path string, cause error) *semantic.SemanticError {
	// Try to extract line/column from goccy/go-yaml error format
	// Format: "[<line>:<col>] <message>"
	lineCol := extractLineColumn(cause)

	var message string
	if lineCol != "" {
		message = fmt.Sprintf("invalid YAML syntax in %s at %s", path, lineCol)
	} else {
		message = fmt.Sprintf("invalid YAML syntax in %s", path)
	}

	return &semantic.SemanticError{
		Type:    semantic.ErrTypeInvalidInput,
		Message: message,
		Cause:   cause,
		Hint:    "Check for proper indentation, missing colons, or unclosed quotes near the indicated location.",
	}
}

// ErrConfigMissingSemantic creates an error for when the semantic: section is missing
func ErrConfigMissingSemantic(path string) *semantic.SemanticError {
	return &semantic.SemanticError{
		Type:    semantic.ErrTypeConfiguration,
		Message: fmt.Sprintf("config file missing required 'semantic:' section: %s", path),
		Hint:    "Add a 'semantic:' section to your config file. Example:\n  semantic:\n    code_collection: my_code\n    code_storage: sqlite",
	}
}

// ErrConfigPathEmpty creates an error for when the config path is empty or whitespace
func ErrConfigPathEmpty() *semantic.SemanticError {
	return &semantic.SemanticError{
		Type:    semantic.ErrTypeInvalidInput,
		Message: "config file path cannot be empty or whitespace",
		Hint:    "Provide a valid file path with --config or omit the flag to use defaults.",
	}
}

// ErrProfileNotFound creates an error for when the profile doesn't exist
func ErrProfileNotFound(name string, available []string) *semantic.SemanticError {
	var hint string
	if len(available) > 0 {
		hint = fmt.Sprintf("Available profiles: %s", strings.Join(available, ", "))
	} else {
		hint = "No profiles are defined. Valid built-in profiles: code, docs, memory"
	}

	return &semantic.SemanticError{
		Type:    semantic.ErrTypeNotFound,
		Message: fmt.Sprintf("profile '%s' not found", name),
		Hint:    hint,
	}
}

// ErrProfileInvalidValue creates an error for invalid profile field values
func ErrProfileInvalidValue(profile, field, expected, actual string) *semantic.SemanticError {
	return &semantic.SemanticError{
		Type:    semantic.ErrTypeInvalidInput,
		Message: fmt.Sprintf("invalid value for '%s' in profile '%s': expected %s, got %s", field, profile, expected, actual),
		Hint:    fmt.Sprintf("Check the value type for '%s' in your config file.", field),
	}
}

// ErrNoProfilesDefined creates an error for when no profiles section exists
func ErrNoProfilesDefined() *semantic.SemanticError {
	return &semantic.SemanticError{
		Type:    semantic.ErrTypeConfiguration,
		Message: "no profiles defined in config file",
		Hint:    "Use one of the built-in profiles (code, docs, memory) or add profile-specific settings to your config.",
	}
}

// extractLineColumn extracts line:column from goccy/go-yaml error messages
// Returns format like "line 5, column 3" or empty string if not found
func extractLineColumn(err error) string {
	if err == nil {
		return ""
	}

	errStr := err.Error()

	// goccy/go-yaml format: "[<line>:<col>] <message>"
	re := regexp.MustCompile(`\[(\d+):(\d+)\]`)
	matches := re.FindStringSubmatch(errStr)
	if len(matches) == 3 {
		return fmt.Sprintf("line %s, column %s", matches[1], matches[2])
	}

	return ""
}

// WrapReadError wraps an os error from reading a config file with appropriate SemanticError
func WrapReadError(path string, err error) *semantic.SemanticError {
	if os.IsNotExist(err) {
		return ErrConfigNotFound(path)
	}
	if os.IsPermission(err) {
		return ErrConfigPermissionDenied(path, err)
	}
	// Generic read error
	return &semantic.SemanticError{
		Type:    semantic.ErrTypeNotFound,
		Message: fmt.Sprintf("failed to read config file: %s", path),
		Cause:   err,
		Hint:    "Check that the file exists and is readable.",
	}
}
