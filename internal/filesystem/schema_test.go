package filesystem

import (
	"encoding/json"
	"testing"
)

// TestSchemaParityToolCount validates we have exactly 27 tools
func TestSchemaParityToolCount(t *testing.T) {
	tools := GetToolDefinitions()
	if len(tools) != 27 {
		t.Errorf("Expected 27 tools, got %d", len(tools))
	}
}

// TestSchemaParityRequiredFields validates all tools have required fields
func TestSchemaParityRequiredFields(t *testing.T) {
	tools := GetToolDefinitions()

	for _, tool := range tools {
		if tool.Name == "" {
			t.Error("Tool has empty name")
		}
		if tool.Description == "" {
			t.Errorf("Tool %s has empty description", tool.Name)
		}
		if len(tool.InputSchema) == 0 {
			t.Errorf("Tool %s has empty schema", tool.Name)
		}
	}
}

// TestSchemaParityValidJSON validates all schemas are valid JSON
func TestSchemaParityValidJSON(t *testing.T) {
	tools := GetToolDefinitions()

	for _, tool := range tools {
		var schema map[string]interface{}
		if err := json.Unmarshal(tool.InputSchema, &schema); err != nil {
			t.Errorf("Tool %s has invalid JSON schema: %v", tool.Name, err)
		}
	}
}

// TestSchemaParityHasTypeObject validates all schemas have type=object
func TestSchemaParityHasTypeObject(t *testing.T) {
	tools := GetToolDefinitions()

	for _, tool := range tools {
		var schema map[string]interface{}
		json.Unmarshal(tool.InputSchema, &schema)

		schemaType, ok := schema["type"].(string)
		if !ok || schemaType != "object" {
			t.Errorf("Tool %s schema should have type=object, got %v", tool.Name, schema["type"])
		}
	}
}

// TestSchemaParityHasProperties validates most schemas have properties
func TestSchemaParityHasProperties(t *testing.T) {
	tools := GetToolDefinitions()

	// Tools that legitimately take no arguments
	noArgsTools := map[string]bool{
		"llm_filesystem_list_allowed_directories": true,
	}

	for _, tool := range tools {
		if noArgsTools[tool.Name] {
			continue // Skip tools that take no arguments
		}

		var schema map[string]interface{}
		json.Unmarshal(tool.InputSchema, &schema)

		props, ok := schema["properties"].(map[string]interface{})
		if !ok || len(props) == 0 {
			t.Errorf("Tool %s schema should have properties", tool.Name)
		}
	}
}

// TestSchemaParityNamingConvention validates all tools follow fast_ prefix
func TestSchemaParityNamingConvention(t *testing.T) {
	tools := GetToolDefinitions()

	for _, tool := range tools {
		if len(tool.Name) < 15 || tool.Name[:15] != "llm_filesystem_" {
			t.Errorf("Tool %s should start with 'llm_filesystem_' prefix", tool.Name)
		}
	}
}

// TestSchemaParityExpectedTools validates all expected tools exist
func TestSchemaParityExpectedTools(t *testing.T) {
	tools := GetToolDefinitions()

	// Build map for lookup
	toolMap := make(map[string]bool)
	for _, tool := range tools {
		toolMap[tool.Name] = true
	}

	// Expected tools from fast-filesystem-mcp v3.5.1
	expectedTools := []string{
		// Core file operations
		"llm_filesystem_read_file",
		"llm_filesystem_read_multiple_files",
		"llm_filesystem_write_file",
		"llm_filesystem_large_write_file",

		// Directory operations
		"llm_filesystem_list_directory",
		"llm_filesystem_get_directory_tree",
		"llm_filesystem_create_directory",

		// File info
		"llm_filesystem_get_file_info",

		// Search operations
		"llm_filesystem_search_files",
		"llm_filesystem_search_code",

		// Edit operations
		"llm_filesystem_edit_block",
		"llm_filesystem_edit_blocks",
		"llm_filesystem_edit_multiple_blocks",
		"llm_filesystem_safe_edit",
		"llm_filesystem_edit_file",
		"llm_filesystem_search_and_replace",
		"llm_filesystem_extract_lines",

		// File management
		"llm_filesystem_copy_file",
		"llm_filesystem_move_file",
		"llm_filesystem_delete_file",
		"llm_filesystem_batch_file_operations",

		// Advanced operations
		"llm_filesystem_get_disk_usage",
		"llm_filesystem_find_large_files",
		"llm_filesystem_compress_files",
		"llm_filesystem_extract_archive",
		"llm_filesystem_sync_directories",

		// Info
		"llm_filesystem_list_allowed_directories",
	}

	for _, expected := range expectedTools {
		if !toolMap[expected] {
			t.Errorf("Missing expected tool: %s", expected)
		}
	}

	// Check for unexpected tools
	expectedMap := make(map[string]bool)
	for _, e := range expectedTools {
		expectedMap[e] = true
	}
	for _, tool := range tools {
		if !expectedMap[tool.Name] {
			t.Errorf("Unexpected tool found: %s", tool.Name)
		}
	}
}

