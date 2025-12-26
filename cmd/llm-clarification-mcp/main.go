package main

import (
	"fmt"
	"os"

	"github.com/samestrin/llm-tools/internal/clarification/mcpserver"
	"github.com/samestrin/llm-tools/internal/mcp"
)

const (
	serverName    = "llm-clarification-mcp"
	serverVersion = "1.0.0"
)

func main() {
	// Verify llm-clarification binary exists
	if _, err := os.Stat(mcpserver.BinaryPath); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "ERROR: llm-clarification binary not found at %s\n", mcpserver.BinaryPath)
		fmt.Fprintf(os.Stderr, "Please ensure llm-clarification is installed and accessible.\n")
		os.Exit(1)
	}

	// Create MCP server
	server := mcp.NewServer(os.Stdin, os.Stdout)
	server.SetServerInfo(serverName, serverVersion)

	// Register all tools
	tools := mcpserver.GetTools()
	for _, tool := range tools {
		// Capture tool name for closure
		toolName := tool.Name
		server.RegisterTool(tool, func(args map[string]interface{}) (string, error) {
			return mcpserver.ExecuteHandler(toolName, args)
		})
	}

	// Log startup
	fmt.Fprintf(os.Stderr, "%s v%s started with %d tools\n", serverName, serverVersion, len(tools))

	// Run server
	if err := server.ServeWithSignalHandler(); err != nil {
		fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
		os.Exit(1)
	}
}
