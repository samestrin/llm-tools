package mcp

import (
	"encoding/json"
	"testing"
)

func TestNewParseError(t *testing.T) {
	id := json.RawMessage(`1`)
	resp := NewParseError(id, "invalid json")

	if resp.JSONRPC != "2.0" {
		t.Errorf("JSONRPC = %s, want 2.0", resp.JSONRPC)
	}
	if resp.Error == nil {
		t.Fatal("Expected error in response")
	}
	if resp.Error.Code != ParseError {
		t.Errorf("Error.Code = %d, want %d", resp.Error.Code, ParseError)
	}
	if resp.Error.Message != "invalid json" {
		t.Errorf("Error.Message = %s, want 'invalid json'", resp.Error.Message)
	}
}

func TestNewInvalidRequestError(t *testing.T) {
	id := json.RawMessage(`"abc"`)
	resp := NewInvalidRequestError(id, "missing method")

	if resp.Error == nil {
		t.Fatal("Expected error in response")
	}
	if resp.Error.Code != InvalidRequest {
		t.Errorf("Error.Code = %d, want %d", resp.Error.Code, InvalidRequest)
	}
}

func TestNewMethodNotFoundError(t *testing.T) {
	id := json.RawMessage(`2`)
	resp := NewMethodNotFoundError(id, "unknown/method")

	if resp.Error == nil {
		t.Fatal("Expected error in response")
	}
	if resp.Error.Code != MethodNotFound {
		t.Errorf("Error.Code = %d, want %d", resp.Error.Code, MethodNotFound)
	}
	if resp.Error.Message != "Method not found: unknown/method" {
		t.Errorf("Error.Message = %s, want 'Method not found: unknown/method'", resp.Error.Message)
	}
}

func TestNewInvalidParamsError(t *testing.T) {
	id := json.RawMessage(`3`)
	resp := NewInvalidParamsError(id, "missing required field")

	if resp.Error == nil {
		t.Fatal("Expected error in response")
	}
	if resp.Error.Code != InvalidParams {
		t.Errorf("Error.Code = %d, want %d", resp.Error.Code, InvalidParams)
	}
}

func TestNewInternalError(t *testing.T) {
	id := json.RawMessage(`4`)
	resp := NewInternalError(id, "unexpected error")

	if resp.Error == nil {
		t.Fatal("Expected error in response")
	}
	if resp.Error.Code != InternalError {
		t.Errorf("Error.Code = %d, want %d", resp.Error.Code, InternalError)
	}
}

func TestNewErrorWithNullID(t *testing.T) {
	id := json.RawMessage(`null`)
	resp := NewParseError(id, "parse error")

	if string(resp.ID) != "null" {
		t.Errorf("ID = %s, want null", string(resp.ID))
	}
}

func TestRequestSerialization(t *testing.T) {
	req := Request{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Method:  "test/method",
		Params:  json.RawMessage(`{"key":"value"}`),
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var parsed Request
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if parsed.Method != req.Method {
		t.Errorf("Method = %s, want %s", parsed.Method, req.Method)
	}
}

func TestResponseSerialization(t *testing.T) {
	resp := Response{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Result:  json.RawMessage(`{"status":"ok"}`),
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var parsed Response
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if parsed.JSONRPC != "2.0" {
		t.Errorf("JSONRPC = %s, want 2.0", parsed.JSONRPC)
	}
}

func TestResponseWithError(t *testing.T) {
	resp := NewMethodNotFoundError(json.RawMessage(`1`), "test/unknown")

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var parsed Response
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if parsed.Error == nil {
		t.Fatal("Expected error in response")
	}
	if parsed.Error.Code != MethodNotFound {
		t.Errorf("Error.Code = %d, want %d", parsed.Error.Code, MethodNotFound)
	}
}

func TestErrorConstants(t *testing.T) {
	// Verify JSON-RPC 2.0 error codes
	if ParseError != -32700 {
		t.Errorf("ParseError = %d, want -32700", ParseError)
	}
	if InvalidRequest != -32600 {
		t.Errorf("InvalidRequest = %d, want -32600", InvalidRequest)
	}
	if MethodNotFound != -32601 {
		t.Errorf("MethodNotFound = %d, want -32601", MethodNotFound)
	}
	if InvalidParams != -32602 {
		t.Errorf("InvalidParams = %d, want -32602", InvalidParams)
	}
	if InternalError != -32603 {
		t.Errorf("InternalError = %d, want -32603", InternalError)
	}
}

func TestErrorSerialization(t *testing.T) {
	err := Error{
		Code:    MethodNotFound,
		Message: "Method not found",
		Data:    json.RawMessage(`"extra info"`),
	}

	data, jsonErr := json.Marshal(err)
	if jsonErr != nil {
		t.Fatalf("Marshal error: %v", jsonErr)
	}

	var parsed Error
	if jsonErr := json.Unmarshal(data, &parsed); jsonErr != nil {
		t.Fatalf("Unmarshal error: %v", jsonErr)
	}

	if parsed.Code != err.Code {
		t.Errorf("Code = %d, want %d", parsed.Code, err.Code)
	}
}

func TestRequestWithoutParams(t *testing.T) {
	req := Request{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Method:  "simple/method",
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var parsed Request
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if parsed.Params != nil && len(parsed.Params) > 0 {
		t.Error("Expected nil or empty Params")
	}
}

func TestResponseWithNilResult(t *testing.T) {
	resp := Response{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var parsed Response
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if parsed.Error != nil {
		t.Error("Expected nil Error")
	}
}
