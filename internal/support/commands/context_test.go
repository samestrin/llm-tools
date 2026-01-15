package commands

import (
	"bytes"
	"encoding/json"
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

// =============================================================================
// AC 02-01: Value Retrieval (Get) Tests
// =============================================================================

func TestContext_Get_ExistingKey(t *testing.T) {
	tempDir := t.TempDir()
	runContextCmd(t, "init", "--dir", tempDir)
	runContextCmd(t, "set", "--dir", tempDir, "MY_VAR", "test_value")

	stdout, _, err := runContextCmd(t, "get", "--dir", tempDir, "MY_VAR")
	if err != nil {
		t.Fatalf("context get failed: %v", err)
	}

	if !strings.Contains(stdout, "MY_VAR: test_value") {
		t.Errorf("expected 'MY_VAR: test_value', got: %s", stdout)
	}
}

func TestContext_Get_MissingKeyWithDefault(t *testing.T) {
	tempDir := t.TempDir()
	runContextCmd(t, "init", "--dir", tempDir)

	stdout, _, err := runContextCmd(t, "get", "--dir", tempDir, "MISSING_VAR", "--default", "fallback")
	if err != nil {
		t.Fatalf("context get with default failed: %v", err)
	}

	if !strings.Contains(stdout, "MISSING_VAR: fallback") {
		t.Errorf("expected 'MISSING_VAR: fallback', got: %s", stdout)
	}
}

func TestContext_Get_MissingKeyNoDefault(t *testing.T) {
	tempDir := t.TempDir()
	runContextCmd(t, "init", "--dir", tempDir)

	_, _, err := runContextCmd(t, "get", "--dir", tempDir, "MISSING_VAR")
	if err == nil {
		t.Error("expected error for missing key without default")
	}
}

func TestContext_Get_JsonOutput(t *testing.T) {
	tempDir := t.TempDir()
	runContextCmd(t, "init", "--dir", tempDir)
	runContextCmd(t, "set", "--dir", tempDir, "MY_VAR", "test_value")

	stdout, _, err := runContextCmd(t, "get", "--dir", tempDir, "MY_VAR", "--json")
	if err != nil {
		t.Fatalf("context get --json failed: %v", err)
	}

	if !strings.Contains(stdout, `"key"`) || !strings.Contains(stdout, `"MY_VAR"`) {
		t.Errorf("expected JSON with key, got: %s", stdout)
	}
	if !strings.Contains(stdout, `"value"`) || !strings.Contains(stdout, `"test_value"`) {
		t.Errorf("expected JSON with value, got: %s", stdout)
	}
}

func TestContext_Get_MinOutput(t *testing.T) {
	tempDir := t.TempDir()
	runContextCmd(t, "init", "--dir", tempDir)
	runContextCmd(t, "set", "--dir", tempDir, "MY_VAR", "test_value")

	stdout, _, err := runContextCmd(t, "get", "--dir", tempDir, "MY_VAR", "--min")
	if err != nil {
		t.Fatalf("context get --min failed: %v", err)
	}

	// Min output should be just the value
	if strings.TrimSpace(stdout) != "test_value" {
		t.Errorf("expected 'test_value', got: %q", stdout)
	}
}

func TestContext_Get_LastValueWins(t *testing.T) {
	tempDir := t.TempDir()
	runContextCmd(t, "init", "--dir", tempDir)
	runContextCmd(t, "set", "--dir", tempDir, "COUNTER", "1")
	runContextCmd(t, "set", "--dir", tempDir, "COUNTER", "2")
	runContextCmd(t, "set", "--dir", tempDir, "COUNTER", "3")

	stdout, _, err := runContextCmd(t, "get", "--dir", tempDir, "COUNTER", "--min")
	if err != nil {
		t.Fatalf("context get failed: %v", err)
	}

	// Should get the last value
	if strings.TrimSpace(stdout) != "3" {
		t.Errorf("expected '3' (last value), got: %q", stdout)
	}
}

// =============================================================================
// AC 02-02: List All Values Tests
// =============================================================================

func TestContext_List_MultipleValues(t *testing.T) {
	tempDir := t.TempDir()
	runContextCmd(t, "init", "--dir", tempDir)
	runContextCmd(t, "set", "--dir", tempDir, "VAR1", "A")
	runContextCmd(t, "set", "--dir", tempDir, "VAR2", "B")

	stdout, _, err := runContextCmd(t, "list", "--dir", tempDir)
	if err != nil {
		t.Fatalf("context list failed: %v", err)
	}

	if !strings.Contains(stdout, "VAR1") || !strings.Contains(stdout, "VAR2") {
		t.Errorf("expected both variables in output, got: %s", stdout)
	}
}

func TestContext_List_EmptyContext(t *testing.T) {
	tempDir := t.TempDir()
	runContextCmd(t, "init", "--dir", tempDir)

	stdout, _, err := runContextCmd(t, "list", "--dir", tempDir)
	if err != nil {
		t.Fatalf("context list on empty should not error: %v", err)
	}

	// Empty context should produce no output (or just whitespace)
	trimmed := strings.TrimSpace(stdout)
	if trimmed != "" {
		t.Errorf("expected empty output for empty context, got: %q", stdout)
	}
}

func TestContext_List_JsonOutput(t *testing.T) {
	tempDir := t.TempDir()
	runContextCmd(t, "init", "--dir", tempDir)
	runContextCmd(t, "set", "--dir", tempDir, "VAR1", "A")
	runContextCmd(t, "set", "--dir", tempDir, "VAR2", "B")

	stdout, _, err := runContextCmd(t, "list", "--dir", tempDir, "--json")
	if err != nil {
		t.Fatalf("context list --json failed: %v", err)
	}

	if !strings.Contains(stdout, "{") || !strings.Contains(stdout, "}") {
		t.Errorf("expected JSON object, got: %s", stdout)
	}
	if !strings.Contains(stdout, `"VAR1"`) || !strings.Contains(stdout, `"VAR2"`) {
		t.Errorf("expected both keys in JSON, got: %s", stdout)
	}
}

func TestContext_List_DeduplicatesKeys(t *testing.T) {
	tempDir := t.TempDir()
	runContextCmd(t, "init", "--dir", tempDir)
	runContextCmd(t, "set", "--dir", tempDir, "COUNTER", "1")
	runContextCmd(t, "set", "--dir", tempDir, "COUNTER", "2")

	stdout, _, err := runContextCmd(t, "list", "--dir", tempDir)
	if err != nil {
		t.Fatalf("context list failed: %v", err)
	}

	// Should only show COUNTER once with last value
	count := strings.Count(stdout, "COUNTER")
	if count != 1 {
		t.Errorf("expected COUNTER to appear once (deduplicated), got %d times in: %s", count, stdout)
	}
}

// =============================================================================
// AC 02-03: Dump Shell-Sourceable Tests
// =============================================================================

func TestContext_Dump_ShellFormat(t *testing.T) {
	tempDir := t.TempDir()
	runContextCmd(t, "init", "--dir", tempDir)
	runContextCmd(t, "set", "--dir", tempDir, "VAR1", "value1")

	stdout, _, err := runContextCmd(t, "dump", "--dir", tempDir)
	if err != nil {
		t.Fatalf("context dump failed: %v", err)
	}

	// Should be shell-sourceable format
	if !strings.Contains(stdout, "VAR1='value1'") {
		t.Errorf("expected shell format VAR1='value1', got: %s", stdout)
	}
}

func TestContext_Dump_EmptyContext(t *testing.T) {
	tempDir := t.TempDir()
	runContextCmd(t, "init", "--dir", tempDir)

	stdout, _, err := runContextCmd(t, "dump", "--dir", tempDir)
	if err != nil {
		t.Fatalf("context dump on empty should not error: %v", err)
	}

	trimmed := strings.TrimSpace(stdout)
	if trimmed != "" {
		t.Errorf("expected empty output for empty context, got: %q", stdout)
	}
}

func TestContext_Dump_DeduplicatesKeys(t *testing.T) {
	tempDir := t.TempDir()
	runContextCmd(t, "init", "--dir", tempDir)
	runContextCmd(t, "set", "--dir", tempDir, "VAR", "first")
	runContextCmd(t, "set", "--dir", tempDir, "VAR", "second")

	stdout, _, err := runContextCmd(t, "dump", "--dir", tempDir)
	if err != nil {
		t.Fatalf("context dump failed: %v", err)
	}

	// Should only have last value
	if !strings.Contains(stdout, "VAR='second'") {
		t.Errorf("expected VAR='second', got: %s", stdout)
	}
	if strings.Contains(stdout, "VAR='first'") {
		t.Errorf("should not contain first value, got: %s", stdout)
	}
}

// =============================================================================
// AC 02-04: Clear Context Tests
// =============================================================================

func TestContext_Clear_RemovesAllValues(t *testing.T) {
	tempDir := t.TempDir()
	runContextCmd(t, "init", "--dir", tempDir)
	runContextCmd(t, "set", "--dir", tempDir, "VAR1", "A")
	runContextCmd(t, "set", "--dir", tempDir, "VAR2", "B")

	_, _, err := runContextCmd(t, "clear", "--dir", tempDir)
	if err != nil {
		t.Fatalf("context clear failed: %v", err)
	}

	// List should now be empty
	stdout, _, _ := runContextCmd(t, "list", "--dir", tempDir)
	trimmed := strings.TrimSpace(stdout)
	if trimmed != "" {
		t.Errorf("expected empty list after clear, got: %q", stdout)
	}
}

func TestContext_Clear_PreservesHeader(t *testing.T) {
	tempDir := t.TempDir()
	runContextCmd(t, "init", "--dir", tempDir)
	runContextCmd(t, "set", "--dir", tempDir, "VAR1", "A")

	runContextCmd(t, "clear", "--dir", tempDir)

	// File should still have header
	content, _ := os.ReadFile(filepath.Join(tempDir, "context.env"))
	if !strings.Contains(string(content), "# llm-support context file") {
		t.Errorf("header was not preserved after clear: %s", content)
	}
}

func TestContext_Clear_EmptyContext(t *testing.T) {
	tempDir := t.TempDir()
	runContextCmd(t, "init", "--dir", tempDir)

	// Clear empty context should succeed (no-op)
	_, _, err := runContextCmd(t, "clear", "--dir", tempDir)
	if err != nil {
		t.Fatalf("clear on empty context should not error: %v", err)
	}
}

// =============================================================================
// AC 03-01: init-temp Integration Tests
// =============================================================================

func TestContext_InitTempWorkflow(t *testing.T) {
	// Simulate init-temp output by creating a temp directory
	tempDir := t.TempDir()

	// Step 1: Init context
	stdout, _, err := runContextCmd(t, "init", "--dir", tempDir)
	if err != nil {
		t.Fatalf("context init failed: %v", err)
	}
	if !strings.Contains(stdout, "CONTEXT_FILE") {
		t.Errorf("expected CONTEXT_FILE in output, got: %s", stdout)
	}

	// Step 2: Set a value
	_, _, err = runContextCmd(t, "set", "--dir", tempDir, "MY_KEY", "my_value")
	if err != nil {
		t.Fatalf("context set failed: %v", err)
	}

	// Step 3: Get the value
	stdout, _, err = runContextCmd(t, "get", "--dir", tempDir, "MY_KEY", "--min")
	if err != nil {
		t.Fatalf("context get failed: %v", err)
	}
	if strings.TrimSpace(stdout) != "my_value" {
		t.Errorf("expected 'my_value', got: %q", stdout)
	}

	// Step 4: Verify context.env exists in temp directory
	contextFile := filepath.Join(tempDir, "context.env")
	if _, err := os.Stat(contextFile); os.IsNotExist(err) {
		t.Error("context.env should exist in temp directory")
	}
}

func TestContext_WorksWithAnyDirectory(t *testing.T) {
	// Context should work with any valid directory, not just init-temp created ones
	customDir := t.TempDir()

	// Should work fine
	_, _, err := runContextCmd(t, "init", "--dir", customDir)
	if err != nil {
		t.Fatalf("context init failed on custom directory: %v", err)
	}
}

// =============================================================================
// AC 03-02: Error Messages Tests
// =============================================================================

func TestContext_Init_ErrorMessageSuggestsInitTemp(t *testing.T) {
	_, _, err := runContextCmd(t, "init", "--dir", "/nonexistent/directory/path")
	if err == nil {
		t.Fatal("expected error for nonexistent directory")
	}

	errMsg := err.Error()
	if !strings.Contains(errMsg, "init-temp") {
		t.Errorf("error message should suggest init-temp, got: %s", errMsg)
	}
}

func TestContext_Set_ErrorMessageSuggestsInitTemp(t *testing.T) {
	_, _, err := runContextCmd(t, "set", "--dir", "/nonexistent/directory/path", "KEY", "value")
	if err == nil {
		t.Fatal("expected error for nonexistent directory")
	}

	errMsg := err.Error()
	if !strings.Contains(errMsg, "init") {
		t.Errorf("error message should mention init, got: %s", errMsg)
	}
}

func TestContext_Get_ErrorMessageForMissingFile(t *testing.T) {
	tempDir := t.TempDir()
	// Don't init, try to get directly

	_, _, err := runContextCmd(t, "get", "--dir", tempDir, "KEY")
	if err == nil {
		t.Fatal("expected error for missing context file")
	}

	errMsg := err.Error()
	if !strings.Contains(errMsg, "context init") {
		t.Errorf("error message should suggest context init, got: %s", errMsg)
	}
}

// =============================================================================
// Multiset Tests (Sprint 8.2)
// =============================================================================

func TestContextMultiSet(t *testing.T) {
	tempDir := t.TempDir()
	runContextCmd(t, "init", "--dir", tempDir)

	// Set multiple key-value pairs
	stdout, _, err := runContextCmd(t, "multiset", "--dir", tempDir, "KEY1", "value1", "KEY2", "value2", "KEY3", "value3")
	if err != nil {
		t.Fatalf("multiset failed: %v", err)
	}

	// Verify output confirms all keys
	if !strings.Contains(stdout, "KEY1") || !strings.Contains(stdout, "KEY2") || !strings.Contains(stdout, "KEY3") {
		t.Errorf("expected all keys in output, got: %s", stdout)
	}

	// Verify values are in file
	content, _ := os.ReadFile(filepath.Join(tempDir, "context.env"))
	if !strings.Contains(string(content), "KEY1='value1'") {
		t.Errorf("expected KEY1='value1' in file, got: %s", content)
	}
	if !strings.Contains(string(content), "KEY2='value2'") {
		t.Errorf("expected KEY2='value2' in file, got: %s", content)
	}
	if !strings.Contains(string(content), "KEY3='value3'") {
		t.Errorf("expected KEY3='value3' in file, got: %s", content)
	}
}

func TestContextMultiSetValidation(t *testing.T) {
	tempDir := t.TempDir()
	runContextCmd(t, "init", "--dir", tempDir)

	// All keys validated before any writes
	_, _, err := runContextCmd(t, "multiset", "--dir", tempDir, "VALID_KEY", "value1", "1INVALID", "value2")
	if err == nil {
		t.Error("expected error for invalid key")
	}

	// Verify NO keys were written (atomic validation)
	content, _ := os.ReadFile(filepath.Join(tempDir, "context.env"))
	if strings.Contains(string(content), "VALID_KEY") {
		t.Errorf("valid key should not be written when invalid key present: %s", content)
	}
}

func TestContextMultiSetOddArgs(t *testing.T) {
	tempDir := t.TempDir()
	runContextCmd(t, "init", "--dir", tempDir)

	// Odd number of arguments should fail
	_, _, err := runContextCmd(t, "multiset", "--dir", tempDir, "KEY1", "value1", "KEY2")
	if err == nil {
		t.Error("expected error for odd argument count")
	}

	errMsg := err.Error()
	if !strings.Contains(errMsg, "pairs") {
		t.Errorf("error should mention pairs, got: %s", errMsg)
	}
}

func TestContextMultiSetInvalidKey(t *testing.T) {
	tempDir := t.TempDir()
	runContextCmd(t, "init", "--dir", tempDir)

	// Key starting with digit
	_, _, err := runContextCmd(t, "multiset", "--dir", tempDir, "1KEY", "value")
	if err == nil {
		t.Error("expected error for key starting with digit")
	}

	// Key with hyphen
	_, _, err = runContextCmd(t, "multiset", "--dir", tempDir, "my-key", "value")
	if err == nil {
		t.Error("expected error for key with hyphen")
	}
}

func TestContextMultiSetEmptyValue(t *testing.T) {
	tempDir := t.TempDir()
	runContextCmd(t, "init", "--dir", tempDir)

	// Empty value should be allowed
	_, _, err := runContextCmd(t, "multiset", "--dir", tempDir, "KEY1", "", "KEY2", "value2")
	if err != nil {
		t.Fatalf("multiset with empty value failed: %v", err)
	}

	// Verify empty value is stored
	content, _ := os.ReadFile(filepath.Join(tempDir, "context.env"))
	if !strings.Contains(string(content), "KEY1=''") {
		t.Errorf("expected KEY1='' in file, got: %s", content)
	}
}

func TestContextMultiSetNoArgs(t *testing.T) {
	tempDir := t.TempDir()
	runContextCmd(t, "init", "--dir", tempDir)

	// No arguments should fail
	_, _, err := runContextCmd(t, "multiset", "--dir", tempDir)
	if err == nil {
		t.Error("expected error for no arguments")
	}
}

// =============================================================================
// Multiget Tests (Sprint 8.2)
// =============================================================================

func TestContextMultiGet(t *testing.T) {
	tempDir := t.TempDir()
	runContextCmd(t, "init", "--dir", tempDir)
	runContextCmd(t, "set", "--dir", tempDir, "KEY1", "value1")
	runContextCmd(t, "set", "--dir", tempDir, "KEY2", "value2")

	// Get multiple keys
	stdout, _, err := runContextCmd(t, "multiget", "--dir", tempDir, "KEY1", "KEY2")
	if err != nil {
		t.Fatalf("multiget failed: %v", err)
	}

	// Default output should show both keys
	if !strings.Contains(stdout, "KEY1") || !strings.Contains(stdout, "KEY2") {
		t.Errorf("expected both keys in output, got: %s", stdout)
	}
	if !strings.Contains(stdout, "value1") || !strings.Contains(stdout, "value2") {
		t.Errorf("expected both values in output, got: %s", stdout)
	}
}

func TestContextMultiGetJSON(t *testing.T) {
	tempDir := t.TempDir()
	runContextCmd(t, "init", "--dir", tempDir)
	runContextCmd(t, "set", "--dir", tempDir, "KEY1", "value1")
	runContextCmd(t, "set", "--dir", tempDir, "KEY2", "value2")

	stdout, _, err := runContextCmd(t, "multiget", "--dir", tempDir, "KEY1", "KEY2", "--json")
	if err != nil {
		t.Fatalf("multiget --json failed: %v", err)
	}

	// Should be valid JSON with both keys
	if !strings.Contains(stdout, "{") || !strings.Contains(stdout, "}") {
		t.Errorf("expected JSON object, got: %s", stdout)
	}
	if !strings.Contains(stdout, `"KEY1"`) || !strings.Contains(stdout, `"KEY2"`) {
		t.Errorf("expected both keys in JSON, got: %s", stdout)
	}
	if !strings.Contains(stdout, `"value1"`) || !strings.Contains(stdout, `"value2"`) {
		t.Errorf("expected both values in JSON, got: %s", stdout)
	}
}

func TestContextMultiGetMin(t *testing.T) {
	tempDir := t.TempDir()
	runContextCmd(t, "init", "--dir", tempDir)
	runContextCmd(t, "set", "--dir", tempDir, "KEY1", "value1")
	runContextCmd(t, "set", "--dir", tempDir, "KEY2", "value2")

	// Get in specific order: KEY2 first, then KEY1
	stdout, _, err := runContextCmd(t, "multiget", "--dir", tempDir, "KEY2", "KEY1", "--min")
	if err != nil {
		t.Fatalf("multiget --min failed: %v", err)
	}

	// Values should be newline-separated in argument order
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	if len(lines) != 2 {
		t.Errorf("expected 2 lines, got %d: %q", len(lines), stdout)
	}
	if lines[0] != "value2" {
		t.Errorf("expected first value 'value2', got: %s", lines[0])
	}
	if lines[1] != "value1" {
		t.Errorf("expected second value 'value1', got: %s", lines[1])
	}
}

func TestContextMultiGetMissingKey(t *testing.T) {
	tempDir := t.TempDir()
	runContextCmd(t, "init", "--dir", tempDir)
	runContextCmd(t, "set", "--dir", tempDir, "KEY1", "value1")

	// Request existing and missing key
	_, _, err := runContextCmd(t, "multiget", "--dir", tempDir, "KEY1", "MISSING")
	if err == nil {
		t.Error("expected error for missing key")
	}

	errMsg := err.Error()
	if !strings.Contains(errMsg, "MISSING") {
		t.Errorf("error should mention missing key, got: %s", errMsg)
	}
}

func TestContextMultiGetNoArgs(t *testing.T) {
	tempDir := t.TempDir()
	runContextCmd(t, "init", "--dir", tempDir)

	// No keys should fail
	_, _, err := runContextCmd(t, "multiget", "--dir", tempDir)
	if err == nil {
		t.Error("expected error for no keys")
	}
}

func TestContextMultiGetDefaults(t *testing.T) {
	tempDir := t.TempDir()
	runContextCmd(t, "init", "--dir", tempDir)
	runContextCmd(t, "set", "--dir", tempDir, "KEY1", "value1")

	// Request existing key and missing key with default
	stdout, _, err := runContextCmd(t, "multiget", "--dir", tempDir, "KEY1", "MISSING", "--defaults", `{"MISSING": "default_value"}`)
	if err != nil {
		t.Fatalf("multiget with defaults failed: %v", err)
	}

	// Should contain both values
	if !strings.Contains(stdout, "value1") {
		t.Errorf("expected value1 in output, got: %s", stdout)
	}
	if !strings.Contains(stdout, "default_value") {
		t.Errorf("expected default_value in output, got: %s", stdout)
	}
}

func TestContextMultiGetDefaultsJSON(t *testing.T) {
	tempDir := t.TempDir()
	runContextCmd(t, "init", "--dir", tempDir)
	runContextCmd(t, "set", "--dir", tempDir, "KEY1", "value1")

	// Request with defaults and JSON output
	stdout, _, err := runContextCmd(t, "multiget", "--dir", tempDir, "KEY1", "MISSING", "--defaults", `{"MISSING": "fallback"}`, "--json")
	if err != nil {
		t.Fatalf("multiget with defaults --json failed: %v", err)
	}

	var result map[string]string
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("invalid JSON output: %v", err)
	}

	if result["KEY1"] != "value1" {
		t.Errorf("expected KEY1=value1, got: %s", result["KEY1"])
	}
	if result["MISSING"] != "fallback" {
		t.Errorf("expected MISSING=fallback, got: %s", result["MISSING"])
	}
}

