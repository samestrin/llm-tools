package toon

import (
	"fmt"
	"io"
	"strings"
	"testing"
)

// repeatReader yields a fixed number of filler bytes without ever holding them.
// Proving a 64 MiB ceiling by allocating 64 MiB would make the test the exact
// thing the ceiling exists to prevent.
type repeatReader struct{ remaining int }

func (r *repeatReader) Read(p []byte) (int, error) {
	if r.remaining <= 0 {
		return 0, io.EOF
	}
	n := len(p)
	if n > r.remaining {
		n = r.remaining
	}
	for i := range p[:n] {
		p[i] = 'x'
	}
	r.remaining -= n
	return n, nil
}

// Findings from the atcr reviewer panel on this change (review
// 2026-09-11_feat-toon-go-decoder, 11 of 16 agents reporting). Each test names
// the finding it pins so a later change cannot quietly undo the fix.

// --- HIGH (mira): io.ReadAll in DecodeAny is unbounded

func TestDecodeAnyRefusesAnOversizedDocument(t *testing.T) {
	// DecodeAny is the public entry point behind `toon parse --shape document`,
	// and it read its input with no ceiling. toon-go's API takes a []byte, so
	// the whole document has to be resident — but "has to be resident" is not
	// the same as "may be any size", and a reader is not always a file whose
	// size can be checked first.
	// Generated rather than allocated: materialising 64 MiB to prove a 64 MiB
	// ceiling would make the test the very thing the ceiling exists to prevent.
	big := io.MultiReader(
		strings.NewReader("k: "),
		&repeatReader{remaining: MaxDocumentBytes + 1},
	)
	_, err := DecodeAny(big)
	if err == nil {
		t.Fatal("an oversized document decoded cleanly")
	}
	if !strings.Contains(err.Error(), "too large") {
		t.Errorf("error %q does not say the document is too large", err)
	}
}

func TestDecodeAnyAcceptsADocumentAtTheLimit(t *testing.T) {
	// The cap must not reject an ordinary payload. A findings review runs to
	// hundreds of rows with KB-scale free text, so the ceiling exists to stop
	// /dev/zero, not to stop real work.
	doc, err := DecodeAny(strings.NewReader("k: " + strings.Repeat("x", 64*1024) + "\n"))
	if err != nil {
		t.Fatalf("a 64KB document must decode: %v", err)
	}
	if _, ok := doc.(map[string]any); !ok {
		t.Errorf("got %T, want a map", doc)
	}
}

// --- MED (mira, otto, pace): the emptiness check missed a top-level slice

func TestDecodeAnyRejectsAnEmptyTopLevelArray(t *testing.T) {
	// `[0]:` decodes to a top-level []any, not a map, so it fell straight
	// through the type switch and was returned as a valid empty document.
	// Probed: toon.DecodeString("[0]:\n") yields []interface{}{}.
	if _, err := DecodeAny(strings.NewReader("[0]:\n")); err == nil {
		t.Fatal("an empty top-level array decoded cleanly")
	}
}

func TestDecodeAnyKeepsANonEmptyTopLevelArray(t *testing.T) {
	// The other half of the same fix: `[2]: a,b` is a real document and the
	// emptiness guard must not swallow it.
	doc, err := DecodeAny(strings.NewReader("[2]: a,b\n"))
	if err != nil {
		t.Fatalf("a top-level array is a valid document: %v", err)
	}
	arr, ok := doc.([]any)
	if !ok || len(arr) != 2 {
		t.Fatalf("got %#v, want a 2-element array", doc)
	}
}

// --- MED (greta): only the FIRST sibling line was kept

func TestEverySiblingLineReachesMeta(t *testing.T) {
	// Document.Meta is documented as holding "`key: value` sibling lines",
	// plural, but the row loop broke after the first match and dropped the
	// rest. atcr emits one sibling today, so nothing lost data yet — but the
	// contract said otherwise and a second sibling would have vanished in
	// silence.
	src := "findings[1|]{a}:\n  x\ntruncated: true\ntotal: 42\n"
	doc, err := Decode(strings.NewReader(src))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if got := doc.Meta["truncated"]; got != "true" {
		t.Errorf("Meta[truncated] = %q, want true", got)
	}
	if got := doc.Meta["total"]; got != "42" {
		t.Errorf("Meta[total] = %q, want 42 — a second sibling line was dropped", got)
	}
}

