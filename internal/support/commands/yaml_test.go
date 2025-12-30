package commands

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Test helpers

func createTempDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "yaml-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return dir
}

func createTestYAML(t *testing.T, dir, content string) string {
	t.Helper()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create test YAML: %v", err)
	}
	return path
}

// ============================================================================
// US-01: Configuration Initialization Tests
// ============================================================================

func TestYamlInit_BasicInit(t *testing.T) {
	// AC 01-01: yaml init --file creates a new YAML config file with default structure
	dir := createTempDir(t)
	configPath := filepath.Join(dir, "config.yaml")

	cmd := newYamlCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"init", "--file", configPath})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Fatal("config file was not created")
	}

	// Verify output contains CONFIG_FILE and STATUS
	output := buf.String()
	if !strings.Contains(output, "CONFIG_FILE:") {
		t.Errorf("output missing CONFIG_FILE, got: %s", output)
	}
	if !strings.Contains(output, "STATUS: CREATED") {
		t.Errorf("output missing STATUS: CREATED, got: %s", output)
	}

	// Verify file is valid YAML with expected sections
	content, _ := os.ReadFile(configPath)
	if !strings.Contains(string(content), "helper:") {
		t.Error("config missing helper section")
	}
}

func TestYamlInit_WithTemplate(t *testing.T) {
	// AC 01-02: yaml init --template planning uses the planning template
	dir := createTempDir(t)
	configPath := filepath.Join(dir, "config.yaml")

	cmd := newYamlCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"init", "--file", configPath, "--template", "planning"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Verify planning template sections exist
	content, _ := os.ReadFile(configPath)
	contentStr := string(content)

	expectedSections := []string{"helper:", "project:", "testing:", "commands:", "tools:"}
	for _, section := range expectedSections {
		if !strings.Contains(contentStr, section) {
			t.Errorf("planning template missing section: %s", section)
		}
	}
}

func TestYamlInit_ForceOverwrite(t *testing.T) {
	// AC 01-03: yaml init --force overwrites an existing file
	dir := createTempDir(t)
	configPath := filepath.Join(dir, "config.yaml")

	// Create existing file with custom content
	os.WriteFile(configPath, []byte("old: content\n"), 0644)

	cmd := newYamlCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"init", "--file", configPath, "--force"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Verify file was overwritten
	content, _ := os.ReadFile(configPath)
	if strings.Contains(string(content), "old: content") {
		t.Error("file was not overwritten")
	}
}

func TestYamlInit_ExistsWithoutForce(t *testing.T) {
	// yaml init without --force should not overwrite existing file
	dir := createTempDir(t)
	configPath := filepath.Join(dir, "config.yaml")

	// Create existing file
	os.WriteFile(configPath, []byte("existing: content\n"), 0644)

	cmd := newYamlCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"init", "--file", configPath})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Verify STATUS: EXISTS
	output := buf.String()
	if !strings.Contains(output, "STATUS: EXISTS") {
		t.Errorf("expected STATUS: EXISTS, got: %s", output)
	}
}

// ============================================================================
// US-02: Single Value Operations Tests
// ============================================================================

func TestYamlGet_SimpleKey(t *testing.T) {
	// AC 02-01: yaml get retrieves value at dot-notation key
	dir := createTempDir(t)
	configPath := createTestYAML(t, dir, `
helper:
  llm: gemini
  script: llm-support
`)

	cmd := newYamlCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"get", "--file", configPath, "helper.llm"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	output := strings.TrimSpace(buf.String())
	if !strings.Contains(output, "gemini") {
		t.Errorf("expected output to contain 'gemini', got: %s", output)
	}
}

func TestYamlGet_NestedKey(t *testing.T) {
	dir := createTempDir(t)
	configPath := createTestYAML(t, dir, `
deeply:
  nested:
    value: found
`)

	cmd := newYamlCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"get", "--file", configPath, "deeply.nested.value", "--min"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	output := strings.TrimSpace(buf.String())
	if output != "found" {
		t.Errorf("expected 'found', got: %s", output)
	}
}

