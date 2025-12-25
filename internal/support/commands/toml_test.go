package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTOMLParseCommand(t *testing.T) {
	tmpDir := t.TempDir()
	validTOML := filepath.Join(tmpDir, "config.toml")
	invalidTOML := filepath.Join(tmpDir, "invalid.toml")

	os.WriteFile(validTOML, []byte(`
[server]
host = "localhost"
port = 8080

[database]
name = "mydb"
`), 0644)
	os.WriteFile(invalidTOML, []byte(`[invalid`), 0644)

	tests := []struct {
		name     string
		args     []string
		expected []string
		hasError bool
	}{
		{
			name:     "parse valid toml",
			args:     []string{"parse", validTOML},
			expected: []string{"server", "localhost", "8080", "database", "mydb"},
		},
		{
			name:     "parse invalid toml",
			args:     []string{"parse", invalidTOML},
			hasError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := newTOMLCmd()
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

func TestTOMLQueryCommand(t *testing.T) {
	tmpDir := t.TempDir()
	tomlFile := filepath.Join(tmpDir, "config.toml")
	os.WriteFile(tomlFile, []byte(`
[server]
host = "localhost"
port = 8080
`), 0644)

	tests := []struct {
		name     string
		args     []string
		expected []string
		hasError bool
	}{
		{
			name:     "query section",
			args:     []string{"query", tomlFile, "server"},
			expected: []string{"host", "localhost"},
		},
		{
			name:     "query nested key",
			args:     []string{"query", tomlFile, "server.port"},
			expected: []string{"8080"},
		},
		{
			name:     "query invalid path",
			args:     []string{"query", tomlFile, "nonexistent"},
			hasError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := newTOMLCmd()
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

func TestTOMLValidateCommand(t *testing.T) {
	tmpDir := t.TempDir()
	validTOML := filepath.Join(tmpDir, "valid.toml")
	invalidTOML := filepath.Join(tmpDir, "invalid.toml")

	os.WriteFile(validTOML, []byte(`key = "value"`), 0644)
	os.WriteFile(invalidTOML, []byte(`[invalid`), 0644)

	tests := []struct {
		name     string
		args     []string
		expected []string
		hasError bool
	}{
		{
			name:     "validate valid toml",
			args:     []string{"validate", validTOML},
			expected: []string{"✓", "VALID"},
		},
		{
			name:     "validate invalid toml",
			args:     []string{"validate", invalidTOML},
			expected: []string{"✗", "INVALID"},
			hasError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := newTOMLCmd()
			buf := new(bytes.Buffer)
			errBuf := new(bytes.Buffer)
			cmd.SetOut(buf)
			cmd.SetErr(errBuf)
			cmd.SetArgs(tt.args)

			err := cmd.Execute()

			if tt.hasError {
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
