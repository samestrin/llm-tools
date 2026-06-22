package commands

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fullReadme is a realistic TD README with two sprint sections (one mixed,
// one fully resolved), a prose cell containing a literal "[x]", and a Stats
// section that is intentionally stale so regeneration can be observed.
const fullReadme = `# Technical Debt

**Last Modified:** 2026-01-01 | **Open Items:** 9 | **Deferred Items:** 9 | **Resolved Items:** 9 | **Total Items:** 27

Some intro text describing the backlog.

### [2026-06-01] From Sprint: 7.0_content_safety

| Group | | Severity | File:Line | Problem | Fix | Category | Confidence | Est |
|-------|---|----------|-----------|---------|-----|----------|-----------|-----|
| 1 | [x] | HIGH | a.go:10 | resolved thing | did it | bug | HIGH | 30 |
| 1 | [ ] | MEDIUM | b.go:20 | open thing see [x] in notes | todo | bug | MEDIUM | 60 |
| solo | [/] | LOW | c.go:5 | deferred thing | later | perf | LOW | 15 |

### [2026-06-02] From Sprint: 7.1_all_resolved

| Group | | Severity | File:Line | Problem | Fix | Category | Confidence | Est |
|-------|---|----------|-----------|---------|-----|----------|-----------|-----|
| solo | [X] | CRITICAL | d.go:1 | resolved crit | fixed | security | HIGH | 45 |

## Stats

| Severity | Open | Deferred | Resolved |
|----------|------|----------|----------|
| CRITICAL | 9 | 9 | 9 |
| HIGH | 9 | 9 | 9 |
| MEDIUM | 9 | 9 | 9 |
| LOW | 9 | 9 | 9 |
`

func writeTemp(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "README.md")
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		t.Fatalf("write temp: %v", err)
	}
	return p
}

func runClean(t *testing.T, args ...string) (TDCleanResult, string) {
	t.Helper()
	cmd := newTDCleanCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v\noutput: %s", err, buf.String())
	}
	var res TDCleanResult
	if err := json.Unmarshal(buf.Bytes(), &res); err != nil {
		t.Fatalf("unmarshal json: %v\noutput: %s", err, buf.String())
	}
	return res, buf.String()
}

func TestTDCleanStripsResolvedAndEmptySections(t *testing.T) {
	p := writeTemp(t, fullReadme)
	res, _ := runClean(t, "--path", p, "--today", "2026-06-22", "--json")

	if res.RemovedRows != 2 {
		t.Errorf("RemovedRows = %d, want 2", res.RemovedRows)
	}
	if res.RemovedSections != 1 {
		t.Errorf("RemovedSections = %d, want 1", res.RemovedSections)
	}

	out, _ := os.ReadFile(p)
	s := string(out)

	// Resolved rows gone.
	if strings.Contains(s, "a.go:10") {
		t.Error("resolved row a.go:10 should be removed")
	}
	if strings.Contains(s, "d.go:1") {
		t.Error("resolved row d.go:1 should be removed")
	}
	// Open / deferred rows kept.
	if !strings.Contains(s, "b.go:20") {
		t.Error("open row b.go:20 should be kept")
	}
	if !strings.Contains(s, "c.go:5") {
		t.Error("deferred row c.go:5 should be kept")
	}
	// The all-resolved section is gone; the mixed section remains.
	if strings.Contains(s, "7.1_all_resolved") {
		t.Error("emptied section 7.1_all_resolved should be removed")
	}
	if !strings.Contains(s, "7.0_content_safety") {
		t.Error("non-empty section 7.0_content_safety should be kept")
	}
}

func TestTDCleanPreservesProseBracketX(t *testing.T) {
	p := writeTemp(t, fullReadme)
	runClean(t, "--path", p, "--today", "2026-06-22", "--json")

	out, _ := os.ReadFile(p)
	s := string(out)
	// b.go:20 has "[x]" inside its prose Problem cell but checkbox is [ ];
	// it must survive (whole-cell match, not substring).
	if !strings.Contains(s, "open thing see [x] in notes") {
		t.Error("row with literal [x] in a prose cell must be preserved")
	}
}

func TestTDCleanRegeneratesStats(t *testing.T) {
	p := writeTemp(t, fullReadme)
	res, _ := runClean(t, "--path", p, "--today", "2026-06-22", "--json")

	// After cleanup: MEDIUM open=1, LOW deferred=1, resolved=0, total=2.
	if res.Open != 1 || res.Deferred != 1 || res.Resolved != 0 || res.Total != 2 {
		t.Errorf("counts open=%d deferred=%d resolved=%d total=%d, want 1/1/0/2",
			res.Open, res.Deferred, res.Resolved, res.Total)
	}

	out, _ := os.ReadFile(p)
	s := string(out)
	// Stale 9/9/9 stats must be gone, regenerated table present.
	if strings.Contains(s, "| HIGH | 9 | 9 | 9 |") {
		t.Error("stale stats row should be regenerated")
	}
	if !strings.Contains(s, "| MEDIUM | 1 | 0 | 0 |") {
		t.Errorf("regenerated MEDIUM stats row missing:\n%s", s)
	}
	if !strings.Contains(s, "| LOW | 0 | 1 | 0 |") {
		t.Errorf("regenerated LOW stats row missing:\n%s", s)
	}
}

func TestTDCleanUpdatesLastModified(t *testing.T) {
	p := writeTemp(t, fullReadme)
	runClean(t, "--path", p, "--today", "2026-06-22", "--json")

	out, _ := os.ReadFile(p)
	s := string(out)
	want := "**Last Modified:** 2026-06-22 | **Open Items:** 1 | **Deferred Items:** 1 | **Resolved Items:** 0 | **Total Items:** 2"
	if !strings.Contains(s, want) {
		t.Errorf("Last Modified line not updated, want:\n%s\ngot:\n%s", want, s)
	}
}

