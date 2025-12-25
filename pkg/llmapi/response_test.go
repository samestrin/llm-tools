package llmapi

import (
	"testing"
)

func TestCleanResponse_PlainJSON(t *testing.T) {
	input := `{"status": "success", "count": 5}`
	result := CleanResponse(input)

	if result != input {
		t.Errorf("plain JSON should remain unchanged, got %s", result)
	}
}

func TestCleanResponse_WithCodeFence(t *testing.T) {
	input := "```json\n{\"status\": \"success\"}\n```"
	expected := `{"status": "success"}`
	result := CleanResponse(input)

	if result != expected {
		t.Errorf("expected %s, got %s", expected, result)
	}
}

func TestCleanResponse_WithCodeFenceNoLanguage(t *testing.T) {
	input := "```\n{\"status\": \"success\"}\n```"
	expected := `{"status": "success"}`
	result := CleanResponse(input)

	if result != expected {
		t.Errorf("expected %s, got %s", expected, result)
	}
}

func TestCleanResponse_WithSurroundingText(t *testing.T) {
	input := "Here is the response:\n```json\n{\"status\": \"success\"}\n```\nThat's all."
	expected := `{"status": "success"}`
	result := CleanResponse(input)

	if result != expected {
		t.Errorf("expected %s, got %s", expected, result)
	}
}

func TestCleanResponse_WhitespaceOnly(t *testing.T) {
	input := "   \n\t  "
	result := CleanResponse(input)

	if result != "" {
		t.Errorf("expected empty string, got %s", result)
	}
}

func TestExtractJSON_ValidJSON(t *testing.T) {
	input := `{"question": "Test?", "answer": "Yes"}`
	result, err := ExtractJSON(input)

	if err != nil {
		t.Fatalf("ExtractJSON failed: %v", err)
	}
	if result != input {
		t.Errorf("expected %s, got %s", input, result)
	}
}

func TestExtractJSON_WithCodeFence(t *testing.T) {
	input := "```json\n{\"question\": \"Test?\"}\n```"
	expected := `{"question": "Test?"}`
	result, err := ExtractJSON(input)

	if err != nil {
		t.Fatalf("ExtractJSON failed: %v", err)
	}
	if result != expected {
		t.Errorf("expected %s, got %s", expected, result)
	}
}

func TestExtractJSON_InvalidJSON(t *testing.T) {
	input := "This is not JSON at all"
	_, err := ExtractJSON(input)

	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestExtractJSON_JSONArray(t *testing.T) {
	input := "[{\"id\": 1}, {\"id\": 2}]"
	result, err := ExtractJSON(input)

	if err != nil {
		t.Fatalf("ExtractJSON failed: %v", err)
	}
	if result != input {
		t.Errorf("expected %s, got %s", input, result)
	}
}

func TestExtractJSON_MalformedCodeFence(t *testing.T) {
	// Missing closing fence
	input := "```json\n{\"status\": \"incomplete\"}"
	result, err := ExtractJSON(input)

	// Should still extract the JSON part
	if err != nil {
		t.Fatalf("ExtractJSON should handle malformed fences: %v", err)
	}
	if result != `{"status": "incomplete"}` {
		t.Errorf("expected JSON content, got %s", result)
	}
}
