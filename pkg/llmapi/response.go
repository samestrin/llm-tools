package llmapi

import (
	"encoding/json"
	"errors"
	"regexp"
	"strings"
)

// codeBlockPattern matches markdown code blocks.
var codeBlockPattern = regexp.MustCompile("(?s)```(?:json)?\\s*\n?(.*?)(?:\n?```|$)")

// CleanResponse removes markdown code fences and extracts the content.
func CleanResponse(response string) string {
	response = strings.TrimSpace(response)
	if response == "" {
		return ""
	}

	// Check for code block
	matches := codeBlockPattern.FindStringSubmatch(response)
	if len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}

	return response
}

// ExtractJSON cleans the response and validates it's valid JSON.
func ExtractJSON(response string) (string, error) {
	cleaned := CleanResponse(response)

	// Validate it's valid JSON
	if !json.Valid([]byte(cleaned)) {
		return "", errors.New("response is not valid JSON")
	}

	return cleaned, nil
}
