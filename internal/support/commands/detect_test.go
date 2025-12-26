package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDetectCommand(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(string) // function to set up project structure
		expected []string
	}{
		{
			name: "detect node project",
			setup: func(dir string) {
				os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"name":"test"}`), 0644)
			},
			expected: []string{"STACK: node", "LANGUAGE: javascript"},
		},
		{
			name: "detect typescript project",
			setup: func(dir string) {
				os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"name":"test"}`), 0644)
				os.WriteFile(filepath.Join(dir, "tsconfig.json"), []byte(`{}`), 0644)
			},
			expected: []string{"STACK: node", "LANGUAGE: typescript"},
		},
		{
			name: "detect go project",
			setup: func(dir string) {
				os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test"), 0644)
			},
			expected: []string{"STACK: go", "LANGUAGE: go"},
		},
		{
			name: "detect python project",
			setup: func(dir string) {
				os.WriteFile(filepath.Join(dir, "pyproject.toml"), []byte("[project]"), 0644)
			},
			expected: []string{"STACK: python", "LANGUAGE: python"},
		},
		{
			name: "detect rust project",
			setup: func(dir string) {
				os.WriteFile(filepath.Join(dir, "Cargo.toml"), []byte("[package]"), 0644)
			},
			expected: []string{"STACK: rust", "LANGUAGE: rust"},
		},
		{
			name: "detect npm package manager",
			setup: func(dir string) {
				os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{}`), 0644)
				os.WriteFile(filepath.Join(dir, "package-lock.json"), []byte(`{}`), 0644)
			},
			expected: []string{"PACKAGE_MANAGER: npm"},
		},
		{
			name: "detect nextjs framework",
			setup: func(dir string) {
				os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{}`), 0644)
				os.WriteFile(filepath.Join(dir, "next.config.js"), []byte("module.exports = {}"), 0644)
			},
			expected: []string{"FRAMEWORK: nextjs"},
		},
		{
			name: "detect tests directory",
			setup: func(dir string) {
				os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{}`), 0644)
				os.MkdirAll(filepath.Join(dir, "tests"), 0755)
			},
			expected: []string{"HAS_TESTS: true"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			tt.setup(tmpDir)

			cmd := newDetectCmd()
			buf := new(bytes.Buffer)
			cmd.SetOut(buf)
			cmd.SetArgs([]string{"--path", tmpDir})

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

func TestDetectJSONOutput(t *testing.T) {
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte("module test"), 0644)

	cmd := newDetectCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--path", tmpDir, "--json"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, `"stack"`) || !strings.Contains(output, `"go"`) {
		t.Errorf("JSON output should contain stack field: %s", output)
	}
}
