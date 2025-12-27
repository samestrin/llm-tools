package mcp

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// TestServerInitialize tests the initialize handshake
func TestServerInitialize(t *testing.T) {
	input := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}
`
	var output bytes.Buffer
	server := NewServer(strings.NewReader(input), &output)
	server.SetServerInfo("test-server", "1.0.0")

	// Process one request
	err := server.HandleOne()
	if err != nil {
		t.Fatalf("HandleOne() error = %v", err)
	}

	// Parse response
	var resp Response
	if err := json.Unmarshal(output.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	// Verify response
	if resp.Error != nil {
		t.Errorf("Expected success, got error: %v", resp.Error)
	}

	// Parse result
	var result InitializeResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("Failed to parse result: %v", err)
	}

	// Verify capabilities
	if result.ProtocolVersion == "" {
		t.Error("Expected protocol version in result")
	}
	if result.ServerInfo.Name != "test-server" {
		t.Errorf("ServerInfo.Name = %v, want test-server", result.ServerInfo.Name)
	}
	if result.Capabilities.Tools == nil {
		t.Error("Expected tools capability to be present")
	}
}

// TestServerInitialized tests the initialized notification
func TestServerInitialized(t *testing.T) {
	// initialized is a notification (no id), so no response expected
	input := `{"jsonrpc":"2.0","method":"initialized"}
`
	var output bytes.Buffer
	server := NewServer(strings.NewReader(input), &output)

	// Process notification
	err := server.HandleOne()
	if err != nil {
		t.Fatalf("HandleOne() error = %v", err)
	}

	// Notifications should not produce a response
	if output.Len() > 0 {
		t.Errorf("Expected no output for notification, got: %s", output.String())
	}
}

// TestServerToolsList tests the tools/list method
func TestServerToolsList(t *testing.T) {
	input := `{"jsonrpc":"2.0","id":2,"method":"tools/list"}
`
	var output bytes.Buffer
	server := NewServer(strings.NewReader(input), &output)

	// Register a test tool
	server.RegisterTool(Tool{
		Name:        "test_tool",
		Description: "A test tool",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"arg":{"type":"string"}}}`),
	}, func(args map[string]interface{}) (string, error) {
		return "test result", nil
	})

	// Process request
	err := server.HandleOne()
	if err != nil {
		t.Fatalf("HandleOne() error = %v", err)
	}

	// Parse response
	var resp Response
	if err := json.Unmarshal(output.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if resp.Error != nil {
		t.Errorf("Expected success, got error: %v", resp.Error)
	}

	// Parse tools list
	var result ToolsListResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("Failed to parse result: %v", err)
	}

	if len(result.Tools) != 1 {
		t.Errorf("Expected 1 tool, got %d", len(result.Tools))
	}
	if result.Tools[0].Name != "test_tool" {
		t.Errorf("Tool name = %v, want test_tool", result.Tools[0].Name)
	}
}

// TestServerToolsCall tests the tools/call method
func TestServerToolsCall(t *testing.T) {
	input := `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"echo_tool","arguments":{"message":"hello"}}}
`
	var output bytes.Buffer
	server := NewServer(strings.NewReader(input), &output)

	// Register a test tool
	server.RegisterTool(Tool{
		Name:        "echo_tool",
		Description: "Echoes the message",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"message":{"type":"string"}}}`),
	}, func(args map[string]interface{}) (string, error) {
		msg, _ := args["message"].(string)
		return "Echo: " + msg, nil
	})

	// Process request
	err := server.HandleOne()
	if err != nil {
		t.Fatalf("HandleOne() error = %v", err)
	}

	// Parse response
	var resp Response
	if err := json.Unmarshal(output.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if resp.Error != nil {
		t.Errorf("Expected success, got error: %v", resp.Error)
	}

	// Parse call result
	var result ToolsCallResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("Failed to parse result: %v", err)
	}

	if len(result.Content) != 1 {
		t.Errorf("Expected 1 content item, got %d", len(result.Content))
	}
	if result.Content[0].Type != "text" {
		t.Errorf("Content type = %v, want text", result.Content[0].Type)
	}
	if result.Content[0].Text != "Echo: hello" {
		t.Errorf("Content text = %v, want 'Echo: hello'", result.Content[0].Text)
	}
}

// TestServerMethodNotFound tests handling of unknown methods
func TestServerMethodNotFound(t *testing.T) {
	input := `{"jsonrpc":"2.0","id":4,"method":"unknown/method"}
`
	var output bytes.Buffer
	server := NewServer(strings.NewReader(input), &output)

	// Process request
	err := server.HandleOne()
	if err != nil {
		t.Fatalf("HandleOne() error = %v", err)
	}

	// Parse response
	var resp Response
	if err := json.Unmarshal(output.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if resp.Error == nil {
		t.Error("Expected error response for unknown method")
	}
	if resp.Error.Code != MethodNotFound {
		t.Errorf("Error code = %v, want %v", resp.Error.Code, MethodNotFound)
	}
}

// TestServerToolNotFound tests handling of unknown tool
func TestServerToolNotFound(t *testing.T) {
	input := `{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"nonexistent","arguments":{}}}
`
	var output bytes.Buffer
	server := NewServer(strings.NewReader(input), &output)

	// Process request
	err := server.HandleOne()
	if err != nil {
		t.Fatalf("HandleOne() error = %v", err)
	}

	// Parse response
	var resp Response
	if err := json.Unmarshal(output.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	// Tool errors should be returned as text content, not JSON-RPC errors
	if resp.Error != nil {
		t.Errorf("Expected tool error in content, got JSON-RPC error: %v", resp.Error)
	}

	// Parse call result - should have error in content
	var result ToolsCallResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("Failed to parse result: %v", err)
	}

	if len(result.Content) == 0 {
		t.Error("Expected content with error message")
	}
	if !strings.Contains(result.Content[0].Text, "not found") {
		t.Errorf("Expected 'not found' in error message, got: %s", result.Content[0].Text)
	}
}

// TestServerToolError tests handling of tool execution errors
func TestServerToolError(t *testing.T) {
	input := `{"jsonrpc":"2.0","id":6,"method":"tools/call","params":{"name":"failing_tool","arguments":{}}}
`
	var output bytes.Buffer
	server := NewServer(strings.NewReader(input), &output)

	// Register a tool that returns an error
	server.RegisterTool(Tool{
		Name:        "failing_tool",
		Description: "A tool that fails",
		InputSchema: json.RawMessage(`{"type":"object"}`),
	}, func(args map[string]interface{}) (string, error) {
		return "", NewToolError("intentional failure")
	})

	// Process request
	err := server.HandleOne()
	if err != nil {
		t.Fatalf("HandleOne() error = %v", err)
	}

	// Parse response
	var resp Response
	if err := json.Unmarshal(output.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	// Tool errors should be returned as text content, not JSON-RPC errors
	if resp.Error != nil {
		t.Errorf("Expected tool error in content, got JSON-RPC error: %v", resp.Error)
	}

	// Parse call result
	var result ToolsCallResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("Failed to parse result: %v", err)
	}

	if result.IsError != true {
		t.Error("Expected isError to be true for tool error")
	}
	if len(result.Content) == 0 {
		t.Error("Expected content with error message")
	}
	if !strings.Contains(result.Content[0].Text, "intentional failure") {
		t.Errorf("Expected error message in content, got: %s", result.Content[0].Text)
	}
}
