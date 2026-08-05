package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const threeStateREADME = `# Tech Debt

## Stats

| Severity | Open | Deferred | Resolved |
|----------|------|----------|----------|
| CRITICAL | 0 | 0 | 0 |
| HIGH | 0 | 0 | 0 |
| MEDIUM | 0 | 0 | 0 |
| LOW | 0 | 0 | 0 |

**Last Modified:** 2020-01-01 | **Open Items:** 0 | **Deferred Items:** 0 | **Resolved Items:** 0 | **Total Items:** 0

### Backlog

| Group | | Severity | File |
|-------|---|----------|------|
| 1 | [ ] | HIGH | a.py |
| 1 | [/] | MEDIUM | b.py |
| 1 | [x] | LOW | c.py |
`

// A file that uses only the original three states must render exactly the block
// it renders today — same columns, same Last Modified fields. Adding a state to
// the tool must not rewrite every TD README that never uses it.
func TestFormatKeepsFourColumnsWithoutUnreproducibleRows(t *testing.T) {
	result, err := parseTDStats(threeStateREADME)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	markdown := formatTDStatsMarkdown(result)

	if !strings.Contains(markdown, "| Severity | Open | Deferred | Resolved |\n") {
		t.Errorf("want the unchanged 4-column header, got:\n%s", markdown)
	}
	if strings.Contains(markdown, "Unreproducible") {
		t.Errorf("a file with no [-] rows must not gain the column:\n%s", markdown)
	}
	if !strings.Contains(markdown, "| HIGH | 1 | 0 | 0 |\n") {
		t.Errorf("want 4-cell HIGH row, got:\n%s", markdown)
	}
}

func TestFormatAddsColumnWhenUnreproducibleRowsExist(t *testing.T) {
	result, err := parseTDStats(threeStateREADME + "| 1 | [-] | HIGH | d.py |\n")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	markdown := formatTDStatsMarkdown(result)

	if !strings.Contains(markdown, "| Severity | Open | Deferred | Resolved | Unreproducible |\n") {
		t.Errorf("want the 5-column header, got:\n%s", markdown)
	}
	if !strings.Contains(markdown, "| HIGH | 1 | 0 | 0 | 1 |\n") {
		t.Errorf("want HIGH with an unreproducible cell, got:\n%s", markdown)
	}
	if !strings.Contains(markdown, "| LOW | 0 | 0 | 1 | 0 |\n") {
		t.Errorf("severities with no [-] still need the cell, got:\n%s", markdown)
	}
	// The separator must match the header width or the table stops rendering.
	if !strings.Contains(markdown, "|----------|------|----------|----------|----------------|\n") {
		t.Errorf("separator width does not match the header, got:\n%s", markdown)
	}
}

