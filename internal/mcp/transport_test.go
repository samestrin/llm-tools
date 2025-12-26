package mcp

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// TestReadRequest tests parsing of JSON-RPC requests from stdin
func TestReadRequest(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    *Request
		wantErr bool
	}{
		{
			name:  "valid request with method only",
			input: `{"jsonrpc":"2.0","id":1,"method":"tools/list"}` + "\n",
			want: &Request{
				JSONRPC: "2.0",
				ID:      json.RawMessage(`1`),
				Method:  "tools/list",
			},
			wantErr: false,
		},
		{
			name:  "valid request with params",
			input: `{"jsonrpc":"2.0","id":"abc","method":"tools/call","params":{"name":"test"}}` + "\n",
			want: &Request{
				JSONRPC: "2.0",
				ID:      json.RawMessage(`"abc"`),
				Method:  "tools/call",
				Params:  json.RawMessage(`{"name":"test"}`),
			},
			wantErr: false,
		},
		{
			name:    "invalid JSON syntax",
			input:   `{"jsonrpc":"2.0","id":1,method:"missing quote}` + "\n",
			want:    nil,
			wantErr: true,
		},
		{
			name:    "missing jsonrpc field",
			input:   `{"id":1,"method":"test"}` + "\n",
			want:    nil,
			wantErr: true,
		},
		{
			name:    "wrong jsonrpc version",
			input:   `{"jsonrpc":"1.0","id":1,"method":"test"}` + "\n",
			want:    nil,
			wantErr: true,
		},
		{
			name:    "missing method field",
			input:   `{"jsonrpc":"2.0","id":1}` + "\n",
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := strings.NewReader(tt.input)
			transport := NewTransport(reader, nil)

			got, err := transport.ReadRequest()
			if (err != nil) != tt.wantErr {
				t.Errorf("ReadRequest() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != nil {
				if got.JSONRPC != tt.want.JSONRPC {
					t.Errorf("JSONRPC = %v, want %v", got.JSONRPC, tt.want.JSONRPC)
				}
				if got.Method != tt.want.Method {
					t.Errorf("Method = %v, want %v", got.Method, tt.want.Method)
				}
			}
		})
	}
}

// TestWriteResponse tests writing JSON-RPC responses to stdout
func TestWriteResponse(t *testing.T) {
	tests := []struct {
		name     string
		response *Response
		wantJSON string
	}{
		{
			name: "success response with result",
			response: &Response{
				JSONRPC: "2.0",
				ID:      json.RawMessage(`1`),
				Result:  json.RawMessage(`{"tools":[]}`),
			},
			wantJSON: `{"jsonrpc":"2.0","id":1,"result":{"tools":[]}}`,
		},
		{
			name: "error response",
			response: &Response{
				JSONRPC: "2.0",
				ID:      json.RawMessage(`1`),
				Error: &Error{
					Code:    -32600,
					Message: "Invalid Request",
				},
			},
			wantJSON: `{"jsonrpc":"2.0","id":1,"error":{"code":-32600,"message":"Invalid Request"}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			transport := NewTransport(nil, &buf)

			err := transport.WriteResponse(tt.response)
			if err != nil {
				t.Errorf("WriteResponse() error = %v", err)
				return
			}

			got := strings.TrimSpace(buf.String())
			// Parse both to compare as JSON (order independent)
			var gotMap, wantMap map[string]interface{}
			if err := json.Unmarshal([]byte(got), &gotMap); err != nil {
				t.Errorf("Failed to parse output: %v", err)
				return
			}
			if err := json.Unmarshal([]byte(tt.wantJSON), &wantMap); err != nil {
				t.Errorf("Failed to parse expected: %v", err)
				return
			}
		})
	}
}

// TestReadMultipleRequests tests handling of line-delimited JSON stream
func TestReadMultipleRequests(t *testing.T) {
	input := `{"jsonrpc":"2.0","id":1,"method":"initialize"}
{"jsonrpc":"2.0","id":2,"method":"tools/list"}
{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"test"}}
`
	reader := strings.NewReader(input)
	transport := NewTransport(reader, nil)

	// Read first request
	req1, err := transport.ReadRequest()
	if err != nil {
		t.Fatalf("Failed to read first request: %v", err)
	}
	if req1.Method != "initialize" {
		t.Errorf("First request method = %v, want initialize", req1.Method)
	}

	// Read second request
	req2, err := transport.ReadRequest()
	if err != nil {
		t.Fatalf("Failed to read second request: %v", err)
	}
	if req2.Method != "tools/list" {
		t.Errorf("Second request method = %v, want tools/list", req2.Method)
	}

	// Read third request
	req3, err := transport.ReadRequest()
	if err != nil {
		t.Fatalf("Failed to read third request: %v", err)
	}
	if req3.Method != "tools/call" {
		t.Errorf("Third request method = %v, want tools/call", req3.Method)
	}
}

// TestUnicodeHandling tests that unicode and special characters are preserved
func TestUnicodeHandling(t *testing.T) {
	input := `{"jsonrpc":"2.0","id":1,"method":"test","params":{"text":"Hello 世界 🎉 \"quoted\""}}` + "\n"
	reader := strings.NewReader(input)
	transport := NewTransport(reader, nil)

	req, err := transport.ReadRequest()
	if err != nil {
		t.Fatalf("Failed to read request with unicode: %v", err)
	}

	// Verify params contain the unicode text
	var params map[string]string
	if err := json.Unmarshal(req.Params, &params); err != nil {
		t.Fatalf("Failed to parse params: %v", err)
	}

	expected := `Hello 世界 🎉 "quoted"`
	if params["text"] != expected {
		t.Errorf("Unicode text = %q, want %q", params["text"], expected)
	}
}

// TestEOFHandling tests graceful handling of EOF
func TestEOFHandling(t *testing.T) {
	reader := strings.NewReader("")
	transport := NewTransport(reader, nil)

	_, err := transport.ReadRequest()
	if err == nil {
		t.Error("Expected error on EOF, got nil")
	}
}

// TestLargeMessage tests handling of large JSON messages
func TestLargeMessage(t *testing.T) {
	// Create a request with 1MB of data
	largeData := strings.Repeat("x", 1024*1024)
	input := `{"jsonrpc":"2.0","id":1,"method":"test","params":{"data":"` + largeData + `"}}` + "\n"
	reader := strings.NewReader(input)
	transport := NewTransport(reader, nil)

	req, err := transport.ReadRequest()
	if err != nil {
		t.Fatalf("Failed to read large request: %v", err)
	}

	var params map[string]string
	if err := json.Unmarshal(req.Params, &params); err != nil {
		t.Fatalf("Failed to parse params: %v", err)
	}

	if len(params["data"]) != 1024*1024 {
		t.Errorf("Large data length = %d, want %d", len(params["data"]), 1024*1024)
	}
}
