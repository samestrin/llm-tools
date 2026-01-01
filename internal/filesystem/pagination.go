package filesystem

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
)

// ContinuationToken holds state for resuming paginated operations
type ContinuationToken struct {
	Version string `json:"v"`    // Token version for compatibility
	Type    string `json:"t"`    // Operation type: "list", "read", "search"
	Page    int    `json:"p"`    // Current page number
	Offset  int64  `json:"o"`    // Byte offset for file reads
	Path    string `json:"path"` // Original path for validation
}

// EncodeContinuationToken creates a base64-encoded token string
func EncodeContinuationToken(token *ContinuationToken) (string, error) {
	if token == nil {
		return "", nil
	}
	token.Version = "1"

	data, err := json.Marshal(token)
	if err != nil {
		return "", fmt.Errorf("failed to encode token: %w", err)
	}

	return base64.StdEncoding.EncodeToString(data), nil
}

// DecodeContinuationToken parses a base64-encoded token string
func DecodeContinuationToken(tokenStr string) (*ContinuationToken, error) {
	if tokenStr == "" {
		return nil, nil
	}

	data, err := base64.StdEncoding.DecodeString(tokenStr)
	if err != nil {
		return nil, fmt.Errorf("invalid continuation token: %w", err)
	}

	var token ContinuationToken
	if err := json.Unmarshal(data, &token); err != nil {
		return nil, fmt.Errorf("invalid continuation token format: %w", err)
	}

	// Validate version
	if token.Version != "1" {
		return nil, fmt.Errorf("unsupported token version: %s", token.Version)
	}

	return &token, nil
}

// CreateListToken creates a token for list_directory pagination
func CreateListToken(path string, nextPage int) (string, error) {
	token := &ContinuationToken{
		Type: "list",
		Page: nextPage,
		Path: path,
	}
	return EncodeContinuationToken(token)
}

// CreateReadToken creates a token for read_file chunked reads
func CreateReadToken(path string, offset int64) (string, error) {
	token := &ContinuationToken{
		Type:   "read",
		Offset: offset,
		Path:   path,
	}
	return EncodeContinuationToken(token)
}

// CreateSearchToken creates a token for search result pagination
func CreateSearchToken(path string, offset int64) (string, error) {
	token := &ContinuationToken{
		Type:   "search",
		Offset: offset,
		Path:   path,
	}
	return EncodeContinuationToken(token)
}

// ValidateToken checks if token is valid for the given path and operation
func ValidateToken(token *ContinuationToken, path, operationType string) error {
	if token == nil {
		return nil
	}

	if token.Type != operationType {
		return fmt.Errorf("token type mismatch: expected %s, got %s", operationType, token.Type)
	}

	if token.Path != path {
		return fmt.Errorf("token path mismatch: token was created for a different path")
	}

	return nil
}
