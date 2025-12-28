package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/spf13/cobra"
)

// Helper to create a test context command
func newTestContextCmd() *cobra.Command {
	return newContextCmd()
}

// Helper to run context command with args
func runContextCmd(t *testing.T, args ...string) (string, string, error) {
	t.Helper()
	cmd := newTestContextCmd()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return stdout.String(), stderr.String(), err
}

// =============================================================================
// AC 01-01: Context Initialization Tests
// =============================================================================

func TestContext_Init_CreatesContextFile(t *testing.T) {
	tempDir := t.TempDir()

	stdout, _, err := runContextCmd(t, "init", "--dir", tempDir)
	if err != nil {
		t.Fatalf("context init failed: %v", err)
	}

	// Verify context.env was created
	contextFile := filepath.Join(tempDir, "context.env")
	if _, err := os.Stat(contextFile); os.IsNotExist(err) {
		t.Errorf("context.env was not created in %s", tempDir)
	}

	// Verify output mentions success
	if !strings.Contains(stdout, "CONTEXT_FILE") {
		t.Errorf("expected CONTEXT_FILE in output, got: %s", stdout)
	}
}

func TestContext_Init_FileHasHeader(t *testing.T) {
	tempDir := t.TempDir()

	_, _, err := runContextCmd(t, "init", "--dir", tempDir)
	if err != nil {
		t.Fatalf("context init failed: %v", err)
	}

	// Read the file
	contextFile := filepath.Join(tempDir, "context.env")
	content, err := os.ReadFile(contextFile)
	if err != nil {
		t.Fatalf("failed to read context.env: %v", err)
	}

	// Verify header
	if !strings.Contains(string(content), "# llm-support context file") {
		t.Errorf("expected header comment, got: %s", content)
	}
	if !strings.Contains(string(content), "# Created:") {
		t.Errorf("expected Created timestamp, got: %s", content)
	}
}

func TestContext_Init_PreservesExistingContext(t *testing.T) {
	tempDir := t.TempDir()

	// First init
	_, _, err := runContextCmd(t, "init", "--dir", tempDir)
	if err != nil {
		t.Fatalf("first init failed: %v", err)
	}

	// Write a value manually
	contextFile := filepath.Join(tempDir, "context.env")
	existingContent, _ := os.ReadFile(contextFile)
	newContent := string(existingContent) + "TEST_VAR='existing_value'\n"
	os.WriteFile(contextFile, []byte(newContent), 0644)

	// Second init (should preserve)
	_, _, err = runContextCmd(t, "init", "--dir", tempDir)
	if err != nil {
		t.Fatalf("second init failed: %v", err)
	}

	// Verify existing content preserved
	content, _ := os.ReadFile(contextFile)
	if !strings.Contains(string(content), "TEST_VAR='existing_value'") {
		t.Errorf("existing content was not preserved: %s", content)
	}
}

func TestContext_Init_MissingDirectory(t *testing.T) {
	_, _, err := runContextCmd(t, "init", "--dir", "/nonexistent/path/that/does/not/exist")
	if err == nil {
		t.Error("expected error for nonexistent directory, got nil")
	}
}

func TestContext_Init_MissingDirFlag(t *testing.T) {
	_, _, err := runContextCmd(t, "init")
	if err == nil {
		t.Error("expected error for missing --dir flag, got nil")
	}
}

// =============================================================================
// AC 01-02: Value Storage (Set) Tests
// =============================================================================

func TestContext_Set_SimpleValue(t *testing.T) {
	tempDir := t.TempDir()

	// Init first
	runContextCmd(t, "init", "--dir", tempDir)

	// Set a value
	_, _, err := runContextCmd(t, "set", "--dir", tempDir, "MY_VAR", "simple_value")
	if err != nil {
		t.Fatalf("context set failed: %v", err)
	}

	// Verify value in file
	content, _ := os.ReadFile(filepath.Join(tempDir, "context.env"))
	if !strings.Contains(string(content), "MY_VAR='simple_value'") {
		t.Errorf("expected MY_VAR='simple_value', got: %s", content)
	}
}

func TestContext_Set_ValueWithSpaces(t *testing.T) {
	tempDir := t.TempDir()
	runContextCmd(t, "init", "--dir", tempDir)

	_, _, err := runContextCmd(t, "set", "--dir", tempDir, "GREETING", "Hello World")
	if err != nil {
		t.Fatalf("context set failed: %v", err)
	}

	content, _ := os.ReadFile(filepath.Join(tempDir, "context.env"))
	if !strings.Contains(string(content), "GREETING='Hello World'") {
		t.Errorf("expected GREETING='Hello World', got: %s", content)
	}
}

func TestContext_Set_ValueWithSingleQuotes(t *testing.T) {
	tempDir := t.TempDir()
	runContextCmd(t, "init", "--dir", tempDir)

	_, _, err := runContextCmd(t, "set", "--dir", tempDir, "MESSAGE", "It's working")
	if err != nil {
		t.Fatalf("context set failed: %v", err)
	}

	content, _ := os.ReadFile(filepath.Join(tempDir, "context.env"))
	// Single quotes should be escaped as '\''
	if !strings.Contains(string(content), "MESSAGE='It'\\''s working'") {
		t.Errorf("expected properly escaped single quotes, got: %s", content)
	}
}

