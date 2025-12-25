package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitTempCommand(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		setup    func(string)
		expected []string
	}{
		{
			name:     "create new temp dir",
			args:     []string{"test-session"},
			expected: []string{"TEMP_DIR:", "STATUS: CREATED"},
		},
		{
			name: "clean existing temp dir",
			args: []string{"test-session"},
			setup: func(dir string) {
				tempDir := filepath.Join(dir, ".planning", ".temp", "test-session")
				os.MkdirAll(tempDir, 0755)
				os.WriteFile(filepath.Join(tempDir, "old.txt"), []byte("old"), 0644)
			},
			expected: []string{"STATUS: CREATED", "CLEANED:"},
		},
		{
			name: "preserve existing temp dir",
			args: []string{"test-session", "--preserve"},
			setup: func(dir string) {
				tempDir := filepath.Join(dir, ".planning", ".temp", "test-session")
				os.MkdirAll(tempDir, 0755)
				os.WriteFile(filepath.Join(tempDir, "keep.txt"), []byte("keep"), 0644)
			},
			expected: []string{"STATUS: EXISTS", "EXISTING_FILES: 1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			origDir, _ := os.Getwd()
			os.Chdir(tmpDir)
			defer os.Chdir(origDir)

			if tt.setup != nil {
				tt.setup(tmpDir)
			}

			cmd := newInitTempCmd()
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
