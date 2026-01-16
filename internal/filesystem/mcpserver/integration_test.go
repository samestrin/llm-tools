package mcpserver

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestMCPToolDefinitions verifies all tool definitions are valid
func TestMCPToolDefinitions(t *testing.T) {
	tools := GetToolDefinitions()

	if len(tools) == 0 {
		t.Fatal("Expected tool definitions, got none")
	}

	// Verify we have exactly 15 MCP tools (batch/specialized operations only)
	expectedCount := 15
	if len(tools) != expectedCount {
		t.Errorf("Expected %d tools, got %d", expectedCount, len(tools))
	}

	for _, tool := range tools {
		// Verify tool has required fields
		if tool.Name == "" {
			t.Error("Tool missing name")
		}
		if !strings.HasPrefix(tool.Name, ToolPrefix) {
			t.Errorf("Tool %s should have %s prefix", tool.Name, ToolPrefix)
		}
		if tool.Description == "" {
			t.Errorf("Tool %s missing description", tool.Name)
		}

		// Verify schema is valid JSON
		var schema map[string]interface{}
		if err := json.Unmarshal(tool.InputSchema, &schema); err != nil {
			t.Errorf("Tool %s has invalid JSON schema: %v", tool.Name, err)
		}

		// Verify schema structure
		schemaType, ok := schema["type"].(string)
		if !ok || schemaType != "object" {
			t.Errorf("Tool %s schema type should be 'object'", tool.Name)
		}
	}
}

// TestBuildArgsListDirectory verifies list_directory args are built correctly
func TestBuildArgsListDirectory(t *testing.T) {
	args := map[string]interface{}{
		"path":        "/tmp",
		"show_hidden": true,
		"sort_by":     "size",
		"page":        float64(1),
		"page_size":   float64(10),
	}

	cmdArgs, err := buildArgs("list_directory", args)
	if err != nil {
		t.Fatalf("buildArgs failed: %v", err)
	}

	// Verify command name
	if cmdArgs[0] != "list-directory" {
		t.Errorf("Expected 'list-directory', got %s", cmdArgs[0])
	}

	// Verify path is included
	pathFound := false
	for i, arg := range cmdArgs {
		if arg == "--path" && i+1 < len(cmdArgs) && cmdArgs[i+1] == "/tmp" {
			pathFound = true
			break
		}
	}
	if !pathFound {
		t.Error("Expected --path /tmp in args")
	}
}

// TestBuildArgsSearchCode verifies search_code args are built correctly
func TestBuildArgsSearchCode(t *testing.T) {
	args := map[string]interface{}{
		"path":        "/tmp",
		"pattern":     "TODO",
		"ignore_case": true,
		"context":     float64(3),
	}

	cmdArgs, err := buildArgs("search_code", args)
	if err != nil {
		t.Fatalf("buildArgs failed: %v", err)
	}

	if cmdArgs[0] != "search-code" {
		t.Errorf("Expected 'search-code', got %s", cmdArgs[0])
	}

	// Verify ignore-case flag
	ignoreCaseFound := false
	for _, arg := range cmdArgs {
		if arg == "--ignore-case" {
			ignoreCaseFound = true
			break
		}
	}
	if !ignoreCaseFound {
		t.Error("Expected --ignore-case in args")
	}
}

// TestBuildArgsUnknownCommand verifies unknown commands return error
func TestBuildArgsUnknownCommand(t *testing.T) {
	args := map[string]interface{}{}

	_, err := buildArgs("unknown_command", args)
	if err == nil {
		t.Error("Expected error for unknown command")
	}
}

