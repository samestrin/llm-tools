package commands

import (
	"testing"
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
