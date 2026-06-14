package commands

import (
	"reflect"
	"sort"
	"strings"
	"testing"
)

// A fully canonical entry (atcr shape) is conformant and repair is a no-op.
const canonicalEntry = `---
id: mem-2026-06-11-7dae42
question: "Where should registry live?"
created: 2026-06-11
last_retrieved: ""
sprints: []
files:
- internal/registry/config.go
tags:
- architecture
retrievals: 0
status: active
type: clarifications
---

# Where should registry live

## Decision
Use a dedicated registry.yaml.
`

func TestParseEntry_CanonicalIsConformant(t *testing.T) {
	e := parseEntry("clarifications-1.3-Q1.md", []byte(canonicalEntry))
	if !e.HasFrontmatter || e.ParseErr != "" {
		t.Fatalf("parse failed: hasFM=%v err=%q", e.HasFrontmatter, e.ParseErr)
	}
	if e.Title != "Where should registry live" {
		t.Errorf("title = %q", e.Title)
	}
	r := e.schemaReport()
	if !r.Conformant {
		t.Errorf("expected conformant; missing=%v unknown=%v aliased=%v", r.Missing, r.Unknown, r.Aliased)
	}
	if len(r.Missing) != 0 || len(r.Unknown) != 0 {
		t.Errorf("missing=%v unknown=%v, want empty", r.Missing, r.Unknown)
	}
}

func TestRepair_CanonicalIsNoOp(t *testing.T) {
	e := parseEntry("clarifications-1.3-Q1.md", []byte(canonicalEntry))
	out, changed := e.repair("2026-06-11")
	if changed {
		t.Errorf("canonical entry should not change on repair; got:\n%s", out)
	}
}

// `date` alias is renamed to `created`; body is preserved byte-for-byte.
func TestRepair_DateAliasRenamed(t *testing.T) {
	src := `---
id: mem-2026-01-02-aaa111
question: "Q?"
date: 2026-01-02
status: active
type: knowledge
---

# Title here

## Decision
Body content that must survive verbatim.
`
	e := parseEntry("x.md", []byte(src))
	out, changed := e.repair("2026-05-05")
	if !changed {
		t.Fatal("expected change (date->created, missing keys filled)")
	}
	re := parseEntry("x.md", out)
	got := frontMap(re)
	if got["created"] != "2026-01-02" {
		t.Errorf("created = %v, want 2026-01-02 (from date alias, NOT git date)", got["created"])
	}
	if _, hasDate := got["date"]; hasDate {
		t.Errorf("alias key 'date' should be gone")
	}
	// Body must be preserved exactly.
	wantBody := "\n# Title here\n\n## Decision\nBody content that must survive verbatim.\n"
	if !strings.HasSuffix(string(out), wantBody) {
		t.Errorf("body not preserved.\n got tail=%q", string(out)[len(out)-len(wantBody):])
	}
}

// Missing created is filled from the git date; missing files derived from body
// code citations; bookkeeping defaults applied; id preserved.
func TestRepair_FillsFromGitAndBody(t *testing.T) {
	src := `---
id: mem-2026-03-03-keepme
question: "Q?"
type: knowledge
---

# Heading

## Code Reference
- internal/registry/config.go
- pkg/output/formatter.go
`
	e := parseEntry("x.md", []byte(src))
	out, changed := e.repair("2026-03-03")
	if !changed {
		t.Fatal("expected change")
	}
	got := frontMap(parseEntry("x.md", out))
	if got["id"] != "mem-2026-03-03-keepme" {
		t.Errorf("id not preserved: %v", got["id"])
	}
	if got["created"] != "2026-03-03" {
		t.Errorf("created = %v, want git date 2026-03-03", got["created"])
	}
	if got["status"] != "active" || got["retrievals"] != uint64(0) && got["retrievals"] != 0 {
		t.Errorf("defaults wrong: status=%v retrievals=%v", got["status"], got["retrievals"])
	}
	files := frontList(parseEntry("x.md", out), "files")
	sort.Strings(files)
	want := []string{"internal/registry/config.go", "pkg/output/formatter.go"}
	if !reflect.DeepEqual(files, want) {
		t.Errorf("derived files = %v, want %v", files, want)
	}
}

// id is synthesized only when absent; type inferred from filename prefix.
func TestRepair_SynthesizesIdAndInfersType(t *testing.T) {
	src := `---
question: "Q?"
created: 2026-04-04
---

# Some Title
`
	e := parseEntry("clarifications-2.0_foo-Q3.md", []byte(src))
	out, _ := e.repair("2026-04-04")
	got := frontMap(parseEntry("x.md", out))
	id, _ := got["id"].(string)
	if !strings.HasPrefix(id, "mem-2026-04-04-") || len(id) != len("mem-2026-04-04-")+6 {
		t.Errorf("synthesized id malformed: %q", id)
	}
	if got["type"] != "clarifications" {
		t.Errorf("type = %v, want clarifications (from filename prefix)", got["type"])
	}
}

