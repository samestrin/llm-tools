package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/samestrin/llm-tools/internal/filesystem/mcpserver"
)

const (
	serverName         = "llm-filesystem-mcp"
	serverVersion      = "1.7.0"
	serverInstructions = "llm-filesystem-mcp provides high-performance filesystem operations for Claude Code. It wraps the llm-filesystem CLI with 27 commands for reading, writing, editing, searching, and managing files."
)

func main() {
	// Parse allowed directories from command line
	for i, arg := range os.Args[1:] {
		if arg == "--allowed-dirs" && i+1 < len(os.Args)-1 {
			dirs := strings.Split(os.Args[i+2], ",")
			mcpserver.AllowedDirs = append(mcpserver.AllowedDirs, dirs...)
		}
	}

	// Verify llm-filesystem binary exists
	if _, err := os.Stat(mcpserver.BinaryPath); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "ERROR: llm-filesystem binary not found at %s\n", mcpserver.BinaryPath)
		fmt.Fprintf(os.Stderr, "Please ensure llm-filesystem is installed and accessible.\n")
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
