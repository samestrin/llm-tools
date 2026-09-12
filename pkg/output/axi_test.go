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

// --- MED (review): --json and --axi disagreed about large integers

type bigResult struct {
	Total int64   `json:"total"`
	Huge  uint64  `json:"huge"`
	Small int     `json:"small"`
	Ratio float64 `json:"ratio"`
}

func TestAXIPreservesIntegersBeyondFloat64(t *testing.T) {
	// Measured before the fix:
	//
	//   --json  {"also":12345678901234567890,"big":9007199254740993}
	//   --axi   also: 12345678901234567000
	//           big:  9007199254740992
	//
	// The two flags disagreed about the DATA, silently. The cause was mine:
	// projecting through JSON decodes every number into float64, and anything
	// past 2^53 rounds. Encoding the struct directly preserves the digits — but
	// then the column names become Go identifiers, which is why the projection
	// exists at all.
	//
	// json.Number alone does NOT fix it: probed, go-axi re-parses it back into
	// a number and the digits die again. Only the out-of-range values are
	// carried across verbatim, which is also what toon-go does natively when it
	// encodes a large int64.
	buf := new(bytes.Buffer)
	f := New(false, false, buf)
	f.AXI = true
	r := bigResult{Total: 9007199254740993, Huge: 12345678901234567890, Small: 42, Ratio: 1.5}
	if err := f.Print(r, nil); err != nil {
		t.Fatalf("Print: %v", err)
	}
	got := buf.String()
	for _, want := range []string{"9007199254740993", "12345678901234567890"} {
		if !strings.Contains(got, want) {
			t.Errorf("digits lost: %q missing from\n%s", want, got)
		}
	}
	// Ordinary numbers must stay numeric, or --axi stops matching --json for
	// every payload that is not huge.
	if !strings.Contains(got, "small: 42") {
		t.Errorf("an in-range integer was not left numeric:\n%s", got)
	}
	if !strings.Contains(got, "ratio: 1.5") {
		t.Errorf("a float was not left numeric:\n%s", got)
	}
}

// --- MED (review): the usage classifier matched bare substrings

func TestExitCodeForDoesNotClassifyWrappedSubprocessErrors(t *testing.T) {
	// cobra's real wording carries a flag or command reference:
	//
	//   invalid argument "nope" for "--depth" flag: ...
	//   unknown command "nosuch" for "probe"
	//
	// A subprocess failure can contain the leading fragment alone. Classifying
	// that as exit 2 tells the caller its invocation was malformed and that
	// retrying is pointless, when the truth is a transient runtime failure.
	for name, err := range map[string]error{
		"git invalid argument":   errors.New(`fatal: invalid argument "HEAD~1" in rev-parse`),
		"nested cli unknown cmd": errors.New(`docker: unknown command "buildx" was reported by the daemon`),
		"path containing phrase": errors.New(`open /tmp/invalid argument "x": no such file or directory`),
		"required flag in prose": errors.New(`the config declares required flag(s) "file" but the loader failed`),
	} {
		t.Run(name, func(t *testing.T) {
			if got := ExitCodeFor(err); got != int(goaxi.ExitError) {
				t.Errorf("ExitCodeFor(%q) = %d, want %d — a runtime failure was reported as a usage error",
					err, got, int(goaxi.ExitError))
			}
		})
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
