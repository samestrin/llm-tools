package mcpserver

import (
	"encoding/json"
	"testing"
)

func TestGetToolDefinitions(t *testing.T) {
	tools := GetToolDefinitions()

	// Verify we have exactly 52 tools
	if len(tools) != 52 {
		t.Errorf("Expected 52 tools, got %d", len(tools))
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
		"llm_support_highest",
		"llm_support_plan_type",
		"llm_support_git_changes",
		"llm_support_context_multiset",
		"llm_support_context_multiget",
		"llm_support_context",
		"llm_support_yaml_get",
		"llm_support_yaml_set",
		"llm_support_yaml_multiget",
		"llm_support_yaml_multiset",
		"llm_support_args",
		"llm_support_catfiles",
		"llm_support_decode",
		"llm_support_diff",
		"llm_support_encode",
		"llm_support_extract",
		"llm_support_foreach",
		"llm_support_hash",
		"llm_support_init_temp",
		"llm_support_math",
		"llm_support_prompt",
		"llm_support_report",
		"llm_support_stats",
		"llm_support_toml_query",
		"llm_support_toml_validate",
		"llm_support_toml_parse",
		"llm_support_transform_case",
		"llm_support_transform_csv_to_json",
		"llm_support_transform_json_to_csv",
		"llm_support_transform_filter",
		"llm_support_transform_sort",
		"llm_support_validate",
		"llm_support_clean_temp",
		"llm_support_runtime",
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
		"llm_support_context":          {"operation", "dir"},
		"llm_support_context_multiset": {"dir", "pairs"},
		"llm_support_context_multiget": {"dir", "keys"},
		"llm_support_yaml_get":         {"file", "key"},
		"llm_support_yaml_set":         {"file", "key", "value"},
		"llm_support_yaml_multiget":    {"file", "keys"},
		"llm_support_yaml_multiset":    {"file", "pairs"},
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
