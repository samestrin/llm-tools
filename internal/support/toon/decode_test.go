package toon

import (
	"strings"
	"testing"
)

// These cover the move from the hand-rolled value splitter to toon-go.
//
// The design they pin: the pre-scan isolates ONE array and hands toon-go a
// synthesized canonical document — physical row count, original field list, a
// delimiter toon-go accepts — with the row lines verbatim. Strict mode stays on.
// Trailing blocks and sibling scalars never reach toon-go. Three probe-verified
// facts force that shape: an explicitly written comma is rejected, a truncated
// trailing block fails the whole document, and an unquoted colon silently ends
// an array.
//
// Some of these pass against the old reader. They are here because toon-go
// changes value TYPES and value PARSING, and nothing else in the suite would
// notice a silent regression in either.

// --- the delimiter is DECLARED, for every delimiter TOON allows

func TestDelimiterTabIsReadFromTheHeader(t *testing.T) {
	// The existing suite covers ',' and '|' only. Tab is the third delimiter
	// toon-go supports, and a reader that assumes one mis-splits every row of
	// the other two without complaining.
	src := "rows[1\t]{a\tb\tc}:\n  one\ttwo\tthree\n"
	doc, err := Decode(strings.NewReader(src))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if doc.Delimiter != '\t' {
		t.Errorf("Delimiter = %q, want tab", doc.Delimiter)
	}
	if doc.Rows[0]["b"] != "two" {
		t.Errorf("b = %q, want two", doc.Rows[0]["b"])
	}
}

func TestExplicitCommaDelimiterStillDecodes(t *testing.T) {
	// toon-go REJECTS an explicitly written comma: `invalid delimiter symbol
	// ','`. It is only legal in the omitted form. This payload ships today
	// (toon_test.go:55), so the pre-scan normalizes it before delegating. A
	// design that patches the count in place and forwards the rest breaks here.
	doc, err := Decode(strings.NewReader("rows[1,]{a,b,c}:\n  one,two,three\n"))
	if err != nil {
		t.Fatalf("an explicit comma delimiter is a shipped payload shape: %v", err)
	}
	if doc.Delimiter != ',' {
		t.Errorf("Delimiter = %q, want ','", doc.Delimiter)
	}
	if doc.Rows[0]["b"] != "two" {
		t.Errorf("b = %q, want two", doc.Rows[0]["b"])
	}
}

// --- values reach the caller as strings, whatever TOON calls them

func TestValuesProjectToStrings(t *testing.T) {
	// toon-go types its output: an unquoted 15 decodes to float64(15), true to
	// bool. Document.Rows is map[string]string and three callers read it that
	// way — group_td asserts item["EST_MINUTES"] == "15" as a string. A number
	// arriving as a number makes group_td report zero totals while still
	// looking successful, which is the exact failure class this package exists
	// to remove.
	src := "findings[1|]{est_minutes|zero|frac|flag|blank|quoted}:\n" +
		`  15|0|1.5|true||"42"` + "\n"
	doc, err := Decode(strings.NewReader(src))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	row := doc.Rows[0]
	for _, tc := range []struct{ field, want string }{
		{"est_minutes", "15"},
		{"zero", "0"},
		{"frac", "1.5"},
		{"flag", "true"},
		{"blank", ""},
		// A quoted number and a bare number are indistinguishable to the caller
		// today. That equivalence is load-bearing: atcr quotes some numerics and
		// not others, and no consumer branches on which.
		{"quoted", "42"},
	} {
		if got := row[tc.field]; got != tc.want {
			t.Errorf("%s = %q, want %q", tc.field, got, tc.want)
		}
	}
}

func TestNullAndBooleanTokensProject(t *testing.T) {
	// toon-go returns nil ONLY for the literal token `null`, and "" (a string)
	// for an empty token. So "null" is an exact projection of what the payload
	// said, not a guess, and it matches what the old splitter produced.
	doc, err := Decode(strings.NewReader("r[1|]{n|e|t|f}:\n  null||true|false\n"))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	row := doc.Rows[0]
	for _, tc := range []struct{ field, want string }{
		{"n", "null"},
		{"e", ""},
		{"t", "true"},
		{"f", "false"},
	} {
		if got := row[tc.field]; got != tc.want {
			t.Errorf("%s = %q, want %q", tc.field, got, tc.want)
		}
	}
}

// --- APPROVED DELTAS: toon-go parses values now, and it parses them differently