func TestContext_Set_ValueWithDoubleQuotes(t *testing.T) {
	tempDir := t.TempDir()
	runContextCmd(t, "init", "--dir", tempDir)

	_, _, err := runContextCmd(t, "set", "--dir", tempDir, "QUOTED", `She said "hello"`)
	if err != nil {
		t.Fatalf("context set failed: %v", err)
	}

	content, _ := os.ReadFile(filepath.Join(tempDir, "context.env"))
	// Double quotes preserved inside single quotes
	if !strings.Contains(string(content), `QUOTED='She said "hello"'`) {
		t.Errorf("expected double quotes preserved, got: %s", content)
	}
}

func TestContext_Set_MultiLineValue(t *testing.T) {
	tempDir := t.TempDir()
	runContextCmd(t, "init", "--dir", tempDir)

	multiline := "Line 1\nLine 2\nLine 3"
	_, _, err := runContextCmd(t, "set", "--dir", tempDir, "MULTI", multiline)
	if err != nil {
		t.Fatalf("context set failed: %v", err)
	}

	content, _ := os.ReadFile(filepath.Join(tempDir, "context.env"))
	if !strings.Contains(string(content), "MULTI='Line 1\nLine 2\nLine 3'") {
		t.Errorf("expected multiline value preserved, got: %s", content)
	}
}

func TestContext_Set_UpdateExistingKey(t *testing.T) {
	tempDir := t.TempDir()
	runContextCmd(t, "init", "--dir", tempDir)

	// Set initial value
	runContextCmd(t, "set", "--dir", tempDir, "COUNTER", "1")

	// Update value
	_, _, err := runContextCmd(t, "set", "--dir", tempDir, "COUNTER", "2")
	if err != nil {
		t.Fatalf("context set update failed: %v", err)
	}

	// File should have both (shell sourcing uses last)
	content, _ := os.ReadFile(filepath.Join(tempDir, "context.env"))
	if !strings.Contains(string(content), "COUNTER='1'") || !strings.Contains(string(content), "COUNTER='2'") {
		t.Errorf("expected both values in file for shell sourcing, got: %s", content)
	}
}

// =============================================================================
// AC 01-03: Key Validation Tests
// =============================================================================

func TestContext_Set_ValidKeys(t *testing.T) {
	tempDir := t.TempDir()
	runContextCmd(t, "init", "--dir", tempDir)

	validKeys := []string{"VAR", "MY_VAR", "VAR1", "_VAR", "A", "A1", "_1"}
	for _, key := range validKeys {
		_, _, err := runContextCmd(t, "set", "--dir", tempDir, key, "value")
		if err != nil {
			t.Errorf("key %q should be valid, got error: %v", key, err)
		}
	}
}

func TestContext_Set_InvalidKeyStartsWithDigit(t *testing.T) {
	tempDir := t.TempDir()
	runContextCmd(t, "init", "--dir", tempDir)

	_, _, err := runContextCmd(t, "set", "--dir", tempDir, "1VAR", "value")
	if err == nil {
		t.Error("expected error for key starting with digit")
	}
}

func TestContext_Set_InvalidKeyWithHyphen(t *testing.T) {
	tempDir := t.TempDir()
	runContextCmd(t, "init", "--dir", tempDir)

	_, _, err := runContextCmd(t, "set", "--dir", tempDir, "my-var", "value")
	if err == nil {
		t.Error("expected error for key with hyphen")
	}
}

func TestContext_Set_InvalidKeyWithDot(t *testing.T) {
	tempDir := t.TempDir()
	runContextCmd(t, "init", "--dir", tempDir)

	_, _, err := runContextCmd(t, "set", "--dir", tempDir, "my.var", "value")
	if err == nil {
		t.Error("expected error for key with dot")
	}
}

func TestContext_Set_LowercaseKeyUppercased(t *testing.T) {
	tempDir := t.TempDir()
	runContextCmd(t, "init", "--dir", tempDir)

	_, _, err := runContextCmd(t, "set", "--dir", tempDir, "myvar", "value")
	if err != nil {
		t.Fatalf("lowercase key should be accepted: %v", err)
	}

	// Key should be uppercased in file
	content, _ := os.ReadFile(filepath.Join(tempDir, "context.env"))
	if !strings.Contains(string(content), "MYVAR='value'") {
		t.Errorf("expected uppercased key MYVAR, got: %s", content)
	}
}

// =============================================================================
// AC 01-03: Concurrent Write Safety Tests
// =============================================================================

func TestContext_Set_ConcurrentWritesSafe(t *testing.T) {
	tempDir := t.TempDir()
	runContextCmd(t, "init", "--dir", tempDir)

	// Launch multiple concurrent writes
	var wg sync.WaitGroup
	numGoroutines := 10
	errChan := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			key := strings.ToUpper(string(rune('A' + idx)))
			_, _, err := runContextCmd(t, "set", "--dir", tempDir, key, "value")
			if err != nil {
				errChan <- err
			}
		}(i)
	}

	wg.Wait()
	close(errChan)

	// Check for errors
	for err := range errChan {
		t.Errorf("concurrent write failed: %v", err)
	}

	// Verify file is not corrupted
	content, err := os.ReadFile(filepath.Join(tempDir, "context.env"))
	if err != nil {
		t.Fatalf("failed to read context file: %v", err)
	}

	// File should contain proper lines (not interleaved)
	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// Each line should be a valid KEY='value' format
		if !strings.Contains(line, "='") || !strings.HasSuffix(line, "'") {
			t.Errorf("corrupted line detected: %q", line)
		}
	}
}
