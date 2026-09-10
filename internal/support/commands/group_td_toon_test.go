package commands

import (
	"bytes"
	"encoding/json"
	"testing"
)

// toonFindings is the shape `atcr report --format axi` emits, with the field
// names group_td reads. The delimiter is DECLARED in the header rather than
// assumed, and the payload names its own columns — which is why --headers is
// not required for this format.
const toonFindings = `findings[2|]{SEVERITY|FILE_LINE|PROBLEM|FIX|CATEGORY|EST_MINUTES|EVIDENCE|REVIEWER}:
  HIGH|"src/auth/login.ts:10"|"Guard rejects a|b style input"|Split on the delimiter first|security|30|"if (x|y)"|bruce
  MEDIUM|"src/auth/logout.ts:20"|Session not cleared|Clear the cookie|security|20|no clear call|kai
`

// allItems flattens every grouped and ungrouped item, so a test does not depend
// on how the grouper happened to bucket two findings.
func allItems(res GroupTDResult) []map[string]interface{} {
	out := append([]map[string]interface{}{}, res.Ungrouped...)
	for _, g := range res.Groups {
		out = append(out, g.Items...)
	}
	return out
}

func runGroupTDToon(t *testing.T, args ...string) GroupTDResult {
	t.Helper()
	cmd := newGroupTDCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	var res GroupTDResult
	if err := json.Unmarshal(buf.Bytes(), &res); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, buf.String())
	}
	return res
}

// --- AC 4: TOON input, and --headers is not required for it

func TestGroupTDReadsTOONWithoutHeadersFlag(t *testing.T) {
	// No --headers. The payload declares its own fields, which is the point:
	// eight sites currently hand-maintain a headers list that must be kept in
	// sync with seven formats by hand.
	res := runGroupTDToon(t, "--content", toonFindings, "--format", "toon", "--json")
	if res.Summary.TotalItems != 2 {
		t.Fatalf("TotalItems = %d, want 2", res.Summary.TotalItems)
	}
	items := allItems(res)
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2", len(items))
	}
}

func TestGroupTDTOONMapsDeclaredFieldNames(t *testing.T) {
	res := runGroupTDToon(t, "--content", toonFindings, "--format", "toon", "--json")
	var found map[string]interface{}
	for _, it := range allItems(res) {
		if fl, _ := it["FILE_LINE"].(string); fl == "src/auth/logout.ts:20" {
			found = it
		}
	}
	if found == nil {
		t.Fatal("no item keyed by the declared FILE_LINE field name")
	}
	if got, _ := found["PROBLEM"].(string); got != "Session not cleared" {
		t.Errorf("PROBLEM = %q, want the real problem text", got)
	}
	if got, _ := found["REVIEWER"].(string); got != "kai" {
		t.Errorf("REVIEWER = %q, want kai", got)
	}
}

// --- the reason this format exists at all

func TestGroupTDTOONKeepsAPipeInsideAFinding(t *testing.T) {
	// v1's pipe format has no quoting, so a '|' inside PROBLEM or FIX shifts
	// every column after it — which is why the skills were told to corrupt it to
	// '/'. TOON quotes, so the character survives as data.
	res := runGroupTDToon(t, "--content", toonFindings, "--format", "toon", "--json")
	var problem, evidence string
	for _, it := range allItems(res) {
		if fl, _ := it["FILE_LINE"].(string); fl == "src/auth/login.ts:10" {
			problem, _ = it["PROBLEM"].(string)
			evidence, _ = it["EVIDENCE"].(string)
		}
	}
	if problem != "Guard rejects a|b style input" {
		t.Errorf("PROBLEM = %q — the pipe inside the finding did not survive", problem)
	}
	if evidence != "if (x|y)" {
		t.Errorf("EVIDENCE = %q — the pipe inside the evidence did not survive", evidence)
	}
}

// --- a malformed payload must not parse as an empty success

func TestGroupTDTOONRejectsAMalformedPayload(t *testing.T) {
	// A header claiming more rows than it carries is atcr's truncation signal;
	// a header claiming FEWER than it carries is a disagreement no contract
	// allows, and must not silently yield a short list.
	cmd := newGroupTDCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"--content", "f[1|]{A}:\n  x\n  y\n", "--format", "toon", "--json"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("a payload carrying more rows than its header declares parsed cleanly")
	}
}

// atcrRealShape is what `atcr report --format axi` ACTUALLY emits: field names
// in LOWER case, and "file:line" rather than "FILE_LINE". Verified through the
// built binary against a real 22-finding review.
//
// group_td reads item["FILE:LINE"], item["FILE_LINE"], item["SEVERITY"] and so
// on — all upper case. So a real atcr payload decoded verbatim produces 22 items
// that group_td cannot key: grouping, minute totals and severity handling all
// degrade silently while total_items still reads 22. Looking successful while
// doing nothing is the failure this whole change exists to remove.
const atcrRealShape = `findings[2|]{severity|"file:line"|problem|fix|category|est_minutes|evidence|reviewers|confidence}:
  CRITICAL|"fanout.go:37"|Returning incomplete client|Use NewClient|correctness|15|lines 37-42|ronin|MEDIUM
  HIGH|"src/lib.rs:4"|Function adds extra 1|Remove the +1|correctness|5|a + b + 1|ronin|MEDIUM
`

func TestGroupTDTOONNormalisesAtcrsRealFieldNames(t *testing.T) {
	res := runGroupTDToon(t, "--content", atcrRealShape, "--format", "toon", "--json")
	if res.Summary.TotalItems != 2 {
		t.Fatalf("TotalItems = %d, want 2", res.Summary.TotalItems)
	}
	var keyed map[string]interface{}
	for _, it := range allItems(res) {
		fl, _ := it["FILE:LINE"].(string)
		if fl == "fanout.go:37" {
			keyed = it
		}
	}
	if keyed == nil {
		t.Fatal(`no item readable at item["FILE:LINE"] — a real atcr payload ` +
			`decoded to keys group_td cannot read, so every row is inert`)
	}
	if got, _ := keyed["SEVERITY"].(string); got != "CRITICAL" {
		t.Errorf("SEVERITY = %q, want CRITICAL", got)
	}
	if got, _ := keyed["EST_MINUTES"].(string); got != "15" {
		t.Errorf("EST_MINUTES = %q, want 15", got)
	}
	if got, _ := keyed["PROBLEM"].(string); got != "Returning incomplete client" {
		t.Errorf("PROBLEM = %q, want the real problem text", got)
	}
}
