package commands

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// matrixReadme exercises: multiple groups (numeric + non-numeric), open/
// deferred/resolved states, two From-sections, and a stray non-TD table plus
// the Stats table that must NOT be counted.
const matrixReadme = `# Technical Debt

**Last Modified:** 2026-06-22 | **Open Items:** 6 | **Deferred Items:** 1 | **Resolved Items:** 1 | **Total Items:** 8

### [2026-06-01] From Sprint: alpha

| Group | | Severity | File:Line | Problem | Fix | Category | Est |
|-------|---|----------|-----------|---------|-----|----------|-----|
| 1 | [ ] | HIGH | a.go:1 | p | f | bug | 30 |
| 1 | [ ] | MEDIUM | b.go:2 | p | f | bug | 60 |
| 2 | [ ] | HIGH | c.go:3 | p | f | bug | 15 |
| 2 | [x] | CRITICAL | d.go:4 | p | f | bug | 45 |
| solo | [/] | LOW | e.go:5 | p | f | perf | 20 |

### [2026-06-02] From Sprint: beta

| Group | | Severity | File:Line | Problem | Fix | Category | Est |
|-------|---|----------|-----------|---------|-----|----------|-----|
| 2 | [ ] | LOW | f.go:6 | p | f | bug | 10 |
| 10 | [ ] | LOW | h.go:8 | p | f | bug | 10 |
| solo | [ ] | CRITICAL | g.go:7 | p | f | security | 90 |

## Notes

| Col A | Col B |
|-------|-------|
| x | y |

## Stats

| Severity | Open | Deferred | Resolved |
|----------|------|----------|----------|
| CRITICAL | 1 | 0 | 1 |
| HIGH | 2 | 0 | 0 |
| MEDIUM | 1 | 0 | 0 |
| LOW | 2 | 1 | 0 |
`

func runMatrix(t *testing.T, args ...string) (TDMatrixResult, string) {
	t.Helper()
	cmd := newTDMatrixCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v\noutput: %s", err, buf.String())
	}
	var res TDMatrixResult
	if err := json.Unmarshal(buf.Bytes(), &res); err != nil {
		t.Fatalf("unmarshal: %v\noutput: %s", err, buf.String())
	}
	return res, buf.String()
}

func TestTDMatrixCounts(t *testing.T) {
	p := writeTemp(t, matrixReadme)
	res, _ := runMatrix(t, "--path", p, "--json")

	if res.Total != 6 {
		t.Errorf("Total = %d, want 6 (open items only)", res.Total)
	}
	// Group order: numeric ascending, then non-numeric alpha.
	wantGroups := []string{"1", "2", "10", "solo"}
	if strings.Join(res.Groups, ",") != strings.Join(wantGroups, ",") {
		t.Errorf("Groups = %v, want %v", res.Groups, wantGroups)
	}
	checks := []struct {
		group, sev string
		want       int
	}{
		{"1", "HIGH", 1}, {"1", "MEDIUM", 1}, {"1", "CRITICAL", 0},
		{"2", "HIGH", 1}, {"2", "LOW", 1},
		{"10", "LOW", 1},
		{"solo", "CRITICAL", 1}, {"solo", "LOW", 0},
	}
	for _, c := range checks {
		if got := res.Counts[c.group][c.sev]; got != c.want {
			t.Errorf("Counts[%s][%s] = %d, want %d", c.group, c.sev, got, c.want)
		}
	}
	if res.ColTotals["HIGH"] != 2 || res.ColTotals["LOW"] != 2 || res.ColTotals["CRITICAL"] != 1 || res.ColTotals["MEDIUM"] != 1 {
		t.Errorf("ColTotals wrong: %v", res.ColTotals)
	}
	if res.RowTotals["1"] != 2 || res.RowTotals["2"] != 2 || res.RowTotals["10"] != 1 || res.RowTotals["solo"] != 1 {
		t.Errorf("RowTotals wrong: %v", res.RowTotals)
	}
}

func TestTDMatrixExcludesClosed(t *testing.T) {
	p := writeTemp(t, matrixReadme)
	res, _ := runMatrix(t, "--path", p, "--json")
	// The only CRITICAL open item is solo (g.go); group 2's CRITICAL is [x].
	if res.Counts["2"]["CRITICAL"] != 0 {
		t.Errorf("resolved [x] CRITICAL must not be counted, got %d", res.Counts["2"]["CRITICAL"])
	}
	// solo LOW is [/] deferred — must not be counted.
	if res.Counts["solo"]["LOW"] != 0 {
		t.Errorf("deferred [/] LOW must not be counted, got %d", res.Counts["solo"]["LOW"])
	}
}