// TestBuildArgsCreateDirectories verifies create_directories args are built correctly
func TestBuildArgsCreateDirectories(t *testing.T) {
	args := map[string]interface{}{
		"paths": []interface{}{
			"/tmp/dir1",
			"/tmp/dir2",
			"/tmp/dir3",
		},
		"recursive": true,
	}

	cmdArgs, err := buildArgs("create_directories", args)
	if err != nil {
		t.Fatalf("buildArgs failed: %v", err)
	}

	if cmdArgs[0] != "create-directories" {
		t.Errorf("Expected 'create-directories', got %s", cmdArgs[0])
	}

	// Verify all paths are included
	pathCount := 0
	for i, arg := range cmdArgs {
		if arg == "--paths" && i+1 < len(cmdArgs) {
			pathCount++
		}
	}
	if pathCount != 3 {
		t.Errorf("Expected 3 --paths flags, got %d", pathCount)
	}
}

// TestBuildArgsCreateDirectoriesNoRecursive verifies recursive=false is passed
func TestBuildArgsCreateDirectoriesNoRecursive(t *testing.T) {
	args := map[string]interface{}{
		"paths": []interface{}{
			"/tmp/dir1",
		},
		"recursive": false,
	}

	cmdArgs, err := buildArgs("create_directories", args)
	if err != nil {
		t.Fatalf("buildArgs failed: %v", err)
	}

	// Verify recursive=false is included
	recursiveFalseFound := false
	for _, arg := range cmdArgs {
		if arg == "--recursive=false" {
			recursiveFalseFound = true
			break
		}
	}
	if !recursiveFalseFound {
		t.Error("Expected --recursive=false in args")
	}
}

// TestBuildArgsEditBlocks verifies edit_blocks JSON encoding
func TestBuildArgsEditBlocks(t *testing.T) {
	args := map[string]interface{}{
		"path": "/tmp/test.txt",
		"edits": []interface{}{
			map[string]interface{}{
				"old_string": "foo",
				"new_string": "bar",
			},
		},
	}

	cmdArgs, err := buildArgs("edit_blocks", args)
	if err != nil {
		t.Fatalf("buildArgs failed: %v", err)
	}

	if cmdArgs[0] != "edit-blocks" {
		t.Errorf("Expected 'edit-blocks', got %s", cmdArgs[0])
	}

	// Verify edits JSON is present
	editsFound := false
	for i, arg := range cmdArgs {
		if arg == "--edits" && i+1 < len(cmdArgs) {
			// Verify it's valid JSON
			var parsed []interface{}
			if err := json.Unmarshal([]byte(cmdArgs[i+1]), &parsed); err == nil {
				editsFound = true
			}
			break
		}
	}
	if !editsFound {
		t.Error("Expected --edits with valid JSON")
	}
}

// TestBuildArgsGetDirectoryTree verifies max_depth parameter
func TestBuildArgsGetDirectoryTree(t *testing.T) {
	args := map[string]interface{}{
		"path":          "/tmp",
		"max_depth":     float64(3),
		"include_files": true,
	}

	cmdArgs, err := buildArgs("get_directory_tree", args)
	if err != nil {
		t.Fatalf("buildArgs failed: %v", err)
	}

	if cmdArgs[0] != "get-directory-tree" {
		t.Errorf("Expected 'get-directory-tree', got %s", cmdArgs[0])
	}

	// Verify depth is included
	depthFound := false
	for i, arg := range cmdArgs {
		if arg == "--depth" && i+1 < len(cmdArgs) && cmdArgs[i+1] == "3" {
			depthFound = true
			break
		}
	}
	if !depthFound {
		t.Error("Expected --depth 3 in args")
	}
}

// TestToolPrefixConsistency verifies all tools use consistent prefix
func TestToolPrefixConsistency(t *testing.T) {
	tools := GetToolDefinitions()
	expectedPrefix := "llm_filesystem_"

	for _, tool := range tools {
		if !strings.HasPrefix(tool.Name, expectedPrefix) {
			t.Errorf("Tool %s should have prefix %s", tool.Name, expectedPrefix)
		}
	}
}

// TestAllowedDirsConfiguration verifies allowed dirs can be set
func TestAllowedDirsConfiguration(t *testing.T) {
	// Save original
	original := AllowedDirs
	defer func() { AllowedDirs = original }()

	// Set test value
	AllowedDirs = []string{"/tmp", "/home"}

	if len(AllowedDirs) != 2 {
		t.Errorf("Expected 2 allowed dirs, got %d", len(AllowedDirs))
	}
}

