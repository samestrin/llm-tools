package commands

import "strings"

// Checkbox states a technical-debt row can carry.
//
// Every command that reads a TD table asks the same two questions — "is this
// cell a checkbox?" and "which state is it?" — and each used to answer them
// with its own literal switch. Those switches drifted: td-stats could not
// detect a column whose rows all used a state it did not list, and td-clean
// treated a section holding only such rows as empty and deleted it.
//
// One table answers both questions for every caller.
type checkboxState struct {
	// Marker is the cell content, matched against the whole trimmed cell so a
	// literal "[x]" inside prose is never mistaken for a checkbox.
	Marker string
	// Key identifies the state in counts and JSON output. Aliases share a Key.
	Key string
	// Column is the Stats-table heading for this state.
	Column string
	// Always renders this state's column even when the file contains none of
	// it. States added after the original three are opt-in so an existing TD
	// README's Stats block is left byte-identical until it actually uses the
	// new state.
	Always bool
}

// checkboxStateOrder is both the recognised set and the column order. To add a
// state, add one entry here (and, if it needs to appear in JSON, one field on
// TDStatsSeverity/TDStatsTotals):
//
//	{"[_]", "<key>", "<Column>", false},
var checkboxStateOrder = []checkboxState{
	{Marker: "[ ]", Key: "open", Column: "Open", Always: true},
	{Marker: "[/]", Key: "deferred", Column: "Deferred", Always: true},
	{Marker: "[x]", Key: "resolved", Column: "Resolved", Always: true},
	{Marker: "[X]", Key: "resolved", Column: "Resolved", Always: true},
}

// stateOf returns the state a table cell denotes. The cell is trimmed and
// matched whole — a substring match would classify prose mentioning "[x]" as a
// resolved row.
func stateOf(cell string) (checkboxState, bool) {
	trimmed := strings.TrimSpace(cell)
	for _, state := range checkboxStateOrder {
		if state.Marker == trimmed {
			return state, true
		}
	}
	return checkboxState{}, false
}

// isCheckboxCell reports whether a cell is a recognised checkbox marker.
func isCheckboxCell(cell string) bool {
	_, ok := stateOf(cell)
	return ok
}

// rowHasCheckbox reports whether any cell of a split table row is a checkbox,
// which is what distinguishes a data row from a header or a prose table.
func rowHasCheckbox(cells []string) bool {
	for _, cell := range cells {
		if isCheckboxCell(cell) {
			return true
		}
	}
	return false
}

// isSeverityValue reports whether a cell holds a severity level, letting a
// table with no header row still be read. Kept in step with severityOrder,
// which is the display order for the same set.
func isSeverityValue(cell string) bool {
	trimmed := strings.ToUpper(strings.TrimSpace(cell))
	for _, severity := range severityOrder {
		if severity == trimmed {
			return true
		}
	}
	return false
}
