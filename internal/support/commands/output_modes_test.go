package commands

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestOutputModes_ContextMultiget tests all four output modes for context multiget
func TestOutputModes_ContextMultiget(t *testing.T) {
	// Setup: create temp dir and context
	tmpDir := t.TempDir()

	// Initialize context
	initCmd := newContextInitCmd()
	initCmd.SetArgs([]string{"--dir", tmpDir})
	if err := initCmd.Execute(); err != nil {
		t.Fatalf("Failed to init context: %v", err)
	}

	// Set some values
	setCmd := newContextSetCmd()
	setCmd.SetArgs([]string{"--dir", tmpDir, "KEY1", "value1"})
	if err := setCmd.Execute(); err != nil {
		t.Fatalf("Failed to set KEY1: %v", err)
	}
	setCmd = newContextSetCmd()
	setCmd.SetArgs([]string{"--dir", tmpDir, "KEY2", "value2"})
	if err := setCmd.Execute(); err != nil {
		t.Fatalf("Failed to set KEY2: %v", err)
	}

	tests := []struct {
		name     string
		args     []string
		validate func(t *testing.T, output string)
	}{
		{
			name: "default output",
			args: []string{"--dir", tmpDir, "KEY1", "KEY2"},
			validate: func(t *testing.T, output string) {
				if !strings.Contains(output, "KEY1=value1") {
					t.Errorf("Expected KEY1=value1, got: %s", output)
				}
				if !strings.Contains(output, "KEY2=value2") {
					t.Errorf("Expected KEY2=value2, got: %s", output)
				}
			},
		},
		{
			name: "--json output",
			args: []string{"--dir", tmpDir, "KEY1", "KEY2", "--json"},
			validate: func(t *testing.T, output string) {
				var result map[string]string
				if err := json.Unmarshal([]byte(output), &result); err != nil {
					t.Errorf("Expected valid JSON object, got: %s", output)
					return
				}
				if result["KEY1"] != "value1" || result["KEY2"] != "value2" {
					t.Errorf("Expected {KEY1:value1, KEY2:value2}, got: %v", result)
				}
			},
		},
		{
			name: "--min output",
			args: []string{"--dir", tmpDir, "KEY1", "KEY2", "--min"},
			validate: func(t *testing.T, output string) {
				lines := strings.Split(strings.TrimSpace(output), "\n")
				if len(lines) != 2 || lines[0] != "value1" || lines[1] != "value2" {
					t.Errorf("Expected value1\\nvalue2, got: %s", output)
				}
			},
		},
		{
			name: "--json --min output",
			args: []string{"--dir", tmpDir, "KEY1", "KEY2", "--json", "--min"},
			validate: func(t *testing.T, output string) {
				var result []string
				if err := json.Unmarshal([]byte(output), &result); err != nil {
					t.Errorf("Expected valid JSON array, got: %s", output)
					return
				}
				if len(result) != 2 || result[0] != "value1" || result[1] != "value2" {
					t.Errorf("Expected [value1, value2], got: %v", result)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			cmd := newContextMultiGetCmd()
			cmd.SetOut(&buf)
			cmd.SetArgs(tt.args)

			if err := cmd.Execute(); err != nil {
				t.Fatalf("Command failed: %v", err)
			}

			tt.validate(t, buf.String())
		})
	}
}

// TestOutputModes_ContextGet tests all four output modes for context get
func TestOutputModes_ContextGet(t *testing.T) {
	tmpDir := t.TempDir()

	// Initialize and set a value
	initCmd := newContextInitCmd()
	initCmd.SetArgs([]string{"--dir", tmpDir})
	initCmd.Execute()

	setCmd := newContextSetCmd()
	setCmd.SetArgs([]string{"--dir", tmpDir, "TESTKEY", "testvalue"})
	setCmd.Execute()

	tests := []struct {
		name     string
		args     []string
		validate func(t *testing.T, output string)
	}{
		{
			name: "default output",
			args: []string{"--dir", tmpDir, "TESTKEY"},
			validate: func(t *testing.T, output string) {
				if !strings.Contains(output, "TESTKEY: testvalue") {
					t.Errorf("Expected 'TESTKEY: testvalue', got: %s", output)
				}
			},
		},
		{
			name: "--json output",
			args: []string{"--dir", tmpDir, "TESTKEY", "--json"},
			validate: func(t *testing.T, output string) {
				var result map[string]string
				if err := json.Unmarshal([]byte(output), &result); err != nil {
					t.Errorf("Expected valid JSON, got: %s", output)
					return
				}
				if result["key"] != "TESTKEY" || result["value"] != "testvalue" {
					t.Errorf("Expected {key:TESTKEY, value:testvalue}, got: %v", result)
				}
			},
		},
		{
			name: "--min output",
			args: []string{"--dir", tmpDir, "TESTKEY", "--min"},
			validate: func(t *testing.T, output string) {
				if strings.TrimSpace(output) != "testvalue" {
					t.Errorf("Expected 'testvalue', got: %s", output)
				}
			},
		},
		{
			name: "--json --min output",
			args: []string{"--dir", tmpDir, "TESTKEY", "--json", "--min"},
			validate: func(t *testing.T, output string) {
				var result string
				if err := json.Unmarshal([]byte(output), &result); err != nil {
					t.Errorf("Expected valid JSON string, got: %s", output)
					return
				}
				if result != "testvalue" {
					t.Errorf("Expected \"testvalue\", got: %s", result)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			cmd := newContextGetCmd()
			cmd.SetOut(&buf)
			cmd.SetArgs(tt.args)

			if err := cmd.Execute(); err != nil {
				t.Fatalf("Command failed: %v", err)
			}

			tt.validate(t, buf.String())
		})
	}
}

// TestOutputModes_YamlGet tests all four output modes for yaml get
func TestOutputModes_YamlGet(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")

	// Create a test config file
	content := `
helper:
  llm: gemini
  max_lines: 2500
`
	if err := os.WriteFile(configFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create config file: %v", err)
	}

	tests := []struct {
		name     string
		args     []string
		validate func(t *testing.T, output string)
	}{
		{
			name: "default output",
			args: []string{"--file", configFile, "helper.llm"},
			validate: func(t *testing.T, output string) {
				if !strings.Contains(output, "helper.llm: gemini") {
					t.Errorf("Expected 'helper.llm: gemini', got: %s", output)
				}
			},
		},
		{
			name: "--json output",
			args: []string{"--file", configFile, "helper.llm", "--json"},
			validate: func(t *testing.T, output string) {
				var result map[string]interface{}
				if err := json.Unmarshal([]byte(output), &result); err != nil {
					t.Errorf("Expected valid JSON, got: %s", output)
					return
				}
				if result["key"] != "helper.llm" || result["value"] != "gemini" {
					t.Errorf("Expected {key:helper.llm, value:gemini}, got: %v", result)
				}
			},
		},
		{
			name: "--min output",
			args: []string{"--file", configFile, "helper.llm", "--min"},
			validate: func(t *testing.T, output string) {
				if strings.TrimSpace(output) != "gemini" {
					t.Errorf("Expected 'gemini', got: %s", output)
				}
			},
		},
		{
			name: "--json --min output",
			args: []string{"--file", configFile, "helper.llm", "--json", "--min"},
			validate: func(t *testing.T, output string) {
				var result string
				if err := json.Unmarshal([]byte(output), &result); err != nil {
					t.Errorf("Expected valid JSON string, got: %s", output)
					return
				}
				if result != "gemini" {
					t.Errorf("Expected \"gemini\", got: %s", result)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			cmd := newYamlGetCmd()
			cmd.SetOut(&buf)
			cmd.SetArgs(tt.args)

			if err := cmd.Execute(); err != nil {
				t.Fatalf("Command failed: %v", err)
			}

			tt.validate(t, buf.String())
		})
	}
}

// TestOutputModes_YamlMultiget tests all four output modes for yaml multiget
func TestOutputModes_YamlMultiget(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")

	// Create a test config file
	content := `
helper:
  llm: gemini
  max_lines: 2500
project:
  type: golang
`
	if err := os.WriteFile(configFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create config file: %v", err)
	}

	tests := []struct {
		name     string
		args     []string
		validate func(t *testing.T, output string)
	}{
		{
			name: "default output",
			args: []string{"--file", configFile, "helper.llm", "project.type"},
			validate: func(t *testing.T, output string) {
				if !strings.Contains(output, "helper.llm=gemini") {
					t.Errorf("Expected 'helper.llm=gemini', got: %s", output)
				}
				if !strings.Contains(output, "project.type=golang") {
					t.Errorf("Expected 'project.type=golang', got: %s", output)
				}
			},
		},
		{
			name: "--json output",
			args: []string{"--file", configFile, "helper.llm", "project.type", "--json"},
			validate: func(t *testing.T, output string) {
				var result map[string]string
				if err := json.Unmarshal([]byte(output), &result); err != nil {
					t.Errorf("Expected valid JSON object, got: %s", output)
					return
				}
				if result["helper.llm"] != "gemini" || result["project.type"] != "golang" {
					t.Errorf("Expected correct values, got: %v", result)
				}
			},
		},
		{
			name: "--min output",
			args: []string{"--file", configFile, "helper.llm", "project.type", "--min"},
			validate: func(t *testing.T, output string) {
				lines := strings.Split(strings.TrimSpace(output), "\n")
				if len(lines) != 2 || lines[0] != "gemini" || lines[1] != "golang" {
					t.Errorf("Expected gemini\\ngolang, got: %s", output)
				}
			},
		},
		{
			name: "--json --min output",
			args: []string{"--file", configFile, "helper.llm", "project.type", "--json", "--min"},
			validate: func(t *testing.T, output string) {
				var result []string
				if err := json.Unmarshal([]byte(output), &result); err != nil {
					t.Errorf("Expected valid JSON array, got: %s", output)
					return
				}
				if len(result) != 2 || result[0] != "gemini" || result[1] != "golang" {
					t.Errorf("Expected [gemini, golang], got: %v", result)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			cmd := newYamlMultigetCmd()
			cmd.SetOut(&buf)
			cmd.SetArgs(tt.args)

			if err := cmd.Execute(); err != nil {
				t.Fatalf("Command failed: %v", err)
			}

			tt.validate(t, buf.String())
		})
	}
}