func TestYamlGet_WithDefault(t *testing.T) {
	// AC 02-02: yaml get --default returns fallback for missing keys
	dir := createTempDir(t)
	configPath := createTestYAML(t, dir, `helper:
  llm: gemini
`)

	cmd := newYamlCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"get", "--file", configPath, "missing.key", "--default", "fallback", "--min"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	output := strings.TrimSpace(buf.String())
	if output != "fallback" {
		t.Errorf("expected 'fallback', got: %s", output)
	}
}

func TestYamlGet_MissingKeyNoDefault(t *testing.T) {
	dir := createTempDir(t)
	configPath := createTestYAML(t, dir, `helper:
  llm: gemini
`)

	cmd := newYamlCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"get", "--file", configPath, "missing.key"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing key, got none")
	}

	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error should mention 'not found', got: %v", err)
	}
}

func TestYamlSet_NewValue(t *testing.T) {
	// AC 02-03: yaml set creates/updates value
	dir := createTempDir(t)
	configPath := createTestYAML(t, dir, `helper:
  llm: gemini
`)

	cmd := newYamlCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"set", "--file", configPath, "helper.llm", "claude"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Verify value was updated
	content, _ := os.ReadFile(configPath)
	if !strings.Contains(string(content), "claude") {
		t.Error("value was not updated in file")
	}
}

func TestYamlSet_CreatesIntermediateKeys(t *testing.T) {
	// AC 02-04: yaml set creates intermediate keys
	dir := createTempDir(t)
	configPath := createTestYAML(t, dir, `existing: value
`)

	cmd := newYamlCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"set", "--file", configPath, "new.nested.key", "newvalue"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Verify nested structure was created
	content, _ := os.ReadFile(configPath)
	contentStr := string(content)
	if !strings.Contains(contentStr, "new:") {
		t.Error("intermediate key 'new' was not created")
	}
	if !strings.Contains(contentStr, "nested:") {
		t.Error("intermediate key 'nested' was not created")
	}
	if !strings.Contains(contentStr, "newvalue") {
		t.Error("value was not set")
	}
}

func TestYamlSet_PreservesComments(t *testing.T) {
	// AC 02-05: yaml set preserves YAML comments
	dir := createTempDir(t)
	configPath := createTestYAML(t, dir, `# This is a header comment
helper:
  # LLM configuration
  llm: gemini
  script: llm-support
`)

	cmd := newYamlCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"set", "--file", configPath, "helper.llm", "claude"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Verify comments are preserved
	content, _ := os.ReadFile(configPath)
	contentStr := string(content)
	if !strings.Contains(contentStr, "# This is a header comment") {
		t.Error("header comment was not preserved")
	}
	if !strings.Contains(contentStr, "# LLM configuration") {
		t.Error("inline comment was not preserved")
	}
}

// ============================================================================
// US-03: Batch Operations Tests
// ============================================================================

func TestYamlMultiget_MultipleKeys(t *testing.T) {
	// AC 03-01: yaml multiget retrieves multiple values in order
	dir := createTempDir(t)
	configPath := createTestYAML(t, dir, `
helper:
  llm: gemini
  script: llm-support
project:
  type: go
`)

	cmd := newYamlCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"multiget", "--file", configPath, "helper.llm", "project.type", "--min"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d: %v", len(lines), lines)
	}
	if lines[0] != "gemini" {
		t.Errorf("first value should be 'gemini', got: %s", lines[0])
	}
	if lines[1] != "go" {
		t.Errorf("second value should be 'go', got: %s", lines[1])
	}
}

func TestYamlMultiget_WithDefaults(t *testing.T) {
	// AC 03-02: yaml multiget --defaults provides defaults for missing keys
	dir := createTempDir(t)
	configPath := createTestYAML(t, dir, `
helper:
  llm: gemini
`)

	cmd := newYamlCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"multiget", "--file", configPath, "helper.llm", "missing.key",
		"--defaults", `{"missing.key": "default_value"}`, "--min"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
	if lines[1] != "default_value" {
		t.Errorf("missing key should return default, got: %s", lines[1])
	}
}

