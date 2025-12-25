package commands

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// newTestRootCmd creates a fresh root command for testing
func newTestRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "llm-clarification",
		Short:   "LLM Clarification Learning System",
		Long:    `A CLI tool for tracking and managing clarifications gathered during LLM-assisted development.`,
		Version: Version,
	}
	return cmd
}

func TestRootCommandCreation(t *testing.T) {
	if rootCmd == nil {
		t.Fatal("rootCmd is nil")
	}

	if rootCmd.Use != "llm-clarification" {
		t.Errorf("expected Use 'llm-clarification', got %s", rootCmd.Use)
	}

	if rootCmd.Short == "" {
		t.Error("rootCmd.Short should not be empty")
	}

	if rootCmd.Long == "" {
		t.Error("rootCmd.Long should not be empty")
	}
}

func TestVersionFlag(t *testing.T) {
	// Test that version flag is set
	if rootCmd.Version == "" {
		t.Error("rootCmd.Version should not be empty")
	}
}

func TestHelpOutput(t *testing.T) {
	cmd := newTestRootCmd()

	// Verify command metadata is set correctly
	if cmd.Use != "llm-clarification" {
		t.Errorf("Use should be 'llm-clarification', got: %s", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("Short description should not be empty")
	}

	if !strings.Contains(cmd.Long, "clarification") {
		t.Error("Long description should mention 'clarification'")
	}

	// Verify UsageString is not empty
	usageStr := cmd.UsageString()
	if usageStr == "" {
		t.Error("UsageString should not be empty")
	}
}

func TestVersionOutput(t *testing.T) {
	cmd := newTestRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--version"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}

	output := buf.String()

	// Verify version output contains version info
	if !strings.Contains(output, "llm-clarification") {
		t.Errorf("Version output should contain 'llm-clarification', got: %s", output)
	}
}

func TestExecuteNoArgs(t *testing.T) {
	cmd := newTestRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("Execute() with no args returned error: %v", err)
	}
}

func TestInvalidFlag(t *testing.T) {
	cmd := newTestRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--invalid-flag"})

	err := cmd.Execute()
	if err == nil {
		t.Error("Execute() with invalid flag should return error")
	}
}
