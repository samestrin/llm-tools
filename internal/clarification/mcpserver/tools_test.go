package mcpserver

import (
	"encoding/json"
	"testing"
)

func TestGetToolDefinitions(t *testing.T) {
	tools := GetToolDefinitions()

	// Verify we have exactly 8 tools
	if len(tools) != 8 {
		t.Errorf("Expected 8 tools, got %d", len(tools))
	}

	// Verify all tools have the correct prefix
	for _, tool := range tools {
		if tool.Name[:len(ToolPrefix)] != ToolPrefix {
			t.Errorf("Tool %s doesn't have prefix %s", tool.Name, ToolPrefix)
		}
	}

	// Verify all tools have valid JSON schemas
	for _, tool := range tools {
		var schema map[string]interface{}
		if err := json.Unmarshal(tool.InputSchema, &schema); err != nil {
			t.Errorf("Tool %s has invalid JSON schema: %v", tool.Name, err)
		}

		// Verify schema has type: object
		if schemaType, ok := schema["type"].(string); !ok || schemaType != "object" {
			t.Errorf("Tool %s schema should have type: object", tool.Name)
		}
	}

	// Verify specific tool names exist
	expectedTools := []string{
		"llm_clarify_match",
		"llm_clarify_cluster",
		"llm_clarify_detect_conflicts",
		"llm_clarify_validate",
		"llm_clarify_init",
		"llm_clarify_add",
		"llm_clarify_promote",
		"llm_clarify_list",
	}

	toolMap := make(map[string]bool)
	for _, tool := range tools {
		toolMap[tool.Name] = true
	}

	for _, expected := range expectedTools {
		if !toolMap[expected] {
			t.Errorf("Missing expected tool: %s", expected)
		}
	}
}

func TestToolDescriptions(t *testing.T) {
	tools := GetToolDefinitions()

	for _, tool := range tools {
		if tool.Description == "" {
			t.Errorf("Tool %s has empty description", tool.Name)
		}
		// Description should be reasonably sized
		if len(tool.Description) < 10 {
			t.Errorf("Tool %s has too short description: %s", tool.Name, tool.Description)
		}
	}
}

func TestToolSchemaRequiredFields(t *testing.T) {
	tools := GetToolDefinitions()

	// Map of tools to their required fields
	requiredFields := map[string][]string{
		"llm_clarify_match":            {"question"},
		"llm_clarify_detect_conflicts": {"tracking_file"},
		"llm_clarify_validate":         {"tracking_file"},
		"llm_clarify_init":             {"output"},
		"llm_clarify_add":              {"tracking_file", "question"},
		"llm_clarify_promote":          {"tracking_file", "id", "target"},
		"llm_clarify_list":             {"tracking_file"},
	}

	for _, tool := range tools {
		expected, hasRequired := requiredFields[tool.Name]
		if !hasRequired {
			continue // No required fields defined for this tool
		}

		var schema map[string]interface{}
		if err := json.Unmarshal(tool.InputSchema, &schema); err != nil {
			t.Errorf("Tool %s: failed to parse schema: %v", tool.Name, err)
			continue
		}

		required, ok := schema["required"].([]interface{})
		if !ok {
			t.Errorf("Tool %s: missing 'required' array in schema", tool.Name)
			continue
		}

		// Convert to string slice
		requiredStrs := make([]string, len(required))
		for i, r := range required {
			requiredStrs[i] = r.(string)
		}

		// Check each expected required field
		for _, exp := range expected {
			found := false
			for _, r := range requiredStrs {
				if r == exp {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Tool %s: missing required field %s", tool.Name, exp)
			}
		}
	}
}

func TestAPIRequiringTools(t *testing.T) {
	tools := GetToolDefinitions()

	// These tools require API
	apiTools := map[string]bool{
		"llm_clarify_match":            true,
		"llm_clarify_cluster":          true,
		"llm_clarify_detect_conflicts": true,
		"llm_clarify_validate":         true,
	}

	for _, tool := range tools {
		if apiTools[tool.Name] {
			// Should mention API requirement in description
			if !containsAny(tool.Description, []string{"API", "REQUIRES"}) {
				t.Errorf("Tool %s requires API but description doesn't mention it", tool.Name)
			}
		}
	}
}

func containsAny(s string, substrs []string) bool {
	for _, sub := range substrs {
		if contains(s, sub) {
			return true
		}
	}
	return false
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
