package commands

import "testing"

// "[-]" (UNREPRODUCIBLE — closed without a fix) is a distinct outcome from
// resolved: nothing was changed, so folding it into Resolved overstates what
// the work actually closed.
func TestParseTDStatsCountsUnreproducible(t *testing.T) {
	content := `| Group | | Severity | File |
|-------|---|----------|------|
| 1 | [ ] | HIGH | a.py |
| 1 | [-] | HIGH | b.py |
| 1 | [-] | MEDIUM | c.py |
| 1 | [x] | MEDIUM | d.py |
`
	result, err := parseTDStats(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	high := result.Severity["HIGH"]
	if high.Open != 1 || high.Unreproducible != 1 || high.Resolved != 0 {
		t.Errorf("HIGH: want open=1 unreproducible=1 resolved=0, got %+v", high)
	}
	medium := result.Severity["MEDIUM"]
	if medium.Unreproducible != 1 || medium.Resolved != 1 {
		t.Errorf("MEDIUM: want unreproducible=1 resolved=1, got %+v", medium)
	}

	if result.Summary.Unreproducible != 2 {
		t.Errorf("summary unreproducible: want 2, got %d", result.Summary.Unreproducible)
	}
	// Total is the sum of ALL states — a Total that omits them under-reports
	// the file, which is what sent the coned README's counts out of true.
	if result.Summary.Total != 4 {
		t.Errorf("summary total: want 4, got %d", result.Summary.Total)
	}
}

// Bug 2: the checkbox column was detected only from "[ ] [x] [X] [/]", so a
// table whose rows are all "[-]" had no detectable checkbox column and vanished
// entirely.
func TestParseTDStatsCountsTableOfOnlyUnreproducibleRows(t *testing.T) {
	content := `| Group | | Severity | File |
|-------|---|----------|------|
| 1 | [-] | HIGH | a.py |
| 2 | [-] | LOW | b.py |
`
	result, err := parseTDStats(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Summary.Unreproducible != 2 || result.Summary.Total != 2 {
		t.Errorf("want unreproducible=2 total=2, got %+v", result.Summary)
	}
	if result.Severity["HIGH"].Unreproducible != 1 {
		t.Errorf("HIGH: want unreproducible=1, got %+v", result.Severity["HIGH"])
	}
}

// A "[-]" row must not be counted as open — it is closed, just not by a fix.
func TestParseTDStatsDoesNotCountUnreproducibleAsOpen(t *testing.T) {
	content := `| 1 | [-] | HIGH | a.py |
`
	result, err := parseTDStats(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Summary.Open != 0 {
		t.Errorf("want open=0, got %d", result.Summary.Open)
	}
}
