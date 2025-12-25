package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMultigrepCommand(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test files
	os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(`package main

func processData(input string) string {
	return input
}

func main() {
	result := processData("test")
	println(result)
}
`), 0644)

	os.WriteFile(filepath.Join(tmpDir, "utils.go"), []byte(`package main

const MaxRetries = 3

func helper() {
	// uses MaxRetries
	for i := 0; i < MaxRetries; i++ {
		processData("retry")
	}
}
`), 0644)

	tests := []struct {
		name     string
		args     []string
		expected []string
	}{
		{
			name:     "search single keyword",
			args:     []string{"--path", tmpDir, "--keywords", "processData"},
			expected: []string{"processData", "DEFINITIONS:", "main.go"},
		},
		{
			name:     "search multiple keywords",
			args:     []string{"--path", tmpDir, "--keywords", "processData,MaxRetries"},
			expected: []string{"processData", "MaxRetries", "KEYWORDS_SEARCHED: 2"},
		},
		{
			name:     "filter by extension",
			args:     []string{"--path", tmpDir, "--keywords", "func", "--extensions", "go"},
			expected: []string{"func", ".go"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := newMultigrepCmd()
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
					t.Errorf("output should contain %q, got: %s", exp, output)
				}
			}
		})
	}
}

func TestMultigrepDefinitionPriority(t *testing.T) {
	tmpDir := t.TempDir()

	os.WriteFile(filepath.Join(tmpDir, "code.ts"), []byte(`
export const ApiEndpoint = "/api/v1";

function useApiEndpoint() {
	return ApiEndpoint;
}

// ApiEndpoint is used here
console.log(ApiEndpoint);
`), 0644)

	cmd := newMultigrepCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--path", tmpDir, "--keywords", "ApiEndpoint"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	// Should show definitions first
	if !strings.Contains(output, "DEFINITIONS:") {
		t.Errorf("output should contain DEFINITIONS section: %s", output)
	}
}

func TestMultigrepJSON(t *testing.T) {
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "test.go"), []byte("func test() {}"), 0644)

	cmd := newMultigrepCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--path", tmpDir, "--keywords", "test", "--json"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, `"keywords_searched"`) {
		t.Errorf("JSON output should contain keywords_searched: %s", output)
	}
}
