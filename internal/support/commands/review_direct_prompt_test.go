package commands

import (
	"strings"
	"testing"
)

// The reviewer prompt must force pipe-only output. Before this was hardened,
// weaker models (e.g. kai, mira) returned long prose reviews with no pipe rows
// — mira once emitted 117KB of prose — yielding zero parseable findings.
func TestBuildReviewTaskMessage_EnforcesPipeOnly(t *testing.T) {
	msg := buildReviewTaskMessage("+func foo() {}\n", "")
	low := strings.ToLower(msg)

	for _, want := range []string{
		"only", // "Output ONLY ..."
		"one finding per line",
		"no prose",
	} {
		if !strings.Contains(low, want) {
			t.Errorf("prompt missing output constraint %q", want)
		}
	}

	// Empty-result guidance keeps verbose models from inventing prose.
	if !strings.Contains(low, "no issues") {
		t.Error("prompt missing the empty-output guidance (no issues -> output nothing)")
	}

	// The column schema and a concrete example must still be present.
	if !strings.Contains(msg, "SEVERITY|FILE:LINE|PROBLEM|FIX|CATEGORY|EST_MINUTES|EVIDENCE|REVIEWER") {
		t.Error("prompt lost the 8-column schema line")
	}
	if !strings.Contains(msg, "HIGH|src/auth.go:42|") {
		t.Error("prompt lost the example finding row")
	}

	// The diff itself must still be embedded.
	if !strings.Contains(msg, "+func foo() {}") {
		t.Error("diff not embedded in the task message")
	}
}
