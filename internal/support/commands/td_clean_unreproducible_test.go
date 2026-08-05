package commands

import (
	"strings"
	"testing"
)

// Bug 3, and the worst of the three: silent data loss.
//
// removeEmptySections deletes any "###" section that has a table but no
// recognised checkbox row. hasCheckboxCell did not list "[-]", so a section
// whose surviving rows were all "[-]" read as empty and was deleted — taking
// rows that isResolvedRow had deliberately spared. It fires whenever some other
// row in the file is [x], which is the ordinary case for /resolve-td --cleanup.
func TestCleanKeepsASectionOfOnlyUnreproducibleRows(t *testing.T) {
	content := `# Tech Debt

## Stats

| Severity | Open | Deferred | Resolved |
|----------|------|----------|----------|
| HIGH | 0 | 0 | 0 |

**Last Modified:** 2020-01-01 | **Open Items:** 0 | **Deferred Items:** 0 | **Resolved Items:** 0 | **Total Items:** 0

### [2026-08-01] From Sprint: closed-without-a-fix

| Group | | Severity | File | Problem |
|-------|---|----------|------|---------|
| 1 | [-] | HIGH | a.py:1 | Could not reproduce |
| 2 | [-] | LOW | b.py:1 | Filed in error |

### [2026-08-02] From Sprint: has-a-resolved-row

| Group | | Severity | File | Problem |
|-------|---|----------|------|---------|
| 3 | [x] | MEDIUM | c.py:1 | Fixed |
| 4 | [ ] | MEDIUM | d.py:1 | Still open |
`
	cleaned, result := cleanTDReadme(content, "2026-08-04")

	if result.RemovedRows != 1 {
		t.Errorf("want 1 stripped [x] row, got %d", result.RemovedRows)
	}
	if result.RemovedSections != 0 {
		t.Errorf("want 0 removed sections, got %d — an all-[-] section was deleted", result.RemovedSections)
	}
	if !strings.Contains(cleaned, "closed-without-a-fix") {
		t.Errorf("the all-[-] section was deleted:\n%s", cleaned)
	}
	if !strings.Contains(cleaned, "| 1 | [-] | HIGH | a.py:1 |") {
		t.Errorf("an unreproducible row was lost:\n%s", cleaned)
	}
	if !strings.Contains(cleaned, "| 2 | [-] | LOW | b.py:1 |") {
		t.Errorf("an unreproducible row was lost:\n%s", cleaned)
	}
}

// [-] is closed, but not by a fix — td-clean strips resolved rows only, so an
// unreproducible row must survive the strip pass itself.
func TestCleanDoesNotStripUnreproducibleRows(t *testing.T) {
	content := `### S

| Group | | Severity | File |
|-------|---|----------|------|
| 1 | [-] | HIGH | a.py |
| 2 | [x] | HIGH | b.py |
`
	cleaned, result := cleanTDReadme(content, "2026-08-04")

	if result.RemovedRows != 1 {
		t.Errorf("want only the [x] row stripped, got %d removed", result.RemovedRows)
	}
	if !strings.Contains(cleaned, "[-]") {
		t.Errorf("the [-] row was stripped:\n%s", cleaned)
	}
	if strings.Contains(cleaned, "b.py") {
		t.Errorf("the [x] row survived:\n%s", cleaned)
	}
}

// The counts td-clean reports must account for unreproducible rows, or its
// Total disagrees with the file it just wrote.
func TestCleanReportsUnreproducibleCount(t *testing.T) {
	content := `### S

| Group | | Severity | File |
|-------|---|----------|------|
| 1 | [-] | HIGH | a.py |
| 2 | [ ] | HIGH | b.py |
| 3 | [x] | HIGH | c.py |
`
	_, result := cleanTDReadme(content, "2026-08-04")

	if result.Unreproducible != 1 {
		t.Errorf("want unreproducible=1, got %d", result.Unreproducible)
	}
	if result.Open != 1 {
		t.Errorf("want open=1, got %d", result.Open)
	}
	// One [x] was stripped, so two rows remain.
	if result.Total != 2 {
		t.Errorf("want total=2 (the rows actually left in the file), got %d", result.Total)
	}
}

// A section that really is empty after stripping is still removed — the fix
// must not turn the empty-section pass into a no-op.
func TestCleanStillRemovesAGenuinelyEmptySection(t *testing.T) {
	content := `### S1

| Group | | Severity | File |
|-------|---|----------|------|
| 1 | [x] | HIGH | a.py |

### S2

| Group | | Severity | File |
|-------|---|----------|------|
| 2 | [ ] | HIGH | b.py |
`
	cleaned, result := cleanTDReadme(content, "2026-08-04")

	if result.RemovedSections != 1 {
		t.Errorf("want 1 removed section, got %d", result.RemovedSections)
	}
	if strings.Contains(cleaned, "### S1") {
		t.Errorf("the genuinely empty section survived:\n%s", cleaned)
	}
	if !strings.Contains(cleaned, "### S2") {
		t.Errorf("the non-empty section was removed:\n%s", cleaned)
	}
}
