package toon

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// atcrGolden is atcr's own encoder fixture, byte for byte
// (atcr/internal/report/testdata/report.axi). It is the contract this reader
// exists to consume: atcr already emits TOON via `report --format axi`, and the
// cadence skills were throwing it away and re-escaping pipes by hand.
const atcrGolden = `findings[2|]{severity|"file:line"|problem|fix|category|est_minutes|evidence|reviewers|confidence}:
  CRITICAL|"auth.go:42"|token never expires|check expiry|security|15|expiresAt unread|greta,host|HIGH
  LOW|"util.go:7"|unused var|""|style|0|""|otto|MEDIUM
`

// --- AC 1: a valid payload decodes to ordered rows, delimiter read from header

func TestDecodesTheAtcrGolden(t *testing.T) {
	doc, err := Decode(strings.NewReader(atcrGolden))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if doc.Name != "findings" {
		t.Errorf("Name = %q, want findings", doc.Name)
	}
	wantFields := []string{"severity", "file:line", "problem", "fix", "category",
		"est_minutes", "evidence", "reviewers", "confidence"}
	if len(doc.Fields) != len(wantFields) {
		t.Fatalf("Fields = %v, want %v", doc.Fields, wantFields)
	}
	for i, f := range wantFields {
		if doc.Fields[i] != f {
			t.Errorf("Fields[%d] = %q, want %q", i, doc.Fields[i], f)
		}
	}
	if len(doc.Rows) != 2 {
		t.Fatalf("got %d rows, want 2", len(doc.Rows))
	}
	// The header field is `"file:line"` QUOTED, because a colon forces quoting.
	// A reader that keeps the quotes would key every consumer wrong.
	if got := doc.Rows[0]["file:line"]; got != "auth.go:42" {
		t.Errorf("file:line = %q, want auth.go:42", got)
	}
	if got := doc.Rows[0]["severity"]; got != "CRITICAL" {
		t.Errorf("severity = %q, want CRITICAL", got)
	}
	if got := doc.Rows[1]["fix"]; got != "" {
		t.Errorf(`fix = %q, want "" (the empty-string form, not two quote chars)`, got)
	}
}

func TestDelimiterIsReadFromTheHeaderNotAssumed(t *testing.T) {
	// A reader that hardcodes `|` silently mis-splits any other delimiter, and
	// the header declares it precisely so it does not have to be guessed.
	src := "rows[1,]{a,b,c}:\n  one,two,three\n"
	doc, err := Decode(strings.NewReader(src))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if doc.Delimiter != ',' {
		t.Errorf("Delimiter = %q, want ','", doc.Delimiter)
	}
	if doc.Rows[0]["b"] != "two" {
		t.Errorf("b = %q, want two", doc.Rows[0]["b"])
	}
}

// --- AC 2: the declared count and the column count are GATES

func TestDeclaredCountIsAGate(t *testing.T) {
	for _, tc := range []struct{ name, src string }{
		{"too few", "findings[2|]{a}:\n  x\n"},
		{"too many", "findings[1|]{a}:\n  x\n  y\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Decode(strings.NewReader(tc.src))
			if err == nil {
				t.Fatal("a row count that disagrees with the header built cleanly")
			}
			// Both numbers, so the reader says what it expected AND what it got.
			for _, want := range []string{"1", "2"} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error %q does not name %q", err, want)
				}
			}
		})
	}
}

func TestARowWithTheWrongColumnCountIsAnError(t *testing.T) {
	src := "findings[1|]{a|b|c}:\n  one|two\n"
	_, err := Decode(strings.NewReader(src))
	if err == nil {
		t.Fatal("a short row built cleanly — every consumer would misalign")
	}
	if !strings.Contains(err.Error(), "line 2") {
		t.Errorf("error %q does not name the line", err)
	}
}

// --- AC 3: quoting round-trips