func TestTDMatrixMarkdown(t *testing.T) {
	p := writeTemp(t, matrixReadme)
	res, _ := runMatrix(t, "--path", p, "--json")
	md := res.Markdown
	if !strings.Contains(md, "| Group | CRITICAL | HIGH | MEDIUM | LOW | Total |") {
		t.Errorf("missing header row:\n%s", md)
	}
	if !strings.Contains(md, "| 1 | 0 | 1 | 1 | 0 | 2 |") {
		t.Errorf("missing group 1 row:\n%s", md)
	}
	if !strings.Contains(md, "| **Total** | 1 | 2 | 1 | 2 | 6 |") {
		t.Errorf("missing totals row:\n%s", md)
	}
}

func TestTDMatrixNonStandardSeverity(t *testing.T) {
	content := `# TD

### [2026-06-01] From Sprint: x

| Group | | Severity | File:Line | Problem | Fix | Category | Est |
|-------|---|----------|-----------|---------|-----|----------|-----|
| 1 | [ ] | INFO | a.go:1 | p | f | bug | 5 |
| 1 | [ ] | HIGH | b.go:2 | p | f | bug | 5 |
`
	p := writeTemp(t, content)
	res, _ := runMatrix(t, "--path", p, "--json")
	if res.Counts["1"]["INFO"] != 1 {
		t.Errorf("non-standard severity INFO not counted: %v", res.Counts)
	}
	// INFO appears after the four standard severities in the column order.
	found := false
	for _, s := range res.Severities {
		if s == "INFO" {
			found = true
		}
	}
	if !found {
		t.Errorf("INFO missing from Severities column order: %v", res.Severities)
	}
}

func TestTDMatrixNoOpenItems(t *testing.T) {
	content := `# TD

### [2026-06-01] From Sprint: x

| Group | | Severity | File:Line | Problem | Fix | Category | Est |
|-------|---|----------|-----------|---------|-----|----------|-----|
| 1 | [x] | HIGH | a.go:1 | p | f | bug | 5 |
| solo | [/] | LOW | b.go:2 | p | f | bug | 5 |
`
	p := writeTemp(t, content)
	res, raw := runMatrix(t, "--path", p, "--json")
	if res.Total != 0 {
		t.Errorf("Total = %d, want 0", res.Total)
	}
	if len(res.Groups) != 0 {
		t.Errorf("Groups = %v, want empty", res.Groups)
	}
	// Markdown should still render a valid header + zero totals row.
	if !strings.Contains(res.Markdown, "| **Total** | 0 | 0 | 0 | 0 | 0 |") {
		t.Errorf("expected zero totals row:\n%s", raw)
	}
}

func TestTDMatrixIgnoresNonFromTables(t *testing.T) {
	p := writeTemp(t, matrixReadme)
	res, _ := runMatrix(t, "--path", p, "--json")
	// The "## Notes" table and "## Stats" table must not inflate counts;
	// grand total stays 6.
	if res.Total != 6 {
		t.Errorf("non-TD tables leaked into counts, Total = %d, want 6", res.Total)
	}
}

func TestTDMatrixReadOnly(t *testing.T) {
	p := writeTemp(t, matrixReadme)
	before, _ := os.ReadFile(p)
	runMatrix(t, "--path", p, "--json")
	after, _ := os.ReadFile(p)
	if !bytes.Equal(before, after) {
		t.Error("td-matrix must not modify the README")
	}
}

func TestTDMatrixMissingFile(t *testing.T) {
	cmd := newTDMatrixCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--path", filepath.Join(t.TempDir(), "nope.md"), "--json"})
	if err := cmd.Execute(); err == nil {
		t.Error("expected error for missing file")
	}
}

func TestTDMatrixJSONShape(t *testing.T) {
	p := writeTemp(t, matrixReadme)
	_, raw := runMatrix(t, "--path", p, "--json")
	for _, key := range []string{"severities", "groups", "counts", "row_totals", "col_totals", "total", "markdown"} {
		if !strings.Contains(raw, key) {
			t.Errorf("JSON missing key %q:\n%s", key, raw)
		}
	}
}
