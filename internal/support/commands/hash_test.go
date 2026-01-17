package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHashCommand(t *testing.T) {
	// Create temp file for testing
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	err := os.WriteFile(testFile, []byte("hello world\n"), 0644)
	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	tests := []struct {
		name     string
		args     []string
		expected string
		hasError bool
	}{
		{
			name:     "sha256 hash",
			args:     []string{testFile, "-a", "sha256"},
			expected: "a948904f2f0f479b8f8197694b30184b0d2ed1c1cd2a1ec0fb85d299a192a447",
		},
		{
			name:     "md5 hash",
			args:     []string{testFile, "-a", "md5"},
			expected: "6f5902ac237024bdd0c176cb93063dc4",
		},
		{
			name:     "sha1 hash",
			args:     []string{testFile, "-a", "sha1"},
			expected: "22596363b3de40b06f981fb85d82312e8c0ed511",
		},
		{
			name:     "default algorithm is sha256",
			args:     []string{testFile},
			expected: "a948904f2f0f479b8f8197694b30184b0d2ed1c1cd2a1ec0fb85d299a192a447",
		},
		{
			name:     "unsupported algorithm",
			args:     []string{testFile, "-a", "invalid"},
			hasError: true,
		},
		{
			name:     "non-existent file",
			args:     []string{"/nonexistent/file.txt"},
			hasError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := newHashCmd()
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
			if !strings.Contains(output, tt.expected) {
				t.Errorf("output %q should contain %q", output, tt.expected)
			}
		})
	}
}

// TestHashSHA512 tests SHA512 algorithm
func TestHashSHA512(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("hello world\n"), 0644)

	cmd := newHashCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{testFile, "-a", "sha512"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	// SHA512 produces 128 hex characters - output should contain the hash
	if len(output) < 128 {
		t.Errorf("expected SHA512 hash in output (128+ chars), got: %s", output)
	}
}

// TestHashJSONOutput tests JSON output mode
func TestHashJSONOutput(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("hello world\n"), 0644)

	cmd := newHashCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{testFile, "--json"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, `"hash"`) && !strings.Contains(output, `"algorithm"`) {
		t.Errorf("JSON output should contain hash or algorithm keys, got: %s", output)
	}
}

// TestHashMinimalOutput tests minimal output mode
func TestHashMinimalOutput(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("hello world\n"), 0644)

	cmd := newHashCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{testFile, "--min"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	// Minimal output should be more compact
	if len(output) == 0 {
		t.Error("expected output for minimal mode")
	}
}

// TestHashMultipleFiles tests hashing multiple files
func TestHashMultipleFiles(t *testing.T) {
	tmpDir := t.TempDir()
	testFile1 := filepath.Join(tmpDir, "file1.txt")
	testFile2 := filepath.Join(tmpDir, "file2.txt")
	os.WriteFile(testFile1, []byte("content1"), 0644)
	os.WriteFile(testFile2, []byte("content2"), 0644)

	cmd := newHashCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{testFile1, testFile2})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	// Should contain hashes for both files
	if !strings.Contains(output, "file1.txt") || !strings.Contains(output, "file2.txt") {
		t.Errorf("output should contain both filenames, got: %s", output)
	}
}
