package commands

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/samestrin/llm-tools/internal/support/findings"
)

// v1Stream is a findings stream in the atcr-findings/v1 shape. It carries NO
// column-header row: the columns are implied by the contract, which is the whole
// point of a versioned format.
//
// parse_stream auto-detects its headers from line 0 and does not skip comments,
// so today it reads the version header itself as the column list — one column
// named "# atcr-findings/v1" — and then treats every real finding as a row with
// eight values too many.
const v1PipeStream = `# atcr-findings/v1
CRITICAL|auth.go:42|token never expires|check expiry|security|15|expiresAt unread|greta
LOW|util.go:7|unused var||style|0||otto
`

// legacyCommented is the older shape: a block of # comments, then rows. Same
// trap — the first comment becomes the column list.
const legacyCommented = `# TD_STREAM - Technical Debt Items
# Format: SEVERITY|FILE:LINE|PROBLEM|FIX|CATEGORY|EST_MINUTES|EVIDENCE|REVIEWER
HIGH|db.go:88|N+1 query|Batch the lookups|performance|30|loop query|mira
`

const toonPayload = `findings[1|]{SEVERITY|FILE_LINE|PROBLEM|FIX}:
  HIGH|"src/auth.ts:10"|"Rejects a|b input"|Split first
`

func execParseStreamCmd(t *testing.T, args ...string) ParseStreamResult {
	t.Helper()
	cmd := newParseStreamCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	var res ParseStreamResult
	if err := json.Unmarshal(buf.Bytes(), &res); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, buf.String())
	}
	return res
}

// --- the version header must never become a column name

func TestParseStreamUsesCanonicalColumnsForAV1Stream(t *testing.T) {
	res := execParseStreamCmd(t, "--content", v1PipeStream, "--json")

	for _, h := range res.Headers {
		if h == findings.Version {
			t.Fatalf("Headers = %v — the version header was read as a column name",
				res.Headers)
		}
	}
	// A v1 stream has no header row, so the contract supplies the columns.
	if len(res.Headers) != 8 {
		t.Fatalf("Headers = %v, want the canonical 8 columns", res.Headers)
	}
	if res.Headers[0] != "SEVERITY" || res.Headers[1] != "FILE:LINE" {
		t.Errorf("Headers begin %v, want SEVERITY, FILE:LINE", res.Headers[:2])
	}
	if len(res.Rows) != 2 {
		t.Fatalf("got %d rows, want 2 — no finding may be lost to the header", len(res.Rows))
	}
	if got, _ := res.Rows[0]["SEVERITY"].(string); got != "CRITICAL" {
		t.Errorf("Rows[0][SEVERITY] = %q, want CRITICAL", got)
	}
	if got, _ := res.Rows[0]["FILE:LINE"].(string); got != "auth.go:42" {
		t.Errorf("Rows[0][FILE:LINE] = %q, want auth.go:42", got)
	}
}

func TestParseStreamSkipsALegacyCommentBlock(t *testing.T) {
	res := execParseStreamCmd(t, "--content", legacyCommented, "--json")
	for _, h := range res.Headers {
		if len(h) > 0 && h[0] == '#' {
			t.Fatalf("Headers = %v — a comment line was read as a column name", res.Headers)
		}
	}
	if len(res.Rows) != 1 {
		t.Fatalf("got %d rows, want 1", len(res.Rows))
	}
}

// --- explicit headers still win, and existing behaviour is untouched

func TestParseStreamExplicitHeadersStillOverride(t *testing.T) {
	res := execParseStreamCmd(t, "--content", v1PipeStream,
		"--headers", "A,B,C,D,E,F,G,H", "--json")
	if len(res.Headers) == 0 || res.Headers[0] != "A" {
		t.Fatalf("Headers = %v, want the explicit list", res.Headers)
	}
	if len(res.Rows) != 2 {
		t.Errorf("got %d rows, want 2", len(res.Rows))
	}
}

// --- AC 4: parse_stream reads TOON too

func TestParseStreamReadsTOON(t *testing.T) {
	res := execParseStreamCmd(t, "--content", toonPayload, "--format", "toon", "--json")
	if len(res.Rows) != 1 {
		t.Fatalf("got %d rows, want 1", len(res.Rows))
	}
	if got, _ := res.Rows[0]["FILE_LINE"].(string); got != "src/auth.ts:10" {
		t.Errorf("FILE_LINE = %q, want src/auth.ts:10", got)
	}
	// The reason the format is worth wiring in at all.
	if got, _ := res.Rows[0]["PROBLEM"].(string); got != "Rejects a|b input" {
		t.Errorf("PROBLEM = %q — the pipe inside the finding did not survive", got)
	}
}
