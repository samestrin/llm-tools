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