// TestSchemaParityPropertyTypes validates property types in schemas
func TestSchemaParityPropertyTypes(t *testing.T) {
	tools := GetToolDefinitions()

	validTypes := map[string]bool{
		"string":  true,
		"number":  true,
		"boolean": true,
		"array":   true,
		"object":  true,
	}

	for _, tool := range tools {
		var schema map[string]interface{}
		json.Unmarshal(tool.InputSchema, &schema)

		props, ok := schema["properties"].(map[string]interface{})
		if !ok {
			continue
		}

		for propName, propVal := range props {
			prop, ok := propVal.(map[string]interface{})
			if !ok {
				t.Errorf("Tool %s property %s is not an object", tool.Name, propName)
				continue
			}

			propType, ok := prop["type"].(string)
			if !ok {
				t.Errorf("Tool %s property %s has no type", tool.Name, propName)
				continue
			}

			if !validTypes[propType] {
				t.Errorf("Tool %s property %s has invalid type: %s", tool.Name, propName, propType)
			}
		}
	}
}

// TestSchemaParityRequiredIsArray validates required field is an array
func TestSchemaParityRequiredIsArray(t *testing.T) {
	tools := GetToolDefinitions()

	for _, tool := range tools {
		var schema map[string]interface{}
		json.Unmarshal(tool.InputSchema, &schema)

		if required, exists := schema["required"]; exists {
			if _, ok := required.([]interface{}); !ok {
				t.Errorf("Tool %s 'required' should be an array, got %T", tool.Name, required)
			}
		}
	}
}

// TestSchemaParityRequiredPropertiesExist validates required properties exist in schema
func TestSchemaParityRequiredPropertiesExist(t *testing.T) {
	tools := GetToolDefinitions()

	for _, tool := range tools {
		var schema map[string]interface{}
		json.Unmarshal(tool.InputSchema, &schema)

		props, ok := schema["properties"].(map[string]interface{})
		if !ok {
			continue
		}

		required, ok := schema["required"].([]interface{})
		if !ok {
			continue
		}

		for _, req := range required {
			reqStr, ok := req.(string)
			if !ok {
				continue
			}
			if _, exists := props[reqStr]; !exists {
				t.Errorf("Tool %s requires property '%s' but it's not defined", tool.Name, reqStr)
			}
		}
	}
}

// TestSchemaParityDescriptionsNotEmpty validates property descriptions exist
func TestSchemaParityDescriptionsNotEmpty(t *testing.T) {
	tools := GetToolDefinitions()

	for _, tool := range tools {
		var schema map[string]interface{}
		json.Unmarshal(tool.InputSchema, &schema)

		props, ok := schema["properties"].(map[string]interface{})
		if !ok {
			continue
		}

		for propName, propVal := range props {
			prop, ok := propVal.(map[string]interface{})
			if !ok {
				continue
			}

			desc, _ := prop["description"].(string)
			if desc == "" {
				t.Errorf("Tool %s property '%s' has no description", tool.Name, propName)
			}
		}
	}
}
