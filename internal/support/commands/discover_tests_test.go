package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiscoverTestsCommand(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(string)
		expected []string
	}{
		{
			name: "detect vitest",
			setup: func(dir string) {
				os.WriteFile(filepath.Join(dir, "vitest.config.ts"), []byte("export default {}"), 0644)
				os.MkdirAll(filepath.Join(dir, "src"), 0755)
				os.MkdirAll(filepath.Join(dir, "tests"), 0755)
				os.WriteFile(filepath.Join(dir, "tests", "app.test.ts"), []byte("test()"), 0644)
			},
			expected: []string{"TEST_RUNNER: vitest", "PATTERN: SEPARATED"},
		},
		{
			name: "detect jest",
			setup: func(dir string) {
				os.WriteFile(filepath.Join(dir, "jest.config.js"), []byte("module.exports = {}"), 0644)
				os.MkdirAll(filepath.Join(dir, "src"), 0755)
				os.MkdirAll(filepath.Join(dir, "__tests__"), 0755)
				os.WriteFile(filepath.Join(dir, "__tests__", "app.test.js"), []byte("test()"), 0644)
			},
			expected: []string{"TEST_RUNNER: jest", "PATTERN: SEPARATED"},
		},
		{
			name: "detect nextjs framework",
			setup: func(dir string) {
				os.WriteFile(filepath.Join(dir, "next.config.js"), []byte("module.exports = {}"), 0644)
			},
			expected: []string{"FRAMEWORK: nextjs"},
		},
		{
			name: "detect e2e directory",
			setup: func(dir string) {
				os.MkdirAll(filepath.Join(dir, "e2e"), 0755)
				os.WriteFile(filepath.Join(dir, "e2e", "app.spec.ts"), []byte("test()"), 0644)
			},
			expected: []string{"E2E_DIR: e2e/"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			tt.setup(tmpDir)

			cmd := newDiscoverTestsCmd()
			buf := new(bytes.Buffer)
			cmd.SetOut(buf)
			cmd.SetArgs([]string{tmpDir})

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