func TestQuotingRoundTrips(t *testing.T) {
	src := "r[1|]{empty|piped|esc|plain|num}:\n" +
		`  ""|"a|b"|"tab\there\nand \"quoted\" and \\"|plain|"42"` + "\n"
	doc, err := Decode(strings.NewReader(src))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	row := doc.Rows[0]
	for _, tc := range []struct{ field, want string }{
		{"empty", ""},
		{"piped", "a|b"}, // the delimiter survives INSIDE a quoted value
		{"esc", "tab\there\nand \"quoted\" and \\"},
		{"plain", "plain"},
		{"num", "42"}, // quoted so it stays a string, and stays a string here
	} {
		if got := row[tc.field]; got != tc.want {
			t.Errorf("%s = %q, want %q", tc.field, got, tc.want)
		}
	}
}

// --- AC 4: the empty form is well-formed, not an error

func TestEmptyArrayFormIsNotAnError(t *testing.T) {
	doc, err := Decode(strings.NewReader("findings[0]:\n"))
	if err != nil {
		t.Fatalf("a zero-findings review is a well-formed payload: %v", err)
	}
	if doc.Name != "findings" {
		t.Errorf("Name = %q, want findings", doc.Name)
	}
	if len(doc.Rows) != 0 {
		t.Errorf("got %d rows, want 0", len(doc.Rows))
	}
}

// --- AC 6: malformed input errors with a line number, and never panics

func TestMalformedInputIsAnErrorNotAPanic(t *testing.T) {
	for name, src := range map[string]string{
		"no header":            "  just|a|row\n",
		"unterminated quote":   "r[1|]{a}:\n  \"never closed\n",
		"count not a number":   "r[x|]{a}:\n  v\n",
		"missing colon":        "r[1|]{a}\n  v\n",
		"empty input":          "",
		"header only, count>0": "r[2|]{a}:\n",
	} {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("panicked on malformed input: %v", r)
				}
			}()
			if _, err := Decode(strings.NewReader(src)); err == nil {
				t.Fatalf("malformed input decoded cleanly")
			}
		})
	}
}

func TestDecodeFileReportsTheFileName(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "report.axi")
	if err := os.WriteFile(p, []byte(atcrGolden), 0o644); err != nil {
		t.Fatal(err)
	}
	doc, err := DecodeFile(p)
	if err != nil {
		t.Fatalf("DecodeFile: %v", err)
	}
	if len(doc.Rows) != 2 {
		t.Errorf("got %d rows, want 2", len(doc.Rows))
	}
}

// --- findings from the hostile pass over this package's own diff

func TestDuplicateFieldNamesAreAnError(t *testing.T) {
	// Two columns sharing a name collapse to one map key, last value wins, and
	// the caller loses a column with no signal. Silently-wrong is the failure
	// mode this reader exists to remove, so it is an error.
	_, err := Decode(strings.NewReader("r[1|]{a|b|a}:\n  1|2|3\n"))
	if err == nil {
		t.Fatal("a duplicate field name decoded cleanly, losing a column")
	}
	if !strings.Contains(err.Error(), "a") {
		t.Errorf("error %q does not name the duplicated field", err)
	}
}

func TestAnEmptyFieldNameIsAnError(t *testing.T) {
	// `{}` with a non-zero count yields one field named "" — a key no consumer
	// can ask for. The zero-row EMPTY FORM (`findings[0]:`) is the legitimate
	// way to say "no fields", and it has no braces at all.
	_, err := Decode(strings.NewReader("r[1|]{}:\n  x\n"))
	if err == nil {
		t.Fatal("an empty field name decoded cleanly")
	}
}

