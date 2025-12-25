package commands

import (
	"bytes"
	"strings"
	"testing"
)

func TestMathCommand(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		expected string
		hasError bool
	}{
		{
			name:     "simple addition",
			args:     []string{"2 + 3"},
			expected: "5",
		},
		{
			name:     "subtraction",
			args:     []string{"10 - 4"},
			expected: "6",
		},
		{
			name:     "multiplication",
			args:     []string{"6 * 7"},
			expected: "42",
		},
		{
			name:     "division",
			args:     []string{"20 / 4"},
			expected: "5",
		},
		{
			name:     "complex expression",
			args:     []string{"(2 + 3) * 4"},
			expected: "20",
		},
		{
			name:     "floating point",
			args:     []string{"10 / 3"},
			expected: "3.333",
		},
		{
			name:     "power",
			args:     []string{"2 ** 8"},
			expected: "256",
		},
		{
			name:     "modulo",
			args:     []string{"17 % 5"},
			expected: "2",
		},
		{
			name:     "negative numbers",
			args:     []string{"-5 + 10"},
			expected: "5",
		},
		{
			name:     "abs function",
			args:     []string{"abs(-42)"},
			expected: "42",
		},
		{
			name:     "max function",
			args:     []string{"max(1, 5, 3)"},
			expected: "5",
		},
		{
			name:     "min function",
			args:     []string{"min(1, 5, 3)"},
			expected: "1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := newMathCmd()
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
			if !strings.Contains(output, tt.expected) {
				t.Errorf("output %q should contain %q", output, tt.expected)
			}
		})
	}
}
