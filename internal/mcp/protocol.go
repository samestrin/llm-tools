package mcp

import "encoding/json"

// Request represents a JSON-RPC 2.0 request
type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// Response represents a JSON-RPC 2.0 response
type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *Error          `json:"error,omitempty"`
}

// Error represents a JSON-RPC 2.0 error object
type Error struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

// Standard JSON-RPC 2.0 error codes
const (
	ParseError     = -32700
	InvalidRequest = -32600
	MethodNotFound = -32601
	InvalidParams  = -32602
	InternalError  = -32603
)

// NewParseError creates a parse error response
func NewParseError(id json.RawMessage, message string) *Response {
	return &Response{
		JSONRPC: "2.0",
		ID:      id,
		Error: &Error{
			Code:    ParseError,
			Message: message,
		},
	}
}

// NewInvalidRequestError creates an invalid request error response
func NewInvalidRequestError(id json.RawMessage, message string) *Response {
	return &Response{
		JSONRPC: "2.0",
		ID:      id,
		Error: &Error{
			Code:    InvalidRequest,
			Message: message,
		},
	}
}

// NewMethodNotFoundError creates a method not found error response
func NewMethodNotFoundError(id json.RawMessage, method string) *Response {
	return &Response{
		JSONRPC: "2.0",
		ID:      id,
		Error: &Error{
			Code:    MethodNotFound,
			Message: "Method not found: " + method,
		},
	}
}

// NewInvalidParamsError creates an invalid params error response
func NewInvalidParamsError(id json.RawMessage, message string) *Response {
	return &Response{
		JSONRPC: "2.0",
		ID:      id,
		Error: &Error{
			Code:    InvalidParams,
			Message: message,
		},
	}
}

// NewInternalError creates an internal error response
func NewInternalError(id json.RawMessage, message string) *Response {
	return &Response{
		JSONRPC: "2.0",
		ID:      id,
		Error: &Error{
			Code:    InternalError,
			Message: message,
		},
	}
}
