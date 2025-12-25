package output

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestNewFormatter(t *testing.T) {
	tests := []struct {
		name     string
		format   string
		expected Format
	}{
		{"text format", "text", FormatText},
		{"json format", "json", FormatJSON},
		{"invalid defaults to text", "invalid", FormatText},
		{"empty defaults to text", "", FormatText},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := NewFormatter(tt.format)
			if f.format != tt.expected {
				t.Errorf("NewFormatter(%q).format = %v, want %v", tt.format, f.format, tt.expected)
			}
		})
	}
}

func TestPrintKeyValue(t *testing.T) {
	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	f := NewFormatter("text")
	f.PrintKeyValue("status", "ok")

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if !strings.Contains(output, "STATUS: ok") {
		t.Errorf("PrintKeyValue output = %q, want to contain 'STATUS: ok'", output)
	}
}

func TestPrintKeyValueJSON(t *testing.T) {
	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	f := NewFormatter("json")
	f.PrintKeyValue("status", "ok")

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if !strings.Contains(output, `"status"`) || !strings.Contains(output, `"ok"`) {
		t.Errorf("PrintKeyValue JSON output = %q, should contain JSON", output)
	}
}

func TestPrintSuccess(t *testing.T) {
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	f := NewFormatter("text")
	f.PrintSuccess("completed")

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if !strings.Contains(output, "✓") || !strings.Contains(output, "completed") {
		t.Errorf("PrintSuccess output = %q, want checkmark and message", output)
	}
}

func TestPrintFailure(t *testing.T) {
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	f := NewFormatter("text")
	f.PrintFailure("failed")

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if !strings.Contains(output, "✗") || !strings.Contains(output, "failed") {
		t.Errorf("PrintFailure output = %q, want cross and message", output)
	}
}
