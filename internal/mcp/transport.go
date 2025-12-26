package mcp

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// Transport handles JSON-RPC 2.0 communication over stdio
type Transport struct {
	reader  io.Reader
	writer  io.Writer
	scanner *bufio.Scanner
	encoder *json.Encoder
}

// NewTransport creates a new Transport with the given reader and writer
func NewTransport(reader io.Reader, writer io.Writer) *Transport {
	t := &Transport{
		reader: reader,
		writer: writer,
	}
	if reader != nil {
		t.scanner = bufio.NewScanner(reader)
		// Set a large buffer to handle large messages (up to 10MB)
		const maxScanTokenSize = 10 * 1024 * 1024
		buf := make([]byte, maxScanTokenSize)
		t.scanner.Buffer(buf, maxScanTokenSize)
	}
	if writer != nil {
		t.encoder = json.NewEncoder(writer)
	}
	return t
}

// ReadRequest reads and parses a JSON-RPC 2.0 request from the transport
func (t *Transport) ReadRequest() (*Request, error) {
	if t.scanner == nil {
		return nil, errors.New("no reader configured")
	}

	// Read next line
	if !t.scanner.Scan() {
		if err := t.scanner.Err(); err != nil {
			return nil, fmt.Errorf("read error: %w", err)
		}
		return nil, io.EOF
	}

	line := t.scanner.Bytes()
	if len(line) == 0 {
		return nil, io.EOF
	}

	// Parse JSON
	var req Request
	if err := json.Unmarshal(line, &req); err != nil {
		return nil, &JSONRPCError{
			Code:    ParseError,
			Message: "Parse error: " + err.Error(),
		}
	}

	// Validate JSON-RPC 2.0 requirements
	if err := validateRequest(&req); err != nil {
		return nil, err
	}

	return &req, nil
}

// validateRequest validates that a request conforms to JSON-RPC 2.0
func validateRequest(req *Request) error {
	if req.JSONRPC == "" {
		return &JSONRPCError{
			Code:    InvalidRequest,
			Message: "Invalid Request: missing jsonrpc field",
		}
	}
	if req.JSONRPC != "2.0" {
		return &JSONRPCError{
			Code:    InvalidRequest,
			Message: "Invalid Request: jsonrpc must be '2.0'",
		}
	}
	if req.Method == "" {
		return &JSONRPCError{
			Code:    InvalidRequest,
			Message: "Invalid Request: missing method field",
		}
	}
	return nil
}

// WriteResponse writes a JSON-RPC 2.0 response to the transport
func (t *Transport) WriteResponse(resp *Response) error {
	if t.encoder == nil {
		return errors.New("no writer configured")
	}
	return t.encoder.Encode(resp)
}

// WriteError writes a JSON-RPC 2.0 error response
func (t *Transport) WriteError(id json.RawMessage, code int, message string) error {
	resp := &Response{
		JSONRPC: "2.0",
		ID:      id,
		Error: &Error{
			Code:    code,
			Message: message,
		},
	}
	return t.WriteResponse(resp)
}

// JSONRPCError represents a JSON-RPC protocol error
type JSONRPCError struct {
	Code    int
	Message string
}

func (e *JSONRPCError) Error() string {
	return fmt.Sprintf("JSON-RPC error %d: %s", e.Code, e.Message)
}

// IsJSONRPCError checks if an error is a JSONRPCError
func IsJSONRPCError(err error) (*JSONRPCError, bool) {
	var rpcErr *JSONRPCError
	if errors.As(err, &rpcErr) {
		return rpcErr, true
	}
	return nil, false
}