func TestContextMultiGetDefaultsMin(t *testing.T) {
	tempDir := t.TempDir()
	runContextCmd(t, "init", "--dir", tempDir)
	runContextCmd(t, "set", "--dir", tempDir, "KEY1", "value1")

	// Request with defaults and min output - order should be preserved
	stdout, _, err := runContextCmd(t, "multiget", "--dir", tempDir, "MISSING", "KEY1", "--defaults", `{"MISSING": "default"}`, "--min")
	if err != nil {
		t.Fatalf("multiget with defaults --min failed: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d: %s", len(lines), stdout)
	}
	if lines[0] != "default" {
		t.Errorf("expected first line to be 'default', got: %s", lines[0])
	}
	if lines[1] != "value1" {
		t.Errorf("expected second line to be 'value1', got: %s", lines[1])
	}
}

func TestContextMultiGetDefaultsPartialMiss(t *testing.T) {
	tempDir := t.TempDir()
	runContextCmd(t, "init", "--dir", tempDir)
	runContextCmd(t, "set", "--dir", tempDir, "KEY1", "value1")

	// Request two missing keys but only provide default for one - should fail
	_, _, err := runContextCmd(t, "multiget", "--dir", tempDir, "KEY1", "MISSING1", "MISSING2", "--defaults", `{"MISSING1": "default1"}`)
	if err == nil {
		t.Error("expected error when not all missing keys have defaults")
	}

	errMsg := err.Error()
	if !strings.Contains(errMsg, "MISSING2") {
		t.Errorf("expected error to mention MISSING2, got: %s", errMsg)
	}
}

