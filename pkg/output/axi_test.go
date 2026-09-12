package output

import (
	"bytes"
	"errors"
	"io"
	"math"
	"strings"
	"testing"

	goaxi "github.com/samestrin/go-axi"
)

// --axi is opt-in and additive. Default output must not move a byte, because
// every existing consumer reads it.

type axiRow struct {
	File  string `json:"file"`
	Line  int    `json:"line"`
	Match string `json:"match"`
}

type axiResult struct {
	Count int      `json:"count"`
	Rows  []axiRow `json:"rows"`
}

func sampleResult() axiResult {
	return axiResult{Count: 2, Rows: []axiRow{
		{File: "a.go", Line: 1, Match: "x"},
		{File: "b.go", Line: 2, Match: "y"},
	}}
}

func TestAXIEmitsTheJSONTagNamesNotTheGoFieldNames(t *testing.T) {
	// The reason the value is encoded through its JSON projection rather than
	// handed to the encoder directly. toon-go does NOT fall back to the `json`
	// tag — probed: encoding the struct itself emits `Count` and
	// `Rows{File,Line,Match}`, the Go identifiers. Every result type in this
	// repo carries only `json` tags, so a direct encode would publish Go
	// identifiers as the column contract for 60-odd commands.
	buf := new(bytes.Buffer)
	f := New(false, false, buf)
	f.AXI = true
	if err := f.Print(sampleResult(), nil); err != nil {
		t.Fatalf("Print: %v", err)
	}
	got := buf.String()
	if !strings.Contains(got, "rows[2]{file,line,match}:") {
		t.Errorf("output does not carry the json-tag column names:\n%s", got)
	}
	if strings.Contains(got, "File") || strings.Contains(got, "Rows[") {
		t.Errorf("Go field names leaked into the contract:\n%s", got)
	}
	if !strings.Contains(got, "count: 2") {
		t.Errorf("scalar field missing:\n%s", got)
	}
}

func TestDefaultOutputIsUnchangedByAXISupport(t *testing.T) {
	// The whole point of opt-in. Without the flag, both existing modes emit
	// exactly what they emitted before this change.
	jsonBuf := new(bytes.Buffer)
	if err := New(true, false, jsonBuf).Print(sampleResult(), nil); err != nil {
		t.Fatalf("Print: %v", err)
	}
	const wantJSON = `{
  "count": 2,
  "rows": [
    {
      "file": "a.go",
      "line": 1,
      "match": "x"
    },
    {
      "file": "b.go",
      "line": 2,
      "match": "y"
    }
  ]
}
`
	if jsonBuf.String() != wantJSON {
		t.Errorf("--json output changed:\ngot:\n%s\nwant:\n%s", jsonBuf.String(), wantJSON)
	}

	textBuf := new(bytes.Buffer)
	err := New(false, false, textBuf).Print(sampleResult(), func(w io.Writer, data interface{}) {
		_, _ = w.Write([]byte("plain text\n"))
	})
	if err != nil {
		t.Fatalf("Print: %v", err)
	}
	if textBuf.String() != "plain text\n" {
		t.Errorf("text output changed: %q", textBuf.String())
	}
}

func TestAXITakesPrecedenceOverJSON(t *testing.T) {
	// Both flags set is a malformed-ish invocation the CLI cannot reject (both
	// are valid on their own), so the rule is fixed here rather than left to
	// whichever branch happens to be checked first: --axi wins, because it is
	// the more specific request.
	buf := new(bytes.Buffer)
	f := New(true, false, buf)
	f.AXI = true
	if err := f.Print(sampleResult(), nil); err != nil {
		t.Fatalf("Print: %v", err)
	}
	if strings.HasPrefix(strings.TrimSpace(buf.String()), "{") {
		t.Errorf("--json won over --axi:\n%s", buf.String())
	}
}

func TestAXIStripsAControlByte(t *testing.T) {
	// go-axi sanitizes before encoding. toon-go ERRORS on a raw \x1b rather
	// than emitting it, so without sanitizing this would not be a cosmetic
	// problem — the command would fail outright on data it should print.
	buf := new(bytes.Buffer)
	f := New(false, false, buf)
	f.AXI = true
	r := axiResult{Count: 1, Rows: []axiRow{{File: "a.go", Line: 1, Match: "[31mred[0m"}}}
	if err := f.Print(r, nil); err != nil {
		t.Fatalf("Print: %v", err)
	}
	if strings.ContainsRune(buf.String(), '') {
		t.Error("a raw escape reached the terminal")
	}
	if !strings.Contains(buf.String(), "red") {
		t.Errorf("the visible text did not survive:\n%s", buf.String())
	}
}

func TestAXIErrorStillReturnsTheClassifiedExitCode(t *testing.T) {
	// An --axi run that fails must still exit 2 on a malformed invocation. The
	// output mode and the exit contract are independent, and coupling them
	// would make the status depend on a display flag.
	buf := new(bytes.Buffer)
	f := New(false, false, buf)
	f.AXI = true
	if got := f.PrintError(errors.New("unknown flag: --bogus")); got != int(goaxi.ExitUsage) {
		t.Errorf("PrintError = %d, want %d", got, int(goaxi.ExitUsage))
	}
	if strings.ContainsRune(buf.String(), '{') {
		t.Errorf("an --axi error was emitted as JSON:\n%s", buf.String())
	}
}

func TestAXIReportsAValueItCannotProject(t *testing.T) {
	// A value JSON cannot marshal must surface as an error rather than as
	// nothing. Writing no output and returning nil is the silent-empty-success
	// trap go-axi's Check exists to catch, and a command that printed nothing
	// while exiting 0 would look like a command with no results.
	buf := new(bytes.Buffer)
	f := New(false, false, buf)
	f.AXI = true
	err := f.Print(map[string]interface{}{"ch": make(chan int)}, nil)
	if err == nil {
		t.Fatal("an unmarshalable value was accepted")
	}
	if buf.Len() != 0 {
		t.Errorf("a partial payload was written before failing: %q", buf.String())
	}
}

func TestEncodeAXIReportsAnUnmarshalableValue(t *testing.T) {
	// Same guarantee at the helper llm-filesystem calls directly.
	buf := new(bytes.Buffer)
	if err := EncodeAXI(buf, math.Inf(1)); err == nil {
		t.Fatal("an unmarshalable value was accepted")
	}
	if buf.Len() != 0 {
		t.Errorf("a partial payload was written before failing: %q", buf.String())
	}
}

func TestNewAdoptsThePackageDefault(t *testing.T) {
	// How the flag reaches 60-odd call sites without touching one of them.
	// Every command builds its formatter with output.New(json, min, w); the
	// root sets the default once, from the persistent flag.
	t.Cleanup(func() { SetDefaultAXI(false) })
	SetDefaultAXI(true)
	if !New(false, false, new(bytes.Buffer)).AXI {
		t.Error("New did not adopt the package default")
	}
	SetDefaultAXI(false)
	if New(false, false, new(bytes.Buffer)).AXI {
		t.Error("New kept a stale default")
	}
}
