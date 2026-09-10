package findings

import (
	"errors"
	"strings"
	"testing"
)

// realLegacyStream is copied from a stream that exists on disk today:
// quilt-square/.planning/.temp/code-review/td-stream.txt, written 2026-02-22.
// It is the 9-column ORIGIN/RISK_CLASS shape with a `# Format:` comment block
// and NO version header — exactly what a strict gate would reject.
const realLegacyStream = `# TD_STREAM - Technical Debt Items
# SPRINT_ID: /Users/samestrin/Documents/GitHub/quilt-square/.planning/sprints/active/14.3_assembly-grid-control/
# CREATED: 2026-02-22 18:06:38
# Format: SEVERITY|ORIGIN|RISK_CLASS|FILE:LINE|PROBLEM|FIX|CATEGORY|EST_MINUTES|EVIDENCE
MEDIUM|sprint|anticipated_missed|app/src/components/GridDimensionControl.tsx:38|Input type mismatch|Use parseFloat|correctness|15|line 38
HIGH|sprint|anticipated_missed|app/src/components/GridDimensionControl.tsx:106|No debounce on onChange|Add a debounce|performance|30|line 106
`

const v1Stream = `# atcr-findings/v1
CRITICAL|auth.go:42|token never expires|check expiry|security|15|expiresAt unread|greta
LOW|util.go:7|unused var||style|0||otto
`

// --- AC 1: the header gate, mirroring atcr's own distinctions

func TestAcceptsTheV1Header(t *testing.T) {
	res, err := Inspect(strings.NewReader(v1Stream))
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	if !res.HasHeader {
		t.Error("HasHeader = false, want true")
	}
	if res.Version != "v1" {
		t.Errorf("Version = %q, want v1", res.Version)
	}
	if res.Legacy {
		t.Error("Legacy = true for a properly headed stream")
	}
}

func TestAWellFormedUnknownVersionIsUnknownNotMalformed(t *testing.T) {
	// atcr draws this line deliberately: "Only a well-formed token earns
	// ErrUnknownVersion; a garbage suffix ("v1x", "v1.2") is a malformed header,
	// not an unsupported version." Collapsing the two would tell a user to
	// upgrade when their file is actually corrupt.
	_, err := Inspect(strings.NewReader("# atcr-findings/v2\nHIGH|a.go:1|p|f|bug|5|e|r\n"))
	if !errors.Is(err, ErrUnknownVersion) {
		t.Fatalf("err = %v, want ErrUnknownVersion", err)
	}
	if errors.Is(err, ErrMalformedHeader) {
		t.Error("a well-formed v2 must not also read as malformed")
	}
}

func TestAGarbageSuffixIsMalformedNotUnknown(t *testing.T) {
	for _, bad := range []string{"# atcr-findings/v1x", "# atcr-findings/v1.2", "# atcr-findings/"} {
		t.Run(bad, func(t *testing.T) {
			_, err := Inspect(strings.NewReader(bad + "\nHIGH|a.go:1|p|f|bug|5|e|r\n"))
			if !errors.Is(err, ErrMalformedHeader) {
				t.Fatalf("err = %v, want ErrMalformedHeader", err)
			}
			if errors.Is(err, ErrUnknownVersion) {
				t.Error("a garbage suffix must not read as an unsupported version")
			}
		})
	}
}

// --- AC 2: a missing header is LEGACY, never fatal

func TestARealLegacyStreamParsesAsLegacy(t *testing.T) {
	// The whole point of AC 2, proven against a file that exists on disk rather
	// than one invented for the test. A hard gate here would reject three real
	// repositories' worth of streams.
	res, err := Inspect(strings.NewReader(realLegacyStream))
	if err != nil {
		t.Fatalf("a legacy stream must not be a fatal error: %v", err)
	}
	if res.HasHeader {
		t.Error("HasHeader = true, but this stream has no version header")
	}
	if !res.Legacy {
		t.Error("Legacy = false, want true")
	}
	if res.Warning == "" {
		t.Error("a legacy stream must carry a warning — silent acceptance is how " +
			"the seven formats accumulated in the first place")
	}
}

func TestAnEmptyStreamIsLegacyNotFatal(t *testing.T) {
	res, err := Inspect(strings.NewReader(""))
	if err != nil {
		t.Fatalf("an empty stream must not be fatal: %v", err)
	}
	if !res.Legacy {
		t.Error("Legacy = false for a stream with no header at all")
	}
}

// --- the canonical shape

func TestCanonicalColumnCounts(t *testing.T) {
	// Mirrors atcr/internal/stream/parser.go. A drift here silently re-creates
	// the width-collision this whole change exists to remove.
	if ModelColumns != 7 {
		t.Errorf("ModelColumns = %d, want 7", ModelColumns)
	}
	if PerSourceColumns != 8 {
		t.Errorf("PerSourceColumns = %d, want 8", PerSourceColumns)
	}
	if ReconciledColumns != 9 {
		t.Errorf("ReconciledColumns = %d, want 9", ReconciledColumns)
	}
}

func TestCanonicalColumnNamesMatchTheirCounts(t *testing.T) {
	if len(PerSourceNames) != PerSourceColumns {
		t.Fatalf("PerSourceNames has %d entries, PerSourceColumns is %d",
			len(PerSourceNames), PerSourceColumns)
	}
	want := []string{"SEVERITY", "FILE:LINE", "PROBLEM", "FIX", "CATEGORY",
		"EST_MINUTES", "EVIDENCE", "REVIEWER"}
	for i, w := range want {
		if PerSourceNames[i] != w {
			t.Errorf("PerSourceNames[%d] = %q, want %q", i, PerSourceNames[i], w)
		}
	}
	// The reconciled shape extends the per-source one; it never reorders it.
	for i := range PerSourceNames[:PerSourceColumns-1] {
		if ReconciledNames[i] != PerSourceNames[i] {
			t.Errorf("ReconciledNames[%d] = %q diverges from PerSourceNames[%d] = %q",
				i, ReconciledNames[i], i, PerSourceNames[i])
		}
	}
}

// --- the clean-review sentinel

func TestNoFindingsSentinelIsRecognised(t *testing.T) {
	// A clean review is a POSITIVE signal, not silence — atcr's fan-out treats an
	// empty response as a dead call. llm-tools recognises the sentinel nowhere
	// today, so a converged stream carrying it would parse as a malformed row.
	for _, s := range []string{"NO FINDINGS", "no findings", "  NO FINDINGS  \n"} {
		if !IsNoFindings(s) {
			t.Errorf("IsNoFindings(%q) = false, want true", s)
		}
	}
	for _, s := range []string{"", "NO FINDINGS HERE", "HIGH|a.go:1|p|f|bug|5|e|r"} {
		if IsNoFindings(s) {
			t.Errorf("IsNoFindings(%q) = true, want false", s)
		}
	}
}

// --- a header must not be confused with a data row

func TestProseAndCommentsAreNeverRows(t *testing.T) {
	// atcr calls this "the format's core contract: prose never becomes a row."
	for _, line := range []string{
		"# atcr-findings/v1",
		"# Format: SEVERITY|FILE:LINE",
		"",
		"The reviewer rated this HIGH|risk overall",
	} {
		if IsFindingRow(line) {
			t.Errorf("IsFindingRow(%q) = true, want false", line)
		}
	}
	for _, line := range []string{
		"CRITICAL|auth.go:42|p|f|security|15|e|greta",
		"LOW|util.go:7|p|f|style|0||otto",
	} {
		if !IsFindingRow(line) {
			t.Errorf("IsFindingRow(%q) = false, want true", line)
		}
	}
}
