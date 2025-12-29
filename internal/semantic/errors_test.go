package semantic

import (
	"errors"
	"strings"
	"testing"
)

func TestSemanticError_Error(t *testing.T) {
	err := &SemanticError{
		Type:    ErrTypeNotFound,
		Message: "test error",
	}

	if err.Error() != "test error" {
		t.Errorf("Error() = %q, want 'test error'", err.Error())
	}
}

func TestSemanticError_ErrorWithCause(t *testing.T) {
	cause := errors.New("underlying error")
	err := &SemanticError{
		Type:    ErrTypeConnection,
		Message: "connection failed",
		Cause:   cause,
	}

	expected := "connection failed: underlying error"
	if err.Error() != expected {
		t.Errorf("Error() = %q, want %q", err.Error(), expected)
	}
}

func TestSemanticError_Unwrap(t *testing.T) {
	cause := errors.New("underlying error")
	err := &SemanticError{
		Type:  ErrTypeUnknown,
		Cause: cause,
	}

	if err.Unwrap() != cause {
		t.Error("Unwrap() should return cause")
	}
}

func TestSemanticError_WithHint(t *testing.T) {
	err := &SemanticError{
		Type:    ErrTypeNotFound,
		Message: "not found",
	}

	err.WithHint("try again")

	if err.Hint != "try again" {
		t.Errorf("Hint = %q, want 'try again'", err.Hint)
	}
}

func TestSemanticError_FormatWithHint(t *testing.T) {
	err := &SemanticError{
		Type:    ErrTypeNotFound,
		Message: "not found",
		Hint:    "run index first",
	}

	formatted := err.FormatWithHint()

	if !strings.Contains(formatted, "not found") {
		t.Error("FormatWithHint should contain message")
	}
	if !strings.Contains(formatted, "run index first") {
		t.Error("FormatWithHint should contain hint")
	}
}

func TestErrIndexNotFound(t *testing.T) {
	err := ErrIndexNotFound()

	if err.Type != ErrTypeNotFound {
		t.Errorf("Type = %v, want ErrTypeNotFound", err.Type)
	}
	if err.Hint == "" {
		t.Error("Should have a hint")
	}
}

func TestErrConnectionFailed(t *testing.T) {
	cause := errors.New("connection refused")
	err := ErrConnectionFailed(cause)

	if err.Type != ErrTypeConnection {
		t.Errorf("Type = %v, want ErrTypeConnection", err.Type)
	}
	if err.Cause != cause {
		t.Error("Should have cause")
	}
}

func TestErrInvalidQuery(t *testing.T) {
	err := ErrInvalidQuery("")

	if err.Type != ErrTypeInvalidInput {
		t.Errorf("Type = %v, want ErrTypeInvalidInput", err.Type)
	}
}

func TestErrInvalidPath(t *testing.T) {
	cause := errors.New("file not found")
	err := ErrInvalidPath("/bad/path", cause)

	if err.Type != ErrTypeInvalidInput {
		t.Errorf("Type = %v, want ErrTypeInvalidInput", err.Type)
	}
	if !strings.Contains(err.Error(), "/bad/path") {
		t.Error("Should contain path in message")
	}
}

func TestErrStorageFailure(t *testing.T) {
	cause := errors.New("disk full")
	err := ErrStorageFailure("write", cause)

	if !strings.Contains(err.Error(), "write") {
		t.Error("Should contain operation in message")
	}
}

func TestErrEmbeddingFailure(t *testing.T) {
	testCases := []struct {
		cause       error
		hintContain string
	}{
		{errors.New("connection refused"), "offline"},
		{errors.New("model not found"), "pull"},
		{errors.New("general error"), "api-url"},
	}

	for _, tc := range testCases {
		err := ErrEmbeddingFailure(tc.cause)
		if !strings.Contains(strings.ToLower(err.Hint), tc.hintContain) {
			t.Errorf("Hint for %q should contain %q, got %q", tc.cause, tc.hintContain, err.Hint)
		}
	}
}

func TestErrChunkingFailure(t *testing.T) {
	cause := errors.New("syntax error")
	err := ErrChunkingFailure("test.go", cause)

	if !strings.Contains(err.Error(), "test.go") {
		t.Error("Should contain file path")
	}
}

func TestFormatError_SemanticError(t *testing.T) {
	err := ErrIndexNotFound()
	formatted := FormatError(err)

	if !strings.Contains(formatted, "index not found") {
		t.Error("Should contain error message")
	}
	if !strings.Contains(formatted, "Hint") {
		t.Error("Should contain hint")
	}
}

func TestFormatError_RegularError(t *testing.T) {
	err := errors.New("regular error")
	formatted := FormatError(err)

	if formatted != "regular error" {
		t.Errorf("FormatError = %q, want 'regular error'", formatted)
	}
}
