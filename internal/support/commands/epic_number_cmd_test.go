package commands

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func runEpicNumberCmd(t *testing.T, args ...string) (string, error) {
	t.Helper()
	// Package-level flag vars are shared across cobra command instances, so
	// reset them per run or a previous test's --dir leaks into this one.
	epicNumberDirs, epicNumberParent, epicNumberJSON = nil, "", false
	cmd := newEpicNumberCmd()
	cmd.SetArgs(args)
	var out, errb bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errb)
	err := cmd.Execute()
	return out.String(), err
}

func epicFixture(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	active := filepath.Join(root, "active")
	completed := filepath.Join(root, "completed")
	for _, d := range []string{active, completed} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for p, names := range map[string][]string{
		active:    {"3.0_a.md", "3.1_b.md"},
		completed: {"4.0_shipped.md"},
	} {
		for _, n := range names {
			if err := os.WriteFile(filepath.Join(p, n), []byte("x"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	return active, completed
}

func TestEpicNumberCmd_JSON(t *testing.T) {
	active, completed := epicFixture(t)
	out, err := runEpicNumberCmd(t, "--dir", active, "--dir", completed, "--json")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	var res EpicNumberResult
	if jerr := json.Unmarshal([]byte(out), &res); jerr != nil {
		t.Fatalf("output is not valid JSON: %v\nout=%s", jerr, out)
	}
	if res.Next != "5.0" {
		t.Errorf("next = %q, want 5.0 (4.0 is completed and spent)", res.Next)
	}
	if res.InUse != 3 {
		t.Errorf("in_use = %d, want 3", res.InUse)
	}
}

func TestEpicNumberCmd_TextOutput(t *testing.T) {
	active, completed := epicFixture(t)
	out, err := runEpicNumberCmd(t, "--dir", active, "--dir", completed, "--parent", "3")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(out, "NEXT: 3.2") {
		t.Errorf("expected NEXT: 3.2 in output, got: %s", out)
	}
}

// Forgetting --dir must fail loudly. Defaulting to a guessed directory would
// silently scan the wrong place and hand out a number already in use.
func TestEpicNumberCmd_RequiresDir(t *testing.T) {
	if _, err := runEpicNumberCmd(t, "--parent", "3"); err == nil {
		t.Fatal("missing --dir should be an error, not a silent default")
	}
}

func TestEpicNumberCmd_RejectsBadParent(t *testing.T) {
	active, _ := epicFixture(t)
	if _, err := runEpicNumberCmd(t, "--dir", active, "--parent", "not-a-number"); err == nil {
		t.Error("non-numeric --parent should be an error")
	}
	// Depth is uncapped by design: atcr carries a 35.16.6.N family, and a cap
	// here forced its follow-on work to the back of the queue. Only the
	// components must be numeric.
	if _, err := runEpicNumberCmd(t, "--dir", active, "--parent", "1.2.3.4"); err != nil {
		t.Errorf("four-level --parent must be accepted: %v", err)
	}
	if _, err := runEpicNumberCmd(t, "--dir", active, "--parent", "1.2.x"); err == nil {
		t.Error("a non-numeric level should still be an error")
	}
}
