package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/samestrin/llm-tools/internal/semantic/mcpserver"
)

const (
	serverName         = "llm-semantic-mcp"
	serverVersion      = "1.5.0"
	serverInstructions = "llm-semantic-mcp provides semantic code search using local embedding models. It wraps the llm-semantic CLI with 4 tools for searching, indexing, and managing semantic code indexes."
)

func main() {
	// Verify llm-semantic binary exists
	if _, err := os.Stat(mcpserver.BinaryPath); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "ERROR: llm-semantic binary not found at %s\n", mcpserver.BinaryPath)
		fmt.Fprintf(os.Stderr, "Please ensure llm-semantic is installed and accessible.\n")
		os.Exit(1)
	}

	// Create MCP server using official SDK
	server := mcp.NewServer(&mcp.Implementation{
		Name:    serverName,
		Version: serverVersion,
	}, &mcp.ServerOptions{
		Instructions: serverInstructions,
	})

	// Register all tools
	tools := mcpserver.GetToolDefinitions()
	for _, toolDef := range tools {
		// Capture for closure
		td := toolDef
		server.AddTool(&mcp.Tool{
			Name:        td.Name,
			Description: td.Description,
			InputSchema: td.InputSchema,
		}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			// Unmarshal arguments
			var args map[string]interface{}
			if req.Params.Arguments != nil {
				if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
					return &mcp.CallToolResult{
						Content: []mcp.Content{
							&mcp.TextContent{Text: "Error parsing arguments: " + err.Error()},
						},
						IsError: true,
					}, nil
				}
			}

			// Execute the tool using the handler
			output, err := mcpserver.ExecuteHandler(td.Name, args)
			if err != nil {
				return &mcp.CallToolResult{
					Content: []mcp.Content{
						&mcp.TextContent{Text: "Error: " + err.Error()},
					},
					IsError: true,
				}, nil
			}
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					&mcp.TextContent{Text: output},
				},
			}, nil
		})
	}

	// Log startup
	fmt.Fprintf(os.Stderr, "%s v%s started with %d tools\n", serverName, serverVersion, len(tools))

	// Run server on stdio
	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
		os.Exit(1)
	}
}
