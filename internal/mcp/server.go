package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

// ToolHandler is a function that handles a tool call
type ToolHandler func(args map[string]interface{}) (string, error)

// Tool represents an MCP tool definition
type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

// Server represents an MCP server
type Server struct {
	transport    *Transport
	tools        map[string]Tool
	handlers     map[string]ToolHandler
	serverInfo   ServerInfo
	instructions string
	mu           sync.RWMutex
}

// ServerInfo contains server identification
type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// ToolsCapability represents the tools capability configuration
type ToolsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

// Capabilities represents server capabilities
type Capabilities struct {
	Tools *ToolsCapability `json:"tools,omitempty"`
}

// InitializeParams represents the params for initialize request
type InitializeParams struct {
	ProtocolVersion string       `json:"protocolVersion"`
	Capabilities    Capabilities `json:"capabilities"`
	ClientInfo      struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	} `json:"clientInfo"`
}

// InitializeResult represents the result of initialize
type InitializeResult struct {
	ProtocolVersion string       `json:"protocolVersion"`
	Capabilities    Capabilities `json:"capabilities"`
	ServerInfo      ServerInfo   `json:"serverInfo"`
	Instructions    string       `json:"instructions,omitempty"`
}

// ToolsListResult represents the result of tools/list
type ToolsListResult struct {
	Tools []Tool `json:"tools"`
}

// ToolsCallParams represents the params for tools/call
type ToolsCallParams struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments,omitempty"`
}

// TextContent represents text content in a response
type TextContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// ToolsCallResult represents the result of tools/call
type ToolsCallResult struct {
	Content []TextContent `json:"content"`
	IsError bool          `json:"isError,omitempty"`
}

// ToolError represents an error from a tool execution
type ToolError struct {
	Message string
}

func (e *ToolError) Error() string {
	return e.Message
}

// NewToolError creates a new tool error
func NewToolError(message string) error {
	return &ToolError{Message: message}
}

// NewServer creates a new MCP server
func NewServer(reader io.Reader, writer io.Writer) *Server {
	return &Server{
		transport: NewTransport(reader, writer),
		tools:     make(map[string]Tool),
		handlers:  make(map[string]ToolHandler),
	}
}

// SetServerInfo sets the server identification info
func (s *Server) SetServerInfo(name, version string) {
	s.serverInfo = ServerInfo{Name: name, Version: version}
}

// SetInstructions sets the server instructions (required by some clients like Gemini)
func (s *Server) SetInstructions(instructions string) {
	s.instructions = instructions
}

// RegisterTool registers a tool with its handler
func (s *Server) RegisterTool(tool Tool, handler ToolHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tools[tool.Name] = tool
	s.handlers[tool.Name] = handler
}

// HandleOne reads and handles a single request
func (s *Server) HandleOne() error {
	req, err := s.transport.ReadRequest()
	if err != nil {
		if rpcErr, ok := IsJSONRPCError(err); ok {
			// Send JSON-RPC error response
			return s.transport.WriteError(nil, rpcErr.Code, rpcErr.Message)
		}
		return err
	}

	return s.handleRequest(req)
}

// handleRequest routes a request to the appropriate handler
func (s *Server) handleRequest(req *Request) error {
	switch req.Method {
	case "initialize":
		return s.handleInitialize(req)
	case "initialized":
		// Notification, no response
		return nil
	case "tools/list":
		return s.handleToolsList(req)
	case "tools/call":
		return s.handleToolsCall(req)
	default:
		return s.sendError(req.ID, MethodNotFound, "Method not found: "+req.Method)
	}
}

// handleInitialize handles the initialize request
func (s *Server) handleInitialize(req *Request) error {
	var params InitializeParams
	if req.Params != nil {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return s.sendError(req.ID, InvalidParams, "Invalid params: "+err.Error())
		}
	}

	result := InitializeResult{
		ProtocolVersion: "2024-11-05",
		Capabilities:    Capabilities{Tools: &ToolsCapability{}},
		ServerInfo:      s.serverInfo,
		Instructions:    s.instructions,
	}

	return s.sendResult(req.ID, result)
}

// handleToolsList handles the tools/list request
func (s *Server) handleToolsList(req *Request) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tools := make([]Tool, 0, len(s.tools))
	for _, tool := range s.tools {
		tools = append(tools, tool)
	}

	result := ToolsListResult{Tools: tools}
	return s.sendResult(req.ID, result)
}

// handleToolsCall handles the tools/call request
func (s *Server) handleToolsCall(req *Request) error {
	var params ToolsCallParams
	if req.Params != nil {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return s.sendError(req.ID, InvalidParams, "Invalid params: "+err.Error())
		}
	}

	s.mu.RLock()
	handler, ok := s.handlers[params.Name]
	s.mu.RUnlock()

	if !ok {
		// Tool not found - return as content error, not JSON-RPC error
		return s.sendToolResult(req.ID, fmt.Sprintf("Tool not found: %s", params.Name), true)
	}

	// Execute the tool
	output, err := handler(params.Arguments)
	if err != nil {
		// Tool execution error - return as content error
		return s.sendToolResult(req.ID, "Error: "+err.Error(), true)
	}

	return s.sendToolResult(req.ID, output, false)
}

// sendResult sends a success response
func (s *Server) sendResult(id json.RawMessage, result interface{}) error {
	resultJSON, err := json.Marshal(result)
	if err != nil {
		return s.sendError(id, InternalError, "Failed to marshal result: "+err.Error())
	}

	resp := &Response{
		JSONRPC: "2.0",
		ID:      id,
		Result:  resultJSON,
	}
	return s.transport.WriteResponse(resp)
}

// sendToolResult sends a tool call result
func (s *Server) sendToolResult(id json.RawMessage, text string, isError bool) error {
	result := ToolsCallResult{
		Content: []TextContent{{Type: "text", Text: text}},
		IsError: isError,
	}
	return s.sendResult(id, result)
}

// sendError sends an error response
func (s *Server) sendError(id json.RawMessage, code int, message string) error {
	resp := &Response{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &Error{Code: code, Message: message},
	}
	return s.transport.WriteResponse(resp)
}

// Serve runs the server loop until EOF or context cancellation
func (s *Server) Serve(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			err := s.HandleOne()
			if err == io.EOF {
				return nil
			}
			if err != nil {
				// Log error but continue serving
				fmt.Fprintf(os.Stderr, "MCP server error: %v\n", err)
			}
		}
	}
}

// ServeWithSignalHandler runs the server with graceful shutdown on SIGINT/SIGTERM
func (s *Server) ServeWithSignalHandler() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		cancel()
	}()

	return s.Serve(ctx)
}
