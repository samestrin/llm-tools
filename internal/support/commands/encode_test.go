package commands

import (
	"bytes"
	"strings"
	"testing"
)

func TestEncodeCommand(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		expected []string
		hasError bool
	}{
		{
			name:     "base64 encoding",
			args:     []string{"hello", "-e", "base64"},
			expected: []string{"OUTPUT: aGVsbG8="},
		},
		{
			name:     "base32 encoding",
			args:     []string{"hello", "-e", "base32"},
			expected: []string{"OUTPUT: NBSWY3DP"},
		},
		{
			name:     "hex encoding",
			args:     []string{"hello", "-e", "hex"},
			expected: []string{"OUTPUT: 68656c6c6f"},
		},
		{
			name:     "url encoding",
			args:     []string{"hello world", "-e", "url"},
			expected: []string{"OUTPUT: hello%20world"},
		},
		{
			name:     "default is base64",
			args:     []string{"test"},
			expected: []string{"OUTPUT: dGVzdA=="},
		},
		{
			name:     "unsupported encoding",
			args:     []string{"test", "-e", "invalid"},
			hasError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := newEncodeCmd()
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

func TestDecodeCommand(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		expected []string
		hasError bool
	}{
		{
			name:     "base64 decoding",
			args:     []string{"aGVsbG8=", "-e", "base64"},
			expected: []string{"OUTPUT: hello"},
		},
		{
			name:     "base32 decoding",
			args:     []string{"NBSWY3DP", "-e", "base32"},
			expected: []string{"OUTPUT: hello"},
		},
		{
			name:     "hex decoding",
			args:     []string{"68656c6c6f", "-e", "hex"},
			expected: []string{"OUTPUT: hello"},
		},
		{
			name:     "url decoding",
			args:     []string{"hello%20world", "-e", "url"},
			expected: []string{"OUTPUT: hello world"},
		},
		{
			name:     "default is base64",
			args:     []string{"dGVzdA=="},
			expected: []string{"OUTPUT: test"},
		},
		{
			name:     "invalid base64",
			args:     []string{"not-valid-base64!!!", "-e", "base64"},
			hasError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := newDecodeCmd()
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
