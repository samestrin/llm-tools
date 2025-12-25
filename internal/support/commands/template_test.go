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
