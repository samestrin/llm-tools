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

func runTOONParse(t *testing.T, args ...string) (string, string, error) {
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
	out, stderr, err := runTOONParse(t, "parse", writeAXI(t, atcrAXIGolden))
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
	out, _, err := runTOONParse(t, "parse", writeAXI(t, "findings[0]:\n"))
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
	_, _, err := runTOONParse(t, "parse", writeAXI(t, "findings[2|]{a}:\n  x\n"))
	if err == nil {
		t.Fatal("a payload whose row count disagrees with its header parsed cleanly")
	}
	if !strings.Contains(err.Error(), "2") {
		t.Errorf("error %q does not name the declared count", err)
	}
}

func TestTOONParseCmd_MissingFileNamesIt(t *testing.T) {
	_, _, err := runTOONParse(t, "parse", filepath.Join(t.TempDir(), "nope.axi"))
	if err == nil {
		t.Fatal("a missing file parsed cleanly")
	}
	if !strings.Contains(err.Error(), "nope.axi") {
		t.Errorf("error %q does not name the file", err)
	}
}
