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

// BenchmarkServerStartup measures MCP server initialization time
func BenchmarkServerStartup(b *testing.B) {
	input := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}
`
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var output bytes.Buffer
		server := mcp.NewServer(strings.NewReader(input), &output)
		server.SetServerInfo("llm-support-mcp", "1.0.0")

		// Register all tools
		for _, tool := range supportserver.GetTools() {
			server.RegisterTool(tool, func(args map[string]interface{}) (string, error) {
				return "test", nil
			})
		}

		if err := server.HandleOne(); err != nil {
			b.Fatalf("HandleOne() error = %v", err)
		}
	}
}

// BenchmarkToolsListWithAllTools measures tools/list with 18 tools registered
func BenchmarkToolsListWithAllTools(b *testing.B) {
	input := `{"jsonrpc":"2.0","id":1,"method":"tools/list"}
`
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var output bytes.Buffer
		server := mcp.NewServer(strings.NewReader(input), &output)
		server.SetServerInfo("llm-support-mcp", "1.0.0")

		for _, tool := range supportserver.GetTools() {
			server.RegisterTool(tool, func(args map[string]interface{}) (string, error) {
				return "test", nil
			})
		}

		if err := server.HandleOne(); err != nil {
			b.Fatalf("HandleOne() error = %v", err)
		}
	}
}

// BenchmarkToolRegistration measures tool registration overhead
func BenchmarkToolRegistration(b *testing.B) {
	tools := supportserver.GetTools()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var output bytes.Buffer
		server := mcp.NewServer(strings.NewReader(""), &output)
		for _, tool := range tools {
			server.RegisterTool(tool, func(args map[string]interface{}) (string, error) {
				return "test", nil
			})
		}
	}
}

// BenchmarkJSONParsing measures JSON-RPC parsing performance
func BenchmarkJSONParsing(b *testing.B) {
	requests := []string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"llm_support_tree","arguments":{"path":"."}}}`,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, req := range requests {
			var parsed mcp.Request
			if err := json.Unmarshal([]byte(req), &parsed); err != nil {
				b.Fatalf("Parse error: %v", err)
			}
		}
	}
}

// BenchmarkResponseSerialization measures response serialization
func BenchmarkResponseSerialization(b *testing.B) {
	result := mcp.ToolsListResult{
		Tools: supportserver.GetTools(),
	}
	resultBytes, _ := json.Marshal(result)
	response := mcp.Response{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Result:  resultBytes,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := json.Marshal(response); err != nil {
			b.Fatalf("Marshal error: %v", err)
		}
	}
}

// BenchmarkGetToolsSupport measures GetTools() call overhead for support
func BenchmarkGetToolsSupport(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = supportserver.GetTools()
	}
}

// BenchmarkGetToolsClarify measures GetTools() call overhead for clarification
func BenchmarkGetToolsClarify(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = clarifyserver.GetTools()
	}
}

// BenchmarkSchemaValidation measures schema validation overhead
func BenchmarkSchemaValidation(b *testing.B) {
	tools := supportserver.GetTools()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, tool := range tools {
			var schema map[string]interface{}
			if err := json.Unmarshal(tool.InputSchema, &schema); err != nil {
				b.Fatalf("Schema parse error: %v", err)
			}
		}
	}
}

// BenchmarkFullRequestCycle measures complete request-response cycle
func BenchmarkFullRequestCycle(b *testing.B) {
	requests := []string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}
`,
		`{"jsonrpc":"2.0","id":2,"method":"initialized"}
`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/list"}
`,
	}
	allInput := strings.Join(requests, "")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var output bytes.Buffer
		server := mcp.NewServer(strings.NewReader(allInput), &output)
		server.SetServerInfo("llm-support-mcp", "1.0.0")

		for _, tool := range supportserver.GetTools() {
			server.RegisterTool(tool, func(args map[string]interface{}) (string, error) {
				return "test", nil
			})
		}

		// Process all requests
		for j := 0; j < 3; j++ {
			if err := server.HandleOne(); err != nil {
				b.Fatalf("HandleOne() error = %v", err)
			}
		}
	}
}

// BenchmarkToolCall measures tools/call handler invocation
func BenchmarkToolCall(b *testing.B) {
	input := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"test_tool","arguments":{}}}
`
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var output bytes.Buffer
		server := mcp.NewServer(strings.NewReader(input), &output)
		server.SetServerInfo("test-server", "1.0.0")

		server.RegisterTool(mcp.Tool{
			Name:        "test_tool",
			Description: "Test tool for benchmarking",
			InputSchema: json.RawMessage(`{"type":"object","properties":{}}`),
		}, func(args map[string]interface{}) (string, error) {
			return "benchmark result", nil
		})

		if err := server.HandleOne(); err != nil {
			b.Fatalf("HandleOne() error = %v", err)
		}
	}
}

// BenchmarkClarificationServerStartup measures clarification server startup
func BenchmarkClarificationServerStartup(b *testing.B) {
	input := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}
`
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var output bytes.Buffer
		server := mcp.NewServer(strings.NewReader(input), &output)
		server.SetServerInfo("llm-clarification-mcp", "1.0.0")

		for _, tool := range clarifyserver.GetTools() {
			server.RegisterTool(tool, func(args map[string]interface{}) (string, error) {
				return "test", nil
			})
		}

		if err := server.HandleOne(); err != nil {
			b.Fatalf("HandleOne() error = %v", err)
		}
	}
}

// BenchmarkErrorHandling measures error response generation
func BenchmarkErrorHandling(b *testing.B) {
	input := `{"jsonrpc":"2.0","id":1,"method":"unknown/method"}
`
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var output bytes.Buffer
		server := mcp.NewServer(strings.NewReader(input), &output)
		server.SetServerInfo("test-server", "1.0.0")

		if err := server.HandleOne(); err != nil {
			b.Fatalf("HandleOne() error = %v", err)
		}
	}
}