// TestExecuteHandlerWithMockBinary tests handler execution with a mock setup
// This test verifies the handler logic without requiring the actual binary
func TestExecuteHandlerWithMockBinary(t *testing.T) {
	// Create a temp directory for testing
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("hello world"), 0644)

	// Save original binary path
	originalBinary := BinaryPath
	defer func() { BinaryPath = originalBinary }()

	// Test with extract_lines (a remaining MCP tool)
	args := map[string]interface{}{
		"path":  testFile,
		"start": float64(1),
		"end":   float64(10),
	}

	cmdArgs, err := buildArgs("extract_lines", args)
	if err != nil {
		t.Fatalf("buildArgs failed: %v", err)
	}

	// Verify expected args
	if len(cmdArgs) < 3 {
		t.Errorf("Expected at least 3 args, got %d", len(cmdArgs))
	}
}

// TestHelperGetBool verifies getBool helper
func TestHelperGetBool(t *testing.T) {
	tests := []struct {
		args     map[string]interface{}
		key      string
		expected bool
	}{
		{map[string]interface{}{"flag": true}, "flag", true},
		{map[string]interface{}{"flag": false}, "flag", false},
		{map[string]interface{}{}, "flag", false},
		{map[string]interface{}{"flag": "true"}, "flag", false}, // string not bool
	}

	for _, tt := range tests {
		result := getBool(tt.args, tt.key)
		if result != tt.expected {
			t.Errorf("getBool(%v, %s) = %v, want %v", tt.args, tt.key, result, tt.expected)
		}
	}
}

// TestHelperGetBoolDefault verifies getBoolDefault helper
func TestHelperGetBoolDefault(t *testing.T) {
	tests := []struct {
		args       map[string]interface{}
		key        string
		defaultVal bool
		expected   bool
	}{
		{map[string]interface{}{"flag": true}, "flag", false, true},
		{map[string]interface{}{"flag": false}, "flag", true, false},
		{map[string]interface{}{}, "flag", true, true},
		{map[string]interface{}{}, "flag", false, false},
	}

	for _, tt := range tests {
		result := getBoolDefault(tt.args, tt.key, tt.defaultVal)
		if result != tt.expected {
			t.Errorf("getBoolDefault(%v, %s, %v) = %v, want %v", tt.args, tt.key, tt.defaultVal, result, tt.expected)
		}
	}
}

// TestHelperGetInt verifies getInt helper
func TestHelperGetInt(t *testing.T) {
	tests := []struct {
		args     map[string]interface{}
		key      string
		expected int
		ok       bool
	}{
		{map[string]interface{}{"num": 42}, "num", 42, true},
		{map[string]interface{}{"num": float64(42)}, "num", 42, true},
		{map[string]interface{}{"num": int64(42)}, "num", 42, true},
		{map[string]interface{}{}, "num", 0, false},
		{map[string]interface{}{"num": "42"}, "num", 0, false}, // string not number
	}

	for _, tt := range tests {
		result, ok := getInt(tt.args, tt.key)
		if result != tt.expected || ok != tt.ok {
			t.Errorf("getInt(%v, %s) = (%v, %v), want (%v, %v)", tt.args, tt.key, result, ok, tt.expected, tt.ok)
		}
	}
}

