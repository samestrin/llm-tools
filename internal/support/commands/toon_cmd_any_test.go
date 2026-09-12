package commands

import (
	"encoding/json"
	"strings"
	"testing"
)

// `toon parse` was tabular-only, which is correct for the atcr findings payload
// it was written for and useless for everything else. llm-filesystem-axi's
// get-file-info emits a flat object of scalars with no array anywhere: it did
// not decode at all. list-directory emits an array with a `total` sibling: it
// decoded and silently dropped the total.
//
// NOTE: these fixtures are written from the documented shape, not captured from
// the binary. llm-filesystem-axi emits nothing and exits 0 for every command in
// this environment, including --format json. Re-validate against real output
// when that consumer is migrated off gotoon.

const axiFileInfo = `path: /tmp/x
name: x
size: 1234
is_dir: false
`

const axiListDirectory = `entries[2]{name,type}:
  a,dir
  b,file
total: 6
`

func TestTOONParseCmd_ReadsANonTabularDocument(t *testing.T) {
	out, stderr, err := execTOONCmd(t, "parse", writeAXI(t, axiFileInfo))
	if err != nil {
		t.Fatalf("a scalar document must decode: %v (stderr=%s)", err, stderr)
	}
	var res struct {
		Shape string         `json:"shape"`
		Value map[string]any `json:"value"`
	}
	if jerr := json.Unmarshal([]byte(out), &res); jerr != nil {
		t.Fatalf("output is not valid JSON: %v\nout=%s", jerr, out)
	}
	if res.Shape != "document" {
		t.Fatalf("shape = %q, want document", res.Shape)
	}
	if got, _ := res.Value["path"].(string); got != "/tmp/x" {
		t.Errorf("value.path = %v, want /tmp/x", res.Value["path"])
	}
	// Types are preserved on this branch. A size that arrives as text is less
	// useful than one that arrives as a number, and nothing consumed this shape
	// before, so there is no contract to keep.
	if _, ok := res.Value["size"].(float64); !ok {
		t.Errorf("value.size = %v (%T), want a number", res.Value["size"], res.Value["size"])
	}
	if got, ok := res.Value["is_dir"].(bool); !ok || got {
		t.Errorf("value.is_dir = %v (%T), want the boolean false",
			res.Value["is_dir"], res.Value["is_dir"])
	}
}

func TestTOONParseCmd_KeepsASiblingTotal(t *testing.T) {
	out, _, err := execTOONCmd(t, "parse", "--shape", "document",
		writeAXI(t, axiListDirectory))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	var res struct {
		Shape string         `json:"shape"`
		Value map[string]any `json:"value"`
	}
	if jerr := json.Unmarshal([]byte(out), &res); jerr != nil {
		t.Fatalf("output is not valid JSON: %v\nout=%s", jerr, out)
	}
	if _, ok := res.Value["total"]; !ok {
		t.Fatal("total was dropped — a capped listing is indistinguishable from a complete one")
	}
	entries, ok := res.Value["entries"].([]any)
	if !ok || len(entries) != 2 {
		t.Fatalf("value.entries = %#v, want 2 rows", res.Value["entries"])
	}
}

func TestTOONParseCmd_TabularOutputIsUnchangedByTheNewBranch(t *testing.T) {
	// The tabular payload must still emit today's shape, with no `shape` key
	// added. atcr skills pipe this straight into other tools, so the bytes are
	// the contract.
	out, _, err := execTOONCmd(t, "parse", writeAXI(t, atcrAXIGolden))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if strings.Contains(out, `"shape"`) {
		t.Errorf("a shape key leaked into tabular output:\n%s", out)
	}
	var res TOONResult
	if jerr := json.Unmarshal([]byte(out), &res); jerr != nil {
		t.Fatalf("output is not valid JSON: %v", jerr)
	}
	if res.Name != "findings" || res.Count != 2 || len(res.Fields) != 9 {
		t.Errorf("Name=%q Count=%d Fields=%d, want findings/2/9",
			res.Name, res.Count, len(res.Fields))
	}
}

func TestTOONParseCmd_ShapeTabularRefusesToDowngrade(t *testing.T) {
	// The whole point of a --shape flag. A skill that pipes atcr output passes
	// --shape tabular, and a payload that is not a tabular array then FAILS
	// rather than quietly arriving as a document the caller cannot read. Auto
	// detection is a convenience; this is the guarantee.
	_, _, err := execTOONCmd(t, "parse", "--shape", "tabular", writeAXI(t, axiFileInfo))
	if err == nil {
		t.Fatal("a scalar document was accepted as a tabular array")
	}
	if !strings.Contains(err.Error(), "tabular") {
		t.Errorf("error %q does not explain the shape mismatch", err)
	}
}

func TestTOONParseCmd_RejectsAnUnknownShape(t *testing.T) {
	_, _, err := execTOONCmd(t, "parse", "--shape", "nonsense", writeAXI(t, atcrAXIGolden))
	if err == nil {
		t.Fatal("an unknown --shape value was accepted")
	}
}
