package semantic

import (
	"fmt"
	"strings"
)

// Error types for better error handling
type ErrorType int

const (
	ErrTypeUnknown ErrorType = iota
	ErrTypeNotFound
	ErrTypeConnection
	ErrTypeInvalidInput
	ErrTypeConfiguration
)

// SemanticError provides structured error information
type SemanticError struct {
	Type    ErrorType
	Message string
	Cause   error
	Hint    string
}

func (e *SemanticError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Cause)
	}
	return e.Message
}

func (e *SemanticError) Unwrap() error {
	return e.Cause
}

// WithHint adds a helpful hint to the error
func (e *SemanticError) WithHint(hint string) *SemanticError {
	e.Hint = hint
	return e
}

// FormatWithHint returns the error message with hint if available
func (e *SemanticError) FormatWithHint() string {
	if e.Hint != "" {
		return fmt.Sprintf("%s\n  Hint: %s", e.Error(), e.Hint)
	}
	return e.Error()
}

// Error constructors for common cases

// ErrIndexNotFound creates an error for when the index doesn't exist
func ErrIndexNotFound() *SemanticError {
	return &SemanticError{
		Type:    ErrTypeNotFound,
		Message: "semantic index not found",
		Hint:    "Run 'llm-semantic index <path>' to create an index first.",
	}
}

// ErrConnectionFailed creates an error for API connection failures
func ErrConnectionFailed(cause error) *SemanticError {
	return &SemanticError{
		Type:    ErrTypeConnection,
		Message: "failed to connect to embedding API",
		Cause:   cause,
		Hint:    "Ensure the embedding server is running. Try: ollama serve",
	}
}

// ErrInvalidQuery creates an error for invalid search queries
func ErrInvalidQuery(query string) *SemanticError {
	return &SemanticError{
		Type:    ErrTypeInvalidInput,
		Message: fmt.Sprintf("invalid query: %q", query),
		Hint:    "Provide a non-empty search query describing what you're looking for.",
	}
}

// ErrInvalidPath creates an error for invalid file paths
func ErrInvalidPath(path string, cause error) *SemanticError {
	return &SemanticError{
		Type:    ErrTypeInvalidInput,
		Message: fmt.Sprintf("invalid path: %s", path),
		Cause:   cause,
		Hint:    "Provide a valid file or directory path.",
	}
}

// ErrStorageFailure creates an error for storage-related issues
func ErrStorageFailure(operation string, cause error) *SemanticError {
	return &SemanticError{
		Type:    ErrTypeUnknown,
		Message: fmt.Sprintf("storage %s failed", operation),
		Cause:   cause,
		Hint:    "The index may be corrupted. Try: rm -rf .llm-index && llm-semantic index .",
	}
}

// ErrEmbeddingFailure creates an error for embedding generation failures
func ErrEmbeddingFailure(cause error) *SemanticError {
	errStr := ""
	if cause != nil {
		errStr = cause.Error()
	}

	hint := "Check your API configuration with --api-url and --model flags."
	if strings.Contains(strings.ToLower(errStr), "connection") {
		hint = "The embedding server appears to be offline. Start it with: ollama serve"
	} else if strings.Contains(strings.ToLower(errStr), "model") {
		hint = "The model may not be available. Try: ollama pull nomic-embed-text"
	}

	return &SemanticError{
		Type:    ErrTypeConnection,
		Message: "embedding generation failed",
		Cause:   cause,
		Hint:    hint,
	}
}

// ErrChunkingFailure creates an error for code chunking failures
func ErrChunkingFailure(filePath string, cause error) *SemanticError {
	return &SemanticError{
		Type:    ErrTypeUnknown,
		Message: fmt.Sprintf("failed to parse %s", filePath),
		Cause:   cause,
		Hint:    "The file may contain syntax errors or be in an unsupported format.",
	}
}

// FormatError formats an error with user-friendly output
func FormatError(err error) string {
	if semErr, ok := err.(*SemanticError); ok {
		return semErr.FormatWithHint()
	}
	return err.Error()
}