func TestYamlMultiget_JSONOutput(t *testing.T) {
	// AC 03-03: yaml multiget --json outputs as JSON object
	dir := createTempDir(t)
	configPath := createTestYAML(t, dir, `
helper:
  llm: gemini
project:
  type: go
`)

	cmd := newYamlCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"multiget", "--file", configPath, "helper.llm", "project.type", "--json"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	var result map[string]string
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	if result["helper.llm"] != "gemini" {
		t.Errorf("expected helper.llm=gemini, got: %s", result["helper.llm"])
	}
}

func TestYamlMultiset_AtomicWrite(t *testing.T) {
	// AC 03-04: yaml multiset writes multiple key-value pairs atomically
	dir := createTempDir(t)
	configPath := createTestYAML(t, dir, `
helper:
  llm: gemini
`)

	cmd := newYamlCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"multiset", "--file", configPath,
		"helper.llm", "claude",
		"helper.script", "llm-support",
		"project.type", "go"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Verify all values were set
	content, _ := os.ReadFile(configPath)
	contentStr := string(content)
	if !strings.Contains(contentStr, "claude") {
		t.Error("helper.llm not updated to claude")
	}
	if !strings.Contains(contentStr, "llm-support") {
		t.Error("helper.script not set")
	}
}

func TestYamlMultiset_ValidationBeforeWrite(t *testing.T) {
	// AC 03-05: yaml multiset validates all keys before writing
	dir := createTempDir(t)
	originalContent := `helper:
  llm: gemini
`
	configPath := createTestYAML(t, dir, originalContent)

	cmd := newYamlCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	// Odd number of args should fail validation
	cmd.SetArgs([]string{"multiset", "--file", configPath, "key1", "value1", "key2"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for odd number of arguments")
	}

	// Verify file was not modified
	content, _ := os.ReadFile(configPath)
	if string(content) != originalContent {
		t.Error("file was modified despite validation failure")
	}
}

// ============================================================================
// US-04: Configuration Management Tests
// ============================================================================

func TestYamlList_FlatKeys(t *testing.T) {
	// AC 04-01: yaml list --flat outputs all keys in dot notation
	dir := createTempDir(t)
	configPath := createTestYAML(t, dir, `
helper:
  llm: gemini
  script: llm-support
project:
  type: go
`)

	cmd := newYamlCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"list", "--file", configPath, "--flat"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	output := buf.String()
	expectedKeys := []string{"helper.llm", "helper.script", "project.type"}
	for _, key := range expectedKeys {
		if !strings.Contains(output, key) {
			t.Errorf("output missing key: %s", key)
		}
	}
}

func TestYamlList_WithValues(t *testing.T) {
	// AC 04-02: yaml list --values includes values alongside keys
	dir := createTempDir(t)
	configPath := createTestYAML(t, dir, `
helper:
  llm: gemini
`)

	cmd := newYamlCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"list", "--file", configPath, "--flat", "--values"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "helper.llm=gemini") {
		t.Errorf("expected key=value format, got: %s", output)
	}
}

func TestYamlDelete_ExistingKey(t *testing.T) {
	// AC 04-03: yaml delete removes a key from the config
	dir := createTempDir(t)
	configPath := createTestYAML(t, dir, `
helper:
  llm: gemini
  script: llm-support
`)

	cmd := newYamlCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"delete", "--file", configPath, "helper.script"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Verify key was deleted
	content, _ := os.ReadFile(configPath)
	if strings.Contains(string(content), "script:") {
		t.Error("key was not deleted")
	}
	// Verify other keys remain
	if !strings.Contains(string(content), "llm:") {
		t.Error("other keys were incorrectly deleted")
	}
}

func TestYamlValidate_ValidSyntax(t *testing.T) {
	// AC 04-04: yaml validate checks YAML syntax
	dir := createTempDir(t)
	configPath := createTestYAML(t, dir, `
helper:
  llm: gemini
`)

	cmd := newYamlCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"validate", "--file", configPath})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("expected no error for valid YAML, got: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "VALID: TRUE") {
		t.Errorf("expected VALID: TRUE, got: %s", output)
	}
}

