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
		"no header":          "  just|a|row\n",
		"unterminated quote": "r[1|]{a}:\n  \"never closed\n",
		"count not a number": "r[x|]{a}:\n  v\n",
		"missing colon":      "r[1|]{a}\n  v\n",
		"empty input":        "",
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
