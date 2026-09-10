package commands

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// atcrAXIGolden is atcr's own encoder fixture, byte for byte. A skill pipes
// `atcr report --format axi` straight into this command, so the fixture is the
// contract.
const atcrAXIGolden = `findings[2|]{severity|"file:line"|problem|fix|category|est_minutes|evidence|reviewers|confidence}:
  CRITICAL|"auth.go:42"|token never expires|check expiry|security|15|expiresAt unread|greta,host|HIGH
  LOW|"util.go:7"|unused var|""|style|0|""|otto|MEDIUM
`

func writeAXI(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "report.axi")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func execTOONCmd(t *testing.T, args ...string) (string, string, error) {
	t.Helper()
	cmd := newTOONCmd()
	cmd.SetArgs(args)
	var out, errb bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errb)
	err := cmd.Execute()
	return out.String(), errb.String(), err
}

func TestTOONParseCmd_EmitsJSON(t *testing.T) {
	out, stderr, err := execTOONCmd(t, "parse", writeAXI(t, atcrAXIGolden))
	if err != nil {
		t.Fatalf("parse: %v (stderr=%s)", err, stderr)
	}
	var res TOONResult
	if jerr := json.Unmarshal([]byte(out), &res); jerr != nil {
		t.Fatalf("output is not valid JSON: %v\nout=%s", jerr, out)
	}
	if res.Name != "findings" || res.Count != 2 {
		t.Fatalf("Name=%q Count=%d, want findings/2", res.Name, res.Count)
	}
	if len(res.Rows) != 2 {
		t.Fatalf("got %d rows, want 2", len(res.Rows))
	}
	// The quoted header field is keyed unquoted, and the quoted empty string is
	// an empty string.
	if got := res.Rows[0]["file:line"]; got != "auth.go:42" {
		t.Errorf("file:line = %q, want auth.go:42", got)
	}
	if got := res.Rows[1]["fix"]; got != "" {
		t.Errorf("fix = %q, want empty", got)
	}
	if len(res.Fields) != 9 {
		t.Errorf("got %d fields, want 9", len(res.Fields))
	}
}

func TestTOONParseCmd_ZeroFindingsIsNotAnError(t *testing.T) {
	out, _, err := execTOONCmd(t, "parse", writeAXI(t, "findings[0]:\n"))
	if err != nil {
		t.Fatalf("a clean review must parse, not fail: %v", err)
	}
	var res TOONResult
	if jerr := json.Unmarshal([]byte(out), &res); jerr != nil {
		t.Fatalf("output is not valid JSON: %v", jerr)
	}
	if res.Count != 0 || len(res.Rows) != 0 {
		t.Errorf("Count=%d rows=%d, want 0/0", res.Count, len(res.Rows))
	}
}

func TestTOONParseCmd_MalformedPayloadFails(t *testing.T) {
	// CORRECTED: the fixture used to be "fewer rows than declared", which the
	// shipping atcr binary proved is LEGITIMATE (a truncated payload). More rows
	// than declared is the direction no contract allows.
	_, _, err := execTOONCmd(t, "parse", writeAXI(t, "findings[1|]{a}:\n  x\n  y\n"))
	if err == nil {
		t.Fatal("a payload carrying more rows than its header declares parsed cleanly")
	}
	if !strings.Contains(err.Error(), "1") {
		t.Errorf("error %q does not name the declared count", err)
	}
}

func TestTOONParseCmd_MissingFileNamesIt(t *testing.T) {
	_, _, err := execTOONCmd(t, "parse", filepath.Join(t.TempDir(), "nope.axi"))
	if err == nil {
		t.Fatal("a missing file parsed cleanly")
	}
	if !strings.Contains(err.Error(), "nope.axi") {
		t.Errorf("error %q does not name the file", err)
	}
}

func TestTOONParseCmd_RealCLIShapeWithTruncatedSibling(t *testing.T) {
	// What `atcr report --format axi` actually emits. The command failed on this
	// until it was run against the binary instead of the encoder's golden.
	body := "findings[9|]{severity|problem}:\n  HIGH|a\n  LOW|b\ntruncated: true\n"
	out, _, err := execTOONCmd(t, "parse", writeAXI(t, body))
	if err != nil {
		t.Fatalf("real CLI output failed to parse: %v", err)
	}
	var res TOONResult
	if jerr := json.Unmarshal([]byte(out), &res); jerr != nil {
		t.Fatalf("output is not valid JSON: %v", jerr)
	}
	if res.Declared != 9 {
		t.Errorf("declared = %d, want 9 (the TRUE total)", res.Declared)
	}
	if res.Count != 2 {
		t.Errorf("count = %d, want the 2 rows that arrived", res.Count)
	}
	if res.Meta["truncated"] != "true" {
		t.Errorf("meta.truncated = %q, want true — the caller cannot tell a capped "+
			"payload from a complete one without it", res.Meta["truncated"])
	}
}
