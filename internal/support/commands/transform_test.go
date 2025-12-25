package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTransformCSVToJSONCommand(t *testing.T) {
	tmpDir := t.TempDir()
	csvFile := filepath.Join(tmpDir, "data.csv")
	os.WriteFile(csvFile, []byte(`name,age,city
Alice,30,NYC
Bob,25,LA
`), 0644)

	cmd := newTransformCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"csv-to-json", csvFile})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	expected := []string{"name", "Alice", "age", "30", "city", "NYC"}
	for _, exp := range expected {
		if !strings.Contains(output, exp) {
			t.Errorf("output %q should contain %q", output, exp)
		}
	}
}

func TestTransformJSONToCSVCommand(t *testing.T) {
	tmpDir := t.TempDir()
	jsonFile := filepath.Join(tmpDir, "data.json")
	os.WriteFile(jsonFile, []byte(`[{"name":"Alice","age":30},{"name":"Bob","age":25}]`), 0644)

	cmd := newTransformCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"json-to-csv", jsonFile})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	expected := []string{"name", "age", "Alice", "Bob"}
	for _, exp := range expected {
		if !strings.Contains(output, exp) {
			t.Errorf("output %q should contain %q", output, exp)
		}
	}
}

func TestTransformCaseCommand(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		expected []string
		hasError bool
	}{
		{
			name:     "to camelCase",
			args:     []string{"case", "hello_world", "--to", "camelCase"},
			expected: []string{"helloWorld"},
		},
		{
			name:     "to PascalCase",
			args:     []string{"case", "hello_world", "--to", "PascalCase"},
			expected: []string{"HelloWorld"},
		},
		{
			name:     "to snake_case",
			args:     []string{"case", "helloWorld", "--to", "snake_case"},
			expected: []string{"hello_world"},
		},
		{
			name:     "to kebab-case",
			args:     []string{"case", "helloWorld", "--to", "kebab-case"},
			expected: []string{"hello-world"},
		},
		{
			name:     "to UPPERCASE",
			args:     []string{"case", "hello", "--to", "UPPERCASE"},
			expected: []string{"HELLO"},
		},
		{
			name:     "to lowercase",
			args:     []string{"case", "HELLO", "--to", "lowercase"},
			expected: []string{"hello"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := newTransformCmd()
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

func TestTransformSortCommand(t *testing.T) {
	tmpDir := t.TempDir()
	file := filepath.Join(tmpDir, "lines.txt")
	os.WriteFile(file, []byte("banana\napple\ncherry\napple\n"), 0644)

	tests := []struct {
		name     string
		args     []string
		expected []string
		hasError bool
	}{
		{
			name:     "sort ascending",
			args:     []string{"sort", file},
			expected: []string{"apple"},
		},
		{
			name:     "sort descending",
			args:     []string{"sort", file, "--reverse"},
			expected: []string{"cherry"},
		},
		{
			name:     "sort unique",
			args:     []string{"sort", file, "--unique"},
			expected: []string{"apple", "banana", "cherry"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := newTransformCmd()
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

func TestTransformFilterCommand(t *testing.T) {
	tmpDir := t.TempDir()
	file := filepath.Join(tmpDir, "lines.txt")
	os.WriteFile(file, []byte("apple\nbanana\napricot\ncherry\n"), 0644)

	tests := []struct {
		name     string
		args     []string
		expected []string
		hasError bool
	}{
		{
			name:     "filter matching pattern",
			args:     []string{"filter", file, "^a"},
			expected: []string{"apple", "apricot"},
		},
		{
			name:     "filter inverted",
			args:     []string{"filter", file, "^a", "--invert"},
			expected: []string{"banana", "cherry"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := newTransformCmd()
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
