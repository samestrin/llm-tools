package toon

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// IsTabularHeader is the dispatch point between Decode and DecodeAny, so it
// decides which contract a payload is read under. That choice must be
// SYNTACTIC — "does this line open a tabular array" — and never
// error-recovery. A reader that tries the tabular path and falls back on
// failure turns a garbled findings payload into a document full of junk,
// silently, which is the failure class this package exists to remove.

func TestIsTabularHeader(t *testing.T) {
	for name, tc := range map[string]struct {
		line string
		want bool
	}{
		"atcr findings":   {`findings[2|]{severity|"file:line"}:`, true},
		"empty form":      {"findings[0]:", true},
		"length marker":   {"findings[#2]{a}:", true},
		"comma declared":  {"rows[1,]{a,b}:", true},
		"leading scalar":  {"path: /tmp/x", false},
		"indented row":    {"  CRITICAL|x", false},
		"no bracket":      {"findings:", false},
		"no closing":      {"findings[2|{a}:", false},
		"no colon":        {"findings[2|]{a}", false},
		"blank":           {"", false},
		"bare scalar key": {"total: 6", false},
	} {
		t.Run(name, func(t *testing.T) {
			if got := IsTabularHeader(tc.line); got != tc.want {
				t.Errorf("IsTabularHeader(%q) = %v, want %v", tc.line, got, tc.want)
			}
		})
	}
}

func TestDecodeAnyFileReadsADocument(t *testing.T) {
	p := filepath.Join(t.TempDir(), "info.axi")
	if err := os.WriteFile(p, []byte("path: /tmp/x\nsize: 12\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	doc, err := DecodeAnyFile(p)
	if err != nil {
		t.Fatalf("DecodeAnyFile: %v", err)
	}
	m, ok := doc.(map[string]any)
	if !ok {
		t.Fatalf("DecodeAnyFile returned %T, want a map", doc)
	}
	if got, _ := m["path"].(string); got != "/tmp/x" {
		t.Errorf("path = %v, want /tmp/x", m["path"])
	}
}

func TestDecodeAnyFileNamesAMissingFile(t *testing.T) {
	_, err := DecodeAnyFile(filepath.Join(t.TempDir(), "nope.axi"))
	if err == nil {
		t.Fatal("a missing file decoded cleanly")
	}
	if !strings.Contains(err.Error(), "nope.axi") {
		t.Errorf("error %q does not name the file", err)
	}
}

func TestDecodeAnyRejectsAnEmptyDocument(t *testing.T) {
	// toon-go returns (nil, nil) for empty input. An empty success is exactly
	// the trap go-axi's Check exists to catch, and it must not reach a caller
	// as a valid document.
	if _, err := DecodeAny(strings.NewReader("")); err == nil {
		t.Fatal("an empty document decoded cleanly")
	}
}

func TestDecodeReadsAnUnnamedArray(t *testing.T) {
	// `[2]{a}:` is a top-level array of records — one of the four TOON shapes
	// that start with '[', and the reason a JSON-vs-TOON discriminator can
	// never test the first byte for '['.
	doc, err := Decode(strings.NewReader("[2|]{a|b}:\n  1|2\n  3|4\n"))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if doc.Name != "" {
		t.Errorf("Name = %q, want empty", doc.Name)
	}
	if len(doc.Rows) != 2 {
		t.Fatalf("got %d rows, want 2", len(doc.Rows))
	}
	if doc.Rows[1]["b"] != "4" {
		t.Errorf("Rows[1][b] = %q, want 4", doc.Rows[1]["b"])
	}
}

func TestAnUnsupportedDelimiterIsRejected(t *testing.T) {
	// TOON defines comma, tab and pipe. The old reader accepted any character,
	// so a payload could declare a delimiter no encoder emits and no other
	// reader accepts — and it would parse here and nowhere else.
	_, err := Decode(strings.NewReader("r[1;]{a;b}:\n  1;2\n"))
	if err == nil {
		t.Fatal("a semicolon delimiter decoded cleanly")
	}
	if !strings.Contains(err.Error(), "delimiter") {
		t.Errorf("error %q does not explain the delimiter", err)
	}
}

func TestAQuotedArrayNameDecodes(t *testing.T) {
	// A space or a colon in an array name forces quoting, by exactly the rules
	// that quote a field name. toon-go unquotes the key when it builds the
	// document, so a reader that looks the array up by the RAW header text
	// cannot find it — and a payload that decoded before stops decoding.
	//
	// Found by reviewing the diff, then confirmed against binaries built from
	// main and from this branch: main emitted the rows (with the quotes wrongly
	// kept in `name`), the branch failed with "array is missing from the
	// decoded document".
	doc, err := Decode(strings.NewReader("\"my array\"[1|]{a|b}:\n  1|2\n"))
	if err != nil {
		t.Fatalf("a quoted array name is valid TOON: %v", err)
	}
	if doc.Name != "my array" {
		t.Errorf("Name = %q, want %q — the quotes are syntax, not part of the name",
			doc.Name, "my array")
	}
	if len(doc.Rows) != 1 {
		t.Fatalf("got %d rows, want 1", len(doc.Rows))
	}
	if doc.Rows[0]["b"] != "2" {
		t.Errorf("Rows[0][b] = %q, want 2", doc.Rows[0]["b"])
	}
}
