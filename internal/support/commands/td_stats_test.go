package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTDStatsBasic(t *testing.T) {
	tmpDir := t.TempDir()
	content := `# Tech Debt

| Group | | Severity | File | Problem | Fix | Category | Est Minutes |
|-------|---|----------|------|---------|-----|----------|-------------|
| 1 | [x] | HIGH | foo.py:1 | Fixed issue | Was fixed | error-handling | 30 |
| 1 | [ ] | HIGH | bar.py:1 | Open issue | Needs fix | security | 120 |
| 1 | [/] | MEDIUM | baz.py:1 | Deferred | Later | performance | 60 |
| 1 | [ ] | MEDIUM | qux.py:1 | Open medium | Fix it | maintainability | 45 |
| 1 | [x] | CRITICAL | crit.py:1 | Fixed critical | Done | security | 90 |
| 1 | [ ] | LOW | low.py:1 | Low issue | Fix | maintainability | 15 |
`
	mdFile := filepath.Join(tmpDir, "README.md")
	os.WriteFile(mdFile, []byte(content), 0644)

	cmd := newTDStatsCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--path", mdFile})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()

	// Verify markdown table structure
	if !strings.Contains(output, "## Stats") {
		t.Error("output should contain ## Stats header")
	}
	if !strings.Contains(output, "| Severity | Open | Deferred | Resolved |") {
		t.Error("output should contain table header")
	}

	// CRITICAL: 0 open, 0 deferred, 1 resolved
	if !strings.Contains(output, "| CRITICAL | 0 | 0 | 1 |") {
		t.Errorf("CRITICAL row incorrect, got:\n%s", output)
	}
	// HIGH: 1 open, 0 deferred, 1 resolved
	if !strings.Contains(output, "| HIGH | 1 | 0 | 1 |") {
		t.Errorf("HIGH row incorrect, got:\n%s", output)
	}
	// MEDIUM: 1 open, 1 deferred, 0 resolved
	if !strings.Contains(output, "| MEDIUM | 1 | 1 | 0 |") {
		t.Errorf("MEDIUM row incorrect, got:\n%s", output)
	}
	// LOW: 1 open, 0 deferred, 0 resolved
	if !strings.Contains(output, "| LOW | 1 | 0 | 0 |") {
		t.Errorf("LOW row incorrect, got:\n%s", output)
	}
}

func TestTDStatsJSONOutput(t *testing.T) {
	tmpDir := t.TempDir()
	content := `| Group | | Severity | File |
|-------|---|----------|------|
| 1 | [x] | HIGH | foo.py |
| 1 | [ ] | MEDIUM | bar.py |
`
	mdFile := filepath.Join(tmpDir, "README.md")
	os.WriteFile(mdFile, []byte(content), 0644)

	cmd := newTDStatsCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--path", mdFile, "--json"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, `"severity"`) {
		t.Errorf("JSON output should contain severity key, got: %s", output)
	}
	if !strings.Contains(output, `"HIGH"`) {
		t.Errorf("JSON output should contain HIGH, got: %s", output)
	}
	if !strings.Contains(output, `"markdown"`) {
		t.Errorf("JSON output should contain markdown key, got: %s", output)
	}
	if !strings.Contains(output, "## Stats") {
		t.Errorf("JSON markdown field should contain rendered table, got: %s", output)
	}
}

func TestTDStatsEmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	mdFile := filepath.Join(tmpDir, "README.md")
	os.WriteFile(mdFile, []byte("# No tables here\n"), 0644)

	cmd := newTDStatsCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--path", mdFile})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	// Should still produce a valid table with all zeros
	if !strings.Contains(output, "| CRITICAL | 0 | 0 | 0 |") {
		t.Errorf("empty file should produce zero counts, got:\n%s", output)
	}
}

func TestTDStatsMultipleTables(t *testing.T) {
	tmpDir := t.TempDir()
	content := `# Sprint 17.4

| Group | | Severity | File | Problem | Fix | Category | Est |
|-------|---|----------|------|---------|-----|----------|-----|
| 1 | [x] | HIGH | a.py | Done | Fixed | cat | 30 |
| 1 | [ ] | HIGH | b.py | Open | Fix | cat | 60 |

Some text between tables.

# Sprint 17.3

| Group | | Severity | File | Problem | Fix | Category | Est |
|-------|---|----------|------|---------|-----|----------|-----|
| 1 | [/] | MEDIUM | c.py | Deferred | Later | cat | 45 |
| 1 | [ ] | LOW | d.py | Open | Fix | cat | 15 |
`
	mdFile := filepath.Join(tmpDir, "README.md")
	os.WriteFile(mdFile, []byte(content), 0644)

	cmd := newTDStatsCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--path", mdFile})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	// Should aggregate across both tables
	if !strings.Contains(output, "| HIGH | 1 | 0 | 1 |") {
		t.Errorf("HIGH row incorrect, got:\n%s", output)
	}
	if !strings.Contains(output, "| MEDIUM | 0 | 1 | 0 |") {
		t.Errorf("MEDIUM row incorrect, got:\n%s", output)
	}
	if !strings.Contains(output, "| LOW | 1 | 0 | 0 |") {
		t.Errorf("LOW row incorrect, got:\n%s", output)
	}
}

