package commands

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestMemoryCmd_HasSubcommands(t *testing.T) {
	cmd := memoryCmd()

	subcommands := cmd.Commands()
	if len(subcommands) < 4 {
		t.Errorf("memoryCmd() should have at least 4 subcommands, got %d", len(subcommands))
	}

	// Check expected subcommands exist
	expected := []string{"store", "search", "promote", "import", "list", "delete"}
	subMap := make(map[string]bool)
	for _, sc := range subcommands {
		subMap[sc.Use] = true
	}

	for _, e := range expected {
		found := false
		for name := range subMap {
			// Use can be "store" or "store <args>"
			if name == e || len(name) > len(e) && name[:len(e)] == e {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("memoryCmd() missing subcommand %q", e)
		}
	}
}

func TestMemoryStoreCmd_RequiredFlags(t *testing.T) {
	cmd := memoryStoreCmd()

	// Check that question and answer are required
	qFlag := cmd.Flags().Lookup("question")
	if qFlag == nil {
		t.Error("memoryStoreCmd() missing --question flag")
	}

	aFlag := cmd.Flags().Lookup("answer")
	if aFlag == nil {
		t.Error("memoryStoreCmd() missing --answer flag")
	}
}

func TestMemorySearchCmd_Flags(t *testing.T) {
	cmd := memorySearchCmd()

	// Check expected flags
	flags := []string{"top", "threshold", "tags", "status", "json", "min"}
	for _, f := range flags {
		if cmd.Flags().Lookup(f) == nil {
			t.Errorf("memorySearchCmd() missing --%s flag", f)
		}
	}
}

func TestMemoryPromoteCmd_RequiredFlags(t *testing.T) {
	cmd := memoryPromoteCmd()

	targetFlag := cmd.Flags().Lookup("target")
	if targetFlag == nil {
		t.Error("memoryPromoteCmd() missing --target flag")
	}

	sectionFlag := cmd.Flags().Lookup("section")
	if sectionFlag == nil {
		t.Error("memoryPromoteCmd() missing --section flag")
	}
}

func TestMemoryImportCmd_RequiredFlags(t *testing.T) {
	cmd := memoryImportCmd()

	sourceFlag := cmd.Flags().Lookup("source")
	if sourceFlag == nil {
		t.Error("memoryImportCmd() missing --source flag")
	}

	dryRunFlag := cmd.Flags().Lookup("dry-run")
	if dryRunFlag == nil {
		t.Error("memoryImportCmd() missing --dry-run flag")
	}
}

func TestMemoryListCmd_Flags(t *testing.T) {
	cmd := memoryListCmd()

	// Check expected flags
	flags := []string{"limit", "status", "json", "min"}
	for _, f := range flags {
		if cmd.Flags().Lookup(f) == nil {
			t.Errorf("memoryListCmd() missing --%s flag", f)
		}
	}
}

func TestMemoryDeleteCmd_Flags(t *testing.T) {
	cmd := memoryDeleteCmd()

	// Check expected flags
	flags := []string{"force", "json", "min"}
	for _, f := range flags {
		if cmd.Flags().Lookup(f) == nil {
			t.Errorf("memoryDeleteCmd() missing --%s flag", f)
		}
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		input    string
		maxLen   int
		expected string
	}{
		{"short", 10, "short"},
		{"this is a longer string", 10, "this is a ..."},
		{"exact", 5, "exact"},
		{"", 5, ""},
	}

	for _, tt := range tests {
		result := truncate(tt.input, tt.maxLen)
		if result != tt.expected {
			t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.maxLen, result, tt.expected)
		}
	}
}

func TestTruncateRuneAware(t *testing.T) {
	tests := []struct {
		input    string
		maxRunes int
		expected string
	}{
		{"short", 10, "short"},
		{"this is a longer string", 10, "this is a ..."},
		{"exact", 5, "exact"},
		{"", 5, ""},
		// UTF-8 multi-byte characters (maxRunes is length before adding "...")
		{"日本語テスト", 6, "日本語テスト"},
		{"日本語テスト", 5, "日本語テス..."},
		{"日本語テスト", 2, "日本..."},
		{"🎉🎊🎁", 3, "🎉🎊🎁"},
		{"🎉🎊🎁", 2, "🎉🎊..."},
		{"hello世界", 7, "hello世界"},
		{"hello世界", 5, "hello..."},
	}

	for _, tt := range tests {
		result := truncateRuneAware(tt.input, tt.maxRunes)
		if result != tt.expected {
			t.Errorf("truncateRuneAware(%q, %d) = %q, want %q", tt.input, tt.maxRunes, result, tt.expected)
		}
	}
}

// ===== MEMORY STATS COMMAND TESTS (Task 1.1) =====

func TestMemoryStatsCmd_HasRequiredFlags(t *testing.T) {
	cmd := memoryStatsCmd()

	// Verify all required flags exist
	requiredFlags := []string{"id", "min-retrievals", "history", "prune", "older-than", "yes"}
	for _, flag := range requiredFlags {
		if cmd.Flags().Lookup(flag) == nil {
			t.Errorf("memoryStatsCmd() missing --%s flag", flag)
		}
	}
}

func TestMemoryStatsCmd_RegisteredAsSubcommand(t *testing.T) {
	cmd := memoryCmd()

	// Check that "stats" is a registered subcommand
	var statsCmd *cobra.Command
	for _, subcommands := range cmd.Commands() {
		if subcommands.Use == "stats" {
			statsCmd = subcommands
			break
		}
	}

	if statsCmd == nil {
		t.Error("stats subcommand not registered to memoryCmd()")
	}
}

func TestMemoryStatsCmd_Flags(t *testing.T) {
	cmd := memoryStatsCmd()

	// Check flag types
	idFlag := cmd.Flags().Lookup("id")
	if idFlag == nil {
		t.Fatal("missing --id flag")
	}

	minRetrievalsFlag := cmd.Flags().Lookup("min-retrievals")
	if minRetrievalsFlag == nil {
		t.Fatal("missing --min-retrievals flag")
	}

	// Flags required-together should be validated
	cmd.MarkFlagsRequiredTogether("history", "id")
	cmd.MarkFlagsRequiredTogether("prune", "older-than")
}
