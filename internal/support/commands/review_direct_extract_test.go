package commands

import (
	"reflect"
	"testing"
)

// The greta regression: a reviewer that emits bare pipe rows with NO
// "TD_STREAM" sentinel line (exactly what the prompt's example shows) must
// still be extracted. The old sentinel-gated extractor returned nothing here,
// silently dropping every finding into review.md only.
func TestExtractTDLines_BareRowsNoSentinel(t *testing.T) {
	prose := "MEDIUM|internal/tools/read_file.go:42|Absolute path leaked|Strip paths|security|10|os.PathError|greta\n" +
		"LOW|internal/tools/open_other.go:9|TOCTOU window|Add EvalSymlinks|security|15|no symlink guard|greta"

	got := extractTDLines(prose, "greta")
	want := []string{
		"MEDIUM|internal/tools/read_file.go:42|Absolute path leaked|Strip paths|security|10|os.PathError|greta",
		"LOW|internal/tools/open_other.go:9|TOCTOU window|Add EvalSymlinks|security|15|no symlink guard|greta",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("bare rows not extracted.\n got=%v\nwant=%v", got, want)
	}
	if n := countTDLines(prose); n != 2 {
		t.Errorf("countTDLines = %d, want 2", n)
	}
}

// An already-8-column row whose REVIEWER field is populated must NOT have the
// agent name appended a second time (no "...|greta|greta").
func TestExtractTDLines_NoDoubleReviewer(t *testing.T) {
	prose := "HIGH|a.go:1|p|f|security|5|ev|greta"
	got := extractTDLines(prose, "greta")
	want := []string{"HIGH|a.go:1|p|f|security|5|ev|greta"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("double-append or mangled.\n got=%v\nwant=%v", got, want)
	}
}

// A 7-column row (no REVIEWER) gets the agent name appended as column 8.
func TestExtractTDLines_AppendsReviewerTo7Col(t *testing.T) {
	prose := "HIGH|a.go:1|p|f|security|5|ev"
	got := extractTDLines(prose, "otto")
	want := []string{"HIGH|a.go:1|p|f|security|5|ev|otto"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("7-col append wrong.\n got=%v\nwant=%v", got, want)
	}
}

// A legacy 5-column row pads to 7 then appends REVIEWER, yielding the unified
// 8-column shape — parity with multireview.WriteReviewerOutput.
func TestExtractTDLines_Pads5ColTo8(t *testing.T) {
	prose := "HIGH|a.go:1|prob|fix|security"
	got := extractTDLines(prose, "kai")
	want := []string{"HIGH|a.go:1|prob|fix|security|||kai"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("5-col pad+append wrong.\n got=%v\nwant=%v", got, want)
	}
}

// The legacy sentinel form (prose review followed by a TD_STREAM header and
// rows) keeps working — the TD_STREAM line itself is never emitted as a row.
func TestExtractTDLines_LegacySentinelStillWorks(t *testing.T) {
	prose := "Review: LGTM\n\nTD_STREAM\nMEDIUM|main.go:42|Issue|Fix|error"
	got := extractTDLines(prose, "alice")
	want := []string{"MEDIUM|main.go:42|Issue|Fix|error|||alice"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("legacy sentinel form broke.\n got=%v\nwant=%v", got, want)
	}
	if n := countTDLines(prose); n != 1 {
		t.Errorf("countTDLines = %d, want 1", n)
	}
}

// --- Adversarial ---

// Pure prose with no pipe rows yields nothing — and a prose mention of "HIGH
// severity" must not be mistaken for a finding (pipe must follow severity).
func TestExtractTDLines_ProseOnlyYieldsNothing(t *testing.T) {
	prose := "This change looks fine. There is a HIGH severity risk in theory but I found nothing concrete.\nLGTM."
	if got := extractTDLines(prose, "mira"); len(got) != 0 {
		t.Errorf("prose extracted as findings: %v", got)
	}
	if n := countTDLines(prose); n != 0 {
		t.Errorf("countTDLines = %d, want 0", n)
	}
}

// Mixed prose + bare pipe rows: only the pipe rows survive, in order.
func TestExtractTDLines_MixedProseAndRows(t *testing.T) {
	prose := "Here is my review.\nHIGH|a.go:1|p1|f1|security|5|e1|brad\nSome commentary in between.\nLOW|b.go:2|p2|f2|style|3|e2|brad\nThanks!"
	got := extractTDLines(prose, "brad")
	want := []string{
		"HIGH|a.go:1|p1|f1|security|5|e1|brad",
		"LOW|b.go:2|p2|f2|style|3|e2|brad",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("mixed extraction wrong.\n got=%v\nwant=%v", got, want)
	}
}

// CRITICAL severity is recognized even though the prompt only advertises
// HIGH/MEDIUM/LOW (defensive parity with the shared extractor).
func TestExtractTDLines_CriticalSeverity(t *testing.T) {
	prose := "CRITICAL|a.go:1|rce|patch|security|30|evidence|vera"
	got := extractTDLines(prose, "vera")
	want := []string{"CRITICAL|a.go:1|rce|patch|security|30|evidence|vera"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("CRITICAL not extracted.\n got=%v\nwant=%v", got, want)
	}
}

// Empty prose is safe.
func TestExtractTDLines_Empty(t *testing.T) {
	if got := extractTDLines("", "x"); len(got) != 0 {
		t.Errorf("empty prose yielded %v", got)
	}
	if n := countTDLines(""); n != 0 {
		t.Errorf("countTDLines(empty) = %d, want 0", n)
	}
}
