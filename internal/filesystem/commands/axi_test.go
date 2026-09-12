package commands

import (
	"io"
	"os"
	"strings"
	"testing"
)

// llm-filesystem is a PORT: --axi must be purely additive, and every existing
// path has to emit exactly what it emitted before.
//
// OutputResult and OutputError write straight to stdout rather than to an
// injectable writer. Stdout is captured here rather than restructured, because
// changing the shape of a ported function to make it testable is exactly the
// behaviour change the port rule forbids.

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w
	done := make(chan string, 1)
	go func() {
		b, _ := io.ReadAll(r)
		done <- string(b)
	}()
	fn()
	_ = w.Close()
	os.Stdout = orig
	return <-done
}

type fsRow struct {
	Name string `json:"name"`
	Size int    `json:"size"`
}

type fsResult struct {
	Total   int     `json:"total"`
	Entries []fsRow `json:"entries"`
}

func fsSample() fsResult {
	return fsResult{Total: 2, Entries: []fsRow{
		{Name: "a.txt", Size: 10},
		{Name: "b.txt", Size: 20},
	}}
}

// resetOutputFlags restores the package-level flag state between cases, so one
// test cannot leak a mode into the next.
func resetOutputFlags(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		jsonOutput, minOutput, axiOutput = false, false, false
	})
	jsonOutput, minOutput, axiOutput = false, false, false
}

func TestOutputResultAXIEmitsTOON(t *testing.T) {
	resetOutputFlags(t)
	axiOutput = true
	got := captureStdout(t, func() {
		OutputResult(fsSample(), func() string { return "text" })
	})
	if !strings.Contains(got, "entries[2]{name,size}:") {
		t.Errorf("not TOON with the json-tag column names:\n%s", got)
	}
	if !strings.Contains(got, "total: 2") {
		t.Errorf("the sibling scalar is missing — this is the field llm-tools' own "+
			"reader used to drop:\n%s", got)
	}
}

func TestOutputResultDefaultsAreUnchanged(t *testing.T) {
	resetOutputFlags(t)
	text := captureStdout(t, func() {
		OutputResult(fsSample(), func() string { return "human text" })
	})
	if text != "human text\n" {
		t.Errorf("text mode changed: %q", text)
	}

	jsonOutput = true
	pretty := captureStdout(t, func() {
		OutputResult(fsSample(), func() string { return "unused" })
	})
	const wantPretty = `{
  "total": 2,
  "entries": [
    {
      "name": "a.txt",
      "size": 10
    },
    {
      "name": "b.txt",
      "size": 20
    }
  ]
}
`
	if pretty != wantPretty {
		t.Errorf("--json output changed:\ngot:\n%s\nwant:\n%s", pretty, wantPretty)
	}

	minOutput = true
	compact := captureStdout(t, func() {
		OutputResult(fsSample(), func() string { return "unused" })
	})
	const wantCompact = `{"total":2,"entries":[{"name":"a.txt","size":10},{"name":"b.txt","size":20}]}` + "\n"
	if compact != wantCompact {
		t.Errorf("--json --min output changed:\ngot:  %s\nwant: %s", compact, wantCompact)
	}
}

func TestOutputResultAXIWinsOverJSON(t *testing.T) {
	resetOutputFlags(t)
	jsonOutput, axiOutput = true, true
	got := captureStdout(t, func() {
		OutputResult(fsSample(), func() string { return "text" })
	})
	if strings.HasPrefix(strings.TrimSpace(got), "{") {
		t.Errorf("--json won over --axi:\n%s", got)
	}
}
