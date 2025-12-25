package commands

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// newTestRootCmd creates a fresh copy of the root command for testing
func newTestRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     RootCmd.Use,
		Short:   RootCmd.Short,
		Long:    RootCmd.Long,
		Version: RootCmd.Version,
	}
	cmd.PersistentFlags().BoolP("verbose", "v", false, "Enable verbose output")
	cmd.PersistentFlags().String("format", "text", "Output format: text, json")
	cmd.PersistentFlags().Bool("no-gitignore", false, "Disable .gitignore filtering")
	return cmd
}

// TestRootCommandExists verifies the root command is defined
func TestRootCommandExists(t *testing.T) {
	if RootCmd == nil {
		t.Fatal("RootCmd should be defined")
	}
}

// TestRootCommandUse verifies the root command has correct Use field
func TestRootCommandUse(t *testing.T) {
	if RootCmd.Use != "llm-support" {
		t.Errorf("RootCmd.Use = %q, want %q", RootCmd.Use, "llm-support")
	}
}

// TestRootCommandHasVersion verifies the root command has a version set
func TestRootCommandHasVersion(t *testing.T) {
	if RootCmd.Version == "" {
		t.Error("RootCmd.Version should be set")
	}
}

// TestRootCommandShortDescription verifies short description exists
func TestRootCommandShortDescription(t *testing.T) {
	if RootCmd.Short == "" {
		t.Error("RootCmd.Short should not be empty")
	}
}

// TestRootCommandLongDescription verifies long description exists
func TestRootCommandLongDescription(t *testing.T) {
	if RootCmd.Long == "" {
		t.Error("RootCmd.Long should not be empty")
	}
}

// TestRootCommandHelpFlag tests that --help flag works
func TestRootCommandHelpFlag(t *testing.T) {
	cmd := newTestRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--help"})

	err := cmd.Execute()
	if err != nil {
		t.Errorf("--help should not return error, got: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "llm-support") {
		t.Error("Help output should contain 'llm-support'")
	}
}

// TestRootCommandVersionFlag tests that --version flag works
func TestRootCommandVersionFlag(t *testing.T) {
	cmd := newTestRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--version"})

	err := cmd.Execute()
	if err != nil {
		t.Errorf("--version should not return error, got: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "llm-support version") {
		t.Errorf("Version output should contain 'llm-support version', got: %s", output)
	}
}

// TestRootCommandPersistentFlags verifies persistent flags are registered
func TestRootCommandPersistentFlags(t *testing.T) {
	flags := RootCmd.PersistentFlags()

	// Test verbose flag exists
	if flags.Lookup("verbose") == nil {
		t.Error("--verbose flag should be registered")
	}

	// Test format flag exists
	if flags.Lookup("format") == nil {
		t.Error("--format flag should be registered")
	}

	// Test no-gitignore flag exists
	if flags.Lookup("no-gitignore") == nil {
		t.Error("--no-gitignore flag should be registered")
	}
}

// TestRootCommandVerboseShortFlag tests that -v shorthand works for verbose
func TestRootCommandVerboseShortFlag(t *testing.T) {
	flags := RootCmd.PersistentFlags()
	verboseFlag := flags.ShorthandLookup("v")
	if verboseFlag == nil {
		t.Error("-v shorthand should be registered for verbose flag")
	}
}

// TestVersionFollowsSemver verifies version string follows semver format
func TestVersionFollowsSemver(t *testing.T) {
	// Version should contain dots (semver style)
	if !strings.Contains(RootCmd.Version, ".") {
		t.Errorf("Version %q should follow semver format with dots", RootCmd.Version)
	}
}