// Soft fields (question, tags) are NEVER invented; they are reported needs_input.
func TestSchema_SoftFieldsAreNeedsInput(t *testing.T) {
	src := `---
id: mem-2026-04-04-zzz999
created: 2026-04-04
status: active
type: knowledge
---

# Title
`
	e := parseEntry("x.md", []byte(src))
	r := e.schemaReport()
	if !contains(r.NeedsInput, "question") || !contains(r.NeedsInput, "tags") {
		t.Errorf("needs_input = %v, want question+tags", r.NeedsInput)
	}
	out, _ := e.repair("2026-04-04")
	got := frontMap(parseEntry("x.md", out))
	if q, ok := got["question"].(string); ok && q != "" {
		t.Errorf("question should be left empty, got %q", q)
	}
}

// Idempotence: repairing twice yields no further change.
func TestRepair_Idempotent(t *testing.T) {
	src := `---
date: 2026-01-02
type: knowledge
---

# T

## Code Reference
- a/b/c.go
`
	e := parseEntry("x.md", []byte(src))
	out1, changed1 := e.repair("2026-01-02")
	if !changed1 {
		t.Fatal("first repair should change")
	}
	e2 := parseEntry("x.md", out1)
	out2, changed2 := e2.repair("2026-01-02")
	if changed2 {
		t.Errorf("second repair changed (not idempotent):\n%s", out2)
	}
	if string(out1) != string(out2) {
		t.Errorf("idempotence broken: out1 != out2")
	}
}

// --- Adversarial ---

// Unknown keys are PRESERVED (not silently dropped) and reported.
func TestRepair_UnknownKeysPreserved(t *testing.T) {
	src := `---
id: mem-2026-01-01-abc123
question: "Q?"
created: 2026-01-01
status: active
type: knowledge
custom_field: keepme
source: legacy-import
---

# T
`
	e := parseEntry("x.md", []byte(src))
	r := e.schemaReport()
	if !contains(r.Unknown, "custom_field") || !contains(r.Unknown, "source") {
		t.Errorf("unknown = %v, want custom_field+source", r.Unknown)
	}
	out, _ := e.repair("2026-01-01")
	if !strings.Contains(string(out), "custom_field: keepme") || !strings.Contains(string(out), "source: legacy-import") {
		t.Errorf("unknown keys dropped by repair:\n%s", out)
	}
}

// An entry with no frontmatter at all is surfaced, not panicked.
func TestParseEntry_NoFrontmatter(t *testing.T) {
	e := parseEntry("x.md", []byte("# Just a heading\n\nNo frontmatter here.\n"))
	if e.HasFrontmatter {
		t.Error("should report no frontmatter")
	}
	r := e.schemaReport()
	if r.Conformant {
		t.Error("no-frontmatter entry cannot be conformant")
	}
}

// Malformed YAML frontmatter is surfaced via ParseErr, not a panic.
func TestParseEntry_MalformedYAML(t *testing.T) {
	src := "---\nid: [unclosed\nquestion: \"x\"\n---\n\n# T\n"
	e := parseEntry("x.md", []byte(src))
	if e.HasFrontmatter && e.ParseErr == "" {
		t.Error("malformed YAML should set ParseErr")
	}
}

// CRLF bodies are preserved byte-for-byte through repair.
func TestRepair_CRLFBodyPreserved(t *testing.T) {
	body := "\r\n# Title\r\n\r\n## Decision\r\nWindows line endings.\r\n"
	src := "---\ndate: 2026-02-02\ntype: knowledge\n---" + body
	e := parseEntry("x.md", []byte(src))
	out, _ := e.repair("2026-02-02")
	if !strings.HasSuffix(string(out), body) {
		t.Errorf("CRLF body not preserved verbatim")
	}
}

// deriveFilesFromBody only picks up repo-relative code paths (with a slash and
// a code extension), not prose words or bare basenames.
func TestDeriveFilesFromBody(t *testing.T) {
	body := `See internal/registry/config.go:81-93 and pkg/output/formatter.go.
Also project.go:51 is a bare basename (ignored). The word example.com is not a file.
Duplicate: internal/registry/config.go again.`
	got := deriveFilesFromBody(body)
	sort.Strings(got)
	want := []string{"internal/registry/config.go", "pkg/output/formatter.go"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("derived = %v, want %v", got, want)
	}
}

func TestInferType(t *testing.T) {
	cases := map[string]string{
		"clarifications-1.0_x-Q1.md": "clarifications",
		"sprint-learning-foo.md":     "sprint-learning",
		"random-note.md":             "knowledge",
	}
	for f, want := range cases {
		if got := inferType(f); got != want {
			t.Errorf("inferType(%q) = %q, want %q", f, got, want)
		}
	}
}

// --- test helpers ---

func frontMap(e *Entry) map[string]interface{} {
	m := map[string]interface{}{}
	for _, it := range e.front {
		k, _ := it.Key.(string)
		m[k] = it.Value
	}
	return m
}

func frontList(e *Entry, key string) []string {
	for _, it := range e.front {
		if k, _ := it.Key.(string); k == key {
			out := []string{}
			if lst, ok := it.Value.([]interface{}); ok {
				for _, v := range lst {
					out = append(out, toStr(v))
				}
			}
			return out
		}
	}
	return nil
}