func TestContextMultiGetDefaultsInvalidJSON(t *testing.T) {
	tempDir := t.TempDir()
	runContextCmd(t, "init", "--dir", tempDir)

	// Invalid JSON should fail
	_, _, err := runContextCmd(t, "multiget", "--dir", tempDir, "KEY1", "--defaults", `{invalid}`)
	if err == nil {
		t.Error("expected error for invalid JSON defaults")
	}

	errMsg := err.Error()
	if !strings.Contains(errMsg, "invalid") {
		t.Errorf("expected error to mention invalid JSON, got: %s", errMsg)
	}
}

func TestContextMultiGetDefaultsNotNeeded(t *testing.T) {
	tempDir := t.TempDir()
	runContextCmd(t, "init", "--dir", tempDir)
	runContextCmd(t, "set", "--dir", tempDir, "KEY1", "value1")
	runContextCmd(t, "set", "--dir", tempDir, "KEY2", "value2")

	// Provide defaults but all keys exist - defaults should be ignored
	stdout, _, err := runContextCmd(t, "multiget", "--dir", tempDir, "KEY1", "KEY2", "--defaults", `{"KEY1": "should_not_use", "KEY3": "also_not_used"}`)
	if err != nil {
		t.Fatalf("multiget with unused defaults failed: %v", err)
	}

	// Should use actual values, not defaults
	if !strings.Contains(stdout, "value1") {
		t.Errorf("expected value1 (not default) in output, got: %s", stdout)
	}
	if strings.Contains(stdout, "should_not_use") {
		t.Errorf("should not use default when key exists, got: %s", stdout)
	}
}

