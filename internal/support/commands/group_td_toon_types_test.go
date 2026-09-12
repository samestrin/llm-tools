package commands

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The highest-risk silent regression in the toon-go migration.
//
// mkRow (group_td.go:1141) reads every cell with a discarded comma-ok
// assertion:
//
//	severity, _ := item["SEVERITY"].(string)
//	attempts, _ := item["ATTEMPTS"].(string)
//
// Hand it a float64 or a bool and the cell renders EMPTY — no error, no
// warning, exit 0. group_td writes into .planning/technical-debt/README.md at
// the end of a code review, where nobody diffs the rendered table against the
// source payload, and total_items still counts the row. The summary says N, the
// table has N rows, and the ones whose text happened to look numeric or boolean
// are hollow.
//
// So the toon reader must project every value to a string at the Document
// boundary, and that has to be asserted the way a hostile reader would: FAIL on
// a non-string. A test written the way mkRow is written — reading with
// .(string) and discarding ok — passes while the code is broken, because it
// reproduces the bug in its own assertion.
//
// PROBLEM is 42 and FIX is true deliberately: both are ordinary finding text
// that TOON types as a number and a boolean.

const toonTypedValues = `findings[1|]{SEVERITY|FILE_LINE|PROBLEM|FIX|CATEGORY|EST_MINUTES|ATTEMPTS}:
  HIGH|"a.go:1"|42|true|null|15|2
`

func TestGroupTDTOONNeverEmitsANonStringCell(t *testing.T) {
	cmd := newGroupTDCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"--content", toonTypedValues, "--format", "toon", "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	// Decoded as RawMessage so the test sees the JSON type that was actually
	// emitted. Unmarshalling into interface{} and reading with a comma-ok
	// .(string) would reproduce mkRow's bug inside the assertion and pass while
	// the code is broken.
	var raw struct {
		Groups []struct {
			Items []map[string]json.RawMessage `json:"items"`
		} `json:"groups"`
		Ungrouped []map[string]json.RawMessage `json:"ungrouped"`
	}
	if err := json.Unmarshal(buf.Bytes(), &raw); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, buf.String())
	}

	items := append([]map[string]json.RawMessage{}, raw.Ungrouped...)
	for _, g := range raw.Groups {
		items = append(items, g.Items...)
	}
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}

	for key, rawVal := range items[0] {
		s := string(rawVal)
		if s == "null" {
			continue // an absent field is not a typed field
		}
		if !strings.HasPrefix(s, `"`) {
			t.Errorf("item[%q] = %s — a non-string reached the caller; "+
				"mkRow renders it as an empty cell with no error", key, s)
		}
	}
}

func TestGroupTDTOONRendersTypedValuesInTheTable(t *testing.T) {
	// The paired assertion, on the markdown TABLE path. --output-file is the
	// only mode that reaches mkRow; group-td's plain text output prints just
	// "FILE_LINE: PROBLEM" and never renders the Fix cell, so it cannot see
	// this regression at all.
	//
	// This exists so the stringification cannot be "simplified away" by
	// loosening the JSON test above: a hollow cell is visible here.
	out := filepath.Join(t.TempDir(), "td.md")
	cmd := newGroupTDCmd()
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"--content", toonTypedValues, "--format", "toon",
		"--output-file", out})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("reading the rendered table: %v", err)
	}
	table := string(b)
	if !strings.Contains(table, "42") {
		t.Errorf("the Problem cell lost its numeric-looking text:\n%s", table)
	}
	if !strings.Contains(table, "true") {
		t.Errorf("the Fix cell lost its boolean-looking text:\n%s", table)
	}
}
