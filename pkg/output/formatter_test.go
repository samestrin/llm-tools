package output

import (
	"bytes"
	"encoding/json"
	"io"
	"path/filepath"
	"strings"
	"testing"
)

// TestOutput is a sample struct for testing
type TestOutput struct {
	Name    string `json:"name"`
	Count   int    `json:"count,omitempty"`
	File    string `json:"file,omitempty"`
	Line    int    `json:"line,omitempty"`
	Message string `json:"message,omitempty"`
	Empty   string `json:"empty,omitempty"`
}

func TestNew(t *testing.T) {
	var buf bytes.Buffer
	f := New(true, false, &buf)

	if f == nil {
		t.Fatal("New returned nil")
	}
	if !f.JSON {
		t.Error("JSON should be true")
	}
	if f.Minimal {
		t.Error("Minimal should be false")
	}
}

func TestFormatter_Print_DefaultText(t *testing.T) {
	var buf bytes.Buffer
	f := New(false, false, &buf)

	data := TestOutput{Name: "test", Count: 5}
	textFunc := func(w io.Writer, d interface{}) {
		w.Write([]byte("TEXT OUTPUT"))
	}

	err := f.Print(data, textFunc)
	if err != nil {
		t.Fatalf("Print failed: %v", err)
	}

	if !strings.Contains(buf.String(), "TEXT OUTPUT") {
		t.Errorf("Expected text output, got: %s", buf.String())
	}
}

func TestFormatter_Print_JSON(t *testing.T) {
	var buf bytes.Buffer
	f := New(true, false, &buf)

	data := TestOutput{Name: "test", Count: 5}

	err := f.Print(data, nil)
	if err != nil {
		t.Fatalf("Print failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "\"name\": \"test\"") {
		t.Errorf("Expected JSON output with name, got: %s", output)
	}
	if !strings.Contains(output, "\"count\": 5") {
		t.Errorf("Expected JSON output with count, got: %s", output)
	}
	// Should be pretty-printed (contains newlines)
	if !strings.Contains(output, "\n  ") {
		t.Errorf("Expected pretty-printed JSON, got: %s", output)
	}
}

func TestFormatter_Print_MinimalJSON(t *testing.T) {
	var buf bytes.Buffer
	f := New(true, true, &buf)

	data := TestOutput{
		Name:    "test",
		Count:   5,
		File:    "/path/to/file.go",
		Line:    42,
		Message: "hello",
		Empty:   "", // Should be omitted
	}

	err := f.Print(data, nil)
	if err != nil {
		t.Fatalf("Print failed: %v", err)
	}

	output := strings.TrimSpace(buf.String())

	// Should be single line (no indentation)
	if strings.Contains(output, "\n  ") {
		t.Errorf("Expected single-line JSON, got: %s", output)
	}

	// Should use abbreviated keys
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(output), &parsed); err != nil {
		t.Fatalf("Invalid JSON: %v", err)
	}

	// Check abbreviated keys exist
	if _, ok := parsed["n"]; !ok {
		t.Errorf("Expected abbreviated key 'n' for name, got keys: %v", parsed)
	}
	if _, ok := parsed["f"]; !ok {
		t.Errorf("Expected abbreviated key 'f' for file, got keys: %v", parsed)
	}
	if _, ok := parsed["l"]; !ok {
		t.Errorf("Expected abbreviated key 'l' for line, got keys: %v", parsed)
	}
	if _, ok := parsed["msg"]; !ok {
		t.Errorf("Expected abbreviated key 'msg' for message, got keys: %v", parsed)
	}

	// Empty field should not be present
	if _, ok := parsed["empty"]; ok {
		t.Error("Empty field should be omitted in minimal mode")
	}
}

func TestFormatter_Print_MinimalJSON_OmitsZeroValues(t *testing.T) {
	var buf bytes.Buffer
	f := New(true, true, &buf)

	data := TestOutput{
		Name:  "test",
		Count: 0, // Zero value - should be omitted
		Line:  0, // Zero value - should be omitted
	}

	err := f.Print(data, nil)
	if err != nil {
		t.Fatalf("Print failed: %v", err)
	}

	output := strings.TrimSpace(buf.String())

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(output), &parsed); err != nil {
		t.Fatalf("Invalid JSON: %v", err)
	}

	// Zero values should not be present
	if _, ok := parsed["c"]; ok {
		t.Error("Zero count should be omitted in minimal mode")
	}
	if _, ok := parsed["l"]; ok {
		t.Error("Zero line should be omitted in minimal mode")
	}
}

func TestFormatter_Print_MinimalText(t *testing.T) {
	var buf bytes.Buffer
	f := New(false, true, &buf)

	data := TestOutput{Name: "test", Count: 5}
	called := false
	textFunc := func(w io.Writer, d interface{}) {
		called = true
	}

	err := f.Print(data, textFunc)
	if err != nil {
		t.Fatalf("Print failed: %v", err)
	}

	if !called {
		t.Error("Text function should be called for minimal text mode")
	}
}

