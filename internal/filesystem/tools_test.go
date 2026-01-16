package filesystem

import (
	"encoding/json"
	"testing"
)

func TestGetToolDefinitions(t *testing.T) {
	tools := GetToolDefinitions()

	// Should have 15 batch/specialized tools
	// NOTE: Single-file operations should use Claude's native tools
	if len(tools) != 15 {
		t.Errorf("GetToolDefinitions() = %d tools, want 15", len(tools))
	}

	// Verify all tools have required fields
	for i, tool := range tools {
		if tool.Name == "" {
			t.Errorf("Tool %d: Name should not be empty", i)
		}
		if tool.Description == "" {
			t.Errorf("Tool %d: Description should not be empty", i)
		}
		if len(tool.InputSchema) == 0 {
			t.Errorf("Tool %d (%s): InputSchema should not be empty", i, tool.Name)
		}
	}
}

func TestGetToolDefinitionsValidJSON(t *testing.T) {
	tools := GetToolDefinitions()

	for _, tool := range tools {
		// Verify InputSchema is valid JSON
		var schema map[string]interface{}
		if err := json.Unmarshal(tool.InputSchema, &schema); err != nil {
			t.Errorf("Tool %s: InputSchema is not valid JSON: %v", tool.Name, err)
		}

		// Verify schema has type: object
		if schemaType, ok := schema["type"]; !ok || schemaType != "object" {
			t.Errorf("Tool %s: InputSchema should have type=object", tool.Name)
		}
	}
}

func TestGetToolDefinitionsNames(t *testing.T) {
	tools := GetToolDefinitions()

	// 15 batch/specialized tools - single-file operations use Claude's native tools
	expectedTools := []string{
		// Batch Reading
		"llm_filesystem_read_multiple_files",
		"llm_filesystem_extract_lines",

		// Batch Editing
		"llm_filesystem_edit_blocks",
		"llm_filesystem_search_and_replace",

		// Directory operations
		"llm_filesystem_list_directory",
		"llm_filesystem_get_directory_tree",
		"llm_filesystem_create_directories",

		// Search operations
		"llm_filesystem_search_files",
		"llm_filesystem_search_code",

		// File management
		"llm_filesystem_copy_file",
		"llm_filesystem_move_file",
		"llm_filesystem_delete_file",
		"llm_filesystem_batch_file_operations",

		// Archive operations
		"llm_filesystem_compress_files",
		"llm_filesystem_extract_archive",
	}

	if len(tools) != len(expectedTools) {
		t.Errorf("GetToolDefinitions() = %d tools, want %d", len(tools), len(expectedTools))
	}

	toolNames := make(map[string]bool)
	for _, tool := range tools {
		toolNames[tool.Name] = true
	}

	for _, expected := range expectedTools {
		if !toolNames[expected] {
			t.Errorf("Missing expected tool: %s", expected)
		}
	}
}