// Tests for --json/--min output formats

func TestContextInitJSON(t *testing.T) {
	tempDir := t.TempDir()
	stdout, _, err := runContextCmd(t, "init", "--dir", tempDir, "--json")
	if err != nil {
		t.Fatalf("init --json failed: %v", err)
	}

	var result map[string]string
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("invalid JSON output: %v", err)
	}

	if result["status"] != "CREATED" {
		t.Errorf("expected status CREATED, got: %s", result["status"])
	}
	if result["context_file"] == "" {
		t.Error("expected context_file in output")
	}
}

func TestContextInitMin(t *testing.T) {
	tempDir := t.TempDir()
	stdout, _, err := runContextCmd(t, "init", "--dir", tempDir, "--min")
	if err != nil {
		t.Fatalf("init --min failed: %v", err)
	}

	// Should just be the file path
	expected := filepath.Join(tempDir, "context.env")
	if strings.TrimSpace(stdout) != expected {
		t.Errorf("expected %s, got: %s", expected, stdout)
	}
}

func TestContextSetJSON(t *testing.T) {
	tempDir := t.TempDir()
	runContextCmd(t, "init", "--dir", tempDir)

	stdout, _, err := runContextCmd(t, "set", "--dir", tempDir, "MY_KEY", "my_value", "--json")
	if err != nil {
		t.Fatalf("set --json failed: %v", err)
	}

	var result map[string]string
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("invalid JSON output: %v", err)
	}

	if result["status"] != "ok" {
		t.Errorf("expected status ok, got: %s", result["status"])
	}
	if result["key"] != "MY_KEY" {
		t.Errorf("expected key MY_KEY, got: %s", result["key"])
	}
}

