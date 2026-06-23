package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTDStatsWriteRefreshesStatsWithoutDroppingRows(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "README.md")

	// Stale stats block (says HIGH open=99) over a table whose real counts are
	// HIGH open=1, MEDIUM resolved=1, LOW open=1.
	original := `# Technical Debt Tracking

## Stats

| Severity | Open | Deferred | Resolved |
|----------|------|----------|----------|
| CRITICAL | 0 | 0 | 0 |
| HIGH | 99 | 0 | 0 |
| MEDIUM | 0 | 0 | 0 |
| LOW | 0 | 0 | 0 |

**Last Modified:** 2020-01-01 | **Open Items:** 99 | **Deferred Items:** 0 | **Resolved Items:** 0 | **Total Items:** 99

### [2026-06-22] From Sprint: example

| Group | | Severity | File | Problem |
|-------|---|----------|------|---------|
| 1 | [ ] | HIGH | a.go:1 | open high |
| 1 | [x] | MEDIUM | b.go:2 | resolved medium |
| 1 | [ ] | LOW | c.go:3 | open low |
`
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	cmd := newTDStatsCmd()
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"--path", path, "--write", "--today", "2026-06-22"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	out, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	got := string(out)

	// Stats refreshed to real counts.
	if !strings.Contains(got, "| HIGH | 1 | 0 | 0 |") {
		t.Errorf("HIGH stats not refreshed:\n%s", got)
	}
	if !strings.Contains(got, "| MEDIUM | 0 | 0 | 1 |") {
		t.Errorf("MEDIUM resolved not refreshed:\n%s", got)
	}
	if !strings.Contains(got, "| LOW | 1 | 0 | 0 |") {
		t.Errorf("LOW stats not refreshed:\n%s", got)
	}
	if strings.Contains(got, "| HIGH | 99") {
		t.Error("stale HIGH=99 stats line survived")
	}

	// Last Modified refreshed.
	if !strings.Contains(got, "**Last Modified:** 2026-06-22 | **Open Items:** 2 | **Deferred Items:** 0 | **Resolved Items:** 1 | **Total Items:** 3") {
		t.Errorf("Last Modified line not refreshed:\n%s", got)
	}

	// CRITICAL: every data row is preserved — no truncation.
	for _, row := range []string{"a.go:1 | open high", "b.go:2 | resolved medium", "c.go:3 | open low"} {
		if !strings.Contains(got, row) {
			t.Errorf("data row dropped: %q\n%s", row, got)
		}
	}
}
