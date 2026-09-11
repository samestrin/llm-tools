package commands

import (
	"testing"

	"github.com/samestrin/llm-tools/internal/semantic/config"
	"github.com/spf13/cobra"
)

// scopeTestCmd mirrors how index and index-update declare their scope flags,
// including the non-empty exclude default that makes "was it set?" impossible
// to answer by inspecting the value alone.
func scopeTestCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "scope-test"}
	var includes, excludes []string
	cmd.Flags().StringSliceVarP(&includes, "include", "i", nil, "")
	cmd.Flags().StringSliceVarP(&excludes, "exclude", "e", []string{"vendor", "node_modules", ".git"}, "")
	return cmd
}

func TestResolveScope_UsesConfigWhenFlagsUnset(t *testing.T) {
	defer ResetGlobalsForTesting()
	cleanup := SetConfigForTesting(&config.SemanticConfig{
		CodeInclude: "*.go",
		CodeExclude: "vendor,node_modules,dist,coverage",
	})
	defer cleanup()
	profile = "code"

	inc, exc := resolveScope(scopeTestCmd(), nil, []string{"vendor", "node_modules", ".git"})

	if len(inc) != 1 || inc[0] != "*.go" {
		t.Errorf("include = %v, want [*.go] from config", inc)
	}
	if len(exc) != 4 {
		t.Errorf("exclude = %v, want the 4 configured patterns, not the flag default", exc)
	}
}

func TestResolveScope_ExplicitFlagsWin(t *testing.T) {
	defer ResetGlobalsForTesting()
	cleanup := SetConfigForTesting(&config.SemanticConfig{
		CodeInclude: "*.go",
		CodeExclude: "vendor,dist",
	})
	defer cleanup()
	profile = "code"

	cmd := scopeTestCmd()
	if err := cmd.Flags().Set("include", "*.ts"); err != nil {
		t.Fatalf("Set include: %v", err)
	}
	if err := cmd.Flags().Set("exclude", "sandbox"); err != nil {
		t.Fatalf("Set exclude: %v", err)
	}

	inc, exc := resolveScope(cmd, []string{"*.ts"}, []string{"sandbox"})

	if len(inc) != 1 || inc[0] != "*.ts" {
		t.Errorf("include = %v, want the explicit flag [*.ts]", inc)
	}
	if len(exc) != 1 || exc[0] != "sandbox" {
		t.Errorf("exclude = %v, want the explicit flag [sandbox]", exc)
	}
}

// With no config loaded the flag values must pass through untouched, so the
// command keeps working outside a configured project.
func TestResolveScope_NoConfigKeepsFlagValues(t *testing.T) {
	defer ResetGlobalsForTesting()
	cleanup := SetConfigForTesting(nil)
	defer cleanup()
	profile = "code"

	defaults := []string{"vendor", "node_modules", ".git"}
	inc, exc := resolveScope(scopeTestCmd(), nil, defaults)

	if len(inc) != 0 {
		t.Errorf("include = %v, want empty", inc)
	}
	if len(exc) != len(defaults) {
		t.Errorf("exclude = %v, want the flag defaults %v", exc, defaults)
	}
}