func TestNumericTokensProjectPredictably(t *testing.T) {
	// APPROVED BEHAVIOR CHANGE. toon-go hands back a float64 and the raw token
	// is unreachable, so an unquoted number loses its exact digits. The rule for
	// payload authors is: quote anything whose exact digits matter. atcr already
	// quotes what matters.
	//
	// Pinned rather than left accidental — the day someone changes the format
	// verb, this fails instead of silently reshaping every consumer's numbers.
	src := "r[1|]{two_dp|exp|one_dp|negzero|leadzero|bigint}:\n" +
		"  0.50|1e3|1.0|-0|007|9007199254740993\n"
	doc, err := Decode(strings.NewReader(src))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	row := doc.Rows[0]
	for _, tc := range []struct{ field, want string }{
		{"two_dp", "0.5"},
		{"exp", "1000"},
		{"one_dp", "1"},
		{"negzero", "0"},
		// A leading zero makes it a string, so it survives exactly.
		{"leadzero", "007"},
		// Beyond float64's exact integer range, the last digit is lost.
		{"bigint", "9007199254740992"},
	} {
		if got := row[tc.field]; got != tc.want {
			t.Errorf("%s = %q, want %q", tc.field, got, tc.want)
		}
	}
}

func TestUnquotedPaddingIsTrimmedAndQuotedPaddingSurvives(t *testing.T) {
	// APPROVED BEHAVIOR CHANGE for the unquoted half. The quoted half is the
	// escape hatch, and it must keep working or the rule "quote what matters"
	// is a lie.
	doc, err := Decode(strings.NewReader("r[1|]{bare|quoted}:\n  pad me  |\"  keep me  \"\n"))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if got := doc.Rows[0]["bare"]; got != "pad me" {
		t.Errorf("bare = %q, want %q (unquoted padding is trimmed)", got, "pad me")
	}
	if got := doc.Rows[0]["quoted"]; got != "  keep me  " {
		t.Errorf("quoted = %q, want %q (quoting preserves padding)", got, "  keep me  ")
	}
}

func TestAnUnknownEscapeIsAnError(t *testing.T) {
	// APPROVED BEHAVIOR CHANGE. The old splitter preserved `\q` verbatim to
	// avoid corrupting a Windows path. toon-go rejects any escape outside
	// \\ \" \n \r \t. An error is louder than a guess, which is the trade made
	// here — but it IS a change, so it is pinned.
	if _, err := Decode(strings.NewReader("r[1|]{a}:\n  \"a\\qb\"\n")); err == nil {
		t.Fatal("an unknown escape decoded cleanly")
	}
}

func TestAnUnquotedColonInAValueIsAnError(t *testing.T) {
	// APPROVED BEHAVIOR CHANGE, and the one that must never be silent. toon-go
	// ENDS the array at a row whose unquoted content carries a colon. If that
	// went unreported the caller would get a short list that looks complete —
	// the failure class this package exists to remove. So it must surface as an
	// error, not as fewer rows.
	src := "findings[1|]{a|b}:\n  see line 42: fix it|x\n"
	_, err := Decode(strings.NewReader(src))
	if err == nil {
		t.Fatal("an unquoted colon decoded cleanly — the array ends there silently")
	}
}

// --- RED: a length marker is valid TOON the old reader cannot read

func TestLengthMarkerDecodes(t *testing.T) {
	// `[#2]` is the optional length-marker form toon-go emits under
	// WithLengthMarkers. The hand-rolled reader ran Atoi over "#2" and failed.
	// Nothing in llm-tools produced it, so it went unnoticed until a payload
	// from another encoder arrived.
	doc, err := Decode(strings.NewReader("findings[#2]{a}:\n  x\n  y\n"))
	if err != nil {
		t.Fatalf("a length marker is valid TOON: %v", err)
	}
	if len(doc.Rows) != 2 {
		t.Fatalf("got %d rows, want 2", len(doc.Rows))
	}
	if doc.Declared != 2 {
		t.Errorf("Declared = %d, want 2", doc.Declared)
	}
}

// --- isolation: what follows the array must not be able to break it

func TestATruncatedTrailingBlockDoesNotBreakTheFirstArray(t *testing.T) {
	// atcr Epic 42.0 appends a help block, and that block is itself paginated —
	// so it can declare 9 and carry 1. Handed to toon-go as part of one
	// document, it fails with `tabular length mismatch` at line 4 and takes the
	// findings array down with it. The array must be isolated before delegating.
	src := "findings[1|]{a}:\n  x\nhelp[9|]{cmd}:\n  atcr verify <id>\n"
	doc, err := Decode(strings.NewReader(src))
	if err != nil {
		t.Fatalf("a truncated trailing block broke the first array: %v", err)
	}
	if len(doc.Rows) != 1 || doc.Rows[0]["a"] != "x" {
		t.Errorf("Rows = %v, want one row a=x", doc.Rows)
	}
}

