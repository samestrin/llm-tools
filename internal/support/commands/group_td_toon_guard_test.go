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

// The SECOND instance of the same root cause, found while fixing the first.
//
// detectFrontendItem also reads item["FILE_LINE"] directly, so on TOON input it
// sees nothing and reports every row as non-frontend. The consequence is
// quieter than the guard's but the same shape: reconcile stops suggesting
// --visual for frontend findings that came from atcr, and nothing says so.
//
// extractFileLine already handles FILE:LINE, FILE_LINE and FILE. Both narrow
// readers should use it rather than each re-implementing a subset.
func TestFrontendDetectionSeesTOONFieldNames(t *testing.T) {
	// CORRECTED after the first version proved nothing. Two defects in it:
	//
	//  1. it used CATEGORY "ui", which is in frontendCategoryKeywords — so
	//     rule 3 matched on the category alone and the test passed whether or
	//     not the path was ever read;
	//  2. it asserted on res.Groups with a SINGLE item, which lands in
	//     Ungrouped. Frontend is a group-level flag, so the loop checked
	//     nothing at all.
	//
	// Now: CATEGORY "correctness" (no frontend keyword is a substring of it),
	// .tsx paths so ONLY the file-extension rule can fire, and three items
	// under one prefix so a group actually forms.
	payload := `findings[3|]{severity|"file:line"|problem|category|est_minutes}:
  LOW|"src/components/LimitBar.tsx:42"|bar does not re-render after mutation|correctness|30
  LOW|"src/components/Header.tsx:10"|stale prop threaded through render|correctness|20
  LOW|"src/components/Footer.tsx:7"|dead branch never evaluated|correctness|10
`
	cmd := newGroupTDCmd()
	out := new(bytes.Buffer)
	cmd.SetOut(out)
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"--content", payload, "--format", "toon", "--json",
		"--validate-lines=false"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	var res GroupTDResult
	if err := json.Unmarshal(out.Bytes(), &res); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if len(res.Groups) == 0 {
		t.Fatalf("no group formed from 3 items under one prefix; ungrouped=%d",
			len(res.Ungrouped))
	}
	frontend := false
	for _, g := range res.Groups {
		if g.Frontend {
			frontend = true
		}
	}
	if !frontend {
		t.Error("no group flagged Frontend for .tsx findings — detectFrontendItem " +
			"could not see the path, so TOON rows look like they have none")
	}
}