func TestTDCleanNoResolvedIsNoOp(t *testing.T) {
	content := `# Technical Debt

**Last Modified:** 2026-01-01 | **Open Items:** 1 | **Deferred Items:** 1 | **Resolved Items:** 0 | **Total Items:** 2

### [2026-06-01] From Sprint: only_open

| Group | | Severity | File:Line | Problem | Fix | Category | Confidence | Est |
|-------|---|----------|-----------|---------|-----|----------|-----------|-----|
| 1 | [ ] | MEDIUM | b.go:20 | open | todo | bug | MEDIUM | 60 |
| solo | [/] | LOW | c.go:5 | deferred | later | perf | LOW | 15 |

## Stats

| Severity | Open | Deferred | Resolved |
|----------|------|----------|----------|
| CRITICAL | 0 | 0 | 0 |
| HIGH | 0 | 0 | 0 |
| MEDIUM | 1 | 0 | 0 |
| LOW | 0 | 1 | 0 |
`
	p := writeTemp(t, content)
	before, _ := os.ReadFile(p)
	res, _ := runClean(t, "--path", p, "--today", "2026-06-22", "--json")
	after, _ := os.ReadFile(p)

	if res.RemovedRows != 0 || res.RemovedSections != 0 {
		t.Errorf("expected no removals, got rows=%d sections=%d", res.RemovedRows, res.RemovedSections)
	}
	if !bytes.Equal(before, after) {
		t.Errorf("file should be byte-identical on no-op\nbefore:\n%s\nafter:\n%s", before, after)
	}
}

func TestTDCleanIdempotent(t *testing.T) {
	p := writeTemp(t, fullReadme)
	runClean(t, "--path", p, "--today", "2026-06-22", "--json")
	first, _ := os.ReadFile(p)

	res2, _ := runClean(t, "--path", p, "--today", "2026-06-22", "--json")
	second, _ := os.ReadFile(p)

	if res2.RemovedRows != 0 || res2.RemovedSections != 0 {
		t.Errorf("second run should remove nothing, got rows=%d sections=%d", res2.RemovedRows, res2.RemovedSections)
	}
	if !bytes.Equal(first, second) {
		t.Error("second run should leave the file unchanged")
	}
}

func TestTDCleanMissingFileErrors(t *testing.T) {
	cmd := newTDCleanCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--path", filepath.Join(t.TempDir(), "nope.md"), "--json"})
	if err := cmd.Execute(); err == nil {
		t.Error("expected error for missing file, got nil")
	}
}

func TestTDCleanJSONShape(t *testing.T) {
	p := writeTemp(t, fullReadme)
	_, raw := runClean(t, "--path", p, "--today", "2026-06-22", "--json")
	for _, key := range []string{"removed_rows", "removed_sections", "open", "deferred", "resolved", "total"} {
		if !strings.Contains(raw, key) {
			t.Errorf("JSON output missing key %q:\n%s", key, raw)
		}
	}
}

func TestTDCleanTextOutput(t *testing.T) {
	p := writeTemp(t, fullReadme)
	cmd := newTDCleanCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--path", p, "--today", "2026-06-22"}) // no --json
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, buf.String())
	}
	if !strings.Contains(buf.String(), "Removed 2 resolved row(s), 1 empty section(s)") {
		t.Errorf("unexpected text output:\n%s", buf.String())
	}
}

// A README with no "## Stats" section must get one appended, and a file with
// no trailing newline must not be corrupted.
func TestTDCleanAppendsStatsAndPreservesNoTrailingNewline(t *testing.T) {
	content := "# Technical Debt\n" +
		"\n" +
		"**Last Modified:** 2026-01-01 | **Open Items:** 0 | **Deferred Items:** 0 | **Resolved Items:** 0 | **Total Items:** 0\n" +
		"\n" +
		"### [2026-06-01] From Sprint: mixed\n" +
		"\n" +
		"| Group | | Severity | File:Line | Problem | Fix | Category | Confidence | Est |\n" +
		"|-------|---|----------|-----------|---------|-----|----------|-----------|-----|\n" +
		"| 1 | [x] | HIGH | a.go:10 | done | did | bug | HIGH | 30 |\n" +
		"| 1 | [ ] | LOW | b.go:20 | open | todo | bug | LOW | 60 |" // no trailing newline

	p := writeTemp(t, content)
	res, _ := runClean(t, "--path", p, "--today", "2026-06-22", "--json")
	if res.RemovedRows != 1 {
		t.Errorf("RemovedRows = %d, want 1", res.RemovedRows)
	}
	out, _ := os.ReadFile(p)
	s := string(out)
	if !strings.Contains(s, "## Stats") {
		t.Errorf("Stats section should be appended:\n%s", s)
	}
	if !strings.Contains(s, "| LOW | 1 | 0 | 0 |") {
		t.Errorf("appended stats should reflect the open LOW item:\n%s", s)
	}
	if !strings.Contains(s, "b.go:20") {
		t.Error("open row should survive")
	}
}

func TestTDCleanDefaultsTodayWhenOmitted(t *testing.T) {
	p := writeTemp(t, fullReadme)
	// No --today: should still update the Last Modified line with some date.
	runClean(t, "--path", p, "--json")
	out, _ := os.ReadFile(p)
	s := string(out)
	if strings.Contains(s, "**Last Modified:** 2026-01-01") {
		t.Error("Last Modified date should have been refreshed away from the original")
	}
	if !strings.Contains(s, "**Open Items:** 1") {
		t.Errorf("Last Modified counts not updated:\n%s", s)
	}
}
