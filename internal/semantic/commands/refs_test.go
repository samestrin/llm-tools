package commands

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/samestrin/llm-tools/internal/semantic"
)

func findCommand(t *testing.T, name string) bool {
	t.Helper()
	for _, cmd := range RootCmd().Commands() {
		if strings.HasPrefix(cmd.Use, name) {
			return true
		}
	}
	return false
}

func TestRefCommands_RegisteredInRoot(t *testing.T) {
	for _, name := range []string{"callers", "refs"} {
		if !findCommand(t, name) {
			t.Errorf("%q command is not registered on the root command", name)
		}
	}
}

func TestRefCommands_RequireASymbol(t *testing.T) {
	for _, name := range []string{"callers", "refs"} {
		if _, err := executeCommand(RootCmd(), name); err == nil {
			t.Errorf("%q without a symbol should be an error", name)
		}
	}
}

// sampleEdges is one call into an indexed symbol and one call out to something
// outside the index.
func sampleEdges() []semantic.RefEdge {
	return []semantic.RefEdge{
		{
			ChunkRef:  semantic.ChunkRef{ChunkID: "caller", RefType: semantic.RefCalls, RefName: "w.Process", RefTargetID: "target"},
			FilePath:  "a.go",
			Name:      "Run",
			StartLine: 12,
			EndLine:   18,
		},
		{
			ChunkRef: semantic.ChunkRef{ChunkID: "caller", RefType: semantic.RefCalls, RefName: "fmt.Println"},
		},
	}
}

func TestFormatCallers_Human(t *testing.T) {
	var buf bytes.Buffer
	edges := sampleEdges()[:1]

	if err := formatCallers(&buf, "Process", edges, false, false); err != nil {
		t.Fatalf("formatCallers: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"Process", "a.go:12", "Run"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestFormatCallers_EmptyIsNotAnError(t *testing.T) {
	var buf bytes.Buffer

	if err := formatCallers(&buf, "Nobody", nil, false, false); err != nil {
		t.Fatalf("formatCallers with no results must not error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "No callers of Nobody") {
		t.Errorf("expected a plain no-callers message, got:\n%s", out)
	}
	if !strings.Contains(out, "may not be indexed") {
		t.Errorf("expected a hint that the symbol may be absent, got:\n%s", out)
	}
}

func TestFormatCallers_JSON(t *testing.T) {
	var buf bytes.Buffer
	if err := formatCallers(&buf, "Process", sampleEdges()[:1], true, false); err != nil {
		t.Fatalf("formatCallers: %v", err)
	}

	var got struct {
		Symbol  string `json:"symbol"`
		Count   int    `json:"count"`
		Callers []struct {
			File string `json:"file"`
			Line int    `json:"line"`
			Name string `json:"name"`
			Type string `json:"type"`
		} `json:"callers"`
	}
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, buf.String())
	}

	if got.Symbol != "Process" || got.Count != 1 {
		t.Errorf("symbol/count = %q/%d, want Process/1", got.Symbol, got.Count)
	}
	if len(got.Callers) != 1 {
		t.Fatalf("callers = %d, want 1", len(got.Callers))
	}
	if got.Callers[0].File != "a.go" || got.Callers[0].Line != 12 {
		t.Errorf("caller location = %s:%d, want a.go:12", got.Callers[0].File, got.Callers[0].Line)
	}
	if got.Callers[0].Type != string(semantic.RefCalls) {
		t.Errorf("caller type = %q, want %q", got.Callers[0].Type, semantic.RefCalls)
	}
}

func TestFormatCallers_MinDropsDetail(t *testing.T) {
	var buf bytes.Buffer
	if err := formatCallers(&buf, "Process", sampleEdges()[:1], false, true); err != nil {
		t.Fatalf("formatCallers: %v", err)
	}

	var got map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("min output is not valid JSON: %v\n%s", err, buf.String())
	}

	callers, ok := got["callers"].([]interface{})
	if !ok || len(callers) != 1 {
		t.Fatalf("min output callers = %v, want one entry", got["callers"])
	}
	entry := callers[0].(map[string]interface{})
	if _, present := entry["type"]; present {
		t.Errorf("min output should drop the edge type, got %v", entry)
	}
	if entry["file"] != "a.go" {
		t.Errorf("min output should keep the file, got %v", entry)
	}
}

func TestFormatRefs_MarksExternalReferences(t *testing.T) {
	var buf bytes.Buffer
	if err := formatRefs(&buf, "Run", sampleEdges(), false, false); err != nil {
		t.Fatalf("formatRefs: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "b.go") && !strings.Contains(out, "a.go:12") {
		t.Errorf("resolved reference should show a location:\n%s", out)
	}
	if !strings.Contains(out, "fmt.Println") {
		t.Errorf("external reference should still be listed:\n%s", out)
	}
	if !strings.Contains(out, "(external)") {
		t.Errorf("external reference should be marked:\n%s", out)
	}
}

func TestFormatRefs_JSONFlagsResolution(t *testing.T) {
	var buf bytes.Buffer
	if err := formatRefs(&buf, "Run", sampleEdges(), true, false); err != nil {
		t.Fatalf("formatRefs: %v", err)
	}

	var got struct {
		Symbol     string `json:"symbol"`
		References []struct {
			Ref      string `json:"ref"`
			Resolved bool   `json:"resolved"`
			File     string `json:"file"`
		} `json:"references"`
	}
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, buf.String())
	}

	if len(got.References) != 2 {
		t.Fatalf("references = %d, want 2", len(got.References))
	}

	byRef := map[string]bool{}
	for _, r := range got.References {
		byRef[r.Ref] = r.Resolved
	}
	if !byRef["w.Process"] {
		t.Error("w.Process should be reported resolved")
	}
	if byRef["fmt.Println"] {
		t.Error("fmt.Println should be reported unresolved")
	}
}

func TestFormatRefs_EmptyIsNotAnError(t *testing.T) {
	var buf bytes.Buffer
	if err := formatRefs(&buf, "Nobody", nil, false, false); err != nil {
		t.Fatalf("formatRefs with no results must not error: %v", err)
	}
	if !strings.Contains(buf.String(), "No references from Nobody") {
		t.Errorf("expected a plain no-references message, got:\n%s", buf.String())
	}
}
