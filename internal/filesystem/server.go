package filesystem

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	ServerName    = "llm-filesystem"
	ServerVersion = "1.2.0"
)

// Server wraps the MCP server with filesystem-specific configuration
type Server struct {
	mcpServer   *mcp.Server
	allowedDirs []string
	toolCount   int
}

// NewServer creates a new filesystem MCP server with the given allowed directories
func NewServer(allowedDirs []string) (*Server, error) {
	if allowedDirs == nil {
		allowedDirs = []string{}
	}

	mcpServer := mcp.NewServer(&mcp.Implementation{
		Name:    ServerName,
		Version: ServerVersion,
	}, &mcp.ServerOptions{
		Instructions: "LLM Filesystem MCP provides high-performance file operations for Claude Code workflows.",
	})

	s := &Server{
		mcpServer:   mcpServer,
		allowedDirs: allowedDirs,
		toolCount:   0,
	}

	// Register all tools
	s.registerTools()

	return s, nil
}

// registerTools registers all filesystem tools with the MCP server
func (s *Server) registerTools() {
	tools := GetToolDefinitions()

	for _, toolDef := range tools {
		td := toolDef // capture for closure
		s.mcpServer.AddTool(&mcp.Tool{
			Name:        td.Name,
			Description: td.Description,
			InputSchema: td.InputSchema,
		}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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

			output, err := s.ExecuteHandler(td.Name, args)
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
		s.toolCount++
	}
}

// ToolCount returns the number of registered tools
func (s *Server) ToolCount() int {
	return s.toolCount
}

// Name returns the server name
func (s *Server) Name() string {
	return ServerName
}

// Version returns the server version
func (s *Server) Version() string {
	return ServerVersion
}

// AllowedDirs returns the configured allowed directories
func (s *Server) AllowedDirs() []string {
	return s.allowedDirs
}

// Run starts the MCP server on stdio with graceful shutdown
func (s *Server) Run(ctx context.Context) error {
	// Setup signal handling for graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	go func() {
		<-sigCh
		fmt.Fprintf(os.Stderr, "\nShutting down %s...\n", ServerName)
		cancel()
	}()

	fmt.Fprintf(os.Stderr, "%s v%s started with %d tools\n", ServerName, ServerVersion, s.toolCount)
	return s.mcpServer.Run(ctx, &mcp.StdioTransport{})
}