func TestTDStatsSeverityOrder(t *testing.T) {
	tmpDir := t.TempDir()
	content := `| Group | | Severity | File |
|-------|---|----------|------|
| 1 | [ ] | LOW | a.py |
| 1 | [ ] | CRITICAL | b.py |
| 1 | [ ] | MEDIUM | c.py |
| 1 | [ ] | HIGH | d.py |
`
	mdFile := filepath.Join(tmpDir, "README.md")
	os.WriteFile(mdFile, []byte(content), 0644)

	cmd := newTDStatsCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--path", mdFile})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	// Verify order: CRITICAL before HIGH before MEDIUM before LOW
	critIdx := strings.Index(output, "CRITICAL")
	highIdx := strings.Index(output, "| HIGH")
	medIdx := strings.Index(output, "| MEDIUM")
	lowIdx := strings.Index(output, "| LOW")

	if critIdx > highIdx || highIdx > medIdx || medIdx > lowIdx {
		t.Errorf("severity order should be CRITICAL > HIGH > MEDIUM > LOW, got:\n%s", output)
	}
}

func TestParseTDStats(t *testing.T) {
	content := `| Group | | Severity | File |
|-------|---|----------|------|
| 1 | [x] | HIGH | foo.py |
| 1 | [ ] | HIGH | bar.py |
| 1 | [/] | HIGH | baz.py |
| 1 | [x] | MEDIUM | qux.py |
`
	result, err := parseTDStats(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	high := result.Severity["HIGH"]
	if high.Open != 1 || high.Deferred != 1 || high.Resolved != 1 {
		t.Errorf("HIGH: expected open=1 deferred=1 resolved=1, got open=%d deferred=%d resolved=%d",
			high.Open, high.Deferred, high.Resolved)
	}

	medium := result.Severity["MEDIUM"]
	if medium.Open != 0 || medium.Deferred != 0 || medium.Resolved != 1 {
		t.Errorf("MEDIUM: expected open=0 deferred=0 resolved=1, got open=%d deferred=%d resolved=%d",
			medium.Open, medium.Deferred, medium.Resolved)
	}
}

func TestTDStatsWriteInPlace(t *testing.T) {
	tmpDir := t.TempDir()
	content := `# Tech Debt

## Stats

| Severity | Open | Deferred | Resolved |
|----------|------|----------|----------|
| HIGH | 5 | 0 | 0 |

**Last Modified:** 2026-01-01 | **Open Items:** 5 | **Deferred Items:** 0 | **Resolved Items:** 0 | **Total Items:** 5

| Group | | Severity | File | Problem | Fix | Category | Est Minutes |
|-------|---|----------|------|---------|-----|----------|-------------|
| 1 | [x] | HIGH | foo.py:1 | Fixed issue | Was fixed | error-handling | 30 |
| 1 | [ ] | HIGH | bar.py:1 | Open issue | Needs fix | security | 120 |
`
	mdFile := filepath.Join(tmpDir, "README.md")
	os.WriteFile(mdFile, []byte(content), 0644)

	cmd := newTDStatsCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--path", mdFile, "--write", "--today", "2026-06-22"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(buf.String(), "Written to") {
		t.Errorf("expected write confirmation in output, got:\n%s", buf.String())
	}

	written, err := os.ReadFile(mdFile)
	if err != nil {
		t.Fatalf("failed to read written file: %v", err)
	}
	s := string(written)

	// Stats section reflects the actual data rows (1 open, 1 resolved), not the stale "5 open" it started with.
	if !strings.Contains(s, "| HIGH | 1 | 0 | 1 |") {
		t.Errorf("Stats section not refreshed, got:\n%s", s)
	}
	if !strings.Contains(s, "**Last Modified:** 2026-06-22 | **Open Items:** 1 | **Deferred Items:** 0 | **Resolved Items:** 1 | **Total Items:** 2") {
		t.Errorf("Last Modified line not updated, got:\n%s", s)
	}

	// Data rows must survive untouched — --write never adds or removes a row (that's td-clean's job).
	if !strings.Contains(s, "| 1 | [x] | HIGH | foo.py:1 | Fixed issue | Was fixed | error-handling | 30 |") {
		t.Errorf("resolved data row was removed, but --write must never touch data rows, got:\n%s", s)
	}
	if !strings.Contains(s, "| 1 | [ ] | HIGH | bar.py:1 | Open issue | Needs fix | security | 120 |") {
		t.Errorf("open data row missing, got:\n%s", s)
	}
}

