package commands

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// The compose test: --validate-lines (the phantom-line guard) must work on TOON
// input, not only on JSON.
//
// The two changes arrived from different directions. The guard reads
// item["FILE_LINE"]; the TOON reader emits whatever the payload DECLARES,
// upper-cased — and atcr declares "file:line", which becomes FILE:LINE with a
// COLON. group_td's own extractFileLine accepts both spellings, so the guard is
// narrower than the file around it.
//
// If the guard only understands the underscore form, a TOON payload gets no
// validation at all: nothing is flagged, nothing is quarantined, no warning is
// printed — while --validate-lines defaults to TRUE and the operator believes
// every row was checked. A guard that appears active and protects nothing is
// worse than one that is off.
func TestValidateLinesGuardsTOONInputToo(t *testing.T) {
	tmp := t.TempDir()
	writeShortFile(t, filepath.Join(tmp, "short.go"), 5) // a 5-line file

	// atcr's real shape: lower-case names, "file:line" with a colon.
	payload := `findings[2|]{severity|"file:line"|problem|est_minutes}:
  LOW|"short.go:3"|a real finding on a line that exists|5
  HIGH|"short.go:999"|phantom row citing line 999 of a 5-line file|5
`
	quarantine := filepath.Join(tmp, "quarantine.json")

	cmd := newGroupTDCmd()
	out := new(bytes.Buffer)
	errb := new(bytes.Buffer)
	cmd.SetOut(out)
	cmd.SetErr(errb)
	cmd.SetArgs([]string{
		"--content", payload, "--format", "toon", "--json",
		"--repo-root", tmp, "--quarantine-file", quarantine,
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, errb.String())
	}

	var res GroupTDResult
	if err := json.Unmarshal(out.Bytes(), &res); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, out.String())
	}

	if res.Summary.TotalItems != 1 {
		t.Errorf("TotalItems = %d, want 1 — the phantom row was not quarantined, "+
			"so the guard is inert on TOON input", res.Summary.TotalItems)
	}

	data, err := os.ReadFile(quarantine)
	if err != nil {
		t.Fatalf("no quarantine file written, so nothing was flagged: %v", err)
	}
	var quarantined []map[string]interface{}
	if err := json.Unmarshal(data, &quarantined); err != nil {
		t.Fatalf("quarantine file is not valid JSON: %v", err)
	}
	if len(quarantined) != 1 {
		t.Fatalf("quarantined %d row(s), want 1", len(quarantined))
	}
	if got, _ := quarantined[0]["PROBLEM"].(string); got == "" {
		t.Error("the quarantined row lost its PROBLEM text")
	}
}
