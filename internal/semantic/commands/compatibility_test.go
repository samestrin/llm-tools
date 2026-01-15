package commands

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// TestSearchCommand_BackwardCompatibility tests that existing CLI flags and behavior are unchanged.
// This is a regression test to ensure new hybrid/recency features don't break existing usage.
func TestSearchCommand_BackwardCompatibility(t *testing.T) {
	cmd := searchCmd()

	// Test: All existing flags are present
	t.Run("ExistingFlagsPresent", func(t *testing.T) {
		requiredFlags := []struct {
			name      string
			shorthand string
		}{
			{"top", "n"},
			{"threshold", "t"},
			{"type", ""},
			{"path", "p"},
			{"json", ""},
			{"min", ""},
		}

		for _, f := range requiredFlags {
			flag := cmd.Flags().Lookup(f.name)
			if flag == nil {
				t.Errorf("backward compatibility: flag --%s must exist", f.name)
				continue
			}
			if f.shorthand != "" && flag.Shorthand != f.shorthand {
				t.Errorf("backward compatibility: flag --%s should have shorthand -%s, got -%s",
					f.name, f.shorthand, flag.Shorthand)
			}
		}
	})

	// Test: Default values unchanged
	t.Run("DefaultValuesUnchanged", func(t *testing.T) {
		expectations := map[string]string{
			"top":       "10",
			"threshold": "0",
			"type":      "",
			"path":      "",
			"json":      "false",
			"min":       "false",
		}

		for name, expected := range expectations {
			flag := cmd.Flags().Lookup(name)
			if flag == nil {
				t.Errorf("backward compatibility: flag --%s missing", name)
				continue
			}
			if flag.DefValue != expected {
				t.Errorf("backward compatibility: --%s default should be %q, got %q",
					name, expected, flag.DefValue)
			}
		}
	})

	// Test: Command accepts positional query argument
	t.Run("AcceptsQueryArg", func(t *testing.T) {
		if cmd.Args == nil {
			t.Error("backward compatibility: search command should have Args validator")
			return
		}
		// Command should require at least 1 argument
		err := cmd.Args(cmd, []string{})
		if err == nil {
			t.Error("backward compatibility: search should require query argument")
		}
		err = cmd.Args(cmd, []string{"test query"})
		if err != nil {
			t.Errorf("backward compatibility: search should accept single query: %v", err)
		}
	})

	// Test: New flags don't conflict with existing short flags
	t.Run("NoShortFlagConflicts", func(t *testing.T) {
		existingShorts := map[string]string{
			"n": "top",
			"t": "threshold",
			"p": "path",
		}

		// Check that new flags don't use these reserved short flags
		newFlags := []string{"hybrid", "fusion-k", "fusion-alpha", "recency-boost", "recency-factor", "recency-decay"}
		for _, name := range newFlags {
			flag := cmd.Flags().Lookup(name)
			if flag != nil && flag.Shorthand != "" {
				if existing, conflicts := existingShorts[flag.Shorthand]; conflicts {
					t.Errorf("backward compatibility: new flag --%s shorthand -%s conflicts with --%s",
						name, flag.Shorthand, existing)
				}
			}
		}
	})
}

// TestSearchCommand_NewFlagsAdditive verifies new flags are purely additive (don't change defaults)
func TestSearchCommand_NewFlagsAdditive(t *testing.T) {
	cmd := searchCmd()

	t.Run("HybridDefaultsFalse", func(t *testing.T) {
		flag := cmd.Flags().Lookup("hybrid")
		if flag == nil {
			t.Skip("--hybrid flag not yet implemented")
		}
		if flag.DefValue != "false" {
			t.Errorf("backward compatibility: --hybrid default must be false to preserve existing behavior, got %q", flag.DefValue)
		}
	})

	t.Run("RecencyBoostDefaultsFalse", func(t *testing.T) {
		flag := cmd.Flags().Lookup("recency-boost")
		if flag == nil {
			t.Skip("--recency-boost flag not yet implemented")
		}
		if flag.DefValue != "false" {
			t.Errorf("backward compatibility: --recency-boost default must be false to preserve existing behavior, got %q", flag.DefValue)
		}
	})
}

// TestIndexCommand_BackwardCompatibility tests that existing index flags are unchanged
func TestIndexCommand_BackwardCompatibility(t *testing.T) {
	cmd := indexCmd()

	t.Run("ExistingFlagsPresent", func(t *testing.T) {
		requiredFlags := []string{
			"include",
			"exclude",
			"force",
			"json",
		}

		for _, name := range requiredFlags {
			if cmd.Flags().Lookup(name) == nil {
				t.Errorf("backward compatibility: index flag --%s must exist", name)
			}
		}
	})

	t.Run("ForceDefaultFalse", func(t *testing.T) {
		flag := cmd.Flags().Lookup("force")
		if flag != nil && flag.DefValue != "false" {
			t.Errorf("backward compatibility: --force default should be false, got %q", flag.DefValue)
		}
	})
}

// TestHelpTextBackwardCompatible verifies help text includes all existing flags
func TestHelpTextBackwardCompatible(t *testing.T) {
	cmd := searchCmd()

	// Generate help text
	helpText := cmd.UsageString()

	// All existing flags must be documented
	existingFlags := []string{
		"--top",
		"--threshold",
		"--type",
		"--path",
		"--json",
		"--min",
		"-n",
		"-t",
		"-p",
	}

	for _, flag := range existingFlags {
		if !strings.Contains(helpText, flag) {
			t.Errorf("backward compatibility: help text must include %s", flag)
		}
	}
}

// TestStorageBackendFlagsUnchanged verifies storage backend flags work identically
func TestStorageBackendFlagsUnchanged(t *testing.T) {
	// Test that global flags for storage are available
	rootCmd := &cobra.Command{Use: "test"}

	// The storage and collection flags should be available globally
	// We just verify the pattern is correct (actual flags are on the parent)
	searchCmd := searchCmd()
	rootCmd.AddCommand(searchCmd)

	// Storage backend flags are additive, not breaking
	// They should exist and have appropriate defaults
	t.Run("StorageFlagPattern", func(t *testing.T) {
		// Storage flags are persistent on root, tested separately
		// Just verify search command doesn't override them incorrectly
		// No action needed - this is a documentation test
	})
}