func TestYamlValidate_InvalidSyntax(t *testing.T) {
	dir := createTempDir(t)
	configPath := createTestYAML(t, dir, `
helper:
  llm: gemini
  bad yaml here: [unclosed
`)

	cmd := newYamlCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"validate", "--file", configPath})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestYamlValidate_RequiredKeys(t *testing.T) {
	// AC 04-05: yaml validate --required checks for required keys
	dir := createTempDir(t)
	configPath := createTestYAML(t, dir, `
helper:
  llm: gemini
`)

	cmd := newYamlCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"validate", "--file", configPath, "--required", "helper.llm,missing.key"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing required key")
	}

	if !strings.Contains(err.Error(), "missing.key") {
		t.Errorf("error should mention missing key, got: %v", err)
	}
}

// ============================================================================
// Array Operations Tests (Push/Pop)
// ============================================================================

func TestYamlPush_AddsElement(t *testing.T) {
	dir := createTempDir(t)
	configPath := createTestYAML(t, dir, `
items:
  - first
  - second
`)

	cmd := newYamlCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"push", "--file", configPath, "items", "third"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	content, _ := os.ReadFile(configPath)
	if !strings.Contains(string(content), "third") {
		t.Error("new element was not added")
	}
}

func TestYamlPop_RemovesLastElement(t *testing.T) {
	dir := createTempDir(t)
	configPath := createTestYAML(t, dir, `
items:
  - first
  - second
  - third
`)

	cmd := newYamlCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"pop", "--file", configPath, "items", "--min"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should output the popped value
	output := strings.TrimSpace(buf.String())
	if output != "third" {
		t.Errorf("expected 'third', got: %s", output)
	}

	// Verify element was removed
	content, _ := os.ReadFile(configPath)
	if strings.Contains(string(content), "third") {
		t.Error("popped element was not removed")
	}
}

// ============================================================================
// Additional Coverage Tests
// ============================================================================

func TestYamlInit_MinimalTemplate(t *testing.T) {
	// Test minimal template
	dir := createTempDir(t)
	configPath := filepath.Join(dir, "config.yaml")

	cmd := newYamlCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"init", "--file", configPath, "--template", "minimal"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	content, _ := os.ReadFile(configPath)
	if len(content) == 0 {
		t.Error("minimal template should create non-empty file")
	}
}

func TestYamlInit_JSONOutput(t *testing.T) {
	dir := createTempDir(t)
	configPath := filepath.Join(dir, "config.yaml")

	cmd := newYamlCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"init", "--file", configPath, "--json"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	if result["status"] != "CREATED" && result["status"] != "created" {
		t.Errorf("expected status=CREATED or created, got: %v", result["status"])
	}
}

func TestYamlGet_JSONOutput(t *testing.T) {
	dir := createTempDir(t)
	configPath := createTestYAML(t, dir, `
helper:
  llm: gemini
`)

	cmd := newYamlCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"get", "--file", configPath, "helper.llm", "--json"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	if result["value"] != "gemini" {
		t.Errorf("expected value=gemini, got: %v", result["value"])
	}
}

func TestYamlGet_TopLevelKey(t *testing.T) {
	dir := createTempDir(t)
	configPath := createTestYAML(t, dir, `
simple: value
`)

	cmd := newYamlCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"get", "--file", configPath, "simple", "--min"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	output := strings.TrimSpace(buf.String())
	if output != "value" {
		t.Errorf("expected 'value', got: %s", output)
	}
}

func TestYamlGet_NumericValue(t *testing.T) {
	dir := createTempDir(t)
	configPath := createTestYAML(t, dir, `
port: 8080
ratio: 3.14
enabled: true
`)

	cmd := newYamlCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"get", "--file", configPath, "port", "--min"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	output := strings.TrimSpace(buf.String())
	if output != "8080" {
		t.Errorf("expected '8080', got: %s", output)
	}
}

func TestYamlGet_BoolValue(t *testing.T) {
	dir := createTempDir(t)
	configPath := createTestYAML(t, dir, `
enabled: true
disabled: false
`)

	cmd := newYamlCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"get", "--file", configPath, "enabled", "--min"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	output := strings.TrimSpace(buf.String())
	if output != "true" {
		t.Errorf("expected 'true', got: %s", output)
	}
}