func TestContextSetMin(t *testing.T) {
	tempDir := t.TempDir()
	runContextCmd(t, "init", "--dir", tempDir)

	stdout, _, err := runContextCmd(t, "set", "--dir", tempDir, "MY_KEY", "my_value", "--min")
	if err != nil {
		t.Fatalf("set --min failed: %v", err)
	}

	// Should just be the key name
	if strings.TrimSpace(stdout) != "MY_KEY" {
		t.Errorf("expected MY_KEY, got: %s", stdout)
	}
}

func TestContextMultiSetJSON(t *testing.T) {
	tempDir := t.TempDir()
	runContextCmd(t, "init", "--dir", tempDir)

	stdout, _, err := runContextCmd(t, "multiset", "--dir", tempDir, "KEY1", "value1", "KEY2", "value2", "--json")
	if err != nil {
		t.Fatalf("multiset --json failed: %v", err)
	}

	var result struct {
		Status string   `json:"status"`
		Keys   []string `json:"keys"`
		Count  int      `json:"count"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("invalid JSON output: %v", err)
	}

	if result.Status != "ok" {
		t.Errorf("expected status ok, got: %s", result.Status)
	}
	if result.Count != 2 {
		t.Errorf("expected count 2, got: %d", result.Count)
	}
	if len(result.Keys) != 2 {
		t.Errorf("expected 2 keys, got: %d", len(result.Keys))
	}
}

func TestContextMultiSetMin(t *testing.T) {
	tempDir := t.TempDir()
	runContextCmd(t, "init", "--dir", tempDir)

	stdout, _, err := runContextCmd(t, "multiset", "--dir", tempDir, "KEY1", "value1", "KEY2", "value2", "--min")
	if err != nil {
		t.Fatalf("multiset --min failed: %v", err)
	}

	// Should just be the count
	if strings.TrimSpace(stdout) != "2" {
		t.Errorf("expected 2, got: %s", stdout)
	}
}

func TestContextMultiSetJSONMin(t *testing.T) {
	tempDir := t.TempDir()
	runContextCmd(t, "init", "--dir", tempDir)

	stdout, _, err := runContextCmd(t, "multiset", "--dir", tempDir, "KEY1", "value1", "KEY2", "value2", "--json", "--min")
	if err != nil {
		t.Fatalf("multiset --json --min failed: %v", err)
	}

	var result struct {
		Count  int    `json:"count"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("invalid JSON output: %v", err)
	}

	if result.Count != 2 {
		t.Errorf("expected count 2, got: %d", result.Count)
	}
	if result.Status != "ok" {
		t.Errorf("expected status ok, got: %s", result.Status)
	}
}

