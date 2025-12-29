package filesystem

import (
	"encoding/json"
	"testing"
)

func TestGetToolDefinitions(t *testing.T) {
	tools := GetToolDefinitions()

	// Should have 27 tools
	if len(tools) != 27 {
		t.Errorf("GetToolDefinitions() = %d tools, want 27", len(tools))
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
		"fast_read_file",
		"fast_read_multiple_files",
		"fast_write_file",
		"fast_large_write_file",
		"fast_list_directory",
		"fast_get_file_info",
		"fast_create_directory",
		"fast_search_files",
		"fast_search_code",
		"fast_get_directory_tree",
		"fast_edit_block",
		"fast_safe_edit",
		"fast_edit_multiple_blocks",
		"fast_edit_blocks",
		"fast_extract_lines",
		"fast_copy_file",
		"fast_move_file",
		"fast_delete_file",
		"fast_batch_file_operations",
		"fast_get_disk_usage",
		"fast_find_large_files",
		"fast_compress_files",
		"fast_extract_archive",
		"fast_sync_directories",
		"fast_list_allowed_directories",
		"fast_edit_file",
		"fast_search_and_replace",
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
