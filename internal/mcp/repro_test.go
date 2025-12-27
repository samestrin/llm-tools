package mcp

import (
	"io"
	"testing"
	"time"
)

func TestReadRequest_Blocking(t *testing.T) {
	// Create a pipe to simulate Stdin
	pr, pw := io.Pipe()

	transport := NewTransport(pr, nil)

	// Channel to signal completion
	done := make(chan struct{})
	var err error
	var req *Request

	go func() {
		req, err = transport.ReadRequest()
		close(done)
	}()

	// Write a complete JSON object WITHOUT newline
	// This simulates Gemini sending a request and waiting.
	go func() {
		// Small delay to ensure reader is ready
		time.Sleep(50 * time.Millisecond)
		input := `{"jsonrpc":"2.0","method":"initialize","id":1}`
		pw.Write([]byte(input))
		// Close to signal EOF so json parser can complete
		pw.Close()
	}()

	select {
	case <-done:
		if err != nil {
			t.Fatalf("ReadRequest failed: %v", err)
		}
		if req.Method != "initialize" {
			t.Errorf("Expected method initialize, got %s", req.Method)
		}
		t.Log("ReadRequest returned successfully without newline")
	case <-time.After(1 * time.Second):
		t.Fatal("ReadRequest timed out (blocked) waiting for newline or EOF")
	}
}

func TestReadRequest_WithNewline(t *testing.T) {
	pr, pw := io.Pipe()
	transport := NewTransport(pr, nil)
	done := make(chan struct{})
	var err error
	var req *Request

	go func() {
		req, err = transport.ReadRequest()
		close(done)
	}()

	go func() {
		time.Sleep(50 * time.Millisecond)
		// Write with newline
		input := `{"jsonrpc":"2.0","method":"initialize","id":1}` + "\n"
		pw.Write([]byte(input))
	}()

	select {
	case <-done:
		if err != nil {
			t.Fatalf("ReadRequest failed: %v", err)
		}
		if req.Method != "initialize" {
			t.Errorf("Expected method initialize, got %s", req.Method)
		}
		t.Log("ReadRequest returned successfully with newline")
	case <-time.After(1 * time.Second):
		t.Fatal("ReadRequest timed out")
	}
}

func TestReadRequest_WithHeaders(t *testing.T) {
	pr, pw := io.Pipe()
	transport := NewTransport(pr, nil)
	done := make(chan struct{})
	var err error
	var req *Request

	go func() {
		req, err = transport.ReadRequest()
		close(done)
	}()

	go func() {
		time.Sleep(50 * time.Millisecond)
		// Write with LSP-style headers
		// JSON is 46 bytes: {"jsonrpc":"2.0","method":"initialize","id":1}
		jsonBody := `{"jsonrpc":"2.0","method":"initialize","id":1}`
		input := "Content-Length: 46\r\n\r\n" + jsonBody
		pw.Write([]byte(input))
	}()

	select {
	case <-done:
		if err != nil {
			t.Fatalf("ReadRequest failed with headers: %v", err)
		}
		if req.Method != "initialize" {
			t.Errorf("Expected method initialize, got %s", req.Method)
		}
		t.Log("ReadRequest returned successfully with LSP headers")
	case <-time.After(1 * time.Second):
		t.Fatal("ReadRequest timed out (blocked) on headers")
	}
}

func TestReadRequest_MultipleWithHeaders(t *testing.T) {
	pr, pw := io.Pipe()
	transport := NewTransport(pr, nil)
	done := make(chan struct{})

	go func() {
		defer close(done)

		// First request with headers
		req1, err := transport.ReadRequest()
		if err != nil {
			t.Errorf("First read failed: %v", err)
			return
		}
		if req1.Method != "initialize" {
			t.Errorf("Expected method initialize, got %s", req1.Method)
		}
		t.Log("First read succeeded (with headers)")

		// Second request with headers
		req2, err := transport.ReadRequest()
		if err != nil {
			t.Errorf("Second read failed: %v", err)
			return
		}
		if req2.Method != "initialized" {
			t.Errorf("Expected method initialized, got %s", req2.Method)
		}
		t.Log("Second read succeeded (with headers)")
	}()

	go func() {
		time.Sleep(50 * time.Millisecond)

		// First message: 46 bytes
		json1 := `{"jsonrpc":"2.0","method":"initialize","id":1}`
		msg1 := "Content-Length: 46\r\n\r\n" + json1
		pw.Write([]byte(msg1))

		time.Sleep(50 * time.Millisecond)

		// Second message (notification, no id): 40 bytes
		json2 := `{"jsonrpc":"2.0","method":"initialized"}`
		msg2 := "Content-Length: 40\r\n\r\n" + json2
		pw.Write([]byte(msg2))
	}()

	select {
	case <-done:
		// success
	case <-time.After(2 * time.Second):
		t.Fatal("ReadRequest timed out in multiple messages test")
	}
}

func TestReadRequest_MixedModes(t *testing.T) {
	pr, pw := io.Pipe()
	transport := NewTransport(pr, nil)
	done := make(chan struct{})

	go func() {
		defer close(done)

		// First request: raw JSON
		req1, err := transport.ReadRequest()
		if err != nil {
			t.Errorf("First read (raw JSON) failed: %v", err)
			return
		}
		if req1.Method != "initialize" {
			t.Errorf("Expected method initialize, got %s", req1.Method)
		}
		t.Log("First read succeeded (raw JSON)")

		// Second request: with headers
		req2, err := transport.ReadRequest()
		if err != nil {
			t.Errorf("Second read (with headers) failed: %v", err)
			return
		}
		if req2.Method != "tools/list" {
			t.Errorf("Expected method tools/list, got %s", req2.Method)
		}
		t.Log("Second read succeeded (with headers)")
	}()

	go func() {
		time.Sleep(50 * time.Millisecond)

		// First message: raw JSON with newline (46 bytes + newline)
		json1 := `{"jsonrpc":"2.0","method":"initialize","id":1}` + "\n"
		pw.Write([]byte(json1))

		time.Sleep(50 * time.Millisecond)

		// Second message: with headers (46 bytes)
		json2 := `{"jsonrpc":"2.0","method":"tools/list","id":2}`
		msg2 := "Content-Length: 46\r\n\r\n" + json2
		pw.Write([]byte(msg2))
	}()

	select {
	case <-done:
		// success
	case <-time.After(2 * time.Second):
		t.Fatal("ReadRequest timed out in mixed modes test")
	}
}
