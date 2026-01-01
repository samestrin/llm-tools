package filesystem

import (
	"testing"
)

func TestEncodeContinuationToken(t *testing.T) {
	token := &ContinuationToken{
		Type:   "list",
		Page:   2,
		Offset: 0,
		Path:   "/tmp/test",
	}

	encoded, err := EncodeContinuationToken(token)
	if err != nil {
		t.Fatalf("EncodeContinuationToken() error = %v", err)
	}

	if encoded == "" {
		t.Error("expected non-empty encoded token")
	}

	// Token should be base64 encoded
	if len(encoded) < 10 {
		t.Error("encoded token seems too short")
	}
}

func TestDecodeContinuationToken(t *testing.T) {
	// Create a token and encode it
	original := &ContinuationToken{
		Type:   "list",
		Page:   3,
		Offset: 100,
		Path:   "/tmp/test",
	}

	encoded, err := EncodeContinuationToken(original)
	if err != nil {
		t.Fatalf("EncodeContinuationToken() error = %v", err)
	}

	// Decode it
	decoded, err := DecodeContinuationToken(encoded)
	if err != nil {
		t.Fatalf("DecodeContinuationToken() error = %v", err)
	}

	// Verify fields
	if decoded.Type != original.Type {
		t.Errorf("Type = %v, want %v", decoded.Type, original.Type)
	}
	if decoded.Page != original.Page {
		t.Errorf("Page = %v, want %v", decoded.Page, original.Page)
	}
	if decoded.Offset != original.Offset {
		t.Errorf("Offset = %v, want %v", decoded.Offset, original.Offset)
	}
	if decoded.Path != original.Path {
		t.Errorf("Path = %v, want %v", decoded.Path, original.Path)
	}
	if decoded.Version != "1" {
		t.Errorf("Version = %v, want 1", decoded.Version)
	}
}

func TestDecodeContinuationTokenEmpty(t *testing.T) {
	token, err := DecodeContinuationToken("")
	if err != nil {
		t.Errorf("DecodeContinuationToken() should not error on empty string, got %v", err)
	}
	if token != nil {
		t.Error("expected nil token for empty string")
	}
}

func TestDecodeContinuationTokenInvalid(t *testing.T) {
	tests := []struct {
		name     string
		tokenStr string
	}{
		{"not base64", "not-valid-base64!!!"},
		{"not json", "dGhpcyBpcyBub3QganNvbg=="}, // "this is not json" base64
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := DecodeContinuationToken(tt.tokenStr)
			if err == nil {
				t.Error("expected error for invalid token")
			}
		})
	}
}

func TestCreateListToken(t *testing.T) {
	token, err := CreateListToken("/tmp/test", 5)
	if err != nil {
		t.Fatalf("CreateListToken() error = %v", err)
	}

	decoded, err := DecodeContinuationToken(token)
	if err != nil {
		t.Fatalf("failed to decode token: %v", err)
	}

	if decoded.Type != "list" {
		t.Errorf("Type = %v, want list", decoded.Type)
	}
	if decoded.Page != 5 {
		t.Errorf("Page = %v, want 5", decoded.Page)
	}
	if decoded.Path != "/tmp/test" {
		t.Errorf("Path = %v, want /tmp/test", decoded.Path)
	}
}

func TestCreateReadToken(t *testing.T) {
	token, err := CreateReadToken("/tmp/test.txt", 1024)
	if err != nil {
		t.Fatalf("CreateReadToken() error = %v", err)
	}

	decoded, err := DecodeContinuationToken(token)
	if err != nil {
		t.Fatalf("failed to decode token: %v", err)
	}

	if decoded.Type != "read" {
		t.Errorf("Type = %v, want read", decoded.Type)
	}
	if decoded.Offset != 1024 {
		t.Errorf("Offset = %v, want 1024", decoded.Offset)
	}
}

func TestCreateSearchToken(t *testing.T) {
	token, err := CreateSearchToken("/tmp", 500)
	if err != nil {
		t.Fatalf("CreateSearchToken() error = %v", err)
	}

	decoded, err := DecodeContinuationToken(token)
	if err != nil {
		t.Fatalf("failed to decode token: %v", err)
	}

	if decoded.Type != "search" {
		t.Errorf("Type = %v, want search", decoded.Type)
	}
	if decoded.Offset != 500 {
		t.Errorf("Offset = %v, want 500", decoded.Offset)
	}
}

func TestValidateToken(t *testing.T) {
	token := &ContinuationToken{
		Type: "list",
		Path: "/tmp/test",
	}

	// Valid case
	err := ValidateToken(token, "/tmp/test", "list")
	if err != nil {
		t.Errorf("ValidateToken() unexpected error = %v", err)
	}

	// Wrong type
	err = ValidateToken(token, "/tmp/test", "read")
	if err == nil {
		t.Error("expected error for type mismatch")
	}

	// Wrong path
	err = ValidateToken(token, "/tmp/other", "list")
	if err == nil {
		t.Error("expected error for path mismatch")
	}

	// Nil token should be valid
	err = ValidateToken(nil, "/tmp/test", "list")
	if err != nil {
		t.Errorf("ValidateToken() nil token should be valid, got %v", err)
	}
}

func TestTokenRoundTrip(t *testing.T) {
	// Test complete round-trip for list operation
	path := "/Users/test/documents"
	nextPage := 7

	tokenStr, err := CreateListToken(path, nextPage)
	if err != nil {
		t.Fatalf("CreateListToken() error = %v", err)
	}

	decoded, err := DecodeContinuationToken(tokenStr)
	if err != nil {
		t.Fatalf("DecodeContinuationToken() error = %v", err)
	}

	err = ValidateToken(decoded, path, "list")
	if err != nil {
		t.Errorf("ValidateToken() error = %v", err)
	}

	if decoded.Page != nextPage {
		t.Errorf("Page = %v, want %v", decoded.Page, nextPage)
	}
}
