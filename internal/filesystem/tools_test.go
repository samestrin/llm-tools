package filesystem

import (
	"encoding/json"
	"testing"
)

func TestGetToolDefinitions(t *testing.T) {
	tools := GetToolDefinitions()

	// Should have 28 tools
	if len(tools) != 28 {
		t.Errorf("GetToolDefinitions() = %d tools, want 28", len(tools))
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

	expectedTools := []string{
		"llm_filesystem_read_file",
		"llm_filesystem_read_multiple_files",
		"llm_filesystem_write_file",
		"llm_filesystem_large_write_file",
		"llm_filesystem_list_directory",
		"llm_filesystem_get_file_info",
		"llm_filesystem_create_directory",
		"llm_filesystem_create_directories",
		"llm_filesystem_search_files",
		"llm_filesystem_search_code",
		"llm_filesystem_get_directory_tree",
		"llm_filesystem_edit_block",
		"llm_filesystem_safe_edit",
		"llm_filesystem_edit_multiple_blocks",
		"llm_filesystem_edit_blocks",
		"llm_filesystem_extract_lines",
		"llm_filesystem_copy_file",
		"llm_filesystem_move_file",
		"llm_filesystem_delete_file",
		"llm_filesystem_batch_file_operations",
		"llm_filesystem_get_disk_usage",
		"llm_filesystem_find_large_files",
		"llm_filesystem_compress_files",
		"llm_filesystem_extract_archive",
		"llm_filesystem_sync_directories",
		"llm_filesystem_list_allowed_directories",
		"llm_filesystem_edit_file",
		"llm_filesystem_search_and_replace",
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
