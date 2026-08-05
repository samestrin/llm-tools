package commands

import (
	"strings"
	"testing"
)

// td-filter, td-matrix and group-td all select rows with `cells[1] != "[ ]"`,
// which admits only open rows and therefore already excludes "[-]". That is the
// correct behaviour — an unreproducible row is closed and must not appear in a
// work queue, a cross-tab of open items, or an active-group count.
//
// It is correct by accident, though: those three read the checkbox at a fixed
// index while td-stats auto-detects the column, so a table laid out differently
// is read one way by one command and another way by the next. These tests pin
// the behaviour that matters (closed rows stay out) so a future move to shared
// detection cannot regress it silently.

const openOnlyFixture = `# Tech Debt

### [2026-08-01] From Sprint: s

| Group | | Severity | File | Problem | Fix | Category | Est Minutes |
|-------|---|----------|------|---------|-----|----------|-------------|
| 1 | [ ] | HIGH | open.py:1 | Open | Fix it | correctness | 15 |
| 1 | [-] | HIGH | closed.py:1 | Could not reproduce | none | correctness | 15 |
| 2 | [x] | MEDIUM | fixed.py:1 | Fixed | done | correctness | 15 |
| 2 | [/] | LOW | later.py:1 | Deferred | later | correctness | 15 |
`

func TestTDFilterExcludesClosedAndDeferredRows(t *testing.T) {
	result, err := filterTD(openOnlyFixture, TDFilterOpts{Mode: "all", Max: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Items) != 1 {
		t.Fatalf("want only the open row, got %d: %+v", len(result.Items), result.Items)
	}
	if result.Items[0].FileLine != "open.py:1" {
		t.Errorf("want open.py:1, got %+v", result.Items[0])
	}
	for _, row := range result.Items {
		if row.Checkbox == "[-]" {
			t.Error("an unreproducible row entered the /resolve-td work queue")
		}
	}
}

func TestTDMatrixCountsOpenRowsOnly(t *testing.T) {
	result := buildTDMatrix(openOnlyFixture)

	if result.Total != 1 {
		t.Errorf("want 1 open row counted, got %d — a closed row entered the cross-tab", result.Total)
	}
	if got := result.Counts["1"]["HIGH"]; got != 1 {
		t.Errorf("group 1 HIGH: want 1 open, got %d", got)
	}
	if got := result.Counts["2"]["MEDIUM"]; got != 0 {
		t.Errorf("group 2 MEDIUM: a resolved row was counted (%d)", got)
	}
}

// A group whose only remaining row is "[-]" is not active: there is no open
// work in it, so it must not hold an active group number.
func TestGroupTDTreatsUnreproducibleAsInactive(t *testing.T) {
	_, isUnchecked, ok := parseTableRowForGroupState("| 3 | [-] | HIGH | closed.py:1 | p | f | c | 5 |")
	if !ok {
		t.Fatal("a numeric group row should parse")
	}
	if isUnchecked {
		t.Error("an unreproducible row was treated as open, keeping a dead group active")
	}

	_, isUnchecked, ok = parseTableRowForGroupState("| 3 | [ ] | HIGH | open.py:1 | p | f | c | 5 |")
	if !ok || !isUnchecked {
		t.Error("an open row must still count as active")
	}
}

// Guard the guard: the fixture must actually contain the closed states, or
// every assertion above passes vacuously.
func TestOpenOnlyFixtureContainsTheClosedStates(t *testing.T) {
	for _, marker := range []string{"[ ]", "[-]", "[x]", "[/]"} {
		if !strings.Contains(openOnlyFixture, "| "+marker+" |") {
			t.Errorf("fixture lost its %s row", marker)
		}
	}
}
