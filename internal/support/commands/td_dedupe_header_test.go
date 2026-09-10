package commands

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// These streams are the REAL layouts the cadence skills write today, not
// invented ones. Three of them are eight columns wide with DIFFERENT columns,
// which is precisely what a width-guessing parser cannot tell apart.

// canonicalV1 — the target: atcr-findings/v1, 8 columns ending REVIEWER.
const canonicalV1 = `# atcr-findings/v1
HIGH|auth.go:45|Missing validation|Add a zod schema|security|15|user input unsanitized|bruce
`

// postBlocking — execute-code-review/post.md:219. Eight columns, but ending
// SOURCE|BLOCKING. Width-guessing reads SOURCE as EVIDENCE and BLOCKING as the
// REVIEWER, so the reviewer of every row becomes the literal string "no".
const postBlocking = `# TD_STREAM
# Format: SEVERITY|FILE_LINE|PROBLEM|FIX|CATEGORY|EST_MINUTES|SOURCE|BLOCKING
HIGH|auth.go:45|Missing validation|Add a zod schema|security|15|post-review|no
`

// epicEvidenceSource — execute-epic/instructions.md:176. Eight columns ending
// EVIDENCE|SOURCE, so SOURCE lands in the REVIEWER slot.
const epicEvidenceSource = `# TD_STREAM
# Format: SEVERITY|FILE_LINE|PROBLEM|FIX|CATEGORY|EST_MINUTES|EVIDENCE|SOURCE
HIGH|db.go:88|N+1 query in loop|Batch the lookups|performance|30|loop query|execute-epic
`

// legacyNineCol is the shape of a stream that EXISTS ON DISK today:
// quilt-square/.planning/.temp/code-review/td-stream.txt, written 2026-02-22.
// FILE:LINE sits at position 3, behind ORIGIN and RISK_CLASS — so a width-based
// parser reads the file path as "sprint".
const legacyNineCol = `# TD_STREAM - Technical Debt Items
# Format: SEVERITY|ORIGIN|RISK_CLASS|FILE:LINE|PROBLEM|FIX|CATEGORY|EST_MINUTES|EVIDENCE
MEDIUM|sprint|anticipated_missed|app/src/components/GridDimensionControl.tsx:38|Input type mismatch|Use parseFloat|correctness|15|line 38
`

func onlyRow(t *testing.T, content, tag string) MergedRow {
	t.Helper()
	res, err := dedupeTD([]StreamInput{{Tag: tag, Content: content}}, DedupeOpts{Tolerance: 3})
	if err != nil {
		t.Fatalf("dedupeTD: %v", err)
	}
	if len(res.Merged) != 1 {
		t.Fatalf("got %d merged rows, want 1 — the row did not parse", len(res.Merged))
	}
	return res.Merged[0]
}

// --- AC 3: columns come from the declared header, not from row width

func TestBlockingNeverLandsInTheReviewerField(t *testing.T) {
	// The sharpest case. "no" is a BLOCKING flag, not a person.
	row := onlyRow(t, postBlocking, "post")
	if strings.EqualFold(row.Reviewers, "no") || strings.EqualFold(row.Reviewers, "yes") {
		t.Fatalf("Reviewers = %q — a BLOCKING flag was read as the reviewer", row.Reviewers)
	}
	if row.FileLine != "auth.go:45" {
		t.Errorf("FileLine = %q, want auth.go:45", row.FileLine)
	}
}

func TestEvidenceSourceLayoutKeepsSourceOutOfReviewer(t *testing.T) {
	row := onlyRow(t, epicEvidenceSource, "epic")
	if row.Reviewers == "execute-epic" {
		t.Errorf("Reviewers = %q — the SOURCE column was read as the reviewer", row.Reviewers)
	}
	if row.FileLine != "db.go:88" {
		t.Errorf("FileLine = %q, want db.go:88", row.FileLine)
	}
}

