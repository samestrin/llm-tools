package commands

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"
)

// captureStdout swaps os.Stdout for a pipe. emitJSON writes there rather than
// to an injectable writer, because the 21 sites it replaced did.
//
// The restore is registered with t.Cleanup BEFORE the swap, so a panic or
// t.Fatalf inside fn cannot leave later tests in this package writing to a
// closed pipe — the same defect a reviewer caught in llm-filesystem's copy.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	t.Cleanup(func() { os.Stdout = orig })
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

// withAXI turns TOON output on for one test and guarantees it is off again
// afterwards, so no test can leak the mode into another.
func withAXI(t *testing.T) {
	t.Helper()
	t.Cleanup(func() { GlobalAXIOutput = false })
	GlobalAXIOutput = true
}

// runStatusInEmptyDir drives index-status where no index exists, which reaches
// the COMPACT emit site — the one that has never called SetIndent.
//
// jsonOutput mirrors what the FLAGS actually pass. Passing true unconditionally
// is what made the first version of this test a false green: it drove the
// --json branch and reported success while `llm-semantic index-status --axi`
// printed human text at the command line.
func runStatusInEmptyDir(t *testing.T, jsonOutput bool) string {
	t.Helper()
	t.Chdir(t.TempDir())
	return captureStdout(t, func() {
		if err := runIndexStatus(context.Background(), jsonOutput); err != nil {
			t.Fatalf("runIndexStatus: %v", err)
		}
	})
}

func TestIndexStatusHonoursAXIWithoutJSON(t *testing.T) {
	// --axi ALONE, which is how a caller uses it. Routing every emit site
	// through emitJSON was necessary and not sufficient: those sites sit behind
	// `if jsonOutput`, and that gate has to admit --axi too.
	//
	// Observed before the fix:
	//   $ llm-semantic index-status --axi
	//   Semantic index not found.
	//   Run 'llm-semantic index' to create one.
	withAXI(t)
	got := strings.TrimSpace(runStatusInEmptyDir(t, false))
	if strings.Contains(got, "Semantic index not found.") {
		t.Fatalf("--axi was accepted and ignored; human text was emitted:\n%s", got)
	}
	if strings.HasPrefix(got, "{") {
		t.Fatalf("--axi emitted JSON rather than TOON: %q", got)
	}
	for _, want := range []string{"error", "indexed"} {
		if !strings.Contains(got, want) {
			t.Errorf("field %q missing from TOON output: %q", want, got)
		}
	}
}

func TestIndexStatusTextModeIsUnchanged(t *testing.T) {
	// Neither flag set must still print the human text, or --axi's fix has
	// swallowed the default path.
	got := runStatusInEmptyDir(t, false)
	if !strings.Contains(got, "Semantic index not found.") {
		t.Errorf("text mode changed:\n%s", got)
	}
}

func TestIndexStatusJSONIsUnchangedAndStaysCompact(t *testing.T) {
	// The other half of the contract, and the variant most at risk: this branch
	// builds its encoder inline with no SetIndent, so it must stay on one line.
	// Routing it through an indenting helper would have reformatted --json
	// output that consumers already parse.
	// true: this test is about --json's own output staying compact, so it
	// drives the --json gate deliberately.
	got := strings.TrimSpace(runStatusInEmptyDir(t, true))
	if strings.Contains(got, "\n") {
		t.Errorf("compact JSON became multi-line:\n%s", got)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(got), &payload); err != nil {
		t.Fatalf("not valid JSON: %v (%q)", err, got)
	}
	if payload["indexed"] != false {
		t.Errorf("indexed = %v, want false", payload["indexed"])
	}
	if payload["error"] != "index not found" {
		t.Errorf("error = %v, want \"index not found\"", payload["error"])
	}
}

func TestAXIAndJSONAgreeOnFieldNames(t *testing.T) {
	// AC3: --axi must carry the same field names --json does. A payload that
	// renamed or dropped a field would still "work" and still exit 0.
	// true here: this half exists to read what --json emits, so it drives the
	// --json gate on purpose.
	jsonOut := strings.TrimSpace(runStatusInEmptyDir(t, true))
	var payload map[string]any
	if err := json.Unmarshal([]byte(jsonOut), &payload); err != nil {
		t.Fatalf("not valid JSON: %v", err)
	}

	// false here: --axi ALONE is the invocation a caller makes. Passing true
	// would drive the --json branch and pass while the flag is ignored.
	withAXI(t)
	axiOut := runStatusInEmptyDir(t, false)
	for field := range payload {
		if !strings.Contains(axiOut, field+":") {
			t.Errorf("--json field %q has no counterpart in --axi output:\n%s", field, axiOut)
		}
	}
}
