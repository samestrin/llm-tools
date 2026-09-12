package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/samestrin/llm-tools/pkg/output"
)

// Findings from the atcr reviewer panel on the --axi change
// (review 2026-09-12_feat-axi-output-mode, 13 of 14 agents reporting).

// --- HIGH (brad, otto): AXI mode leaks into later invocations

// runRoot drives the real command tree and returns what it wrote.
func runRoot(t *testing.T, args ...string) string {
	t.Helper()
	buf := new(bytes.Buffer)
	RootCmd.SetOut(buf)
	RootCmd.SetArgs(args)
	if err := RootCmd.Execute(); err != nil {
		t.Fatalf("%v: %v", args, err)
	}
	return buf.String()
}

func TestResetOutputModesClearsAXIBetweenInvocations(t *testing.T) {
	// The fix. Reproduced first, through RootCmd, which owns the persistent
	// --axi flag:
	//
	//   first  (--axi):  "expression: 2+2\nresult: 4\n"
	//   second (--json): "expression: 3+3\nresult: 6\n"   <- TOON, not JSON
	//
	// pflag leaves a bound variable UNTOUCHED when its flag is absent, so the
	// second invocation inherited the first one's mode. Making the publish
	// unconditional did not help, because the variable it publishes was already
	// stale — the reset has to happen before parse.
	t.Cleanup(func() {
		ResetOutputModes()
		RootCmd.SetOut(nil)
	})

	ResetOutputModes()
	first := runRoot(t, "math", "2+2", "--axi")
	if strings.HasPrefix(strings.TrimSpace(first), "{") {
		t.Fatalf("--axi did not take effect: %q", first)
	}

	ResetOutputModes()
	second := runRoot(t, "math", "3+3", "--json")
	if !strings.HasPrefix(strings.TrimSpace(second), "{") {
		t.Errorf("--axi leaked past ResetOutputModes:\nfirst  = %q\nsecond = %q", first, second)
	}
}

func TestWithoutTheResetTheSingletonRetainsItsMode(t *testing.T) {
	// KNOWN LIMITATION, pinned as it actually behaves rather than asserted
	// away. RootCmd is a package-level singleton, so a caller that drives
	// RootCmd.Execute() directly and skips ResetOutputModes still inherits the
	// previous invocation's mode.
	//
	// Execute() calls the reset, so no shipped binary can reach this, and both
	// MCP servers exec a fresh binary per tool call. The structural fix is a
	// RootCmd() constructor like llm-filesystem and llm-semantic use, which is
	// a refactor of every init() in this package and deliberately out of scope
	// here.
	//
	// If someone makes the singleton stateless, this test fails — and it should
	// then be tightened to demand the better behaviour, not deleted.
	t.Cleanup(func() {
		ResetOutputModes()
		RootCmd.SetOut(nil)
	})

	ResetOutputModes()
	_ = runRoot(t, "math", "2+2", "--axi")

	// No reset here, deliberately.
	second := runRoot(t, "math", "3+3", "--json")
	if strings.HasPrefix(strings.TrimSpace(second), "{") {
		t.Errorf("the singleton no longer retains its mode — good news; "+
			"tighten this test and drop ResetOutputModes' caveat. got %q", second)
	}
}

// --- MED (greta, dax): yaml paths that return before the AXI guard

func writeYAMLFixture(t *testing.T) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "c.yaml")
	if err := os.WriteFile(p, []byte("name: demo\nnested:\n  inner: value\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func runYAMLWithAXI(t *testing.T, args ...string) string {
	t.Helper()
	t.Cleanup(func() {
		GlobalAXIOutput = false
		output.SetDefaultAXI(false)
	})
	GlobalAXIOutput = true
	output.SetDefaultAXI(true)

	cmd := newYamlCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute %v: %v", args, err)
	}
	return buf.String()
}

// dryRunBanner is the first line the human preview prints, captured from the
// binary rather than guessed. An earlier version of these tests asserted on
// "DRY-RUN" with a hyphen; the real text has a space, so the assertion matched
// nothing and both tests passed while the bug was live. A test that cannot fail
// is worse than no test, because it is counted as coverage.
const dryRunBanner = "DRY RUN - No changes written"

func TestYAMLDryRunHonoursAXI(t *testing.T) {
	// `yaml set --dry-run` returns through outputDryRunPreview before reaching
	// the AXI guard, so it accepted --axi and printed its ordinary preview.
	// Verified through the binary: the output is byte-identical with and
	// without the flag.
	got := runYAMLWithAXI(t, "set", "--file", writeYAMLFixture(t), "name", "other", "--dry-run")
	if strings.Contains(got, dryRunBanner) {
		t.Errorf("--axi was ignored on the dry-run path:\n%s", got)
	}
	if !strings.Contains(got, "other") {
		t.Errorf("the new value did not survive:\n%s", got)
	}
}

func TestYAMLMultisetDryRunHonoursAXI(t *testing.T) {
	// Same shape, via outputMultiDryRunPreview.
	got := runYAMLWithAXI(t, "multiset", "--file", writeYAMLFixture(t), "name", "other", "--dry-run")
	if strings.Contains(got, dryRunBanner) {
		t.Errorf("--axi was ignored on the multiset dry-run path:\n%s", got)
	}
	if !strings.Contains(got, "other") {
		t.Errorf("the new value did not survive:\n%s", got)
	}
}

func TestYAMLListScalarPrefixHonoursAXI(t *testing.T) {
	// `yaml list PREFIX` where the prefix resolves to a SCALAR takes an early
	// return that prints `prefix=value` and never reaches the guard.
	got := runYAMLWithAXI(t, "list", "--file", writeYAMLFixture(t), "nested.inner")
	if strings.Contains(got, "nested.inner=value") {
		t.Errorf("--axi was ignored on the scalar-prefix early return:\n%s", got)
	}
	if !strings.Contains(got, "value") {
		t.Errorf("the value did not survive:\n%s", got)
	}
}