func TestYamlGet_FloatValue(t *testing.T) {
	dir := createTempDir(t)
	configPath := createTestYAML(t, dir, `
ratio: 3.14159
`)

	cmd := newYamlCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"get", "--file", configPath, "ratio", "--min"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	output := strings.TrimSpace(buf.String())
	if !strings.HasPrefix(output, "3.14") {
		t.Errorf("expected float starting with '3.14', got: %s", output)
	}
}

func TestYamlSet_JSONOutput(t *testing.T) {
	dir := createTempDir(t)
	configPath := createTestYAML(t, dir, `helper:
  llm: gemini
`)

	cmd := newYamlCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"set", "--file", configPath, "helper.llm", "claude", "--json"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	if result["status"] != "set" && result["status"] != "updated" {
		t.Errorf("expected status=set or updated, got: %v", result["status"])
	}
}

func TestYamlSet_NumericValue(t *testing.T) {
	dir := createTempDir(t)
	configPath := createTestYAML(t, dir, `config:
  port: 3000
`)

	cmd := newYamlCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"set", "--file", configPath, "config.port", "8080"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	content, _ := os.ReadFile(configPath)
	if !strings.Contains(string(content), "8080") {
		t.Error("numeric value was not set")
	}
}

func TestYamlSet_BoolValue(t *testing.T) {
	dir := createTempDir(t)
	configPath := createTestYAML(t, dir, `config:
  enabled: false
`)

	cmd := newYamlCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"set", "--file", configPath, "config.enabled", "true"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	content, _ := os.ReadFile(configPath)
	if !strings.Contains(string(content), "true") {
		t.Error("bool value was not set")
	}
}

func TestYamlSet_FileNotFound(t *testing.T) {
	cmd := newYamlCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"set", "--file", "/nonexistent/path/config.yaml", "key", "value"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestYamlMultiget_AllMissing(t *testing.T) {
	dir := createTempDir(t)
	configPath := createTestYAML(t, dir, `
helper:
  llm: gemini
`)

	cmd := newYamlCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"multiget", "--file", configPath, "missing1", "missing2"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for all missing keys")
	}
}

func TestYamlMultiset_JSONOutput(t *testing.T) {
	dir := createTempDir(t)
	configPath := createTestYAML(t, dir, `
helper:
  llm: gemini
`)

	cmd := newYamlCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"multiset", "--file", configPath, "key1", "val1", "key2", "val2", "--json"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	// Check status field exists
	if result["status"] == nil {
		t.Errorf("expected status field in JSON output, got: %v", result)
	}
}

func TestYamlList_JSONOutput(t *testing.T) {
	dir := createTempDir(t)
	configPath := createTestYAML(t, dir, `
helper:
  llm: gemini
`)

	cmd := newYamlCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"list", "--file", configPath, "--json"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
}

func TestYamlList_TopLevelPrefix(t *testing.T) {
	// Test listing with a prefix filter
	dir := createTempDir(t)
	configPath := createTestYAML(t, dir, `
helper:
  llm: gemini
  script: llm-support
project:
  type: go
`)

	cmd := newYamlCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"list", "--file", configPath, "helper", "--flat"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "helper.llm") {
		t.Error("should include helper.llm")
	}
	// Should not include project keys when filtering by helper
	if strings.Contains(output, "project") {
		t.Error("should not include project keys when filtering by helper")
	}
}

func TestYamlList_DefaultHierarchical(t *testing.T) {
	// Test default hierarchical output (without --flat)
	dir := createTempDir(t)
	configPath := createTestYAML(t, dir, `
helper:
  llm: gemini
project:
  type: go
`)

	cmd := newYamlCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"list", "--file", configPath})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "helper") {
		t.Error("should include 'helper'")
	}
	if !strings.Contains(output, "project") {
		t.Error("should include 'project'")
	}
}

func TestYamlDelete_NestedKey(t *testing.T) {
	// Test deleting a deeply nested key
	dir := createTempDir(t)
	configPath := createTestYAML(t, dir, `
deeply:
  nested:
    key: value
    other: keep
`)

	cmd := newYamlCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"delete", "--file", configPath, "deeply.nested.key"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	content, _ := os.ReadFile(configPath)
	if strings.Contains(string(content), "key: value") {
		t.Error("nested key was not deleted")
	}
	if !strings.Contains(string(content), "other: keep") {
		t.Error("sibling key was incorrectly deleted")
	}
}

