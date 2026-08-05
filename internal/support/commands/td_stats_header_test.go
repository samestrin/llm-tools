package commands

import "testing"

// A section's table does not always carry a header row — a TD README that
// appends rows under a prose heading, or whose header was edited away, still
// holds real data. parseTDStats used to treat the first "|" line after any
// non-table line as a header and skip it unconditionally, which ate that row;
// with no cell literally named "Severity" the severity column then stayed
// unset, so the row AFTER it was dropped too. Two silent losses per headerless
// table, reported as a smaller backlog rather than as a parse failure.
func TestParseTDStatsCountsHeaderlessTable(t *testing.T) {
	content := `# Tech Debt

## Backlog

| 20 | [/] | HIGH | a.py:1 | Problem | Fix | cat | 30 |
| 21 | [/] | MEDIUM | b.py:1 | Problem | Fix | cat | 30 |
| 22 | [ ] | LOW | c.py:1 | Problem | Fix | cat | 30 |
`
	result, err := parseTDStats(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := result.Severity["HIGH"].Deferred; got != 1 {
		t.Errorf("HIGH deferred: want 1, got %d (first row eaten as a header)", got)
	}
	if got := result.Severity["MEDIUM"].Deferred; got != 1 {
		t.Errorf("MEDIUM deferred: want 1, got %d (row dropped for want of a severity column)", got)
	}
	if got := result.Severity["LOW"].Open; got != 1 {
		t.Errorf("LOW open: want 1, got %d", got)
	}
	if result.Summary.Total != 3 {
		t.Errorf("total: want 3, got %d", result.Summary.Total)
	}
}

// The real header row must still be recognised, so its cells are never counted
// as data — "Severity" is not a severity value.
func TestParseTDStatsSkipsRealHeaderRow(t *testing.T) {
	content := `| Group | | Severity | File | Problem |
|-------|---|----------|------|---------|
| 1 | [ ] | HIGH | a.py:1 | Problem |
`
	result, err := parseTDStats(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Summary.Total != 1 {
		t.Errorf("total: want 1 (header must not count), got %d", result.Summary.Total)
	}
	if _, ok := result.Severity["SEVERITY"]; ok {
		t.Error("the header row was counted as a data row")
	}
}

// A table interrupted by prose resumes as the same table. Every row after the
// break is data; none of them is a header.
func TestParseTDStatsCountsRowsAfterAProseBreak(t *testing.T) {
	content := `| Group | | Severity | File |
|-------|---|----------|------|
| 1 | [ ] | HIGH | a.py:1 |

Some prose between the rows.

| 2 | [ ] | HIGH | b.py:1 |
| 3 | [x] | HIGH | c.py:1 |
`
	result, err := parseTDStats(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	high := result.Severity["HIGH"]
	if high.Open != 2 || high.Resolved != 1 {
		t.Errorf("HIGH: want open=2 resolved=1, got open=%d resolved=%d", high.Open, high.Resolved)
	}
}

// The file's own "## Stats" block is a markdown table whose rows begin with a
// severity name ("| CRITICAL | 0 | 0 | 1 |"). It carries no checkbox cell, so
// it must contribute nothing — otherwise td-stats counts its own output and
// the numbers compound on every --write.
func TestParseTDStatsIgnoresItsOwnStatsBlock(t *testing.T) {
	content := `# Tech Debt

## Stats

| Severity | Open | Deferred | Resolved |
|----------|------|----------|----------|
| CRITICAL | 0 | 0 | 1 |
| HIGH | 3 | 2 | 19 |

### Backlog

| Group | | Severity | File |
|-------|---|----------|------|
| 1 | [ ] | HIGH | a.py:1 |
`
	result, err := parseTDStats(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Summary.Total != 1 {
		t.Errorf("total: want 1, got %d — the ## Stats block was counted as data", result.Summary.Total)
	}
	if got := result.Severity["HIGH"].Open; got != 1 {
		t.Errorf("HIGH open: want 1, got %d", got)
	}
}