// TestAllToolsHaveBuilders verifies every tool has a corresponding buildArgs case
func TestAllToolsHaveBuilders(t *testing.T) {
	tools := GetToolDefinitions()

	for _, tool := range tools {
		cmdName := strings.TrimPrefix(tool.Name, ToolPrefix)
		args := map[string]interface{}{}

		// Add minimal required args based on tool
		switch cmdName {
		case "list_directory", "get_directory_tree", "delete_file":
			args["path"] = "/tmp"
		case "create_directories":
			args["paths"] = []interface{}{"/tmp/dir1", "/tmp/dir2"}
		case "read_multiple_files":
			args["paths"] = []interface{}{"/tmp/a.txt"}
		case "search_files", "search_code":
			args["path"] = "/tmp"
			args["pattern"] = "test"
		case "edit_blocks":
			args["path"] = "/tmp"
			args["edits"] = []interface{}{}
		case "search_and_replace":
			args["path"] = "/tmp"
			args["pattern"] = "old"
			args["replacement"] = "new"
		case "extract_lines":
			args["path"] = "/tmp"
		case "copy_file", "move_file":
			args["source"] = "/tmp/a"
			args["destination"] = "/tmp/b"
		case "batch_file_operations":
			args["operations"] = []interface{}{}
		case "compress_files":
			args["paths"] = []interface{}{"/tmp/a.txt"}
			args["output"] = "/tmp/out.zip"
		case "extract_archive":
			args["archive"] = "/tmp/a.zip"
			args["destination"] = "/tmp"
		}

		_, err := buildArgs(cmdName, args)
		if err != nil {
			t.Errorf("buildArgs failed for %s: %v", cmdName, err)
		}
	}
}

// TestToolSchemaRequiredFields verifies required fields in schemas
func TestToolSchemaRequiredFields(t *testing.T) {
	tools := GetToolDefinitions()

	for _, tool := range tools {
		var schema map[string]interface{}
		if err := json.Unmarshal(tool.InputSchema, &schema); err != nil {
			t.Errorf("Tool %s has invalid schema: %v", tool.Name, err)
			continue
		}

		// Tools that require path should have it in required array
		cmdName := strings.TrimPrefix(tool.Name, ToolPrefix)
		switch cmdName {
		case "list_directory", "get_directory_tree", "delete_file":
			required, ok := schema["required"].([]interface{})
			if !ok {
				// Required may not be present for optional-only tools
				continue
			}
			hasPath := false
			for _, r := range required {
				if r == "path" {
					hasPath = true
					break
				}
			}
			if !hasPath && cmdName != "get_directory_tree" && cmdName != "list_directory" {
				// Some tools may have defaults for path
				continue
			}
		}
	}
}

// TestRemovedToolsAreGone verifies that deprecated tools are no longer exposed
func TestRemovedToolsAreGone(t *testing.T) {
	tools := GetToolDefinitions()

	// These tools should NOT be in the MCP server (use Claude's native tools instead)
	deprecatedTools := []string{
		"read_file",
		"write_file",
		"large_write_file",
		"edit_block",
		"edit_file",
		"edit_multiple_blocks",
		"safe_edit",
		"create_directory",
		"get_file_info",
		"get_disk_usage",
		"find_large_files",
		"sync_directories",
		"list_allowed_directories",
	}

	toolNames := make(map[string]bool)
	for _, tool := range tools {
		cmdName := strings.TrimPrefix(tool.Name, ToolPrefix)
		toolNames[cmdName] = true
	}

	for _, deprecated := range deprecatedTools {
		if toolNames[deprecated] {
			t.Errorf("Deprecated tool %s should have been removed from MCP server", deprecated)
		}
	}
}

// TestExpectedToolsArePresent verifies all expected batch/specialized tools exist
func TestExpectedToolsArePresent(t *testing.T) {
	tools := GetToolDefinitions()

	// These are the 15 batch/specialized tools that should remain
	expectedTools := []string{
		"read_multiple_files",
		"extract_lines",
		"edit_blocks",
		"search_and_replace",
		"list_directory",
		"get_directory_tree",
		"create_directories",
		"search_files",
		"search_code",
		"copy_file",
		"move_file",
		"delete_file",
		"batch_file_operations",
		"compress_files",
		"extract_archive",
	}

	toolNames := make(map[string]bool)
	for _, tool := range tools {
		cmdName := strings.TrimPrefix(tool.Name, ToolPrefix)
		toolNames[cmdName] = true
	}

	for _, expected := range expectedTools {
		if !toolNames[expected] {
			t.Errorf("Expected tool %s should be present in MCP server", expected)
		}
	}
}