func TestYamlDelete_JSONOutput(t *testing.T) {
	dir := createTempDir(t)
	configPath := createTestYAML(t, dir, `
helper:
  llm: gemini
  script: llm-support
`)

	cmd := newYamlCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"delete", "--file", configPath, "helper.script", "--json"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	if result["status"] != "deleted" {
		t.Errorf("expected status=deleted, got: %v", result["status"])
	}
}

func TestYamlValidate_JSONOutput(t *testing.T) {
	dir := createTempDir(t)
	configPath := createTestYAML(t, dir, `
helper:
  llm: gemini
`)

	cmd := newYamlCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"validate", "--file", configPath, "--json"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	if result["valid"] != true {
		t.Errorf("expected valid=true, got: %v", result["valid"])
	}
}

func TestYamlValidate_AllRequiredPresent(t *testing.T) {
	dir := createTempDir(t)
	configPath := createTestYAML(t, dir, `
helper:
  llm: gemini
  script: llm-support
`)

	cmd := newYamlCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"validate", "--file", configPath, "--required", "helper.llm,helper.script"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("expected no error when all required keys present, got: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "VALID: TRUE") {
		t.Errorf("expected VALID: TRUE, got: %s", output)
	}
}

func TestYamlPush_NewArray(t *testing.T) {
	dir := createTempDir(t)
	configPath := createTestYAML(t, dir, `
helper:
  llm: gemini
`)

	cmd := newYamlCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"push", "--file", configPath, "items", "first"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	content, _ := os.ReadFile(configPath)
	if !strings.Contains(string(content), "first") {
		t.Error("new array with element was not created")
	}
}

func TestYamlPush_JSONOutput(t *testing.T) {
	dir := createTempDir(t)
	configPath := createTestYAML(t, dir, `
items:
  - first
`)

	cmd := newYamlCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"push", "--file", configPath, "items", "second", "--json"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	if result["status"] != "pushed" {
		t.Errorf("expected status=pushed, got: %v", result["status"])
	}
}

func TestYamlPop_EmptyArray(t *testing.T) {
	dir := createTempDir(t)
	configPath := createTestYAML(t, dir, `
items: []
`)

	cmd := newYamlCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"pop", "--file", configPath, "items"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when popping from empty array")
	}
}

func TestYamlPop_NotAnArray(t *testing.T) {
	dir := createTempDir(t)
	configPath := createTestYAML(t, dir, `
helper:
  llm: gemini
`)

	cmd := newYamlCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"pop", "--file", configPath, "helper"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when popping from non-array")
	}
}

func TestYamlPop_JSONOutput(t *testing.T) {
	dir := createTempDir(t)
	configPath := createTestYAML(t, dir, `
items:
  - first
  - second
`)

	cmd := newYamlCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"pop", "--file", configPath, "items", "--json"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	if result["value"] != "second" {
		t.Errorf("expected value=second, got: %v", result["value"])
	}
}

func TestYamlGet_ObjectValue(t *testing.T) {
	// Test getting an object (should return formatted YAML)
	dir := createTempDir(t)
	configPath := createTestYAML(t, dir, `
helper:
  llm: gemini
  script: llm-support
`)

	cmd := newYamlCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"get", "--file", configPath, "helper"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "llm") {
		t.Errorf("expected object output to contain 'llm', got: %s", output)
	}
}

func TestYamlGet_ArrayValue(t *testing.T) {
	dir := createTempDir(t)
	configPath := createTestYAML(t, dir, `
items:
  - first
  - second
`)

	cmd := newYamlCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"get", "--file", configPath, "items"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "first") {
		t.Errorf("expected array output to contain 'first', got: %s", output)
	}
}

func TestYamlDelete_TopLevelKey(t *testing.T) {
	dir := createTempDir(t)
	configPath := createTestYAML(t, dir, `
helper:
  llm: gemini
project:
  type: go
`)

	cmd := newYamlCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"delete", "--file", configPath, "project"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	content, _ := os.ReadFile(configPath)
	if strings.Contains(string(content), "project:") {
		t.Error("top-level key was not deleted")
	}
	if !strings.Contains(string(content), "helper:") {
		t.Error("other keys were incorrectly deleted")
	}
}

