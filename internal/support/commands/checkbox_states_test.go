package commands

import "testing"

func TestStateOfRecognisesEveryDeclaredMarker(t *testing.T) {
	cases := map[string]string{
		"[ ]": "open",
		"[/]": "deferred",
		"[x]": "resolved",
		"[X]": "resolved",
		"[-]": "unreproducible",
	}
	for marker, wantKey := range cases {
		state, ok := stateOf(marker)
		if !ok {
			t.Errorf("%s: not recognised as a checkbox", marker)
			continue
		}
		if state.Key != wantKey {
			t.Errorf("%s: want key %q, got %q", marker, wantKey, state.Key)
		}
	}
}

func TestStateOfRejectsNonCheckboxCells(t *testing.T) {
	for _, cell := range []string{
		"", "HIGH", "x", "[]", "[ x ]", "[[x]]",
		"the doc says [x] means resolved", // prose, not a marker
		"[?]",                             // undeclared state
	} {
		if _, ok := stateOf(cell); ok {
			t.Errorf("%q was accepted as a checkbox", cell)
		}
	}
}

func TestStateOfTrimsSurroundingWhitespace(t *testing.T) {
	if _, ok := stateOf("  [-]  "); !ok {
		t.Error("a padded marker should still be recognised — table cells carry padding")
	}
}

// Aliases collapse: "[x]" and "[X]" are one state, so a caller enumerating
// states does not see "resolved" twice.
func TestStateKeysCollapsesAliases(t *testing.T) {
	keys := stateKeys()
	seen := map[string]int{}
	for _, k := range keys {
		seen[k]++
	}
	for k, n := range seen {
		if n != 1 {
			t.Errorf("key %q appears %d times", k, n)
		}
	}
	want := []string{"open", "deferred", "resolved", "unreproducible"}
	if len(keys) != len(want) {
		t.Fatalf("want %v, got %v", want, keys)
	}
	for i := range want {
		if keys[i] != want[i] {
			t.Errorf("position %d: want %q, got %q — declaration order is the column order", i, want[i], keys[i])
		}
	}
}

func TestIsSeverityValue(t *testing.T) {
	for _, ok := range []string{"CRITICAL", "HIGH", "medium", " low "} {
		if !isSeverityValue(ok) {
			t.Errorf("%q should be a severity value", ok)
		}
	}
	for _, notOK := range []string{"", "Severity", "URGENT", "a.py:1"} {
		if isSeverityValue(notOK) {
			t.Errorf("%q should not be a severity value", notOK)
		}
	}
}
