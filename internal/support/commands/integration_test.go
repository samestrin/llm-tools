// Package commands provides integration tests for CLI commands.
//
// These tests use golden files for output comparison.
// Run with -update flag to update golden files:
//
//	go test ./... -update
package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/samestrin/llm-tools/internal/support/testhelpers"
)

// getProjectRoot returns the project root directory
func getProjectRoot() string {
	dir, _ := os.Getwd()
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

func TestTreeIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	root := getProjectRoot()
	if root == "" {
		t.Skip("could not find project root")
	}

	fixturesDir := filepath.Join(root, "testdata", "fixtures", "sample_project")
	if _, err := os.Stat(fixturesDir); os.IsNotExist(err) {
		t.Skip("test fixtures not found")
	}

	tests := []struct {
		name       string
		args       []string
		goldenFile string
	}{
		{
			name:       "basic tree",
			args:       []string{"--path", fixturesDir},
			goldenFile: "tree/basic.golden",
		},
		{
			name:       "tree with depth 1",
			args:       []string{"--path", fixturesDir, "--depth", "1"},
			goldenFile: "tree/depth1.golden",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create command with args
			cmd := newTreeCmd()
			buf := &bytes.Buffer{}
			cmd.SetOut(buf)
			cmd.SetErr(buf)
			cmd.SetArgs(tt.args)

			// Execute
			if err := cmd.Execute(); err != nil {
				t.Fatalf("command failed: %v", err)
			}

			// Compare with golden file
			testhelpers.AssertGoldenNormalized(t, tt.goldenFile, buf.String())
		})
	}
}

func TestListdirIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	root := getProjectRoot()
	if root == "" {
		t.Skip("could not find project root")
	}

	fixturesDir := filepath.Join(root, "testdata", "fixtures", "sample_project")
	if _, err := os.Stat(fixturesDir); os.IsNotExist(err) {
		t.Skip("test fixtures not found")
	}

	tests := []struct {
		name       string
		args       []string
		goldenFile string
	}{
		{
			name:       "basic listdir",
			args:       []string{"--path", fixturesDir},
			goldenFile: "listdir/basic.golden",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := newListdirCmd()
			buf := &bytes.Buffer{}
			cmd.SetOut(buf)
			cmd.SetErr(buf)
			cmd.SetArgs(tt.args)

			if err := cmd.Execute(); err != nil {
				t.Fatalf("command failed: %v", err)
			}

			testhelpers.AssertGoldenNormalized(t, tt.goldenFile, buf.String())
		})
	}
}

func TestCountIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Create temp file with known content
	content := `# Test File

## Section 1
- [ ] Todo 1
- [x] Done 1
- [ ] Todo 2

## Section 2
- [x] Done 2
`

	tempFile := testhelpers.CreateTempFile(t, "test.md", content)

	tests := []struct {
		name       string
		args       []string
		goldenFile string
	}{
		{
			name:       "count checkboxes",
			args:       []string{"--path", tempFile, "--mode", "checkboxes"},
			goldenFile: "count/checkboxes.golden",
		},
		{
			name:       "count lines",
			args:       []string{"--path", tempFile, "--mode", "lines"},
			goldenFile: "count/lines.golden",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := newCountCmd()
			buf := &bytes.Buffer{}
			cmd.SetOut(buf)
			cmd.SetErr(buf)
			cmd.SetArgs(tt.args)

			if err := cmd.Execute(); err != nil {
				t.Fatalf("command failed: %v", err)
			}

			testhelpers.AssertGoldenNormalized(t, tt.goldenFile, buf.String())
		})
	}
}

func TestHashIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	content := "Hello, World!"
	tempFile := testhelpers.CreateTempFile(t, "test.txt", content)

	tests := []struct {
		name         string
		args         []string
		expectedHash string // Just check the hash prefix
	}{
		{
			name:         "sha256 hash",
			args:         []string{tempFile},
			expectedHash: "dffd6021bb2bd5b0af676290809ec3a53191dd81c7f70a4b28688a362182986f",
		},
		{
			name:         "md5 hash",
			args:         []string{tempFile, "--algorithm", "md5"},
			expectedHash: "65a8e27d8879283831b664bd8b7f0ad4",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := newHashCmd()
			buf := &bytes.Buffer{}
			cmd.SetOut(buf)
			cmd.SetErr(buf)
			cmd.SetArgs(tt.args)

			if err := cmd.Execute(); err != nil {
				t.Fatalf("command failed: %v", err)
			}

			output := buf.String()
			if !strings.HasPrefix(output, tt.expectedHash) {
				t.Errorf("expected hash %s, got output: %s", tt.expectedHash, output)
			}
		})
	}
}

func TestMathIntegration(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		goldenFile string
	}{
		{
			name:       "simple addition",
			args:       []string{"2 + 3"},
			goldenFile: "math/addition.golden",
		},
		{
			name:       "complex expression",
			args:       []string{"(10 + 5) * 2 - 8 / 4"},
			goldenFile: "math/complex.golden",
		},
		{
			name:       "power expression",
			args:       []string{"2 ** 8"},
			goldenFile: "math/power.golden",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := newMathCmd()
			buf := &bytes.Buffer{}
			cmd.SetOut(buf)
			cmd.SetErr(buf)
			cmd.SetArgs(tt.args)

			if err := cmd.Execute(); err != nil {
				t.Fatalf("command failed: %v", err)
			}

			testhelpers.AssertGoldenNormalized(t, tt.goldenFile, buf.String())
		})
	}
}
