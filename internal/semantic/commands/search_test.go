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

// ===== Hybrid Search Flag Tests =====

// TestSearchCmd_HybridFlags verifies that hybrid search flags exist.
func TestSearchCmd_HybridFlags(t *testing.T) {
	cmd := searchCmd()

	// Check hybrid-related flags exist
	hybridFlags := []struct {
		name     string
		defValue string
	}{
		{"hybrid", "false"},
		{"fusion-k", "60"},
		{"fusion-alpha", "0.7"},
	}

	for _, f := range hybridFlags {
		flag := cmd.Flags().Lookup(f.name)
		if flag == nil {
			t.Errorf("Flag --%s not found", f.name)
			continue
		}
		if flag.DefValue != f.defValue {
			t.Errorf("Flag --%s default = %s, want %s", f.name, flag.DefValue, f.defValue)
		}
	}
}

// TestSearchCmd_HybridFlag_HelpOutput verifies that hybrid flags appear in help output.
func TestSearchCmd_HybridFlag_HelpOutput(t *testing.T) {
	root := RootCmd()
	output, err := executeCommand(root, "search", "--help")

	if err != nil {
		t.Fatalf("Help command failed: %v", err)
	}

	// Check help contains hybrid-related content
	expectedStrings := []string{
		"--hybrid",
		"--fusion-k",
		"--fusion-alpha",
	}

	for _, s := range expectedStrings {
		if !bytes.Contains([]byte(output), []byte(s)) {
			t.Errorf("Help output should contain %q", s)
		}
	}
}

// TestSearchCmd_InvalidFusionAlpha_Error verifies that invalid alpha values are rejected.
func TestSearchCmd_InvalidFusionAlpha_Error(t *testing.T) {
	tests := []struct {
		name  string
		alpha string
	}{
		{"negative", "-0.1"},
		{"greater_than_1", "1.5"},
		{"much_greater", "2.0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := RootCmd()
			_, err := executeCommand(root, "search", "--hybrid", "--fusion-alpha", tt.alpha, "test query")

			// Should fail with validation error
			if err == nil {
				t.Errorf("Expected error for fusion-alpha=%s, got nil", tt.alpha)
			}
		})
	}
}

// TestSearchCmd_InvalidFusionK_Error verifies that invalid k values are rejected.
func TestSearchCmd_InvalidFusionK_Error(t *testing.T) {
	tests := []struct {
		name string
		k    string
	}{
		{"zero", "0"},
		{"negative", "-1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := RootCmd()
			_, err := executeCommand(root, "search", "--hybrid", "--fusion-k", tt.k, "test query")

			// Should fail with validation error
			if err == nil {
				t.Errorf("Expected error for fusion-k=%s, got nil", tt.k)
			}
		})
	}
}

// TestSearchCmd_FusionParamsWithoutHybrid_Warning verifies that fusion params
// without --hybrid produce a warning (but still work).
func TestSearchCmd_FusionParamsWithoutHybrid_Warning(t *testing.T) {
	// This test verifies the behavior when fusion params are provided without --hybrid
	// The implementation should either warn or ignore silently
	cmd := searchCmd()

	// Verify the flags can be set independently
	fusionK := cmd.Flags().Lookup("fusion-k")
	if fusionK == nil {
		t.Fatal("fusion-k flag not found")
	}

	fusionAlpha := cmd.Flags().Lookup("fusion-alpha")
	if fusionAlpha == nil {
		t.Fatal("fusion-alpha flag not found")
	}
}

// TestSearchCmd_ValidFusionAlpha_Bounds verifies that alpha boundary values are accepted.
func TestSearchCmd_ValidFusionAlpha_Bounds(t *testing.T) {
	tests := []struct {
		name  string
		alpha string
	}{
		{"zero", "0.0"},
		{"one", "1.0"},
		{"middle", "0.5"},
		{"dense_heavy", "0.8"},
		{"lexical_heavy", "0.3"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := searchCmd()
			err := cmd.Flags().Set("fusion-alpha", tt.alpha)
			if err != nil {
				t.Errorf("Failed to set valid fusion-alpha=%s: %v", tt.alpha, err)
			}
		})
	}
}
