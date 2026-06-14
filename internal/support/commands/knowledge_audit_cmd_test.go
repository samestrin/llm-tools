package commands

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// auditJSON shape for decoding command output in tests.
type auditJSON struct {
	Entries []struct {
		File     string   `json:"file"`
		Repaired bool     `json:"repaired"`
		Flags    []string `json:"flags"`
		Schema   struct {
			Conformant bool `json:"conformant"`
		} `json:"schema"`
	} `json:"entries"`
	Summary struct {
		Total         int `json:"total"`
		Nonconformant int `json:"nonconformant"`
		Repaired      int `json:"repaired"`
	} `json:"summary"`
}

func runAudit(t *testing.T, args ...string) auditJSON {
	t.Helper()
	cmd := newKnowledgeAuditCmd()
	var out, errb bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errb)
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute %v: %v\nstderr: %s", args, err, errb.String())
	}
	var res auditJSON
	if err := json.Unmarshal(out.Bytes(), &res); err != nil {
		t.Fatalf("decode: %v\nout: %s", err, out.String())
	}
	return res
}

func TestKnowledgeAuditCmd_ReportIsReadOnly(t *testing.T) {
	dir := t.TempDir()
	drifted := "---\ndate: 2026-01-02\ntype: knowledge\n---\n\n# T\n\n## Decision\nx\n"
	p := filepath.Join(dir, "a.md")
	if err := os.WriteFile(p, []byte(drifted), 0o644); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(p)

	res := runAudit(t, "--dir", dir, "--json")
	if res.Summary.Total != 1 {
		t.Errorf("total = %d, want 1", res.Summary.Total)
	}
	if res.Summary.Nonconformant != 1 {
		t.Errorf("nonconformant = %d, want 1", res.Summary.Nonconformant)
	}
	if res.Summary.Repaired != 0 {
		t.Errorf("report mode must repair 0, got %d", res.Summary.Repaired)
	}
	after, _ := os.ReadFile(p)
	if !bytes.Equal(before, after) {
		t.Error("report mode modified a file — must be read-only")
	}
}

func TestKnowledgeAuditCmd_RepairWritesAndIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	gitAt(t, dir, "2026-01-01T00:00:00", "init", "-q")
	drifted := "---\ndate: 2026-02-02\ntype: knowledge\n---\n\n# Heading\n\n## Decision\nbody text\n"
	p := filepath.Join(dir, "a.md")
	if err := os.WriteFile(p, []byte(drifted), 0o644); err != nil {
		t.Fatal(err)
	}
	gitAt(t, dir, "2026-02-02T12:00:00", "add", "a.md")
	gitAt(t, dir, "2026-02-02T12:00:00", "commit", "-q", "-m", "add")

	res := runAudit(t, "--dir", dir, "--repair-schema", "--json")
	if res.Summary.Repaired != 1 {
		t.Fatalf("repaired = %d, want 1", res.Summary.Repaired)
	}
	// Re-audit: now conformant, repair is a no-op.
	res2 := runAudit(t, "--dir", dir, "--repair-schema", "--json")
	if res2.Summary.Repaired != 0 {
		t.Errorf("second repair = %d, want 0 (idempotent)", res2.Summary.Repaired)
	}
	if res2.Entries[0].Schema.Conformant && len(res2.Entries[0].Flags) == 0 {
		// fine — fully clean
	}
	// Body preserved.
	content, _ := os.ReadFile(p)
	if !bytes.Contains(content, []byte("## Decision\nbody text\n")) {
		t.Errorf("body not preserved after repair:\n%s", content)
	}
	if bytes.Contains(content, []byte("date:")) || !bytes.Contains(content, []byte("created: 2026-02-02")) {
		t.Errorf("date not renamed to created:\n%s", content)
	}
}

func TestKnowledgeAuditCmd_MissingDir(t *testing.T) {
	cmd := newKnowledgeAuditCmd()
	var out, errb bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errb)
	cmd.SetArgs([]string{"--json"})
	if err := cmd.Execute(); err == nil {
		t.Error("expected error when --dir is missing")
	}
}

func TestKnowledgeAuditCmd_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	res := runAudit(t, "--dir", dir, "--json")
	if res.Summary.Total != 0 {
		t.Errorf("empty dir total = %d, want 0", res.Summary.Total)
	}
}
