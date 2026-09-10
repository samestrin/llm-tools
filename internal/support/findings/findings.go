// Package findings implements the atcr-findings/v1 stream contract as llm-tools
// consumes it.
//
// Seven different pipe-delimited "findings stream" formats grew across the
// cadence skills, three of them EIGHT COLUMNS WIDE WITH DIFFERENT COLUMNS —
// indistinguishable to a consumer that picks its schema by counting fields. The
// contract that resolves this already exists upstream, in
// atcr/internal/stream/parser.go. This package adopts it rather than inventing a
// second one.
//
//	# atcr-findings/v1
//	SEVERITY|FILE:LINE|PROBLEM|FIX|CATEGORY|EST_MINUTES|EVIDENCE|REVIEWER
//
// Two deliberate departures from a naive reading, both mirroring atcr:
//
//   - **A well-formed unknown version is not the same as a garbage one.**
//     "v2" earns ErrUnknownVersion; "v1x" and "v1.2" earn ErrMalformedHeader.
//     Collapsing them tells a user to upgrade when their file is corrupt.
//   - **A missing header is LEGACY, not fatal.** Streams written before this
//     contract exist on disk today, including a 9-column ORIGIN/RISK_CLASS one
//     from 2026-02-22. A hard gate would reject real work. They parse, and they
//     warn — silence is how seven formats accumulated unnoticed.
//
// This package does NOT solve a pipe inside PROBLEM or FIX. v1 has no quoting:
// atcr folds overflow past column 7 into EVIDENCE joined with "/", which is an
// anti-forgery rule (a model can never land a value in the REVIEWER slot the
// engine fills), not an escaping mechanism. The quoting-safe encoding is TOON —
// see internal/support/toon. v1 is the column contract; TOON is the wire format.
package findings

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
)

// Version is the required first non-blank line of a v1 findings stream.
const Version = "# atcr-findings/v1"

// versionPrefix matches any atcr-findings version header, so a WRONG version is
// reported distinctly from a MISSING one.
const versionPrefix = "# atcr-findings/"

// versionTokenRe matches a well-formed version token ("v1", "v10"). Only a
// well-formed token earns ErrUnknownVersion.
var versionTokenRe = regexp.MustCompile(`^v[0-9]+$`)

// severityRe anchors a finding row at a valid severity prefix. This is the
// format's core contract: prose never becomes a row.
var severityRe = regexp.MustCompile(`^(CRITICAL|HIGH|MEDIUM|LOW)\|`)

// NoFindingsSentinel is what a reviewer emits when it has nothing to report, so
// that a clean review is a POSITIVE signal rather than silence.
const NoFindingsSentinel = "NO FINDINGS"

// Column counts for the three stream shapes.
const (
	ModelColumns      = 7 // SEVERITY..EVIDENCE — what a reviewer model emits
	PerSourceColumns  = 8 // ...|EVIDENCE|REVIEWER
	ReconciledColumns = 9 // ...|EVIDENCE|REVIEWERS|CONFIDENCE
)

// PerSourceNames and ReconciledNames are the canonical column names. The
// reconciled shape EXTENDS the per-source one and never reorders it, which is
// what lets one parser read both.
var (
	PerSourceNames = []string{
		"SEVERITY", "FILE:LINE", "PROBLEM", "FIX", "CATEGORY",
		"EST_MINUTES", "EVIDENCE", "REVIEWER",
	}
	ReconciledNames = []string{
		"SEVERITY", "FILE:LINE", "PROBLEM", "FIX", "CATEGORY",
		"EST_MINUTES", "EVIDENCE", "REVIEWERS", "CONFIDENCE",
	}
)

// The only fatal header failures. A missing header is not among them.
var (
	ErrUnknownVersion  = errors.New("unknown findings version")
	ErrMalformedHeader = errors.New("malformed findings version header")
)

// Result describes a stream's header, without consuming its rows.
type Result struct {
	HasHeader bool   // a well-formed `# atcr-findings/vN` line was present
	Version   string // the token, e.g. "v1"; empty when Legacy
	Legacy    bool   // no version header — parse permissively, but say so
	Warning   string // non-empty when Legacy, for the caller to surface
}

// Inspect reads far enough to classify a stream's header.
//
// It stops at the first non-blank line, so it is cheap on a large stream and
// safe to call before deciding how to parse the rest.
func Inspect(r io.Reader) (Result, error) {
	sc := bufio.NewScanner(r)
	// Findings carry free text; the 64KB default would split a long PROBLEM into
	// two lines, and the second half would look like malformed input.
	sc.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)

	first := ""
	for sc.Scan() {
		if strings.TrimSpace(sc.Text()) != "" {
			first = strings.TrimSpace(sc.Text())
			break
		}
	}
	if err := sc.Err(); err != nil {
		return Result{}, err
	}

	if !strings.HasPrefix(first, versionPrefix) {
		return Result{
			Legacy: true,
			Warning: "no `" + Version + "` header: parsing as a legacy stream. " +
				"Add the header so a consumer never silently reads incompatible data.",
		}, nil
	}

	token := strings.TrimSpace(strings.TrimPrefix(first, versionPrefix))
	if !versionTokenRe.MatchString(token) {
		return Result{}, fmt.Errorf("%w: %q is not a version token", ErrMalformedHeader, first)
	}
	if first != Version {
		return Result{}, fmt.Errorf("%w: found %q, this build reads %q",
			ErrUnknownVersion, token, Version)
	}
	return Result{HasHeader: true, Version: token}, nil
}

// IsNoFindings reports whether a reviewer response is the clean-review sentinel.
// Matching is case-insensitive and ignores surrounding whitespace, because the
// sentinel is model-produced and a trailing newline is not an anomaly.
func IsNoFindings(s string) bool {
	return strings.EqualFold(strings.TrimSpace(s), NoFindingsSentinel)
}

// IsFindingRow reports whether a line is a finding rather than a comment, a
// blank, or prose that happens to mention a severity word.
func IsFindingRow(line string) bool {
	return severityRe.MatchString(strings.TrimSpace(line))
}
