package commands

import (
	"bytes"
	"testing"

	"github.com/spf13/cobra"
)

func TestSearchCmd_Help(t *testing.T) {
	cmd := searchCmd()

	if cmd.Use != "search <query>" {
		t.Errorf("Use = %q, want 'search <query>'", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("Short description should not be empty")
	}
}

func TestSearchCmd_Flags(t *testing.T) {
	cmd := searchCmd()

	// Check required flags exist
	flags := []struct {
		name     string
		shortcut string
	}{
		{"top", "n"},
		{"threshold", "t"},
		{"type", ""},
		{"path", "p"},
		{"json", ""},
		{"min", ""},
	}

	for _, f := range flags {
		flag := cmd.Flags().Lookup(f.name)
		if flag == nil {
			t.Errorf("Flag --%s not found", f.name)
			continue
		}
		if f.shortcut != "" {
			shortFlag := cmd.Flags().ShorthandLookup(f.shortcut)
			if shortFlag == nil {
				t.Errorf("Shorthand -%s not found for --%s", f.shortcut, f.name)
			}
		}
	}
}

func TestSearchCmd_RequiresQuery(t *testing.T) {
	cmd := searchCmd()

	// Should require at least 1 argument
	if cmd.Args == nil {
		t.Error("Args validator should be set")
	}

	// Test with no args - should fail
	err := cmd.Args(cmd, []string{})
	if err == nil {
		t.Error("Should require at least one argument")
	}

	// Test with args - should pass
	err = cmd.Args(cmd, []string{"test query"})
	if err != nil {
		t.Errorf("Should accept query argument: %v", err)
	}
}

func TestSearchCmd_DefaultValues(t *testing.T) {
	cmd := searchCmd()

	// Check default values
	topFlag := cmd.Flags().Lookup("top")
	if topFlag.DefValue != "10" {
		t.Errorf("Default top = %s, want 10", topFlag.DefValue)
	}

	thresholdFlag := cmd.Flags().Lookup("threshold")
	if thresholdFlag.DefValue != "0" {
		t.Errorf("Default threshold = %s, want 0", thresholdFlag.DefValue)
	}
}

func TestRootCmd_HasSearchCommand(t *testing.T) {
	root := RootCmd()

	var found bool
	for _, cmd := range root.Commands() {
		if cmd.Name() == "search" {
			found = true
			break
		}
	}

	if !found {
		t.Error("Root command should have 'search' subcommand")
	}
}

func TestRootCmd_GlobalFlags(t *testing.T) {
	root := RootCmd()

	globalFlags := []string{"api-url", "model", "api-key", "index-dir"}

	for _, name := range globalFlags {
		flag := root.PersistentFlags().Lookup(name)
		if flag == nil {
			t.Errorf("Global flag --%s not found", name)
		}
	}
}

func executeCommand(root *cobra.Command, args ...string) (string, error) {
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs(args)

	err := root.Execute()
	return buf.String(), err
}

func TestSearchCmd_HelpOutput(t *testing.T) {
	root := RootCmd()
	output, err := executeCommand(root, "search", "--help")

	if err != nil {
		t.Fatalf("Help command failed: %v", err)
	}

	// Check help contains expected content
	expectedStrings := []string{
		"search",
		"--top",
		"--threshold",
		"--json",
		"--min",
	}

	for _, s := range expectedStrings {
		if !bytes.Contains([]byte(output), []byte(s)) {
			t.Errorf("Help output should contain %q", s)
		}
	}
}
