package mcp_integration

import (
	"encoding/json"
	"testing"

	clarifyserver "github.com/samestrin/llm-tools/internal/clarification/mcpserver"
	supportserver "github.com/samestrin/llm-tools/internal/support/mcpserver"
)

// TestLLMSupportToolCount verifies the correct number of tools
func TestLLMSupportToolCount(t *testing.T) {
	tools := supportserver.GetToolDefinitions()
	expected := 28
	if len(tools) != expected {
		t.Errorf("Expected %d llm-support tools, got %d", expected, len(tools))
	}
}

// TestLLMClarificationToolCount verifies the correct number of tools
func TestLLMClarificationToolCount(t *testing.T) {
	tools := clarifyserver.GetToolDefinitions()
	expected := 13
	if len(tools) != expected {
		t.Errorf("Expected %d llm-clarification tools, got %d", expected, len(tools))
	}
}

// TestLLMSupportToolSchemas validates all tool schemas are valid JSON
func TestLLMSupportToolSchemas(t *testing.T) {
	tools := supportserver.GetToolDefinitions()
	for _, tool := range tools {
		var schema map[string]interface{}
		if err := json.Unmarshal(tool.InputSchema, &schema); err != nil {
			t.Errorf("Tool %s has invalid JSON schema: %v", tool.Name, err)
		}

		// Verify schema structure
		schemaType, ok := schema["type"].(string)
		if !ok || schemaType != "object" {
			t.Errorf("Tool %s schema type should be 'object', got %v", tool.Name, schema["type"])
		}

		// Verify properties exist
		if _, ok := schema["properties"]; !ok {
			t.Errorf("Tool %s schema missing 'properties' field", tool.Name)
		}
	}
}

// TestLLMClarificationToolSchemas validates all tool schemas are valid JSON
func TestLLMClarificationToolSchemas(t *testing.T) {
	tools := clarifyserver.GetToolDefinitions()
	for _, tool := range tools {
		var schema map[string]interface{}
		if err := json.Unmarshal(tool.InputSchema, &schema); err != nil {
			t.Errorf("Tool %s has invalid JSON schema: %v", tool.Name, err)
		}

		// Verify schema structure
		schemaType, ok := schema["type"].(string)
		if !ok || schemaType != "object" {
			t.Errorf("Tool %s schema type should be 'object', got %v", tool.Name, schema["type"])
		}
	}
}

// TestToolPrefixes verifies tool naming conventions
func TestToolPrefixes(t *testing.T) {
	supportTools := supportserver.GetToolDefinitions()
	for _, tool := range supportTools {
		if len(tool.Name) < len("llm_support_") || tool.Name[:12] != "llm_support_" {
			t.Errorf("Tool %s should have 'llm_support_' prefix", tool.Name)
		}
	}

	clarifyTools := clarifyserver.GetToolDefinitions()
	for _, tool := range clarifyTools {
		if len(tool.Name) < len("llm_clarify_") || tool.Name[:12] != "llm_clarify_" {
			t.Errorf("Tool %s should have 'llm_clarify_' prefix", tool.Name)
		}
	}
}

// TestToolDescriptions verifies all tools have meaningful descriptions
func TestToolDescriptions(t *testing.T) {
	// Test support tools
	for _, tool := range supportserver.GetToolDefinitions() {
		if len(tool.Description) < 20 {
			t.Errorf("Tool %s has too short description (%d chars)", tool.Name, len(tool.Description))
		}
	}
	// Test clarify tools
	for _, tool := range clarifyserver.GetToolDefinitions() {
		if len(tool.Description) < 20 {
			t.Errorf("Tool %s has too short description (%d chars)", tool.Name, len(tool.Description))
		}
	}
}

// TestExpectedSupportToolNames verifies all expected tools exist
func TestExpectedSupportToolNames(t *testing.T) {
	expectedNames := []string{
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
	}

	tools := supportserver.GetToolDefinitions()
	toolMap := make(map[string]bool)
	for _, tool := range tools {
		toolMap[tool.Name] = true
	}

	for _, expected := range expectedNames {
		if !toolMap[expected] {
			t.Errorf("Missing expected tool: %s", expected)
		}
	}
}

// TestExpectedClarifyToolNames verifies all expected tools exist
func TestExpectedClarifyToolNames(t *testing.T) {
	expectedNames := []string{
		"llm_clarify_match",
		"llm_clarify_cluster",
		"llm_clarify_detect_conflicts",
		"llm_clarify_validate",
		"llm_clarify_init",
		"llm_clarify_add",
		"llm_clarify_promote",
		"llm_clarify_list",
		"llm_clarify_delete",
		"llm_clarify_export",
		"llm_clarify_import",
		"llm_clarify_optimize",
		"llm_clarify_reconcile",
	}

	tools := clarifyserver.GetToolDefinitions()
	toolMap := make(map[string]bool)
	for _, tool := range tools {
		toolMap[tool.Name] = true
	}

	for _, expected := range expectedNames {
		if !toolMap[expected] {
			t.Errorf("Missing expected tool: %s", expected)
		}
	}
}