func TestALongFieldSurvives(t *testing.T) {
	// A PROBLEM or FIX can be long. bufio.Scanner's 64KB default token would
	// truncate the line, and a truncated finding still looks like a finding.
	long := strings.Repeat("x", 200_000)
	doc, err := Decode(strings.NewReader("r[1|]{a|b}:\n  " + long + "|tail\n"))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if got := doc.Rows[0]["a"]; len(got) != len(long) {
		t.Errorf("field truncated to %d bytes, want %d", len(got), len(long))
	}
	if doc.Rows[0]["b"] != "tail" {
		t.Errorf("b = %q, want tail", doc.Rows[0]["b"])
	}
}

// --- what `atcr report --format axi` ACTUALLY emits
//
// The golden fixture this package was first written against is the PURE encoder
// output (report.Render(FormatAXI)). The shipping CLI goes through
// RenderAXIPaginated, which appends a `truncated: <bool>` SIBLING line after the
// array. Validating against the fixture instead of the command is the mistake
// this project's own rule warns about: only the binary knows what ships.

const atcrCLIOutput = `findings[2|]{severity|"file:line"|problem|fix}:
  CRITICAL|"auth.go:42"|token never expires|check expiry
  LOW|"util.go:7"|unused var|""
truncated: false
`

func TestASiblingLineIsMetadataNotARow(t *testing.T) {
	doc, err := Decode(strings.NewReader(atcrCLIOutput))
	if err != nil {
		t.Fatalf("real CLI output failed to parse: %v", err)
	}
	if len(doc.Rows) != 2 {
		t.Fatalf("got %d rows, want 2 — a sibling line was eaten as a row", len(doc.Rows))
	}
	if got := doc.Meta["truncated"]; got != "false" {
		t.Errorf("Meta[truncated] = %q, want false", got)
	}
}

func TestATruncatedPayloadIsNotAnError(t *testing.T) {
	// atcr's CONSUMER CONTRACT, verbatim: "when truncated, this payload is
	// intentionally NOT length-round-trippable — the header declares N (the true
	// total) while fewer than N rows are physically present ... A consumer must
	// read `truncated` and the header N as authoritative rather than
	// length-checking the array against its physical rows."
	//
	// So a hard equality gate cannot read atcr's paginated output at all.
	src := "findings[9|]{a}:\n  one\n  two\ntruncated: true\n"
	doc, err := Decode(strings.NewReader(src))
	if err != nil {
		t.Fatalf("a truncated payload must parse: %v", err)
	}
	if doc.Declared != 9 {
		t.Errorf("Declared = %d, want 9 (the TRUE total, from the header)", doc.Declared)
	}
	if len(doc.Rows) != 2 {
		t.Errorf("got %d rows, want the 2 physically present", len(doc.Rows))
	}
	if doc.Meta["truncated"] != "true" {
		t.Errorf("the truncation signal was lost")
	}
}

func TestMoreRowsThanDeclaredIsStillAnError(t *testing.T) {
	// Fewer rows than declared is legitimate (truncation). MORE never is — it
	// means the header and the body disagree in the direction no contract allows.
	_, err := Decode(strings.NewReader("r[1|]{a}:\n  x\n  y\n"))
	if err == nil {
		t.Fatal("more rows than the header declares parsed cleanly")
	}
}

func TestATrailingBlockIsNotEatenAsRows(t *testing.T) {
	// atcr Epic 42.0 (AXI Contextual Disclosure, currently DEFERRED) appends a
	// `help[]` block of next-step suggestions after command output. Its AC4 says
	// existing content is unchanged and only appended to — so a reader that runs
	// to EOF breaks the day that ships.
	src := "findings[1|]{a}:\n  x\nhelp[1|]{cmd}:\n  atcr verify <id>\n"
	doc, err := Decode(strings.NewReader(src))
	if err != nil {
		t.Fatalf("a trailing block must not break the first one: %v", err)
	}
	if len(doc.Rows) != 1 {
		t.Fatalf("got %d rows, want 1", len(doc.Rows))
	}
	if doc.Rows[0]["a"] != "x" {
		t.Errorf("a = %q, want x", doc.Rows[0]["a"])
	}
}