// =============================================================================
// Path Normalization Tests
// =============================================================================

func TestNormalizeContextDir_Directory(t *testing.T) {
	tempDir := t.TempDir()

	// Directory should be returned as-is
	result := normalizeContextDir(tempDir)
	if result != tempDir {
		t.Errorf("expected %s, got: %s", tempDir, result)
	}
}

func TestNormalizeContextDir_FilePath(t *testing.T) {
	tempDir := t.TempDir()

	// Create a file
	filePath := filepath.Join(tempDir, "context.env")
	if err := os.WriteFile(filePath, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// File path should return parent directory
	result := normalizeContextDir(filePath)
	if result != tempDir {
		t.Errorf("expected %s, got: %s", tempDir, result)
	}
}

func TestNormalizeContextDir_NonexistentPath(t *testing.T) {
	// Nonexistent path should be returned as-is (let caller handle error)
	nonexistent := "/this/path/does/not/exist"
	result := normalizeContextDir(nonexistent)
	if result != nonexistent {
		t.Errorf("expected %s, got: %s", nonexistent, result)
	}
}

func TestContext_Get_WithFilePathNormalization(t *testing.T) {
	tempDir := t.TempDir()
	runContextCmd(t, "init", "--dir", tempDir)
	runContextCmd(t, "set", "--dir", tempDir, "MY_VAR", "test_value")

	// Pass the full file path instead of directory
	filePath := filepath.Join(tempDir, "context.env")
	stdout, _, err := runContextCmd(t, "get", "--dir", filePath, "MY_VAR", "--min")
	if err != nil {
		t.Fatalf("get with file path should work: %v", err)
	}

	if strings.TrimSpace(stdout) != "test_value" {
		t.Errorf("expected 'test_value', got: %q", stdout)
	}
}

func TestContext_Set_WithFilePathNormalization(t *testing.T) {
	tempDir := t.TempDir()
	runContextCmd(t, "init", "--dir", tempDir)

	// Pass the full file path instead of directory
	filePath := filepath.Join(tempDir, "context.env")
	_, _, err := runContextCmd(t, "set", "--dir", filePath, "MY_VAR", "test_value")
	if err != nil {
		t.Fatalf("set with file path should work: %v", err)
	}

	// Verify value was set
	stdout, _, _ := runContextCmd(t, "get", "--dir", tempDir, "MY_VAR", "--min")
	if strings.TrimSpace(stdout) != "test_value" {
		t.Errorf("expected 'test_value', got: %q", stdout)
	}
}

func TestContext_MultiGet_WithFilePathNormalization(t *testing.T) {
	tempDir := t.TempDir()
	runContextCmd(t, "init", "--dir", tempDir)
	runContextCmd(t, "set", "--dir", tempDir, "KEY1", "value1")
	runContextCmd(t, "set", "--dir", tempDir, "KEY2", "value2")

	// Pass the full file path instead of directory
	filePath := filepath.Join(tempDir, "context.env")
	stdout, _, err := runContextCmd(t, "multiget", "--dir", filePath, "KEY1", "KEY2", "--min")
	if err != nil {
		t.Fatalf("multiget with file path should work: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	if len(lines) != 2 {
		t.Errorf("expected 2 lines, got %d: %q", len(lines), stdout)
	}
}

func TestContext_MultiSet_WithFilePathNormalization(t *testing.T) {
	tempDir := t.TempDir()
	runContextCmd(t, "init", "--dir", tempDir)

	// Pass the full file path instead of directory
	filePath := filepath.Join(tempDir, "context.env")
	_, _, err := runContextCmd(t, "multiset", "--dir", filePath, "KEY1", "value1", "KEY2", "value2")
	if err != nil {
		t.Fatalf("multiset with file path should work: %v", err)
	}

	// Verify values were set
	stdout, _, _ := runContextCmd(t, "multiget", "--dir", tempDir, "KEY1", "KEY2", "--min")
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	if len(lines) != 2 {
		t.Errorf("expected 2 values, got %d: %q", len(lines), stdout)
	}
}

func TestContext_List_WithFilePathNormalization(t *testing.T) {
	tempDir := t.TempDir()
	runContextCmd(t, "init", "--dir", tempDir)
	runContextCmd(t, "set", "--dir", tempDir, "MY_VAR", "test_value")

	// Pass the full file path instead of directory
	filePath := filepath.Join(tempDir, "context.env")
	stdout, _, err := runContextCmd(t, "list", "--dir", filePath)
	if err != nil {
		t.Fatalf("list with file path should work: %v", err)
	}

	if !strings.Contains(stdout, "MY_VAR") {
		t.Errorf("expected MY_VAR in output, got: %s", stdout)
	}
}

func TestContext_Dump_WithFilePathNormalization(t *testing.T) {
	tempDir := t.TempDir()
	runContextCmd(t, "init", "--dir", tempDir)
	runContextCmd(t, "set", "--dir", tempDir, "MY_VAR", "test_value")

	// Pass the full file path instead of directory
	filePath := filepath.Join(tempDir, "context.env")
	stdout, _, err := runContextCmd(t, "dump", "--dir", filePath)
	if err != nil {
		t.Fatalf("dump with file path should work: %v", err)
	}

	if !strings.Contains(stdout, "MY_VAR='test_value'") {
		t.Errorf("expected MY_VAR='test_value', got: %s", stdout)
	}
}

func TestContext_Clear_WithFilePathNormalization(t *testing.T) {
	tempDir := t.TempDir()
	runContextCmd(t, "init", "--dir", tempDir)
	runContextCmd(t, "set", "--dir", tempDir, "MY_VAR", "test_value")

	// Pass the full file path instead of directory
	filePath := filepath.Join(tempDir, "context.env")
	_, _, err := runContextCmd(t, "clear", "--dir", filePath)
	if err != nil {
		t.Fatalf("clear with file path should work: %v", err)
	}

	// Verify it was cleared
	stdout, _, _ := runContextCmd(t, "list", "--dir", tempDir)
	if strings.TrimSpace(stdout) != "" {
		t.Errorf("expected empty list after clear, got: %s", stdout)
	}
}