func TestABlankLineBetweenRowsIsSkipped(t *testing.T) {
	// Today's reader skips blank lines between rows; strict toon-go rejects them
	// (`blank line inside tabular array`). The synthesized document drops them,
	// so today's behavior survives.
	doc, err := Decode(strings.NewReader("r[2|]{a}:\n  x\n\n  y\n"))
	if err != nil {
		t.Fatalf("a blank line between rows must be skipped: %v", err)
	}
	if len(doc.Rows) != 2 {
		t.Fatalf("got %d rows, want 2", len(doc.Rows))
	}
}

func TestALongRowIsStillAnError(t *testing.T) {
	// Strict mode gives the row-width gate for free. Non-strict mode drops the
	// extra value and is undetectable, so this pins the design choice.
	if _, err := Decode(strings.NewReader("r[1|]{a|b}:\n  1|2|3\n")); err == nil {
		t.Fatal("a row wider than its header decoded cleanly")
	}
}

func TestLineNumbersSurviveLeadingBlankLines(t *testing.T) {
	// The header is on line 3, so the bad row is line 4. toon-go's errors carry
	// only a line number and its error type is unexported, so the only way to
	// align them is to pad the synthesized document with the same number of
	// leading blank lines.
	src := "\n\nr[2|]{a|b}:\n  1|2|3\n"
	_, err := Decode(strings.NewReader(src))
	if err == nil {
		t.Fatal("a malformed row decoded cleanly")
	}
	if !strings.Contains(err.Error(), "line 4") {
		t.Errorf("error %q does not name line 4 — line numbers are misaligned", err)
	}
}

// --- RED: a document that is not a tabular array at all

func TestDecodeAnyReadsALeadingScalarDocument(t *testing.T) {
	// The shape `llm-filesystem-axi get-file-info` emits: scalar keys, no table.
	// Decode is tabular-only by contract and rejects it; DecodeAny is the
	// general entry point.
	//
	// NOTE: this fixture is written from the documented shape, not captured from
	// the binary. llm-filesystem-axi emits nothing and exits 0 for every command
	// in this environment, including --format json. Re-validate against real
	// output when that consumer is migrated off gotoon.
	src := "path: /tmp/x\nsize: 12\nis_dir: false\n"
	doc, err := DecodeAny(strings.NewReader(src))
	if err != nil {
		t.Fatalf("DecodeAny: %v", err)
	}
	m, ok := doc.(map[string]any)
	if !ok {
		t.Fatalf("DecodeAny returned %T, want a map", doc)
	}
	if got, _ := m["path"].(string); got != "/tmp/x" {
		t.Errorf("path = %v, want /tmp/x", m["path"])
	}
	if _, ok := m["size"]; !ok {
		t.Error("size is missing")
	}
	if got, ok := m["is_dir"].(bool); !ok || got {
		t.Errorf("is_dir = %v (%T), want the boolean false", m["is_dir"], m["is_dir"])
	}
}

func TestDecodeAnyKeepsASiblingScalar(t *testing.T) {
	// The shape `llm-filesystem-axi list-directory` emits. It decoded before and
	// silently dropped `total`, so the caller could not tell a capped listing
	// from a complete one.
	src := "entries[2]{name,type}:\n  a,dir\n  b,file\ntotal: 6\n"
	doc, err := DecodeAny(strings.NewReader(src))
	if err != nil {
		t.Fatalf("DecodeAny: %v", err)
	}
	m, ok := doc.(map[string]any)
	if !ok {
		t.Fatalf("DecodeAny returned %T, want a map", doc)
	}
	if _, ok := m["total"]; !ok {
		t.Fatal("total was dropped — a capped listing is indistinguishable from a complete one")
	}
	entries, ok := m["entries"].([]any)
	if !ok || len(entries) != 2 {
		t.Fatalf("entries = %#v, want 2 rows", m["entries"])
	}
}

func TestTabularSiblingStillReachesMeta(t *testing.T) {
	// The same sibling, through the tabular entry point, still arrives as a
	// string in Meta. That is what toon parse emits and it must not change.
	doc, err := Decode(strings.NewReader("entries[2,]{name,type}:\n  a,dir\n  b,file\ntotal: 6\n"))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if got := doc.Meta["total"]; got != "6" {
		t.Errorf("Meta[total] = %q, want \"6\"", got)
	}
}

// --- toon-go accepts these; the wrapper must not

func TestEmptyInputIsStillAnError(t *testing.T) {
	// toon-go returns (nil, nil) for empty input. An empty success is the
	// failure mode this package removes, so the wrapper rejects it.
	if _, err := Decode(strings.NewReader("")); err == nil {
		t.Fatal("empty input decoded cleanly")
	}
}

func TestHeaderlessPayloadIsStillAnError(t *testing.T) {
	// toon-go also returns (nil, nil) for a bare indented line with no header.
	if _, err := Decode(strings.NewReader("  just|a|row\n")); err == nil {
		t.Fatal("a headerless payload decoded cleanly")
	}
}