func TestNineColumnLegacyFindsTheRealFilePath(t *testing.T) {
	// A width-based parser takes f[1] as FILE:LINE, which here is "sprint".
	row := onlyRow(t, legacyNineCol, "claude")
	if row.FileLine == "sprint" {
		t.Fatal(`FileLine = "sprint" — ORIGIN was read as the file path`)
	}
	if row.FileLine != "app/src/components/GridDimensionControl.tsx:38" {
		t.Errorf("FileLine = %q, want the real path", row.FileLine)
	}
	if row.Problem != "Input type mismatch" {
		t.Errorf("Problem = %q, want the real problem text", row.Problem)
	}
}

func TestCanonicalV1StillParses(t *testing.T) {
	// The regression guard: fixing the others must not break the target shape.
	row := onlyRow(t, canonicalV1, "claude")
	if row.FileLine != "auth.go:45" {
		t.Errorf("FileLine = %q, want auth.go:45", row.FileLine)
	}
	if row.Reviewers != "bruce" {
		t.Errorf("Reviewers = %q, want bruce", row.Reviewers)
	}
	if row.EstMinutes != 15 {
		t.Errorf("EstMinutes = %v, want 15", row.EstMinutes)
	}
}

// --- AC 2: a headerless stream still parses, and says so

func TestAHeaderlessStreamStillParsesAndWarns(t *testing.T) {
	bare := "HIGH|auth.go:45|Missing validation|Add a zod schema|security|15|e|bruce\n"
	res, err := dedupeTD([]StreamInput{{Tag: "claude", Content: bare}}, DedupeOpts{Tolerance: 3})
	if err != nil {
		t.Fatalf("a headerless stream must not be fatal: %v", err)
	}
	if len(res.Merged) != 1 {
		t.Fatalf("got %d rows, want 1", len(res.Merged))
	}
	if len(res.Summary.Warnings) == 0 {
		t.Error("no warning for a headerless stream — silent acceptance is how " +
			"seven formats accumulated unnoticed")
	}
}

func TestACanonicalStreamWarnsAboutNothing(t *testing.T) {
	res, err := dedupeTD([]StreamInput{{Tag: "claude", Content: canonicalV1}}, DedupeOpts{Tolerance: 3})
	if err != nil {
		t.Fatalf("dedupeTD: %v", err)
	}
	if len(res.Summary.Warnings) != 0 {
		t.Errorf("Warnings = %v, want none for a properly headed stream", res.Summary.Warnings)
	}
}

// --- the warning reaches a caller, not just the struct

func TestTdDedupeCmd_SurfacesTheLegacyWarning(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "legacy.txt")
	if err := os.WriteFile(p, []byte(legacyNineCol), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := newTdDedupeCmd()
	cmd.SetArgs([]string{"--streams", p, "--source-tags", "claude", "--json"})
	var out, errb bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errb)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, errb.String())
	}
	var res DedupeResult
	if err := json.Unmarshal(out.Bytes(), &res); err != nil {
		t.Fatalf("json: %v\n%s", err, out.String())
	}
	if len(res.Summary.Warnings) == 0 {
		t.Error("the legacy warning did not reach the JSON output, so no caller can act on it")
	}
}

func TestALeadingBlankLineDoesNotHideTheFormatComment(t *testing.T) {
	// streamColumns stops scanning at the first line that is not a comment, but
	// findings.Inspect skips blank lines before looking. So a stream that opens
	// with a blank line makes the two disagree about the SAME content: Inspect
	// still sees the version header, while the column scan gives up before
	// reaching the `# Format:` line and silently falls back to width-guessing.
	//
	// A leading newline is the most ordinary thing in a generated file.
	withBlank := "\n" + legacyNineCol
	row := onlyRow(t, withBlank, "claude")
	if row.FileLine == "sprint" {
		t.Fatal(`FileLine = "sprint" — a leading blank line hid the # Format: ` +
			`comment, so ORIGIN was read as the file path again`)
	}
	if row.FileLine != "app/src/components/GridDimensionControl.tsx:38" {
		t.Errorf("FileLine = %q, want the real path", row.FileLine)
	}
}
