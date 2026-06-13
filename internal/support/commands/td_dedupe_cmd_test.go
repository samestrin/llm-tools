package commands

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestTdDedupeCmd_JSON(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.txt")
	b := filepath.Join(dir, "b.txt")
	os.WriteFile(a, []byte("HIGH|auth.go:45|Missing validation|Add zod|security|15|e|bruce\n"), 0o644)
	os.WriteFile(b, []byte("MEDIUM|auth.go:47|No validation on id|Validate id|security|20|e|kai\n"), 0o644)

	cmd := newTdDedupeCmd()
	cmd.SetArgs([]string{"--streams", a + "," + b, "--source-tags", "claude,multi-agent", "--tolerance", "3", "--json"})
	var out, errb bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errb)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, errb.String())
	}
	var res DedupeResult
	if err := json.Unmarshal(out.Bytes(), &res); err != nil {
		t.Fatalf("json: %v\n%s", err, out.String())
	}
	if len(res.Merged) != 1 {
		t.Fatalf("merged = %d, want 1 cluster", len(res.Merged))
	}
	if !res.Merged[0].NeedsReview || res.Merged[0].Confidence != "HIGH" {
		t.Errorf("merged row = %+v, want needs_review + HIGH", res.Merged[0])
	}
}

func TestTdDedupeCmd_MissingStreams(t *testing.T) {
	cmd := newTdDedupeCmd()
	cmd.SetArgs([]string{"--json"})
	var out, errb bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errb)
	if err := cmd.Execute(); err == nil {
		t.Fatal("missing --streams must error")
	}
}

func TestTdDedupeCmd_UnreadableStream(t *testing.T) {
	cmd := newTdDedupeCmd()
	cmd.SetArgs([]string{"--streams", "/no/such/stream.txt"})
	var out, errb bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errb)
	if err := cmd.Execute(); err == nil {
		t.Fatal("unreadable stream must error")
	}
}
