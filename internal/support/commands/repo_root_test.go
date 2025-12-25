package commands

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestRepoRootCommand(t *testing.T) {
	// Create temp directory with git repo
	tmpDir := t.TempDir()

	// Resolve symlinks (macOS /var -> /private/var)
	tmpDir, err := filepath.EvalSymlinks(tmpDir)
	if err != nil {
		t.Fatalf("failed to resolve symlinks: %v", err)
	}

	// Initialize git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to init git repo: %v", err)
	}

	// Create a subdirectory
	subDir := filepath.Join(tmpDir, "src", "pkg")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("failed to create subdir: %v", err)
	}

	tests := []struct {
		name       string
		args       []string
		wantRoot   string
		wantValid  string
		wantError  bool
	}{
		{
			name:     "from repo root",
			args:     []string{"--path", tmpDir},
			wantRoot: tmpDir,
		},
		{
			name:     "from subdirectory",
			args:     []string{"--path", subDir},
			wantRoot: tmpDir,
		},
		{
			name:      "with validate flag",
			args:      []string{"--path", tmpDir, "--validate"},
			wantRoot:  tmpDir,
			wantValid: "TRUE",
		},
		{
			name:      "non-git directory",
			args:      []string{"--path", os.TempDir()},
			wantRoot:  "",
			wantError: false, // Returns empty, not error
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := newRepoRootCmd()
			buf := new(bytes.Buffer)
			cmd.SetOut(buf)
			cmd.SetArgs(tt.args)

			err := cmd.Execute()
			if (err != nil) != tt.wantError {
				t.Errorf("unexpected error: %v", err)
			}

			output := buf.String()

			// Check ROOT line
			if tt.wantRoot != "" {
				if !strings.Contains(output, "ROOT: "+tt.wantRoot) {
					t.Errorf("expected ROOT: %s, got output: %s", tt.wantRoot, output)
				}
			}

			// Check VALID line if expected
			if tt.wantValid != "" {
				if !strings.Contains(output, "VALID: "+tt.wantValid) {
					t.Errorf("expected VALID: %s, got output: %s", tt.wantValid, output)
				}
			}
		})
	}
}

func TestRepoRootNonExistentPath(t *testing.T) {
	cmd := newRepoRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--path", "/nonexistent/path/that/does/not/exist"})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for non-existent path")
	}
}
