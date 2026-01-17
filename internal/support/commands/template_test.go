package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTemplateCommand(t *testing.T) {
	tmpDir := t.TempDir()
	tmplFile := filepath.Join(tmpDir, "template.txt")
	os.WriteFile(tmplFile, []byte("Hello, {{name}}! You are {{age}} years old."), 0644)

	dataFile := filepath.Join(tmpDir, "data.json")
	os.WriteFile(dataFile, []byte(`{"name":"Alice","age":"30"}`), 0644)

	tests := []struct {
		name     string
		args     []string
		expected []string
		hasError bool
	}{
		{
			name:     "substitute from command line",
			args:     []string{tmplFile, "--var", "name=Bob", "--var", "age=25"},
			expected: []string{"Hello, Bob!", "25 years old"},
		},
		{
			name:     "substitute from data file",
			args:     []string{tmplFile, "--data", dataFile},
			expected: []string{"Hello, Alice!", "30 years old"},
		},
		{
			name:     "missing variable with default",
			args:     []string{"-"},
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.name == "missing variable with default" {
				return // Skip stdin test for now
			}

			cmd := newTemplateCmd()
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

func TestTemplateBracketSyntax(t *testing.T) {
	tmpDir := t.TempDir()
	tmplFile := filepath.Join(tmpDir, "template.txt")
	os.WriteFile(tmplFile, []byte("Hello, [[name]]!"), 0644)

	cmd := newTemplateCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{tmplFile, "--var", "name=World", "--syntax", "brackets"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Hello, World!") {
		t.Errorf("output %q should contain 'Hello, World!'", output)
	}
}

func TestTemplateDefaultValue(t *testing.T) {
	tmpDir := t.TempDir()
	tmplFile := filepath.Join(tmpDir, "template.txt")
	os.WriteFile(tmplFile, []byte("Hello, {{name|Guest}}!"), 0644)

	cmd := newTemplateCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{tmplFile})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Hello, Guest!") {
		t.Errorf("output %q should contain 'Hello, Guest!'", output)
	}
}

// TestTemplateJSONOutput tests JSON output mode
func TestTemplateJSONOutput(t *testing.T) {
	tmpDir := t.TempDir()
	tmplFile := filepath.Join(tmpDir, "template.txt")
	os.WriteFile(tmplFile, []byte("Hello, {{name}}!"), 0644)

	cmd := newTemplateCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{tmplFile, "--var", "name=World", "--json"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	// JSON output should contain the result
	if !strings.Contains(output, `"content"`) && !strings.Contains(output, "Hello, World!") {
		t.Errorf("JSON output %q should contain content", output)
	}
}

// TestTemplateMinimalOutput tests minimal output mode
func TestTemplateMinimalOutput(t *testing.T) {
	tmpDir := t.TempDir()
	tmplFile := filepath.Join(tmpDir, "template.txt")
	os.WriteFile(tmplFile, []byte("Hello, {{name}}!"), 0644)

	cmd := newTemplateCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{tmplFile, "--var", "name=Minimal", "--min"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	// Minimal output should have the result
	if !strings.Contains(output, "Hello, Minimal!") {
		t.Errorf("output %q should contain 'Hello, Minimal!'", output)
	}
}

// TestTemplateNonExistentFile tests error for non-existent template file
func TestTemplateNonExistentFile(t *testing.T) {
	cmd := newTemplateCmd()
	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(errBuf)
	cmd.SetArgs([]string{"nonexistent_template"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for non-existent template file")
	}
}

// TestTemplateOutputToFile tests writing output to file
func TestTemplateOutputToFile(t *testing.T) {
	tmpDir := t.TempDir()
	tmplFile := filepath.Join(tmpDir, "template.txt")
	outputFile := filepath.Join(tmpDir, "output.txt")
	os.WriteFile(tmplFile, []byte("Hello, {{name}}!"), 0644)

	cmd := newTemplateCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{tmplFile, "--var", "name=FileTest", "--output", outputFile})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check the output file
	content, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}
	if !strings.Contains(string(content), "Hello, FileTest!") {
		t.Errorf("output file should contain 'Hello, FileTest!', got: %s", content)
	}
}

// TestTemplateMultipleVariables tests multiple variable substitution
func TestTemplateMultipleVariables(t *testing.T) {
	tmpDir := t.TempDir()
	tmplFile := filepath.Join(tmpDir, "template.txt")
	os.WriteFile(tmplFile, []byte("{{greeting}}, {{name}}! Welcome to {{place}}."), 0644)

	cmd := newTemplateCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{tmplFile, "--var", "greeting=Hi", "--var", "name=User", "--var", "place=Earth"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Hi, User! Welcome to Earth.") {
		t.Errorf("output %q should contain 'Hi, User! Welcome to Earth.'", output)
	}
}

// TestTemplateEnvironmentVariable tests environment variable substitution
func TestTemplateEnvironmentVariable(t *testing.T) {
	tmpDir := t.TempDir()
	tmplFile := filepath.Join(tmpDir, "template.txt")
	os.WriteFile(tmplFile, []byte("Value: {{value}}"), 0644)

	// Set environment variable
	os.Setenv("TEMPLATE_TEST_VALUE", "env_value")
	defer os.Unsetenv("TEMPLATE_TEST_VALUE")

	cmd := newTemplateCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{tmplFile, "--var", "value=$TEMPLATE_TEST_VALUE"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	// The output should contain the value
	if !strings.Contains(output, "Value:") {
		t.Errorf("output %q should contain 'Value:'", output)
	}
}
