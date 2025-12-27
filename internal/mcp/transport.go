package mcp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// Transport handles JSON-RPC 2.0 communication over stdio
// Supports both raw JSON streams and LSP-style header-prefixed messages
type Transport struct {
	reader  io.Reader
	writer  io.Writer
	bufRead *bufio.Reader
	encoder *json.Encoder
}

// NewTransport creates a new Transport with the given reader and writer
func NewTransport(reader io.Reader, writer io.Writer) *Transport {
	t := &Transport{
		reader: reader,
		writer: writer,
	}
	if reader != nil {
		t.bufRead = bufio.NewReader(reader)
	}
	if writer != nil {
		t.encoder = json.NewEncoder(writer)
	}
	return t
}

// ReadRequest reads and parses a JSON-RPC 2.0 request from the transport
// Supports two modes:
// 1. Raw JSON: Message starts with '{' - reads complete JSON object
// 2. LSP-style: Message starts with headers (Content-Length) - parses headers then JSON body
func (t *Transport) ReadRequest() (*Request, error) {
	if t.bufRead == nil {
		return nil, errors.New("no reader configured")
	}

	// Skip leading whitespace (spaces, tabs, newlines)
	for {
		b, err := t.bufRead.Peek(1)
		if err != nil {
			if err == io.EOF {
				return nil, io.EOF
			}
			return nil, fmt.Errorf("peek error: %w", err)
		}

		// Skip whitespace
		if b[0] == ' ' || b[0] == '\t' || b[0] == '\n' || b[0] == '\r' {
			t.bufRead.ReadByte() // consume it
			continue
		}
		break
	}

	// Now peek at first non-whitespace byte to determine mode
	firstByte, err := t.bufRead.Peek(1)
	if err != nil {
		if err == io.EOF {
			return nil, io.EOF
		}
		return nil, fmt.Errorf("peek error: %w", err)
	}

	var jsonData []byte

	if firstByte[0] == '{' {
		// Raw JSON mode - read complete JSON object by tracking braces
		jsonData, err = t.readJSONObject()
		if err != nil {
			return nil, err
		}
	} else {
		// LSP-style header mode - parse Content-Length header, then read body
		contentLength, err := t.parseHeaders()
		if err != nil {
			return nil, err
		}

		// Read exactly contentLength bytes
		jsonData = make([]byte, contentLength)
		if _, err := io.ReadFull(t.bufRead, jsonData); err != nil {
			return nil, &JSONRPCError{
				Code:    ParseError,
				Message: "Failed to read message body: " + err.Error(),
			}
		}
	}

	var req Request
	if err := json.Unmarshal(jsonData, &req); err != nil {
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

// readJSONObject reads a complete JSON object from the buffer by tracking braces
func (t *Transport) readJSONObject() ([]byte, error) {
	var buf bytes.Buffer
	depth := 0
	inString := false
	escaped := false

	for {
		b, err := t.bufRead.ReadByte()
		if err != nil {
			if err == io.EOF {
				if buf.Len() > 0 {
					return nil, &JSONRPCError{
						Code:    ParseError,
						Message: "Unexpected EOF in JSON object",
					}
				}
				return nil, io.EOF
			}
			return nil, fmt.Errorf("read error: %w", err)
		}

		buf.WriteByte(b)

		if escaped {
			escaped = false
			continue
		}

		if b == '\\' && inString {
			escaped = true
			continue
		}

		if b == '"' {
			inString = !inString
			continue
		}

		if inString {
			continue
		}

		if b == '{' {
			depth++
		} else if b == '}' {
			depth--
			if depth == 0 {
				// Complete JSON object
				return buf.Bytes(), nil
			}
		}
	}
}

// parseHeaders reads LSP-style headers until empty line
// Returns the Content-Length value
func (t *Transport) parseHeaders() (int, error) {
	var contentLength int
	foundContentLength := false

	for {
		line, err := t.bufRead.ReadString('\n')
		if err != nil {
			return 0, &JSONRPCError{
				Code:    ParseError,
				Message: "Failed to read header: " + err.Error(),
			}
		}

		// Trim \r\n or \n
		line = strings.TrimRight(line, "\r\n")

		// Empty line signals end of headers
		if line == "" {
			break
		}

		// Parse header
		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])

			if strings.EqualFold(key, "Content-Length") {
				length, err := strconv.Atoi(value)
				if err != nil {
					return 0, &JSONRPCError{
						Code:    ParseError,
						Message: "Invalid Content-Length: " + value,
					}
				}
				contentLength = length
				foundContentLength = true
			}
			// Ignore other headers (Content-Type, etc.)
		}
	}

	if !foundContentLength {
		return 0, &JSONRPCError{
			Code:    ParseError,
			Message: "Missing Content-Length header",
		}
	}

	return contentLength, nil
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