func TestYamlList_EmptyFile(t *testing.T) {
	dir := createTempDir(t)
	configPath := createTestYAML(t, dir, ``)

	cmd := newYamlCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"list", "--file", configPath, "--flat"})

	err := cmd.Execute()
	// Should not error, just empty output
	if err != nil {
		t.Fatalf("expected no error for empty file, got: %v", err)
	}
}

func TestYamlValidate_EmptyFile(t *testing.T) {
	dir := createTempDir(t)
	configPath := createTestYAML(t, dir, ``)

	cmd := newYamlCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"validate", "--file", configPath})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("expected no error for empty YAML, got: %v", err)
	}
}

func TestYamlInit_InvalidTemplate(t *testing.T) {
	dir := createTempDir(t)
	configPath := filepath.Join(dir, "config.yaml")

	cmd := newYamlCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"init", "--file", configPath, "--template", "nonexistent_template"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for invalid template")
	}
}

// ============================================================================
// Helper Function Unit Tests
// ============================================================================

func TestGetValueAtPath_DeepNesting(t *testing.T) {
	dir := createTempDir(t)
	configPath := createTestYAML(t, dir, `
level1:
  level2:
    level3:
      level4:
        value: deep
`)

	cmd := newYamlCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"get", "--file", configPath, "level1.level2.level3.level4.value", "--min"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	output := strings.TrimSpace(buf.String())
	if output != "deep" {
		t.Errorf("expected 'deep', got: %s", output)
	}
}

func TestSetValueAtPath_DeepNesting(t *testing.T) {
	dir := createTempDir(t)
	configPath := createTestYAML(t, dir, `
existing: value
`)

	cmd := newYamlCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"set", "--file", configPath, "a.b.c.d.e", "deep_value"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	content, _ := os.ReadFile(configPath)
	if !strings.Contains(string(content), "deep_value") {
		t.Error("deep value was not set")
	}
}

func TestFlattenKeys_NestedArrays(t *testing.T) {
	dir := createTempDir(t)
	configPath := createTestYAML(t, dir, `
simple: value
nested:
  child1: val1
  child2: val2
deep:
  level1:
    level2: val3
`)

	cmd := newYamlCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"list", "--file", configPath, "--flat"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	output := buf.String()
	expectedKeys := []string{"simple", "nested.child1", "nested.child2", "deep.level1.level2"}
	for _, key := range expectedKeys {
		if !strings.Contains(output, key) {
			t.Errorf("expected key %s in output", key)
		}
	}
}

func TestFlattenKeysWithValues_AllTypes(t *testing.T) {
	dir := createTempDir(t)
	configPath := createTestYAML(t, dir, `
string_val: hello
int_val: 42
float_val: 3.14
bool_val: true
`)

	cmd := newYamlCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"list", "--file", configPath, "--flat", "--values"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "string_val=hello") {
		t.Error("string value not shown correctly")
	}
	if !strings.Contains(output, "int_val=42") {
		t.Error("int value not shown correctly")
	}
}

func TestDeleteValueAtPath_MultipleLevels(t *testing.T) {
	dir := createTempDir(t)
	configPath := createTestYAML(t, dir, `
level1:
  level2:
    target: delete_me
    keep: this
  sibling: value
`)

	cmd := newYamlCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"delete", "--file", configPath, "level1.level2.target"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	content, _ := os.ReadFile(configPath)
	contentStr := string(content)
	if strings.Contains(contentStr, "delete_me") {
		t.Error("target was not deleted")
	}
	if !strings.Contains(contentStr, "keep: this") {
		t.Error("sibling was incorrectly deleted")
	}
	if !strings.Contains(contentStr, "sibling: value") {
		t.Error("parent sibling was incorrectly deleted")
	}
}