func TestRelativePath(t *testing.T) {
	tests := []struct {
		path     string
		base     string
		expected string
	}{
		{"/home/user/project/src/file.go", "/home/user/project", "src/file.go"},
		{"/home/user/project/file.go", "/home/user/project", "file.go"},
		{"", "/home/user", ""},
		{"/home/user/file.go", "", "/home/user/file.go"},
	}

	for _, tt := range tests {
		result := RelativePath(tt.path, tt.base)
		// Normalize for cross-platform comparison
		expected := filepath.FromSlash(tt.expected)
		if result != expected && result != tt.expected {
			t.Errorf("RelativePath(%q, %q) = %q, want %q", tt.path, tt.base, result, tt.expected)
		}
	}
}

func TestFilterEmpty(t *testing.T) {
	input := map[string]interface{}{
		"name":   "test",
		"empty":  "",
		"zero":   0,
		"filled": 42,
		"nil":    nil,
	}

	result := FilterEmpty(input)

	if _, ok := result["name"]; !ok {
		t.Error("name should be present")
	}
	if _, ok := result["filled"]; !ok {
		t.Error("filled should be present")
	}
	if _, ok := result["empty"]; ok {
		t.Error("empty should be filtered")
	}
	if _, ok := result["zero"]; ok {
		t.Error("zero should be filtered")
	}
	if _, ok := result["nil"]; ok {
		t.Error("nil should be filtered")
	}
}

func TestFormatter_PrintLine(t *testing.T) {
	var buf bytes.Buffer
	f := New(false, false, &buf)

	f.PrintLine("key", "value")
	if !strings.Contains(buf.String(), "key: value") {
		t.Errorf("Expected 'key: value', got: %s", buf.String())
	}
}

func TestFormatter_PrintLine_MinimalOmitsEmpty(t *testing.T) {
	var buf bytes.Buffer
	f := New(false, true, &buf)

	f.PrintLine("key", "")
	if buf.Len() > 0 {
		t.Errorf("Expected empty output for empty value in minimal mode, got: %s", buf.String())
	}

	f.PrintLine("key", 0)
	if buf.Len() > 0 {
		t.Errorf("Expected empty output for zero value in minimal mode, got: %s", buf.String())
	}
}

func TestKeyAbbreviations(t *testing.T) {
	// Verify expected abbreviations exist
	expectedAbbrevs := map[string]string{
		"file":     "f",
		"line":     "l",
		"name":     "n",
		"type":     "t",
		"question": "q",
		"answer":   "a",
		"message":  "msg",
	}

	for full, abbrev := range expectedAbbrevs {
		if got, ok := KeyAbbreviations[full]; !ok || got != abbrev {
			t.Errorf("KeyAbbreviations[%q] = %q, want %q", full, got, abbrev)
		}
	}
}

// TestNestedStruct tests processing of nested structs
func TestFormatter_Print_NestedStruct(t *testing.T) {
	type Inner struct {
		Name  string `json:"name"`
		Value int    `json:"value,omitempty"`
	}
	type Outer struct {
		Title string `json:"title"`
		Inner Inner  `json:"inner"`
	}

	var buf bytes.Buffer
	f := New(true, true, &buf)

	data := Outer{
		Title: "test",
		Inner: Inner{Name: "nested", Value: 0},
	}

	err := f.Print(data, nil)
	if err != nil {
		t.Fatalf("Print failed: %v", err)
	}

	output := strings.TrimSpace(buf.String())
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(output), &parsed); err != nil {
		t.Fatalf("Invalid JSON: %v", err)
	}

	// Check nested structure
	inner, ok := parsed["inner"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected inner to be a map")
	}
	if _, ok := inner["n"]; !ok {
		t.Error("Expected abbreviated key 'n' in nested struct")
	}
}

// TestSliceOutput tests processing of slices
func TestFormatter_Print_Slice(t *testing.T) {
	type Item struct {
		Name string `json:"name"`
	}

	var buf bytes.Buffer
	f := New(true, true, &buf)

	data := []Item{
		{Name: "first"},
		{Name: "second"},
	}

	err := f.Print(data, nil)
	if err != nil {
		t.Fatalf("Print failed: %v", err)
	}

	output := strings.TrimSpace(buf.String())
	var parsed []map[string]interface{}
	if err := json.Unmarshal([]byte(output), &parsed); err != nil {
		t.Fatalf("Invalid JSON: %v", err)
	}

	if len(parsed) != 2 {
		t.Errorf("Expected 2 items, got %d", len(parsed))
	}
	if _, ok := parsed[0]["n"]; !ok {
		t.Error("Expected abbreviated key 'n' in slice items")
	}
}