// --write on a file that uses only three states must be byte-for-byte a no-op.
// This is the compatibility guarantee: the tool gained a state, the file did not.
func TestWriteIsAByteForByteNoOpOnAThreeStateFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "README.md")
	// Pre-render the block so only a genuine format change can alter the file.
	seeded := strings.Replace(threeStateREADME,
		"| CRITICAL | 0 | 0 | 0 |\n| HIGH | 0 | 0 | 0 |\n| MEDIUM | 0 | 0 | 0 |\n| LOW | 0 | 0 | 0 |",
		"| CRITICAL | 0 | 0 | 0 |\n| HIGH | 1 | 0 | 0 |\n| MEDIUM | 0 | 1 | 0 |\n| LOW | 0 | 0 | 1 |", 1)
	seeded = strings.Replace(seeded,
		"**Last Modified:** 2020-01-01 | **Open Items:** 0 | **Deferred Items:** 0 | **Resolved Items:** 0 | **Total Items:** 0",
		"**Last Modified:** 2026-08-04 | **Open Items:** 1 | **Deferred Items:** 1 | **Resolved Items:** 1 | **Total Items:** 3", 1)
	if err := os.WriteFile(path, []byte(seeded), 0644); err != nil {
		t.Fatal(err)
	}

	cmd := newTDStatsCmd()
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetArgs([]string{"--path", path, "--write", "--today", "2026-08-04"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != seeded {
		t.Errorf("--write changed a three-state file.\n--- before ---\n%s\n--- after ---\n%s", seeded, string(after))
	}
}

func TestWriteAddsUnreproducibleFieldToLastModified(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "README.md")
	if err := os.WriteFile(path, []byte(threeStateREADME+"| 1 | [-] | HIGH | d.py |\n"), 0644); err != nil {
		t.Fatal(err)
	}

	cmd := newTDStatsCmd()
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetArgs([]string{"--path", path, "--write", "--today", "2026-08-04"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := "**Last Modified:** 2026-08-04 | **Open Items:** 1 | **Deferred Items:** 1 | " +
		"**Resolved Items:** 1 | **Unreproducible Items:** 1 | **Total Items:** 4"
	if !strings.Contains(string(got), want) {
		t.Errorf("want Last Modified line:\n%s\ngot file:\n%s", want, string(got))
	}
}

// Non-standard severities are a supported concept — formatTDStatsMarkdown has
// always appended them after the standard four. They must trigger the opt-in
// column like any other row, and render at the header's width.
func TestFormatHandlesUnreproducibleUnderANonStandardSeverity(t *testing.T) {
	content := `| Group | | Severity | File |
|-------|---|----------|------|
| 1 | [-] | URGENT | a.py |
| 2 | [ ] | HIGH | b.py |
`
	result, err := parseTDStats(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	markdown := formatTDStatsMarkdown(result)
	if !strings.Contains(markdown, "| Severity | Open | Deferred | Resolved | Unreproducible |") {
		t.Errorf("a non-standard severity's [-] row must trigger the column:\n%s", markdown)
	}
	if !strings.Contains(markdown, "| URGENT | 0 | 0 | 0 | 1 |") {
		t.Errorf("want a full-width URGENT row, got:\n%s", markdown)
	}
}

// Headerless auto-detection covers the standard severities only; anything else
// needs the header row that names the column. Pinned so the limitation is a
// decision on record rather than a surprise.
func TestHeaderlessTableRequiresAStandardSeverity(t *testing.T) {
	withHeader, err := parseTDStats("| Group | | Severity | File |\n|---|---|---|---|\n| 1 | [-] | URGENT | a.py |\n")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if withHeader.Severity["URGENT"].Unreproducible != 1 {
		t.Errorf("a header row names the column, so any severity counts: %+v", withHeader.Severity["URGENT"])
	}

	headerless, err := parseTDStats("| 1 | [-] | URGENT | a.py |\n")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if headerless.Summary.Total != 0 {
		t.Errorf("headerless auto-detect should not guess a non-standard severity, got %+v", headerless.Summary)
	}
}

// Every row must carry exactly as many cells as the header, in every shape.
func TestStatsTableIsNeverRagged(t *testing.T) {
	for _, content := range []string{
		"| 1 | [ ] | HIGH | a.py |\n",
		"| 1 | [-] | HIGH | a.py |\n",
		"| Group | | Severity | File |\n|---|---|---|---|\n| 1 | [-] | URGENT | a.py |\n| 2 | [ ] | HIGH | b.py |\n",
		"",
	} {
		result, err := parseTDStats(content)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		markdown := formatTDStatsMarkdown(result)
		width := -1
		for _, line := range strings.Split(strings.TrimRight(markdown, "\n"), "\n") {
			if !strings.HasPrefix(line, "|") {
				continue
			}
			if pipes := strings.Count(line, "|"); width == -1 {
				width = pipes
			} else if pipes != width {
				t.Errorf("ragged table for %q:\n%s", content, markdown)
				break
			}
		}
	}
}

// The stats block is replaced in place; prose after the table is not part of it.
// The README that prompted this change keeps its convention notes there.
func TestReplaceStatsSectionPreservesNotesAfterTheTable(t *testing.T) {
	original := `# T

## Stats

| Severity | Open | Deferred | Resolved |
|----------|------|----------|----------|
| HIGH | 0 | 0 | 0 |

> A convention note that must not be eaten.

### Backlog
`
	out := strings.Join(
		replaceStatsSection(strings.Split(original, "\n"),
			"## Stats\n\n| Severity | Open |\n|----------|------|\n| HIGH | 1 |\n"),
		"\n")

	if !strings.Contains(out, "A convention note that must not be eaten.") {
		t.Errorf("note after the table was lost:\n%s", out)
	}
	if !strings.Contains(out, "### Backlog") {
		t.Errorf("following heading was lost:\n%s", out)
	}
}

// Writing twice must be identical to writing once, for both shapes.
func TestWriteIsIdempotentWithUnreproducibleRows(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "README.md")
	if err := os.WriteFile(path, []byte(threeStateREADME+"| 1 | [-] | HIGH | d.py |\n"), 0644); err != nil {
		t.Fatal(err)
	}

	run := func() string {
		cmd := newTDStatsCmd()
		cmd.SetOut(new(bytes.Buffer))
		cmd.SetArgs([]string{"--path", path, "--write", "--today", "2026-08-04"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}

	first := run()
	second := run()
	if first != second {
		t.Errorf("second --write differs from the first.\n--- first ---\n%s\n--- second ---\n%s", first, second)
	}
	// And the re-read of a 5-column block must not double-count itself.
	if !strings.Contains(second, "| HIGH | 1 | 0 | 0 | 1 |") {
		t.Errorf("counts drifted on re-write:\n%s", second)
	}
}

// Rows for non-standard severities came straight off a map range, so Go's
// randomised iteration reordered them between runs: --write produced a
// different file on most invocations, which is a spurious diff every time and
// breaks the idempotence this command promises. Sorted, matching td-matrix.
func TestNonStandardSeverityRowsAreOrderedDeterministically(t *testing.T) {
	content := `| Group | | Severity | File |
|-------|---|----------|------|
| 1 | [ ] | URGENT | a.py |
| 2 | [ ] | TRIVIAL | b.py |
| 3 | [ ] | BLOCKER | c.py |
`
	result, err := parseTDStats(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	first := formatTDStatsMarkdown(result)
	for i := 0; i < 500; i++ {
		if got := formatTDStatsMarkdown(result); got != first {
			t.Fatalf("row order varies between renders (iteration %d):\n--- a ---\n%s\n--- b ---\n%s", i, first, got)
		}
	}

	blocker := strings.Index(first, "| BLOCKER |")
	trivial := strings.Index(first, "| TRIVIAL |")
	urgent := strings.Index(first, "| URGENT |")
	if blocker == -1 || trivial == -1 || urgent == -1 {
		t.Fatalf("a non-standard severity row is missing:\n%s", first)
	}
	if !(blocker < trivial && trivial < urgent) {
		t.Errorf("extras should be alphabetical, got:\n%s", first)
	}
	// And they must still come after the standard four.
	if low := strings.Index(first, "| LOW |"); low > blocker {
		t.Errorf("a non-standard severity preceded the standard four:\n%s", first)
	}
}
