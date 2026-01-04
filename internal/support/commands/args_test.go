package commands

import (
	"bytes"
	"strings"
	"testing"
)

func TestArgsCommand(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		expected []string
	}{
		{
			name:     "no arguments",
			args:     []string{"--"},
			expected: []string{"NO_ARGS"},
		},
		{
			name:     "positional arguments only",
			args:     []string{"--", "file1.txt", "file2.txt"},
			expected: []string{"POSITIONAL: file1.txt file2.txt"},
		},
		{
			name:     "flag with value",
			args:     []string{"--", "--output", "result.txt"},
			expected: []string{"OUTPUT: result.txt"},
		},
		{
			name:     "boolean flag",
			args:     []string{"--", "--verbose"},
			expected: []string{"VERBOSE: true"},
		},
		{
			name:     "mixed positional and flags",
			args:     []string{"--", "file.txt", "--format", "json", "--quiet"},
			expected: []string{"POSITIONAL: file.txt", "FORMAT: json", "QUIET: true"},
		},
		{
			name:     "flag with dashes converted to underscores",
			args:     []string{"--", "--output-file", "test.txt"},
			expected: []string{"OUTPUT_FILE: test.txt"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := newArgsCmd()
			buf := new(bytes.Buffer)
			cmd.SetOut(buf)
			cmd.SetArgs(tt.args)

			err := cmd.Execute()
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
