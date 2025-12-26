package mcp_integration

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	clarifyserver "github.com/samestrin/llm-tools/internal/clarification/mcpserver"
	"github.com/samestrin/llm-tools/internal/mcp"
	supportserver "github.com/samestrin/llm-tools/internal/support/mcpserver"
)

// TestLLMSupportMCPInitialize tests the initialize handshake
func TestLLMSupportMCPInitialize(t *testing.T) {
	input := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}
`
	var output bytes.Buffer
	server := mcp.NewServer(strings.NewReader(input), &output)
	server.SetServerInfo("llm-support-mcp", "1.0.0")

	// Register all support tools
	for _, tool := range supportserver.GetTools() {
		toolName := tool.Name
		server.RegisterTool(tool, func(args map[string]interface{}) (string, error) {
			return "test", nil
		})
		_ = toolName // Avoid unused variable
	}

	// Process request
	if err := server.HandleOne(); err != nil {
		t.Fatalf("HandleOne() error = %v", err)
	}

	// Parse response
	var resp mcp.Response
	if err := json.Unmarshal(output.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if resp.Error != nil {
		t.Errorf("Expected success, got error: %v", resp.Error)
	}

	// Parse result
	var result mcp.InitializeResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("Failed to parse result: %v", err)
	}

	if result.ServerInfo.Name != "llm-support-mcp" {
		t.Errorf("ServerInfo.Name = %v, want llm-support-mcp", result.ServerInfo.Name)
	}
	if !result.Capabilities.Tools {
		t.Error("Expected tools capability to be true")
	}
}

// TestLLMSupportMCPToolsList tests that tools/list returns all 18 tools
func TestLLMSupportMCPToolsList(t *testing.T) {
	input := `{"jsonrpc":"2.0","id":2,"method":"tools/list"}
`
	var output bytes.Buffer
	server := mcp.NewServer(strings.NewReader(input), &output)
	server.SetServerInfo("llm-support-mcp", "1.0.0")

	// Register all support tools
	for _, tool := range supportserver.GetTools() {
		toolName := tool.Name
		server.RegisterTool(tool, func(args map[string]interface{}) (string, error) {
			return "test", nil
		})
		_ = toolName
	}

	// Process request
	if err := server.HandleOne(); err != nil {
		t.Fatalf("HandleOne() error = %v", err)
	}

	// Parse response
	var resp mcp.Response
	if err := json.Unmarshal(output.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if resp.Error != nil {
		t.Errorf("Expected success, got error: %v", resp.Error)
	}

	// Parse result
	var result mcp.ToolsListResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("Failed to parse result: %v", err)
	}

	if len(result.Tools) != 18 {
		t.Errorf("Expected 18 tools, got %d", len(result.Tools))
	}
}

// TestLLMClarificationMCPInitialize tests the initialize handshake
func TestLLMClarificationMCPInitialize(t *testing.T) {
	input := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}
`
	var output bytes.Buffer
	server := mcp.NewServer(strings.NewReader(input), &output)
	server.SetServerInfo("llm-clarification-mcp", "1.0.0")

	// Register all clarification tools
	for _, tool := range clarifyserver.GetTools() {
		toolName := tool.Name
		server.RegisterTool(tool, func(args map[string]interface{}) (string, error) {
			return "test", nil
		})
		_ = toolName
	}

	// Process request
	if err := server.HandleOne(); err != nil {
		t.Fatalf("HandleOne() error = %v", err)
	}

	// Parse response
	var resp mcp.Response
	if err := json.Unmarshal(output.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if resp.Error != nil {
		t.Errorf("Expected success, got error: %v", resp.Error)
	}

	// Parse result
	var result mcp.InitializeResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("Failed to parse result: %v", err)
	}

	if result.ServerInfo.Name != "llm-clarification-mcp" {
		t.Errorf("ServerInfo.Name = %v, want llm-clarification-mcp", result.ServerInfo.Name)
	}
}

// TestLLMClarificationMCPToolsList tests that tools/list returns all 8 tools
func TestLLMClarificationMCPToolsList(t *testing.T) {
	input := `{"jsonrpc":"2.0","id":2,"method":"tools/list"}
`
	var output bytes.Buffer
	server := mcp.NewServer(strings.NewReader(input), &output)
	server.SetServerInfo("llm-clarification-mcp", "1.0.0")

	// Register all clarification tools
	for _, tool := range clarifyserver.GetTools() {
		toolName := tool.Name
		server.RegisterTool(tool, func(args map[string]interface{}) (string, error) {
			return "test", nil
		})
		_ = toolName
	}

	// Process request
	if err := server.HandleOne(); err != nil {
		t.Fatalf("HandleOne() error = %v", err)
	}

	// Parse response
	var resp mcp.Response
	if err := json.Unmarshal(output.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if resp.Error != nil {
		t.Errorf("Expected success, got error: %v", resp.Error)
	}

	// Parse result
	var result mcp.ToolsListResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("Failed to parse result: %v", err)
	}

	if len(result.Tools) != 8 {
		t.Errorf("Expected 8 tools, got %d", len(result.Tools))
	}
}

// TestMCPMethodNotFound tests that unknown methods return proper error
func TestMCPMethodNotFound(t *testing.T) {
	input := `{"jsonrpc":"2.0","id":99,"method":"unknown/method"}
`
	var output bytes.Buffer
	server := mcp.NewServer(strings.NewReader(input), &output)
	server.SetServerInfo("test-server", "1.0.0")

	// Process request
	if err := server.HandleOne(); err != nil {
		t.Fatalf("HandleOne() error = %v", err)
	}

	// Parse response
	var resp mcp.Response
	if err := json.Unmarshal(output.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if resp.Error == nil {
		t.Error("Expected error response for unknown method")
	}
	if resp.Error.Code != mcp.MethodNotFound {
		t.Errorf("Error code = %v, want %v", resp.Error.Code, mcp.MethodNotFound)
	}
}