func TestATrailingBlockStillStopsSiblingCollection(t *testing.T) {
	// Collecting every sibling must not turn into consuming the rest of the
	// file. A following block header is not metadata, and siblingKV already
	// refuses a key carrying brackets — this pins that the two rules compose.
	src := "findings[1|]{a}:\n  x\ntruncated: false\nhelp[1|]{cmd}:\n  atcr verify <id>\n"
	doc, err := Decode(strings.NewReader(src))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if doc.Meta["truncated"] != "false" {
		t.Errorf("Meta[truncated] = %q, want false", doc.Meta["truncated"])
	}
	if _, ok := doc.Meta["cmd"]; ok {
		t.Error("a trailing block header was absorbed as sibling metadata")
	}
	if len(doc.Meta) != 1 {
		t.Errorf("Meta = %v, want only the sibling scalar", doc.Meta)
	}
}

// --- MED (dax, mira, otto, pace): synthesize claimed a test that did not exist

func TestABlankLineBetweenRowsShiftsReportedLineNumbers(t *testing.T) {
	// HONEST PIN OF A KNOWN LIMITATION, not a fix.
	//
	// synthesize dropped blank lines from between rows and its comment claimed
	// the resulting line shift was "pinned by test". No such test existed. Four
	// reviewers flagged the claim, and the drift is real — measured through the
	// binary:
	//
	//   r[3|]{a|b}:        line 1
	//     1|2              line 2
	//                      line 3   <- blank, dropped
	//     3|4|5            line 4   <- reported as line 3
	//
	// It cannot simply be fixed by forwarding the blank: strict toon-go rejects
	// a blank line inside a tabular array outright ("blank line inside tabular
	// array"), and the old reader skipped them, so forwarding would turn a
	// payload that parses today into an error.
	//
	// So the behaviour is asserted as it actually is. If a later change remaps
	// the number correctly, this test fails and should be updated to demand the
	// true line — which is the point of pinning it.
	_, err := Decode(strings.NewReader("r[3|]{a|b}:\n  1|2\n\n  3|4|5\n"))
	if err == nil {
		t.Fatal("a malformed row decoded cleanly")
	}
	if !strings.Contains(err.Error(), "line 3") {
		t.Errorf("error %q: expected the known off-by-one (line 3 for a row on line 4)", err)
	}
}

func TestLineNumbersAreExactWithoutAnInteriorBlank(t *testing.T) {
	// The companion: with no blank to drop, the reported line is exact. This is
	// what makes the drift above a blank-line artefact rather than a general
	// misalignment.
	_, err := Decode(strings.NewReader("r[3|]{a|b}:\n  1|2\n  3|4|5\n"))
	if err == nil {
		t.Fatal("a malformed row decoded cleanly")
	}
	if !strings.Contains(err.Error(), "line 3") {
		t.Errorf("error %q does not name line 3", err)
	}
}

// --- MED (mira): a scanner error arrived with no line context

func TestAnOverlongLineErrorNamesTheLine(t *testing.T) {
	// Every other error in this package reads `toon: line N: ...`. A line past
	// the scanner ceiling returned bufio's bare "token too long" with no
	// context at all, which is the least useful moment to lose it.
	src := "r[1|]{a}:\n  " + strings.Repeat("x", maxLine+1) + "\n"
	_, err := Decode(strings.NewReader(src))
	if err == nil {
		t.Fatal("a line past the scanner ceiling decoded cleanly")
	}
	if !strings.Contains(err.Error(), "toon:") {
		t.Errorf("error %q is not reported in this package's format", err)
	}
	if !strings.Contains(fmt.Sprint(err), "line") {
		t.Errorf("error %q does not name a line", err)
	}
}
