package mcpserver

import (
	"encoding/json"
	"testing"
)

func TestGetToolDefinitions(t *testing.T) {
	tools := GetToolDefinitions()

	// Verify we have exactly 18 tools
	if len(tools) != 18 {
		t.Errorf("Expected 18 tools, got %d", len(tools))
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
		"llm_support_tree",
		"llm_support_grep",
		"llm_support_multiexists",
		"llm_support_json_query",
		"llm_support_markdown_headers",
		"llm_support_template",
		"llm_support_discover_tests",
		"llm_support_multigrep",
		"llm_support_analyze_deps",
		"llm_support_detect",
		"llm_support_count",
		"llm_support_summarize_dir",
		"llm_support_deps",
		"llm_support_git_context",
		"llm_support_validate_plan",
		"llm_support_partition_work",
		"llm_support_repo_root",
		"llm_support_extract_relevant",
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
		"llm_support_grep":             {"pattern", "paths"},
		"llm_support_multiexists":      {"paths"},
		"llm_support_json_query":       {"file", "query"},
		"llm_support_markdown_headers": {"file"},
		"llm_support_template":         {"file"},
		"llm_support_multigrep":        {"keywords"},
		"llm_support_analyze_deps":     {"file"},
		"llm_support_count":            {"mode", "path"},
		"llm_support_summarize_dir":    {"path"},
		"llm_support_deps":             {"manifest"},
		"llm_support_validate_plan":    {"path"},
		"llm_support_extract_relevant": {"context"},
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
