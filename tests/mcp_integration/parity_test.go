package mcp_integration

import (
	"bytes"
	"encoding/json"
	"os/exec"
	"strings"
	"testing"

	"github.com/samestrin/llm-tools/internal/mcp"
	supportserver "github.com/samestrin/llm-tools/internal/support/mcpserver"
	clarifyserver "github.com/samestrin/llm-tools/internal/clarification/mcpserver"
)

// TestToolCountParity verifies Go and Python have same tool counts
func TestToolCountParity(t *testing.T) {
	// Go implementations
	goSupportTools := supportserver.GetTools()
	goClarifyTools := clarifyserver.GetTools()

	// Verify counts match expected
	if len(goSupportTools) != 18 {
		t.Errorf("Go llm-support has %d tools, expected 18", len(goSupportTools))
	}
	if len(goClarifyTools) != 8 {
		t.Errorf("Go llm-clarification has %d tools, expected 8", len(goClarifyTools))
	}
}

// TestToolNameParity verifies all Go tools have matching Python tool names
func TestToolNameParity(t *testing.T) {
	expectedSupportTools := []string{
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

	expectedClarifyTools := []string{
		"llm_clarify_match",
		"llm_clarify_cluster",
		"llm_clarify_detect_conflicts",
		"llm_clarify_validate",
		"llm_clarify_init",
		"llm_clarify_add",
		"llm_clarify_promote",
		"llm_clarify_list",
	}

	// Check support tools
	goSupportTools := supportserver.GetTools()
	supportMap := make(map[string]bool)
	for _, tool := range goSupportTools {
		supportMap[tool.Name] = true
	}
	for _, expected := range expectedSupportTools {
		if !supportMap[expected] {
			t.Errorf("Missing support tool: %s", expected)
		}
	}

	// Check clarify tools
	goClarifyTools := clarifyserver.GetTools()
	clarifyMap := make(map[string]bool)
	for _, tool := range goClarifyTools {
		clarifyMap[tool.Name] = true
	}
	for _, expected := range expectedClarifyTools {
		if !clarifyMap[expected] {
			t.Errorf("Missing clarify tool: %s", expected)
		}
	}
}

// TestSchemaPropertyParity verifies tool schemas have correct property structure
func TestSchemaPropertyParity(t *testing.T) {
	// Key tools with specific required properties
	propertyChecks := map[string][]string{
		"llm_support_tree":             {"path"},
		"llm_support_grep":             {"pattern", "paths"},
		"llm_support_multiexists":      {"paths"},
		"llm_support_json_query":       {"file", "query"},
		"llm_support_markdown_headers": {"file"},
		"llm_support_template":         {"file", "vars"},
		"llm_support_multigrep":        {"keywords"},
		"llm_support_count":            {"mode", "target"},
		"llm_support_summarize_dir":    {"path"},
		"llm_support_deps":             {"manifest"},
		"llm_support_git_context":      {"path"},
		"llm_support_validate_plan":    {"path"},
		"llm_support_partition_work":   {"stories"},
		"llm_support_repo_root":        {"path"},
	}

	tools := supportserver.GetTools()
	toolMap := make(map[string]mcp.Tool)
	for _, tool := range tools {
		toolMap[tool.Name] = tool
	}

	for toolName, expectedProps := range propertyChecks {
		tool, ok := toolMap[toolName]
		if !ok {
			t.Errorf("Tool %s not found", toolName)
			continue
		}

		var schema map[string]interface{}
		if err := json.Unmarshal(tool.InputSchema, &schema); err != nil {
			t.Errorf("Tool %s has invalid schema: %v", toolName, err)
			continue
		}

		properties, ok := schema["properties"].(map[string]interface{})
		if !ok {
			t.Errorf("Tool %s missing properties object", toolName)
			continue
		}

		for _, prop := range expectedProps {
			if _, exists := properties[prop]; !exists {
				t.Errorf("Tool %s missing expected property: %s", toolName, prop)
			}
		}
	}
}

// TestClarifySchemaPropertyParity verifies clarify tool schemas
func TestClarifySchemaPropertyParity(t *testing.T) {
	propertyChecks := map[string][]string{
		"llm_clarify_match":            {"question"},
		"llm_clarify_cluster":          {"questions_file"},
		"llm_clarify_detect_conflicts": {"tracking_file"},
		"llm_clarify_validate":         {"tracking_file"},
		"llm_clarify_init":             {"output"},
		"llm_clarify_add":              {"tracking_file", "question"},
		"llm_clarify_promote":          {"tracking_file", "id", "target"},
		"llm_clarify_list":             {"tracking_file"},
	}

	tools := clarifyserver.GetTools()
	toolMap := make(map[string]mcp.Tool)
	for _, tool := range tools {
		toolMap[tool.Name] = tool
	}

	for toolName, expectedProps := range propertyChecks {
		tool, ok := toolMap[toolName]
		if !ok {
			t.Errorf("Tool %s not found", toolName)
			continue
		}

		var schema map[string]interface{}
		if err := json.Unmarshal(tool.InputSchema, &schema); err != nil {
			t.Errorf("Tool %s has invalid schema: %v", toolName, err)
			continue
		}

		properties, ok := schema["properties"].(map[string]interface{})
		if !ok {
			t.Errorf("Tool %s missing properties object", toolName)
			continue
		}

		for _, prop := range expectedProps {
			if _, exists := properties[prop]; !exists {
				t.Errorf("Tool %s missing expected property: %s", toolName, prop)
			}
		}
	}
}

// TestMCPProtocolParity tests that MCP protocol responses match expected format
func TestMCPProtocolParity(t *testing.T) {
	// Test initialize response format
	t.Run("initialize_response_format", func(t *testing.T) {
		input := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}
`
		var output bytes.Buffer
		server := mcp.NewServer(strings.NewReader(input), &output)
		server.SetServerInfo("test-server", "1.0.0")

		if err := server.HandleOne(); err != nil {
			t.Fatalf("HandleOne() error = %v", err)
		}

		var resp mcp.Response
		if err := json.Unmarshal(output.Bytes(), &resp); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		// Verify response structure
		if resp.JSONRPC != "2.0" {
			t.Errorf("JSONRPC = %v, want 2.0", resp.JSONRPC)
		}
		if resp.Error != nil {
			t.Errorf("Unexpected error: %v", resp.Error)
		}

		var result mcp.InitializeResult
		if err := json.Unmarshal(resp.Result, &result); err != nil {
			t.Fatalf("Failed to parse result: %v", err)
		}

		// Verify capabilities match Python implementation
		if !result.Capabilities.Tools {
			t.Error("Expected tools capability to be true")
		}
	})

	// Test tools/list response format
	t.Run("tools_list_response_format", func(t *testing.T) {
		input := `{"jsonrpc":"2.0","id":1,"method":"tools/list"}
`
		var output bytes.Buffer
		server := mcp.NewServer(strings.NewReader(input), &output)
		server.SetServerInfo("test-server", "1.0.0")

		// Register a test tool
		server.RegisterTool(mcp.Tool{
			Name:        "test_tool",
			Description: "Test tool",
			InputSchema: json.RawMessage(`{"type":"object","properties":{}}`),
		}, func(args map[string]interface{}) (string, error) {
			return "test", nil
		})

		if err := server.HandleOne(); err != nil {
			t.Fatalf("HandleOne() error = %v", err)
		}

		var resp mcp.Response
		if err := json.Unmarshal(output.Bytes(), &resp); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		var result mcp.ToolsListResult
		if err := json.Unmarshal(resp.Result, &result); err != nil {
			t.Fatalf("Failed to parse result: %v", err)
		}

		if len(result.Tools) != 1 {
			t.Errorf("Expected 1 tool, got %d", len(result.Tools))
		}
	})

	// Test error response format
	t.Run("error_response_format", func(t *testing.T) {
		input := `{"jsonrpc":"2.0","id":1,"method":"unknown/method"}
`
		var output bytes.Buffer
		server := mcp.NewServer(strings.NewReader(input), &output)
		server.SetServerInfo("test-server", "1.0.0")

		if err := server.HandleOne(); err != nil {
			t.Fatalf("HandleOne() error = %v", err)
		}

		var resp mcp.Response
		if err := json.Unmarshal(output.Bytes(), &resp); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		if resp.Error == nil {
			t.Fatal("Expected error response")
		}
		if resp.Error.Code != mcp.MethodNotFound {
			t.Errorf("Error code = %v, want %v", resp.Error.Code, mcp.MethodNotFound)
		}
	})
}

// TestBinaryBuildParity verifies both binaries build successfully
func TestBinaryBuildParity(t *testing.T) {
	// Find repo root by looking for go.mod
	cmd := exec.Command("go", "list", "-m", "-f", "{{.Dir}}")
	rootBytes, err := cmd.Output()
	if err != nil {
		t.Fatalf("Failed to find module root: %v", err)
	}
	root := strings.TrimSpace(string(rootBytes))

	binaries := []struct {
		name string
		path string
	}{
		{"llm-support-mcp", root + "/cmd/llm-support-mcp"},
		{"llm-clarification-mcp", root + "/cmd/llm-clarification-mcp"},
	}

	for _, binary := range binaries {
		t.Run(binary.name, func(t *testing.T) {
			cmd := exec.Command("go", "build", "-o", "/dev/null", binary.path)
			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Errorf("Failed to build %s: %v\nOutput: %s", binary.name, err, output)
			}
		})
	}
}

// TestToolDescriptionParity verifies all tools have non-empty descriptions
func TestToolDescriptionParity(t *testing.T) {
	allTools := append(supportserver.GetTools(), clarifyserver.GetTools()...)

	for _, tool := range allTools {
		if tool.Description == "" {
			t.Errorf("Tool %s has empty description", tool.Name)
		}
		// Descriptions should be meaningful (at least 20 chars)
		if len(tool.Description) < 20 {
			t.Errorf("Tool %s has too short description: %q", tool.Name, tool.Description)
		}
	}
}

// TestSchemaTypeParity verifies all schemas have type: object
func TestSchemaTypeParity(t *testing.T) {
	allTools := append(supportserver.GetTools(), clarifyserver.GetTools()...)

	for _, tool := range allTools {
		var schema map[string]interface{}
		if err := json.Unmarshal(tool.InputSchema, &schema); err != nil {
			t.Errorf("Tool %s has invalid schema: %v", tool.Name, err)
			continue
		}

		schemaType, ok := schema["type"].(string)
		if !ok || schemaType != "object" {
			t.Errorf("Tool %s schema type = %v, want object", tool.Name, schema["type"])
		}
	}
}