func TestMultiget_PartialDefaults(t *testing.T) {
	// Test multiget where some keys exist and some use defaults
	dir := createTempDir(t)
	configPath := createTestYAML(t, dir, `
exists1: value1
exists2: value2
`)

	cmd := newYamlCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"multiget", "--file", configPath, "exists1", "missing1", "exists2",
		"--defaults", `{"missing1": "default1"}`, "--min"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(lines))
	}
	if lines[0] != "value1" {
		t.Errorf("first value should be 'value1', got: %s", lines[0])
	}
	if lines[1] != "default1" {
		t.Errorf("second value should be 'default1', got: %s", lines[1])
	}
	if lines[2] != "value2" {
		t.Errorf("third value should be 'value2', got: %s", lines[2])
	}
}

func TestYamlPush_ToNonExistent(t *testing.T) {
	// Push to a key that doesn't exist yet
	dir := createTempDir(t)
	configPath := createTestYAML(t, dir, `
other: value
`)

	cmd := newYamlCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"push", "--file", configPath, "newarray", "item1"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	content, _ := os.ReadFile(configPath)
	if !strings.Contains(string(content), "item1") {
		t.Error("new array item was not added")
	}
}

func TestYamlPush_NotAnArray(t *testing.T) {
	// Push to something that's not an array
	dir := createTempDir(t)
	configPath := createTestYAML(t, dir, `
scalar: value
`)

	cmd := newYamlCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"push", "--file", configPath, "scalar", "item"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when pushing to scalar")
	}
}

func TestYamlSet_NullValue(t *testing.T) {
	dir := createTempDir(t)
	configPath := createTestYAML(t, dir, `
key: oldvalue
`)

	cmd := newYamlCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"set", "--file", configPath, "key", "null"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	content, _ := os.ReadFile(configPath)
	if !strings.Contains(string(content), "null") {
		t.Error("null value was not set")
	}
}

func TestYamlValidate_MultipleRequiredMissing(t *testing.T) {
	dir := createTempDir(t)
	configPath := createTestYAML(t, dir, `
exists: value
`)

	cmd := newYamlCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"validate", "--file", configPath, "--required", "exists,missing1,missing2"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing required keys")
	}

	errStr := err.Error()
	if !strings.Contains(errStr, "missing1") || !strings.Contains(errStr, "missing2") {
		t.Errorf("error should mention missing keys, got: %v", err)
	}
}

func TestYamlList_MinOutput(t *testing.T) {
	dir := createTempDir(t)
	configPath := createTestYAML(t, dir, `
a: 1
b: 2
`)

	cmd := newYamlCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"list", "--file", configPath, "--flat", "--min"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	output := buf.String()
	// Should have minimal output with keys
	if !strings.Contains(output, "a") || !strings.Contains(output, "b") {
		t.Errorf("expected keys in output, got: %s", output)
	}
}

func TestYamlMultiset_CreateFile(t *testing.T) {
	dir := createTempDir(t)
	configPath := filepath.Join(dir, "newconfig.yaml")

	// Create empty file first
	os.WriteFile(configPath, []byte(""), 0644)

	cmd := newYamlCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"multiset", "--file", configPath, "key1", "val1", "key2", "val2"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	content, _ := os.ReadFile(configPath)
	contentStr := string(content)
	if !strings.Contains(contentStr, "val1") || !strings.Contains(contentStr, "val2") {
		t.Error("values were not set in new file")
	}
}

func TestYamlGet_EmptyString(t *testing.T) {
	dir := createTempDir(t)
	configPath := createTestYAML(t, dir, `
empty: ""
`)

	cmd := newYamlCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"get", "--file", configPath, "empty", "--min"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Empty string is valid output
	output := strings.TrimSpace(buf.String())
	if output != "" && output != `""` {
		t.Errorf("expected empty string, got: %q", output)
	}
}

func TestYamlValidate_WithNonRequiredKeys(t *testing.T) {
	// Validate a file that has extra keys beyond required
	dir := createTempDir(t)
	configPath := createTestYAML(t, dir, `
required1: value1
required2: value2
optional1: value3
optional2: value4
`)

	cmd := newYamlCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"validate", "--file", configPath, "--required", "required1,required2"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "VALID: TRUE") {
		t.Errorf("expected VALID: TRUE, got: %s", output)
	}
}
