package commands

import (
	"bytes"
	"strings"
	"testing"

	"github.com/samestrin/llm-tools/pkg/output"
)

// --axi is a PERSISTENT flag on the root, so every command accepts it. Six
// commands declare --json but print it by hand — json.MarshalIndent plus
// Fprintln, and context.go hand-builds the JSON with Fprintf — instead of
// building an output.Formatter. Those six accepted --axi and silently ignored
// it, emitting JSON.
//
// A flag that is accepted and ignored is worse than one that does not exist:
// the caller has no way to tell "this command has no AXI output" from "this
// command produced AXI output", and a consumer parsing TOON gets JSON with no
// error and exit 0. Same class of failure as a decoder that silently drops a
// column.
//
// Commands audited by grepping for a --json declaration with no output.New in
// the same file: epic_number, context, git_changes, plan_type, math, yaml.

func withAXI(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		output.SetDefaultAXI(false)
		GlobalAXIOutput = false
	})
	GlobalAXIOutput = true
	output.SetDefaultAXI(true)
}

func TestMathHonoursAXI(t *testing.T) {
	// math is the cheapest honest end-to-end case: one argument, no fixtures,
	// deterministic output.
	withAXI(t)
	cmd := newMathCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"2+2", "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	got := strings.TrimSpace(buf.String())
	if strings.HasPrefix(got, "{") {
		t.Errorf("--axi was accepted and ignored; JSON was emitted:\n%s", got)
	}
	if !strings.Contains(got, "result") {
		t.Errorf("output does not carry the result field:\n%s", got)
	}
}

func TestMathDefaultOutputIsUnchanged(t *testing.T) {
	// The other half: with the flag absent, both existing modes are untouched.
	cmd := newMathCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"2+2", "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.HasPrefix(strings.TrimSpace(buf.String()), "{") {
		t.Errorf("--json stopped emitting JSON:\n%s", buf.String())
	}
}
