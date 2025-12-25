package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestJSONParseCommand(t *testing.T) {
	tmpDir := t.TempDir()
	validJSON := filepath.Join(tmpDir, "valid.json")
	invalidJSON := filepath.Join(tmpDir, "invalid.json")

	os.WriteFile(validJSON, []byte(`{"name":"test","value":42}`), 0644)
	os.WriteFile(invalidJSON, []byte(`{invalid json}`), 0644)

	tests := []struct {
		name     string
		args     []string
		expected []string
		hasError bool
	}{
		{
			name:     "parse valid json",
			args:     []string{"parse", validJSON},
			expected: []string{"name", "test", "value", "42"},
		},
		{
			name:     "parse with indent",
			args:     []string{"parse", validJSON, "--indent", "4"},
			expected: []string{"name"},
		},
		{
			name:     "parse compact",
			args:     []string{"parse", validJSON, "--compact"},
			expected: []string{`{"name":"test","value":42}`},
		},
		{
			name:     "parse invalid json",
			args:     []string{"parse", invalidJSON},
			hasError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := newJSONCmd()
			buf := new(bytes.Buffer)
			errBuf := new(bytes.Buffer)
			cmd.SetOut(buf)
			cmd.SetErr(errBuf)
			cmd.SetArgs(tt.args)

			err := cmd.Execute()

			if tt.hasError {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			output := buf.String()
			for _, exp := range tt.expected {
				if !strings.Contains(output, exp) {
					t.Errorf("output %q should contain %q", output, exp)
				}
			}
		})
	}
}

func TestJSONQueryCommand(t *testing.T) {
	tmpDir := t.TempDir()
	jsonFile := filepath.Join(tmpDir, "data.json")
	os.WriteFile(jsonFile, []byte(`{"users":[{"name":"alice"},{"name":"bob"}],"count":2}`), 0644)

	tests := []struct {
		name     string
		args     []string
		expected []string
		hasError bool
	}{
		{
			name:     "query simple key",
			args:     []string{"query", jsonFile, "count"},
			expected: []string{"RESULT:", "2"},
		},
		{
			name:     "query nested object",
			args:     []string{"query", jsonFile, "users"},
			expected: []string{"alice", "bob"},
		},
		{
			name:     "query array element",
			args:     []string{"query", jsonFile, "users.0.name"},
			expected: []string{"alice"},
		},
		{
			name:     "query invalid path",
			args:     []string{"query", jsonFile, "nonexistent"},
			hasError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := newJSONCmd()
			buf := new(bytes.Buffer)
			errBuf := new(bytes.Buffer)
			cmd.SetOut(buf)
			cmd.SetErr(errBuf)
			cmd.SetArgs(tt.args)

			err := cmd.Execute()

			if tt.hasError {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			output := buf.String()
			for _, exp := range tt.expected {
				if !strings.Contains(output, exp) {
					t.Errorf("output %q should contain %q", output, exp)
				}
			}
		})
	}
}

func TestJSONValidateCommand(t *testing.T) {
	tmpDir := t.TempDir()
	validJSON := filepath.Join(tmpDir, "valid.json")
	invalidJSON := filepath.Join(tmpDir, "invalid.json")

	os.WriteFile(validJSON, []byte(`{"valid":true}`), 0644)
	os.WriteFile(invalidJSON, []byte(`{not valid}`), 0644)

	tests := []struct {
		name     string
		args     []string
		expected []string
		hasError bool
	}{
		{
			name:     "validate valid json",
			args:     []string{"validate", validJSON},
			expected: []string{"✓", "VALID"},
		},
		{
			name:     "validate invalid json",
			args:     []string{"validate", invalidJSON},
			expected: []string{"✗", "INVALID"},
			hasError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := newJSONCmd()
			buf := new(bytes.Buffer)
			errBuf := new(bytes.Buffer)
			cmd.SetOut(buf)
			cmd.SetErr(errBuf)
			cmd.SetArgs(tt.args)

			err := cmd.Execute()

			if tt.hasError {
				// Check error output
				output := buf.String() + errBuf.String()
				for _, exp := range tt.expected {
					if !strings.Contains(output, exp) {
						t.Errorf("output %q should contain %q", output, exp)
					}
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			output := buf.String()
			for _, exp := range tt.expected {
				if !strings.Contains(output, exp) {
					t.Errorf("output %q should contain %q", output, exp)
				}
			}
		})
	}
}

func TestJSONMergeCommand(t *testing.T) {
	tmpDir := t.TempDir()
	file1 := filepath.Join(tmpDir, "a.json")
	file2 := filepath.Join(tmpDir, "b.json")

	os.WriteFile(file1, []byte(`{"name":"test","value":1}`), 0644)
	os.WriteFile(file2, []byte(`{"value":2,"extra":"data"}`), 0644)

	cmd := newJSONCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"merge", file1, file2})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	// Should have merged: name from file1, value from file2 (override), extra from file2
	if !strings.Contains(output, "name") || !strings.Contains(output, "extra") {
		t.Errorf("output should contain merged keys: %s", output)
	}
}
