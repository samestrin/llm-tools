package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A Stats block that ALREADY has 5 columns must be re-read without its own
// numbers being counted as data (compounding on every --write).
func TestCumFiveColumnStatsBlockIsNotSelfCounted(t *testing.T) {
	content := `## Stats

| Severity | Open | Deferred | Resolved | Unreproducible |
|----------|------|----------|----------|----------------|
| HIGH | 9 | 9 | 9 | 9 |

### S

| Group | | Severity | File |
|-------|---|----------|------|
| 1 | [-] | HIGH | a.py |
`
	r, _ := parseTDStats(content)
	if r.Summary.Total != 1 || r.Severity["HIGH"].Unreproducible != 1 {
		t.Errorf("stats block self-counted: %+v / %+v", r.Summary, r.Severity["HIGH"])
	}
}

// A row whose checkbox cell is "[-]" but which sits in a table where the
// checkbox column is NOT index 1.
func TestCumCheckboxNotAtIndexOne(t *testing.T) {
	r, _ := parseTDStats("| a.py | HIGH | [-] | note |\n")
	if r.Severity["HIGH"].Unreproducible != 1 {
		t.Errorf("checkbox at index 2 not detected: %+v", r.Severity["HIGH"])
	}
}

// Windows line endings must not defeat whole-cell matching.
func TestCumCarriageReturnsDoNotBreakDetection(t *testing.T) {
	r, _ := parseTDStats("| 1 | [-] | HIGH | a.py |\r\n| 2 | [ ] | HIGH | b.py |\r\n")
	if r.Summary.Total != 2 {
		t.Errorf("CRLF broke parsing: %+v", r.Summary)
	}
}

// A cell that is only whitespace, and a row of empty cells, must not panic or
// register.
func TestCumDegenerateRowsAreInert(t *testing.T) {
	for _, c := range []string{"|\n", "| | | |\n", "|||\n", "| [-] |\n"} {
		r, err := parseTDStats(c)
		if err != nil {
			t.Errorf("%q: %v", c, err)
		}
		if r.Summary.Total != 0 {
			t.Errorf("%q counted %+v (no severity present)", c, r.Summary)
		}
	}
}

// td-clean must remain a byte-for-byte no-op on a file with no resolved rows,
// even when that file uses [-]. The guarantee lives in runTDClean, which gates
// the write on removals, so it is asserted through the command.
func TestCumCleanIsNoOpOnAFileOfOnlyUnreproducibleRows(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "README.md")
	content := `### S

| Group | | Severity | File |
|-------|---|----------|------|
| 1 | [-] | HIGH | a.py |
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cmd := newTDCleanCmd()
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetArgs([]string{"--path", path, "--today", "2026-08-04"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != content {
		t.Errorf("file changed on a no-op:\n--- before ---\n%s\n--- after ---\n%s", content, string(after))
	}
}

// The alias "[X]" must behave exactly like "[x]" everywhere.
func TestCumUppercaseResolvedAliasIsEquivalent(t *testing.T) {
	lower, _ := parseTDStats("| 1 | [x] | HIGH | a.py |\n")
	upper, _ := parseTDStats("| 1 | [X] | HIGH | a.py |\n")
	if lower.Summary != upper.Summary {
		t.Errorf("[x]=%+v but [X]=%+v", lower.Summary, upper.Summary)
	}
}

// A file whose [-] rows are all removed must shed the column again.
func TestCumColumnDisappearsWhenLastUnreproducibleRowGoes(t *testing.T) {
	with, _ := parseTDStats("| 1 | [-] | HIGH | a.py |\n| 2 | [ ] | HIGH | b.py |\n")
	without, _ := parseTDStats("| 2 | [ ] | HIGH | b.py |\n")
	if !strings.Contains(formatTDStatsMarkdown(with), "Unreproducible") {
		t.Error("column missing when rows present")
	}
	if strings.Contains(formatTDStatsMarkdown(without), "Unreproducible") {
		t.Error("column persisted after the last [-] row went")
	}
}