func TestTDStatsWriteIdempotent(t *testing.T) {
	tmpDir := t.TempDir()
	// Already-consistent stats/date — --write should leave the file byte-for-byte unchanged and report no write.
	content := `## Stats

| Severity | Open | Deferred | Resolved |
|----------|------|----------|----------|
| CRITICAL | 0 | 0 | 0 |
| HIGH | 1 | 0 | 0 |
| MEDIUM | 0 | 0 | 0 |
| LOW | 0 | 0 | 0 |

**Last Modified:** 2026-06-22 | **Open Items:** 1 | **Deferred Items:** 0 | **Resolved Items:** 0 | **Total Items:** 1

| Group | | Severity | File |
|-------|---|----------|------|
| 1 | [ ] | HIGH | foo.py |
`
	mdFile := filepath.Join(tmpDir, "README.md")
	os.WriteFile(mdFile, []byte(content), 0644)

	before, err := os.ReadFile(mdFile)
	if err != nil {
		t.Fatalf("failed to read fixture: %v", err)
	}
	beforeInfo, err := os.Stat(mdFile)
	if err != nil {
		t.Fatalf("failed to stat fixture: %v", err)
	}

	cmd := newTDStatsCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--path", mdFile, "--write", "--today", "2026-06-22"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(buf.String(), "No change") {
		t.Errorf("expected no-change output for an already up-to-date file, got:\n%s", buf.String())
	}

	after, err := os.ReadFile(mdFile)
	if err != nil {
		t.Fatalf("failed to read file after run: %v", err)
	}
	if string(before) != string(after) {
		t.Errorf("file should be byte-for-byte unchanged when nothing to update, before:\n%s\nafter:\n%s", before, after)
	}

	afterInfo, err := os.Stat(mdFile)
	if err != nil {
		t.Fatalf("failed to stat file after run: %v", err)
	}
	if !beforeInfo.ModTime().Equal(afterInfo.ModTime()) {
		t.Error("file mtime changed even though content was already up to date — WriteFile should have been skipped")
	}
}

func TestTDStatsWithoutWriteFlagLeavesFileUntouched(t *testing.T) {
	tmpDir := t.TempDir()
	content := `## Stats

| Severity | Open | Deferred | Resolved |
|----------|------|----------|----------|
| HIGH | 99 | 0 | 0 |

**Last Modified:** 2020-01-01 | **Open Items:** 99 | **Deferred Items:** 0 | **Resolved Items:** 0 | **Total Items:** 99

| Group | | Severity | File |
|-------|---|----------|------|
| 1 | [ ] | HIGH | foo.py |
`
	mdFile := filepath.Join(tmpDir, "README.md")
	os.WriteFile(mdFile, []byte(content), 0644)

	cmd := newTDStatsCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--path", mdFile})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	after, err := os.ReadFile(mdFile)
	if err != nil {
		t.Fatalf("failed to read file after run: %v", err)
	}
	if string(after) != content {
		t.Errorf("without --write, the file on disk must be untouched (read-only), got:\n%s", after)
	}
}

func TestSplitTableRow(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"| a | b | c |", 3},
		{"| a | b |", 2},
		{"|a|b|c|d|", 4},
	}

	for _, tt := range tests {
		cells := splitTableRow(tt.input)
		if len(cells) != tt.want {
			t.Errorf("splitTableRow(%q) = %d cells, want %d", tt.input, len(cells), tt.want)
		}
	}
}

func TestIsSeparatorRow(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"---", true},
		{":---:", true},
		{":---", true},
		{"---:", true},
		{"content", false},
		{"[ ]", false},
		{"", true}, // empty cells are ok
	}

	for _, tt := range tests {
		cells := []string{tt.input}
		got := isSeparatorRow(cells)
		if got != tt.want {
			t.Errorf("isSeparatorRow([%q]) = %v, want %v", tt.input, got, tt.want)
		}
	}
}
